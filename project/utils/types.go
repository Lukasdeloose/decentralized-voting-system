package utils

import (
	"bitbucket.org/ustraca/crypto/paillier"
	"crypto/rsa"
	"fmt"
	. "github.com/lukasdeloose/decentralized-voting-system/project/constants"
	. "github.com/lukasdeloose/decentralized-voting-system/project/udp"
	"math/big"
)

// Definition of all message types

type Message struct {
	Text        string
	Destination *string
	Voting      *VotingMessage
}

type VotingMessage struct {
	NewVote *NewVote
	NewPoll *NewPoll
	CountRequest *CountRequest
}

type NewVote struct {
	Pollid  uint32
	Vote    bool
}


type NewPoll struct {
	Question string
	Voters []string
}


type CountRequest struct {
	Pollid uint32
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

/****************************** Blockchain types ******************************/
type SerializablePaillierPubKey struct {
	N []byte
	G []byte
}

func (s *SerializablePaillierPubKey) ToPaillier() paillier.PublicKey {
	return paillier.PublicKey{
		N:        (&big.Int{}).SetBytes(s.N),
		G:        (&big.Int{}).SetBytes(s.G),
	}
}

type SerializableRSAPubKey struct {
	N []byte
	E int
}

func (s *SerializableRSAPubKey) ToRSA() rsa.PublicKey {
	return rsa.PublicKey{
		N: (&big.Int{}).SetBytes(s.N),
		E: s.E,
	}
}



type Poll struct {
	Origin   string
	Question string
	Voters   []string // Hashes of Sciper numbers of people who are allowed to vote
	PublicKey SerializablePaillierPubKey
}


// TODO seems like a bad idea to identify polls based on questin/origin/voters could use ID?
func (poll *Poll) IsEqual(poll2 *Poll) bool {
	if poll.Origin != poll2.Origin {
		return false
	}
	if poll.Question != poll.Question {
		return false
	}
	if len(poll.Voters) != len(poll2.Voters) {
		return false
	}
	for i := 0; i < len(poll.Voters); i++ {
		if poll.Voters[i] != poll2.Voters[i] {
			return false
		}
	}
	return true
}

type EncryptedVote struct {
	Origin string
	PollID uint32
	Vote   []byte
}


type Registry struct {
	Origin string
	PublicKey SerializableRSAPubKey
}

// New votes cast
type VoteTx struct {
	ID   uint32
	Vote *EncryptedVote
	Signature []byte
}

// New poll added, finder of block assigns the unique pollID
type PollTx struct {
	Poll *Poll
	ID   uint32
	Signature []byte
}

// New users registered
type RegisterTx struct {
	ID uint32
	Registry *Registry
}

/******************************************************************************/


type GossipPacket struct {
	Rumor *RumorMessage
	Status *StatusPacket
	Private *PrivateMessage
	Transaction *Transaction
}


type Transaction struct {
	Origin string
	ID uint32
	VoteTx *VoteTx
	PollTx *PollTx
	RegisterTx *RegisterTx
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
	SetID(uint32)

	ToGossip() *GossipPacket
}


// Implement the MongerableMessage interface for RumorMessage
func (r *RumorMessage) GetOrigin() string { return r.Origin }
func (r *RumorMessage) GetID() uint32 { return r.ID }
func (r *RumorMessage) SetID(id uint32) { r.ID = id }
func (r *RumorMessage) ToGossip() *GossipPacket { return &GossipPacket{ Rumor: r}}


// Implement the MongerableMessage interface for Transaction
func (t *Transaction) GetOrigin() string { return t.Origin }
func (t *Transaction) GetID() uint32 { return t.ID }
func (t *Transaction) SetID(id uint32) { t.ID = id }
func (t *Transaction) ToGossip() *GossipPacket { return &GossipPacket{ Transaction: t}}


// Get MongerableMessage from GossipPacket
func (g *GossipPacket) ToMongerableMessage() MongerableMessage {
	if g.Rumor != nil {
		return g.Rumor
	} else  if g.Transaction != nil{
		return g.Transaction
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
