package combinator

import (
	"fmt"
	"math"

	"github.com/scionproto/scion/go/lib/addr"
	"github.com/scionproto/scion/go/lib/common"
	"github.com/scionproto/scion/go/lib/ctrl/seg"
	"github.com/scionproto/scion/go/proto"
)

type ASnote struct {
	Note string
}

type ASGeo struct {
	locations []GeoLoc
}

type GeoLoc struct {
	Latitude  float32 `capnp:"latitude"`
	Longitude float32 `capnp:"longitude"`
	Address   string  `capnp:"address"`
}

func (s *GeoLoc) ProtoId() proto.ProtoIdType {
	return proto.DenseStaticInfo_Geo_GPSData_TypeID
}

func (s *GeoLoc) String() string {
	return fmt.Sprintf("Latitude: %f\nLongitude: %f\nAddress: %s\n",
		s.Latitude, s.Longitude, s.Address)
}

type ASLatency struct {
	IntraLatency uint16
	InterLatency uint16
	PeerLatency  uint16
}

type ASHops struct {
	Hops uint8
}

type ASLink struct {
	InterLinkType uint16
	PeerLinkType  uint16
}

type ASBandwidth struct {
	IntraBW uint32
	InterBW uint32
}

type DenseASLinkType struct {
	InterLinkType uint16     `capnp:"interLinkType"`
	PeerLinkType  uint16     `capnp:"peerLinkType"`
	RawIA         addr.IAInt `capnp:"isdas"`
}

func (s *DenseASLinkType) ProtoId() proto.ProtoIdType {
	return proto.DenseStaticInfo_InterfaceLinkType_TypeID
}

func (s *DenseASLinkType) String() string {
	return fmt.Sprintf("InterLinkType: %d\nPeerLinkType: %d\nISD: %d\nAS: %d\n",
		s.InterLinkType, s.PeerLinkType, s.RawIA.IA().I, s.RawIA.IA().A)
}

type DenseGeo struct {
	RouterLocations []GeoLoc   `capnp:"routerLocations"`
	RawIA           addr.IAInt `capnp:"isdas"`
}

func (s *DenseGeo) ProtoId() proto.ProtoIdType {
	return proto.DenseStaticInfo_Geo_TypeID
}

func (s *DenseGeo) String() string {
	return fmt.Sprintf("RouterLocations: %v\nISD: %d\nAS: %d\n",
		s.RouterLocations, s.RawIA.IA().I, s.RawIA.IA().A)
}

type DenseNote struct {
	Note  string     `capnp:"note"`
	RawIA addr.IAInt `capnp:"isdas"`
}

func (s *DenseNote) ProtoId() proto.ProtoIdType {
	return proto.DenseStaticInfo_Note_TypeID
}

func (s *DenseNote) String() string {
	return fmt.Sprintf("Text: %s\nISD: %d\nAS: %d\n",
		s.Note, s.RawIA.IA().I, s.RawIA.IA().A)
}

type RawPathMetadata struct {
	ASLatencies  map[addr.IA]ASLatency
	ASBandwidths map[addr.IA]ASBandwidth
	ASHops       map[addr.IA]ASHops
	Geo          map[addr.IA]ASGeo
	Links        map[addr.IA]ASLink
	Notes        map[addr.IA]ASnote
}

// PathMetadata is the condensed form of metadata retaining only the most important values.
type PathMetadata struct {
	TotalLatency uint16            `capnp:"totalLatency"`
	TotalHops    uint8             `capnp:"totalHops"`
	MinOfMaxBWs  uint32            `capnp:"bandwidthBottleneck"`
	LinkTypes    []DenseASLinkType `capnp:"linkTypes"`
	Locations    []DenseGeo        `capnp:"asLocations"`
	Notes        []DenseNote       `capnp:"notes"`
}

func (s *PathMetadata) ProtoId() proto.ProtoIdType {
	return proto.DenseStaticInfo_TypeID
}

func (s *PathMetadata) String() string {
	return fmt.Sprintf("TotalLatency: %v\nTotalHops: %v\n"+
		"BandwidthBottleneck: %v\nLinkTypes: %v\nASLocations: %v\nNotes: %v\n",
		s.TotalLatency, s.TotalHops, s.MinOfMaxBWs, s.LinkTypes, s.Locations,
		s.Notes)
}

// Condensemetadata takes RawPathMetadata and extracts/condenses
// the most important values to be transmitted to SCIOND
func (data *RawPathMetadata) Condensemetadata() *PathMetadata {
	ret := &PathMetadata{
		TotalLatency: 0,
		TotalHops:    0,
		MinOfMaxBWs:  math.MaxUint32,
	}

	for _, val := range data.ASBandwidths {
		var asmaxbw uint32 = math.MaxUint32
		if val.IntraBW > 0 {
			asmaxbw = uint32(math.Min(float64(val.IntraBW), float64(asmaxbw)))
		}
		if val.InterBW > 0 {
			asmaxbw = uint32(math.Min(float64(val.InterBW), float64(asmaxbw)))
		}
		if asmaxbw < (math.MaxUint32) {
			ret.MinOfMaxBWs = uint32(math.Min(float64(ret.MinOfMaxBWs), float64(asmaxbw)))
		}
	}

	if !(ret.MinOfMaxBWs < math.MaxUint32) {
		ret.MinOfMaxBWs = 0
	}

	for _, val := range data.ASLatencies {
		ret.TotalLatency += val.InterLatency + val.IntraLatency + val.PeerLatency
	}

	for _, val := range data.ASHops {
		ret.TotalHops += val.Hops
	}

	for IA, note := range data.Notes {
		ret.Notes = append(ret.Notes, DenseNote{
			Note:  note.Note,
			RawIA: IA.IAInt(),
		})
	}

	for IA, loc := range data.Geo {
		ret.Locations = append(ret.Locations, DenseGeo{
			RawIA:           IA.IAInt(),
			RouterLocations: loc.locations,
		})
	}

	for IA, link := range data.Links {
		ret.LinkTypes = append(ret.LinkTypes, DenseASLinkType{
			InterLinkType: link.InterLinkType,
			PeerLinkType:  link.PeerLinkType,
			RawIA:         IA.IAInt(),
		})
	}

	return ret
}

type ASEntryList struct {
	Ups      []*seg.ASEntry
	Cores    []*seg.ASEntry
	Downs    []*seg.ASEntry
	UpPeer   int
	DownPeer int
}

func (solution *PathSolution) GatherASEntries() *ASEntryList {
	var res ASEntryList
	for _, solEdge := range solution.edges {
		asEntries := solEdge.segment.ASEntries
		currType := solEdge.segment.Type
		for asEntryIdx := len(asEntries) - 1; asEntryIdx >= solEdge.edge.Shortcut; asEntryIdx-- {
			asEntry := asEntries[asEntryIdx]
			switch currType {
			case proto.PathSegType_up:
				res.Ups = append(res.Ups, asEntry)
			case proto.PathSegType_core:
				res.Cores = append(res.Ups, asEntry)
			case proto.PathSegType_down:
				res.Downs = append(res.Ups, asEntry)
			}
		}
		switch currType {
		case proto.PathSegType_up:
			res.UpPeer = solEdge.edge.Peer
		case proto.PathSegType_down:
			res.DownPeer = solEdge.edge.Peer
		}
	}
	return &res
}

func (res *RawPathMetadata) ExtractPeerdata(asEntry *seg.ASEntry, peerIfID common.IFIDType, includePeer bool) {
	IA := asEntry.IA()
	StaticInfo := asEntry.Exts.StaticInfo
	for i := 0; i < len(StaticInfo.Latency.Peerlatencies); i++ {
		if StaticInfo.Latency.Peerlatencies[i].IfID == peerIfID {
			if includePeer {
				res.ASLatencies[IA] = ASLatency{
					IntraLatency: StaticInfo.Latency.Peerlatencies[i].IntraDelay,
					InterLatency: StaticInfo.Latency.Egresslatency,
					PeerLatency:  StaticInfo.Latency.Peerlatencies[i].Interdelay,
				}
			} else {
				res.ASLatencies[IA] = ASLatency{
					IntraLatency: StaticInfo.Latency.Peerlatencies[i].IntraDelay,
					InterLatency: StaticInfo.Latency.Egresslatency,
				}
			}
		}
	}
	for i := 0; i < len(StaticInfo.Linktype.Peerlinks); i++ {
		if StaticInfo.Linktype.Peerlinks[i].IfID == peerIfID {
			if includePeer {
				res.Links[IA] = ASLink{
					InterLinkType: StaticInfo.Linktype.EgressLinkType,
					PeerLinkType:  StaticInfo.Linktype.Peerlinks[i].LinkType,
				}
			} else {
				res.Links[IA] = ASLink{
					InterLinkType: StaticInfo.Linktype.EgressLinkType,
				}
			}
		}
	}
	for i := 0; i < len(StaticInfo.Bandwidth.Bandwidths); i++ {
		if StaticInfo.Bandwidth.Bandwidths[i].IfID == peerIfID {
			res.ASBandwidths[IA] = ASBandwidth{
				IntraBW: StaticInfo.Bandwidth.Bandwidths[i].BW,
				InterBW: StaticInfo.Bandwidth.EgressBW,
			}
		}
	}
	for i := 0; i < len(StaticInfo.Hops.InterfaceHops); i++ {
		if StaticInfo.Hops.InterfaceHops[i].IfID == peerIfID {
			res.ASHops[IA] = ASHops{
				Hops: StaticInfo.Hops.InterfaceHops[i].Hops,
			}
		}
	}
	res.Geo[IA] = getGeo(asEntry)
	res.Notes[IA] = ASnote{
		Note: StaticInfo.Note,
	}
}

func (res *RawPathMetadata) ExtractNormaldata(asEntry *seg.ASEntry) {
	IA := asEntry.IA()
	StaticInfo := asEntry.Exts.StaticInfo
	res.ASLatencies[IA] = ASLatency{
		IntraLatency: StaticInfo.Latency.IngressToEgressLatency,
		InterLatency: StaticInfo.Latency.Egresslatency,
		PeerLatency:  0,
	}
	res.Links[IA] = ASLink{
		InterLinkType: StaticInfo.Linktype.EgressLinkType,
	}
	res.ASBandwidths[IA] = ASBandwidth{
		IntraBW: StaticInfo.Bandwidth.IngressToEgressBW,
		InterBW: StaticInfo.Bandwidth.EgressBW,
	}
	res.ASHops[IA] = ASHops{
		Hops: StaticInfo.Hops.InToOutHops,
	}
	res.Geo[IA] = getGeo(asEntry)
	res.Notes[IA] = ASnote{
		Note: StaticInfo.Note,
	}
}

func (res *RawPathMetadata) ExtractUpOverdata(oldASEntry *seg.ASEntry, newASEntry *seg.ASEntry) {
	IA := newASEntry.IA()
	StaticInfo := newASEntry.Exts.StaticInfo
	hopEntry := oldASEntry.HopEntries[0]
	HF, _ := hopEntry.HopField()
	oldEgressIFID := HF.ConsEgress
	for i := 0; i < len(StaticInfo.Latency.Childlatencies); i++ {
		if StaticInfo.Latency.Childlatencies[i].IfID == oldEgressIFID {
			res.ASLatencies[IA] = ASLatency{
				IntraLatency: StaticInfo.Latency.Childlatencies[i].Intradelay,
				InterLatency: StaticInfo.Latency.Egresslatency,
				PeerLatency:  oldASEntry.Exts.StaticInfo.Latency.Egresslatency,
			}
		}
	}
	res.Links[IA] = ASLink{
		InterLinkType: StaticInfo.Linktype.EgressLinkType,
		PeerLinkType:  oldASEntry.Exts.StaticInfo.Linktype.EgressLinkType,
	}

	for i := 0; i < len(StaticInfo.Bandwidth.Bandwidths); i++ {
		if StaticInfo.Bandwidth.Bandwidths[i].IfID == oldEgressIFID {
			res.ASBandwidths[IA] = ASBandwidth{
				IntraBW: StaticInfo.Bandwidth.Bandwidths[i].BW,
				InterBW: StaticInfo.Bandwidth.EgressBW,
			}
		}
	}
	for i := 0; i < len(StaticInfo.Hops.InterfaceHops); i++ {
		if StaticInfo.Hops.InterfaceHops[i].IfID == oldEgressIFID {
			res.ASHops[IA] = ASHops{
				Hops: StaticInfo.Hops.InterfaceHops[i].Hops,
			}
		}
	}
	res.Geo[IA] = getGeo(newASEntry)
	res.Notes[IA] = ASnote{
		Note: StaticInfo.Note,
	}
}

func (res *RawPathMetadata) ExtractCoreOverdata(oldASEntry *seg.ASEntry, newASEntry *seg.ASEntry) {
	IA := newASEntry.IA()
	StaticInfo := newASEntry.Exts.StaticInfo
	hopEntry := oldASEntry.HopEntries[0]
	HF, _ := hopEntry.HopField()
	oldIngressIfID := HF.ConsIngress
	for i := 0; i < len(StaticInfo.Latency.Childlatencies); i++ {
		if StaticInfo.Latency.Childlatencies[i].IfID == oldIngressIfID {
			res.ASLatencies[IA] = ASLatency{
				IntraLatency: StaticInfo.Latency.Childlatencies[i].Intradelay,
				InterLatency: StaticInfo.Latency.Egresslatency,
			}
		}
	}
	res.Links[IA] = ASLink{
		InterLinkType: StaticInfo.Linktype.EgressLinkType,
	}

	for i := 0; i < len(StaticInfo.Bandwidth.Bandwidths); i++ {
		if StaticInfo.Bandwidth.Bandwidths[i].IfID == oldIngressIfID {
			res.ASBandwidths[IA] = ASBandwidth{
				IntraBW: StaticInfo.Bandwidth.Bandwidths[i].BW,
				InterBW: StaticInfo.Bandwidth.EgressBW,
			}
		}
	}
	for i := 0; i < len(StaticInfo.Hops.InterfaceHops); i++ {
		if StaticInfo.Hops.InterfaceHops[i].IfID == oldIngressIfID {
			res.ASHops[IA] = ASHops{
				Hops: StaticInfo.Hops.InterfaceHops[i].Hops,
			}
		}
	}
	res.Geo[IA] = getGeo(newASEntry)
	res.Notes[IA] = ASnote{
		Note: StaticInfo.Note,
	}
}

func (ASes *ASEntryList) CombineSegments() *RawPathMetadata {
	var res RawPathMetadata
	var LastUpASEntry *seg.ASEntry
	var LastCoreASEntry *seg.ASEntry
	// Go through ASEntries in the up segment (except for the first one)
	// and extract the static info data from them
	for idx := 0; idx < len(ASes.Ups); idx++ {
		asEntry := ASes.Ups[idx]
		s := asEntry.Exts.StaticInfo
		if s != nil {
			if idx == 0 {
				res.Geo[asEntry.IA()] = getGeo(asEntry)
				continue
			}
			if (idx > 0) && (idx < (len(ASes.Ups) - 1)) {
				res.ExtractNormaldata(asEntry)
			} else {
				if ASes.UpPeer != 0 {
					peerEntry := asEntry.HopEntries[ASes.UpPeer]
					PE, _ := peerEntry.HopField()
					peerIfID := PE.ConsIngress
					res.ExtractPeerdata(asEntry, peerIfID, true)
				} else {
					// If the last up AS is not involved in peering,
					// do nothing except store the as in LastUpASEntry
					LastUpASEntry = asEntry
				}
			}
		}
	}

	// Go through ASEntries in the core segment
	// and extract the static info data from them
	for idx := 0; idx < len(ASes.Cores); idx++ {
		asEntry := ASes.Cores[idx]
		// If we're in the first inspected AS (i.e the last AS of the segment)
		// only set LastCoreASEntry
		s := asEntry.Exts.StaticInfo
		if s != nil {
			if idx == 0 {
				LastCoreASEntry = asEntry
				if len(ASes.Cores) == 0 {
					res.Geo[asEntry.IA()] = getGeo(asEntry)
				}
				continue
			}
			if (idx > 0) && (idx < (len(ASes.Ups) - 1)) {
				res.ExtractNormaldata(asEntry)
			} else {
				if len(ASes.Ups) > 0 {
					// We're in the AS where we cross over from the up to the core segment
					res.ExtractUpOverdata(asEntry, LastUpASEntry)
				}
			}
		}
	}

	// Go through ASEntries in the down segment except for the first one
	// and extract the static info data from them
	for idx := 0; idx < len(ASes.Cores); idx++ {
		asEntry := ASes.Cores[idx]
		s := asEntry.Exts.StaticInfo
		if s != nil {
			if idx == 0 {
				res.Geo[asEntry.IA()] = getGeo(asEntry)
				continue
			}
			if (idx > 0) && (idx < (len(ASes.Ups) - 1)) {
				res.ExtractNormaldata(asEntry)
			} else {
				if ASes.DownPeer != 0 {
					// We're in the AS where we peered over from the up to the down segment
					peerEntry := asEntry.HopEntries[ASes.UpPeer]
					PE, _ := peerEntry.HopField()
					peerIfID := PE.ConsIngress
					res.ExtractPeerdata(asEntry, peerIfID, false)
				} else {
					if len(ASes.Cores) > 0 {
						// We're in the AS where we cross over from the core to the down segment
						res.ExtractCoreOverdata(LastCoreASEntry, asEntry)
					}
					if (len(ASes.Ups) > 0) && (len(ASes.Cores) == 0) {
						// We're in the AS where we cross over from the up to the down segment via a shortcut
						res.ExtractUpOverdata(LastUpASEntry, asEntry)
					}
				}
			}
		}
	}
	return &res
}

func getGeo(asEntry *seg.ASEntry) ASGeo {
	var locations []GeoLoc
	for _, loc := range asEntry.Exts.StaticInfo.Geo.Locations {
		locations = append(locations, GeoLoc{
			Latitude:  loc.GPSData.Latitude,
			Longitude: loc.GPSData.Longitude,
			Address:   loc.GPSData.Address,
		})
	}
	res := ASGeo{
		locations: locations,
	}
	return res
}