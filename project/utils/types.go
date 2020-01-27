package utils

import (
	"fmt"
	. "github.com/lukasdeloose/decentralized-voting-system/project/constants"
	. "github.com/lukasdeloose/decentralized-voting-system/project/udp"
)

// Definition of all message types

type Message struct {
	Text        string
	Destination *string
}


type RumorMessage struct {
	Origin string
	ID     uint32
	Text   string
}

type PrivateMessage struct {
	Origin      string
	ID          uint32
	Text        string
	Destination string
	HopLimit    uint32
}

type PeerStatus struct {
	Identifier string
	NextID     uint32
}

type StatusPacket struct {
	Want []PeerStatus
}

type GossipPacket struct {
	Rumor *RumorMessage
	Status *StatusPacket
	Private *PrivateMessage
}

type AddrGossipPacket struct {
	Address UDPAddr
	Gossip  *GossipPacket
}

type Messages struct {
	Msgs []*RumorMessage
}


// Messages that can be directly sent from peer to peer:
// PrivateMessages
type PointToPointMessage interface {
	GetOrigin() string
	GetDestination() string
	HopIsZero() bool
	DecrHopLimit()

	ToGossip() *GossipPacket
}

// Implement the point to point interface for PrivateMessage
func (p *PrivateMessage) GetOrigin() string       { return p.Origin }
func (p *PrivateMessage) GetDestination() string  { return p.Destination }
func (p *PrivateMessage) HopIsZero() bool         { return p.HopLimit == 0 }
func (p *PrivateMessage) DecrHopLimit()           { p.HopLimit -= 1 }
func (p *PrivateMessage) ToGossip() *GossipPacket { return &GossipPacket{Private: p} }

// Get point to point message from GossipPacket
func (g *GossipPacket) ToP2PMessage() PointToPointMessage {
	if g.Private != nil {
		return g.Private
	} else {
		return nil
	}
}

// Messages that can be mongered
type MongerableMessage interface {
	GetOrigin() string
	GetID()     uint32

	ToGossip() *GossipPacket
}


// Implement the MongerableMessage interface for RumorMessage
func (r *RumorMessage) GetOrigin() string { return r.Origin }
func (r *RumorMessage) GetID() uint32 { return r.ID }
func (r *RumorMessage) ToGossip() *GossipPacket { return &GossipPacket{ Rumor: r}}



// Get MongerableMessage from GossipPacket
func (g *GossipPacket) ToMongerableMessage() MongerableMessage {
	if g.Rumor != nil {
		return g.Rumor
	} else {
		return nil
	}
}

// Helper function to convert []byte hashes to [32]byte hashes
func To32Byte(bs []byte) [32]byte {
	if len(bs) != 32 {
		if Debug {
			fmt.Println("[DEBUG] Warning: To32Byte is transforming byte slice with len != 32")
		}
	}
	var hash [32]byte
	copy(hash[:], bs[:32])
	return hash
}
