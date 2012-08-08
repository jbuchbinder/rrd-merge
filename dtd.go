package main

type Rrd struct {
	Version    string   `xml:"version"`
	Step       int      `xml:"step"`
	LastUpdate int64    `xml:"lastupdate"`
	Ds         RrdDs    `xml:"ds"`
	Rra        []RrdRra `xml:"rra"`
}

type RrdDs struct {
	Name             string `xml:"name"`
	Type             string `xml:"type"`
	MinimalHeartbeat int    `xml:"minimal_heartbeat"`
	Min              string `xml:"min"`
	Max              string `xml:"max"`
	LastDs           string `xml:"last_ds"`
	Value            string `xml:"value"`
	UnknownSec       string `xml:"unknown_sec"`
}

type RrdRra struct {
	Cf                       string `xml:"cf"`
	PdpPerRow                int    `xml:"pdp_per_row"`
	Xff                      string `xml:"params>xff"`
	CdpPrepPrimaryValue      string `xml:"cdp_prep>ds>primary_value"`
	CdpPrepSecondaryValue    string `xml:"cdp_prep>ds>secondary_value"`
	CdpPrepValue             string `xml:"cdp_prep>ds>value"`
	CdpPrepUnknownDatapoints string `xml:"cdp_prep>ds>unknown_datapoints"`
	Database                 RrdDb  `xml:"database"`
}

type RrdDb struct {
	Data []RrdValue `xml:"row"`
}

type RrdValue struct {
	Value string `xml:"v"`
}
