package beaconing

import (
	"encoding/json"
	"errors"
	"github.com/scionproto/scion/go/lib/common"
	"github.com/scionproto/scion/go/lib/serrors"
	"io/ioutil"
	"math"
	"os"

	"github.com/scionproto/scion/go/lib/ctrl/seg"
)

type InterfaceLatencies struct {
	Inter uint16                     `json:"Inter"`
	Intra map[common.IFIDType]uint16 `json:"Intra"`
}

type InterfaceBandwidths struct {
	Inter uint32                     `json:"Inter"`
	Intra map[common.IFIDType]uint32 `json:"Intra"`
}

type InterfaceGeodata struct {
	Longitude float32 `json:"Longitude"`
	Latitude  float32 `json:"Latitude"`
	Address   string  `json:"Address"`
}

type InterfaceHops struct {
	Intra map[common.IFIDType]uint8 `json:"Intra"`
}

// StaticInfoCfg is used to parse data from config.json.
type StaticInfoCfg struct {
	Latency   map[common.IFIDType]InterfaceLatencies  `json:"Latency"`
	Bandwidth map[common.IFIDType]InterfaceBandwidths `json:"Bandwidth"`
	Linktype  map[common.IFIDType]string              `json:"Linktype"`
	Geo       map[common.IFIDType]InterfaceGeodata    `json:"Geo"`
	Hops      map[common.IFIDType]InterfaceHops       `json:"Hops"`
	Note      string                                  `json:"Note"`
}

// gatherlatency extracts latency values from a StaticInfoCfg struct and
// inserts them into the LatencyInfo portion of a StaticInfoExtn struct.
func (cfgdata StaticInfoCfg) gatherlatency(peers map[common.IFIDType]bool, egifID common.IFIDType, inifID common.IFIDType) seg.LatencyInfo {
	l := seg.LatencyInfo{
		Egresslatency:  cfgdata.Latency[egifID].Inter,
		Intooutlatency: cfgdata.Latency[egifID].Intra[inifID],
	}
	for subintfid, intfdelay := range cfgdata.Latency[egifID].Intra {
		//If we're looking at a peering interface, always include the data
		if peers[subintfid] {
			l.Peeringlatencies = append(l.Peeringlatencies, seg.Latencypeeringtriplet{
				IntfID:     subintfid,
				Interdelay: cfgdata.Latency[subintfid].Inter,
				IntraDelay: intfdelay,
			})
			continue
		}
		// If we're looking at a NON-peering interface, only include the data if subintfid>egifID so as to not
		// store redundant information
		if subintfid > egifID {
			l.Childlatencies = append(l.Childlatencies, seg.Latencychildpair{
				Intradelay: intfdelay,
				Interface:  subintfid,
			})
		}
	}
	return l
}

// gatherbw extracts bandwidth values from a StaticInfoCfg struct and
// inserts them into the BandwidthInfo portion of a StaticInfoExtn struct.
func (cfgdata StaticInfoCfg) gatherbw(peers map[common.IFIDType]bool, egifID common.IFIDType, inifID common.IFIDType) seg.BandwidthInfo {
	l := seg.BandwidthInfo{
		EgressBW:  cfgdata.Bandwidth[egifID].Inter,
		IntooutBW: cfgdata.Bandwidth[egifID].Intra[inifID],
	}
	for subintfid, intfbw := range cfgdata.Bandwidth[egifID].Intra {
		//If we're looking at a peering interface, always include the data
		if peers[subintfid] {
			l.BWPairs = append(l.BWPairs, seg.BWPair{
				IntfID: subintfid,
				BW:     uint32(math.Min(float64(intfbw), float64(cfgdata.Bandwidth[subintfid].Inter))),
			})
			continue
		}
		// If we're looking at a NON-peering interface, only include the data if subintfid>egifID so as to not
		// store redundant information
		if subintfid > egifID {
			l.BWPairs = append(l.BWPairs, seg.BWPair{
				BW:     intfbw,
				IntfID: subintfid,
			})
		}
	}
	return l
}

// gatherlinktype extracts linktype values from a StaticInfoCfg struct and
// inserts them into the LinktypeInfo portion of a StaticInfoExtn struct.
func (cfgdata StaticInfoCfg) gatherlinktype(peers map[common.IFIDType]bool, egifID common.IFIDType) seg.LinktypeInfo {

	l := seg.LinktypeInfo{
		EgressLT: cfgdata.Linktype[egifID],
	}
	for intfid, intfLT := range cfgdata.Linktype {
		//If we're looking at a peering interface, include the data for the peering link, otherwise drop it
		if peers[intfid] {
			l.Peeringlinks = append(l.Peeringlinks, seg.LTPeeringpair{
				IntfLT: intfLT,
				IntfID: intfid,
			})
		}
	}
	return l
}

// gatherhops extracts hop values from a StaticInfoCfg struct and
// inserts them into the InternalHopsinfo portion of a StaticInfoExtn struct.
func (cfgdata StaticInfoCfg) gatherhops(peers map[common.IFIDType]bool, egifID common.IFIDType, inifID common.IFIDType) seg.InternalHopsInfo {
	l := seg.InternalHopsInfo{
		Intououthops: cfgdata.Hops[egifID].Intra[inifID],
	}
	for intfid, intfHops := range cfgdata.Hops[egifID].Intra {
		// If we're looking at a peering interface or intfid>egifID, include the data, otherwise drop it
		// so as to not store redundant information
		if (intfid > egifID) || peers[intfid] {
			l.Hoppairs = append(l.Hoppairs, seg.Hoppair{
				Hops:   intfHops,
				IntfID: intfid,
			})
		}
	}
	return l
}

// gathergeo extracts geo values from a StaticInfoCfg struct and
// inserts them into the GeoInfo portion of a StaticInfoExtn struct.
func (cfgdata StaticInfoCfg) gathergeo() seg.GeoInfo {
	l := seg.GeoInfo{}
	for intfid, loc := range cfgdata.Geo {
		var assigned = false
		for k := 0; k < len(l.Locations); k++ {
			if (loc.Longitude == l.Locations[k].GPSData.Longitude) && (loc.Latitude == l.Locations[k].GPSData.Latitude) && (loc.Address == l.Locations[k].GPSData.Address) && (!assigned) {
				l.Locations[k].IntfIDs = append(l.Locations[k].IntfIDs, intfid)
				assigned = true
			}
		}
		if !assigned {
			l.Locations = append(l.Locations, seg.Location{
				GPSData: seg.Coordinates{
					Longitude: loc.Longitude,
					Latitude:  loc.Latitude,
					Address:   loc.Address,
				},
				IntfIDs: []common.IFIDType{intfid},
			})
			assigned = true
		}
	}
	return l
}

// Parseconfigdata parses data from a config file into a StaticInfoCfg struct.
func ParseStaticInfoCfg(file string) (StaticInfoCfg, error) {
	jsonFile, err := os.Open(file)
	if err != nil {
		return StaticInfoCfg{}, errors.New("Failed to open config data file with error: " + err.Error() + "\n")
	}
	defer jsonFile.Close()
	raw, err := ioutil.ReadFile(file)
	if err != nil {
		return StaticInfoCfg{}, serrors.WrapStr("Failed to read static info config file: ", err, "; file: ", file+"\n")
	}
	var cfg StaticInfoCfg
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return StaticInfoCfg{}, serrors.WrapStr("Failed to parse static info config: ", err, "; file: ", file+"\n")
	}
	return cfg, nil
}

// GenerateStaticinfo creates a StaticinfoExtn struct and populates it with data extracted from configdata.
func GenerateStaticinfo(configdata StaticInfoCfg, peers map[common.IFIDType]bool, egifID common.IFIDType, inifID common.IFIDType) seg.StaticInfoExtn {
	var StaticInfo seg.StaticInfoExtn
	StaticInfo.Latency = configdata.gatherlatency(peers, egifID, inifID)
	StaticInfo.Bandwidth = configdata.gatherbw(peers, egifID, inifID)
	StaticInfo.Linktype = configdata.gatherlinktype(peers, egifID)
	StaticInfo.Geo = configdata.gathergeo()
	StaticInfo.Note = configdata.Note
	StaticInfo.Hops = configdata.gatherhops(peers, egifID, inifID)
	return StaticInfo
}
