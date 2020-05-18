package sciond

import (
	"fmt"
	
	"github.com/scionproto/scion/go/lib/addr"
	"github.com/scionproto/scion/go/proto"
)


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
	return proto.PathMetadata_TypeID
}

func (s *PathMetadata) String() string {
	return fmt.Sprintf("TotalLatency: %v\nTotalHops: %v\n"+
		"BandwidthBottleneck: %v\nLinkTypes: %v\nASLocations: %v\nNotes: %v\n",
		s.TotalLatency, s.TotalHops, s.MinOfMaxBWs, s.LinkTypes, s.Locations,
		s.Notes)
}

type DenseASLinkType struct {
	InterLinkType uint16     `capnp:"interLinkType"`
	PeerLinkType  uint16     `capnp:"peerLinkType"`
	RawIA         addr.IAInt `capnp:"isdas"`
}

func (s *DenseASLinkType) ProtoId() proto.ProtoIdType {
	return proto.PathMetadata_InterfaceLinkType_TypeID
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
	return proto.PathMetadata_Geo_TypeID
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
	return proto.PathMetadata_Note_TypeID
}

func (s *DenseNote) String() string {
	return fmt.Sprintf("Text: %s\nISD: %d\nAS: %d\n",
		s.Note, s.RawIA.IA().I, s.RawIA.IA().A)
}

type GeoLoc struct {
	Latitude  float32 `capnp:"latitude"`
	Longitude float32 `capnp:"longitude"`
	Address   string  `capnp:"address"`
}

func (s *GeoLoc) ProtoId() proto.ProtoIdType {
	return proto.PathMetadata_Geo_GPSData_TypeID
}

func (s *GeoLoc) String() string {
	return fmt.Sprintf("Latitude: %f\nLongitude: %f\nAddress: %s\n",
		s.Latitude, s.Longitude, s.Address)
}
