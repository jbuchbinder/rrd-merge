// RRD-MERGE
// https://github.com/jbuchbinder/rrd-merge

package main

import (
	"encoding/xml"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Println("Usage: rrd-merge OLD.rrd NEW.rrd")
		return
	}
	fOld := os.Args[1]
	fNew := os.Args[2]

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

	// Loop through all of the RRAs in dOld, and use those as the constraint
	// for building
	for i := 0; i < len(dOld.Rra); i++ {
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
			fmt.Printf("No RRA found in %s matching period %d sec, skipping\n", fNew, incrOld)
			// Skip past dealing with this iteration
			continue
		}

		// Determine if we're shrinking or growing here...
		rraCountOld := len(dOld.Rra[i].Database.Data)
		rraCountNew := len(dNew.Rra[i].Database.Data)

		// If it's the same or greater, we slice
		if rraCountOld >= rraCountNew {
			var sliceOld []RrdValue
			if rraCountOld == rraCountNew {
				sliceOld = dOld.Rra[i].Database.Data[:]
			} else {
				b := (rraCountOld + 1) - rraCountNew
				e := rraCountOld + 1
				fmt.Printf("try to slice : %d : %d\n", b, e)
				sliceOld = dOld.Rra[i].Database.Data[b:e]
			}
			fmt.Printf("rrdcount old, new = %d, %d, sliceold size = %d\n", rraCountOld, rraCountNew, len(sliceOld))

			rCount := 0
			// Comparison and replace
			for p := 0; p < rraCountNew; p++ {
				if strings.Contains(sliceOld[p].Value, "NaN") && strings.Contains(dNew.Rra[i].Database.Data[p].Value, "NaN") {
					//fmt.Printf("Position %d has value to replace\n", p)
					dNew.Rra[i].Database.Data[p].Value = sliceOld[p].Value
					rCount++
				}
			}
			fmt.Printf("Replaced %d values in rra #%d\n", rCount, i)
		} else {
			fmt.Printf("TODO: Support larger newer data than old")
		}
	}

	// Write out to new
	restoreXml(fNew, dNew)
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
