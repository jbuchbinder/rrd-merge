// RRD-MERGE
// https://github.com/jbuchbinder/rrd-merge

package main

import (
	"encoding/xml"
	"flag"
	"fmt"
	"math"
	"os/exec"
	"strings"
	"sync"
)

var debug bool
var waitGroup sync.WaitGroup

func init() {
	flag.BoolVar(&debug, "debug", false, "debug")
}

func main() {
	flag.Parse()
	args := flag.Args()
	if len(args) != 3 {
		fmt.Println("Usage: rrd-merge [-debug] OLD.rrd NEW.rrd OUTPUT.rrd")
		return
	}
	fOld := args[0]
	fNew := args[1]
	fOut := args[2]

	dOld := Rrd{}
	bOld := dumpXml(fOld)
	err := xml.Unmarshal(bOld, &dOld)
	if err != nil {
		fmt.Printf("error: %v\n", err)
		return
	}
	rrdInfo(fOld, dOld)

	fmt.Println(" ")

	dNew := Rrd{}
	bNew := dumpXml(fNew)
	err = xml.Unmarshal(bNew, &dNew)
	if err != nil {
		fmt.Printf("error: %v\n", err)
		return
	}
	rrdInfo(fNew, dNew)

	fmt.Println(" ")

	if dNew.LastUpdate < dOld.LastUpdate {
		fmt.Println("'New' file has older last update than 'old' file")
		return
	}

	// Loop through all of the RRAs in dOld, and use those as the constraint
	// for building
	for i := 0; i < len(dOld.Rra); i++ {
		waitGroup.Add(1)
		go ProcessRRA(fOld, fNew, &dOld, &dNew, i)
	}
	waitGroup.Wait()

	// Write out to new
	restoreXml(fOut, dNew)
}

func ProcessRRA(fOld string, fNew string, dOld *Rrd, dNew *Rrd, i int) {
	// Form proper increment
	incrOld := dOld.Step * dOld.Rra[i].PdpPerRow

	// Find the matching one in dNew
	newRra := -1
	for j := 0; j < len(dNew.Rra); j++ {
		incrNew := dNew.Step * dNew.Rra[j].PdpPerRow
		if incrNew == incrOld {
			newRra = j
			break
		}
	}
	if newRra == -1 {
		// Nothing found
		fmt.Printf("[RRA-%d] No RRA found in %s matching period %d sec, skipping\n", i, fNew, incrOld)
		// Skip past dealing with this iteration
		waitGroup.Done()
		return
	}

	// Determine "magic offset" for this RRA, if there needs to be one.
	mOffset := 0
	tDiff := dNew.LastUpdate - dOld.LastUpdate
	if int64(incrOld) < tDiff {
		mOffset = int(float64(math.Floor(float64(tDiff / int64(incrOld)))))
	}
	fmt.Printf("[RRA-%d] Time differential between %d and %d is %d, step %d offset %d\n", i, dOld.LastUpdate, dNew.LastUpdate, tDiff, incrOld, mOffset)

	// Determine if we're shrinking or growing here...
	rraCountOld := len(dOld.Rra[i].Database.Data)
	rraCountNew := len(dNew.Rra[i].Database.Data)

	// If it's the same or greater, we slice
	if rraCountOld >= rraCountNew {
		var sliceOld []RrdValue
		if rraCountOld == rraCountNew {
			// If there's an offset, slide down
			if mOffset > 0 {
				fmt.Printf("[RRA-%d] Creating slided offset %d:%d\n", i, mOffset, len(dOld.Rra[i].Database.Data)-mOffset)
				sliceOld = appendSlice(dOld.Rra[i].Database.Data[mOffset:len(dOld.Rra[i].Database.Data)-mOffset], offsetRraSlice(mOffset, len(dOld.Rra[i].Database.Data[mOffset].Value)))
			} else {
				sliceOld = dOld.Rra[i].Database.Data[:]
			}
		} else {
			if mOffset > 0 {
				b := ((rraCountOld + 1) - rraCountNew) - mOffset
				e := (rraCountOld + 1) - mOffset
				if b < 0 && e < 0 {
					fmt.Printf("[RRA-%d] Offsets out of range, skipping RRA ( %d : %d )\n", i, b, e)
					waitGroup.Done()
					return
				}
				fmt.Printf("[RRA-%d] Try to slice with offset %d : %d : %d\n", i, mOffset, b, e)
				if b < 0 {
					fmt.Printf("[RRA-%d] Prepend %d NaN elements so we don't overflow\n", i, mOffset)
					sliceOld = appendSlice(offsetRraSlice(mOffset, len(dOld.Rra[i].Database.Data[0].Value)), dOld.Rra[i].Database.Data[0:e])
				} else {
					sliceOld = dOld.Rra[i].Database.Data[b:e]
				}
			} else {
				b := (rraCountOld + 1) - rraCountNew
				e := rraCountOld + 1
				fmt.Printf("[RRA-%d] Try to slice : %d : %d\n", i, b, e)
				sliceOld = dOld.Rra[i].Database.Data[b:e]
			}
		}
		fmt.Printf("[RRA-%d] rrdcount old, new = %d, %d, sliceold size = %d\n", i, rraCountOld, rraCountNew, len(sliceOld))

		rCount := 0
		// Comparison and replace
		for p := 0; p < rraCountNew; p++ {
			for s := 0; s < len(sliceOld[p].Value); s++ {
				if !strings.Contains(sliceOld[p].Value[s], "NaN") && sliceOld[p].Value[s] != "" && strings.Contains(dNew.Rra[i].Database.Data[p].Value[s], "NaN") {
					if debug {
						fmt.Printf("[RRA-%d] Position %d [%d] has value to replace [%s -> %s]\n", i, p, s, dNew.Rra[i].Database.Data[p].Value[s], sliceOld[p].Value[s])
					}
					dNew.Rra[i].Database.Data[p].Value[s] = sliceOld[p].Value[s]
					rCount++
				}
			}
		}
		fmt.Printf("[RRA-%d] Replaced %d values\n", i, rCount)
	} else {
		// Support larger new data RRA than old
		sliceOld := dOld.Rra[i].Database.Data[:]

		// TODO: Figure in for mOffset
		diff := rraCountNew - rraCountOld

		rCount := 0
		// Comparison and replace
		for p := diff; p < rraCountNew; p++ {
			for s := 0; s < len(sliceOld[p-diff].Value); s++ {
				if !strings.Contains(sliceOld[p-diff].Value[s], "NaN") && sliceOld[p-diff].Value[s] != "" && strings.Contains(dNew.Rra[i].Database.Data[p].Value[s], "NaN") {
					if debug {
						fmt.Printf("[RRA-%d] Position %d (old pos %d) [%d] has value to replace [%s -> %s]\n", i, p, p-diff, s, dNew.Rra[i].Database.Data[p].Value[s], sliceOld[p-diff].Value[s])
					}
					dNew.Rra[i].Database.Data[p].Value[s] = sliceOld[p-diff].Value[s]
					rCount++
				}
			}
		}
		fmt.Printf("[RRA-%d] Replaced %d values\n", i, rCount)
	}
	waitGroup.Done()
}

func rrdInfo(file string, rrd Rrd) {
	fmt.Printf("%s has %d RRAs\n", file, len(rrd.Rra))
	for i := 0; i < len(rrd.Rra); i++ {
		endTs := rrd.LastUpdate
		entries := len(rrd.Rra[i].Database.Data)
		incr := rrd.Step * rrd.Rra[i].PdpPerRow
		beginTs := endTs - ((int64(entries) - 1) * int64(incr))
		fmt.Printf("\t[%d] has %d entries\n", i, entries)
		fmt.Printf("\t\tRepresents %d sec increments (%d - %d)\n", incr, beginTs, endTs)
	}
}

func dumpXml(file string) []byte {
	out, err := exec.Command("rrdtool", "dump", file).Output()
	if err != nil {
		panic(err)
	}
	return out
}

func restoreXml(file string, rrd Rrd) {
	cmd := exec.Command("rrdtool", "restore", "-f", "-", file)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		panic(err)
	}
	err = cmd.Start()
	if err != nil {
		panic(err)
	}
	bin, err := xml.Marshal(rrd)
	if debug {
		fmt.Println(string(bin))
	}
	_, err = stdin.Write([]byte(bin))
	if err != nil {
		panic(err)
	}
	stdin.Close()
	err = cmd.Wait()
	if err != nil {
		panic(err)
	}
}

func offsetRraSlice(offset int, sz int) []RrdValue {
	inner := make([]string, sz)
	for j := 0; j < sz; j++ {
		inner[j] = "NaN"
	}
	ret := make([]RrdValue, offset)
	for i := 0; i < offset; i++ {
		ret[i] = RrdValue{Value: inner}
	}
	return ret
}

func appendSlice(orig []RrdValue, a []RrdValue) []RrdValue {
	o := orig[:]
	for i := 0; i < len(a); i++ {
		o = append(o, a[i])
	}
	return o
}
