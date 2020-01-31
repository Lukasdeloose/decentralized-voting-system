package utils

import (
	"bitbucket.org/ustraca/crypto/paillier"
	"crypto/rsa"
	"encoding/hex"
	"fmt"
	. "github.com/lukasdeloose/decentralized-voting-system/project/constants"
	. "github.com/lukasdeloose/decentralized-voting-system/project/udp"
	"math/big"
	"time"
)

// Definition of all message types

type Message struct {
	Text        string
	Destination *string
	Voting      *VotingMessage
}

type VotingMessage struct {
	NewVote      *NewVote
	NewPoll      *NewPoll
	CountRequest *CountRequest
}

type NewVote struct {
	Pollid uint32
	Vote   bool
}

type NewPoll struct {
	Question string
	Voters   []string
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
type Block struct {
	ID             uint32
	Timestamp      time.Time
	Transactions   Transactions
	PaillierPublic paillier.PublicKey
	Difficulty     int
	Origin         string
	Nonce          string
	PrevHash       string
	Hash           string
}

// Convert the fields of the block to a string representation, allowing us to hash it
func (b Block) ToString() string {
	//str := ""
	//str = fmt.Sprint(b.ID, b.Timestamp.String(), b.Difficulty, b.Transactions.ToString(), b.PaillierPublic.N.String(),
	//	b.PaillierPublic.G.String(), b.PrevHash, b.Nonce)
	str := fmt.Sprint(b.Nonce, b.Origin, b.Difficulty, b.ID, b.PrevHash, b.Transactions.ToString(), b.PaillierPublic.N.String(),
		b.PaillierPublic.G.String())
	return str
}

// Transactions that happened since last Block
type Transactions struct {
	Votes     []*VoteTx
	Polls     []*PollTx
	Registers []*RegisterTx
	Results   []*ResultTx
}

// Helper function to convert transactions to string
func (tx Transactions) ToString() string {
	str := ""
	for _, vote := range tx.Votes {
		str += fmt.Sprint(vote.ID, vote.Vote.Origin, vote.Vote.PollID, hex.EncodeToString(vote.Vote.Vote))
	}
	for _, poll := range tx.Polls {
		str += fmt.Sprint(poll.ID, poll.Poll.Origin, poll.Poll.Question)
		for _, voter := range poll.Poll.Voters {
			str += fmt.Sprint(voter)
		}
	}
	// TODO: Registers
	return str
}

type SerializablePaillierPubKey struct {
	N []byte
	G []byte
}

func (s *SerializablePaillierPubKey) ToPaillier() paillier.PublicKey {
	return paillier.PublicKey{
		N: (&big.Int{}).SetBytes(s.N),
		G: (&big.Int{}).SetBytes(s.G),
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
	Origin    string
	Id        uint32
	Question  string
	Voters    []string // Hashes of Sciper numbers of people who are allowed to vote
	Deadline  time.Time
	PublicKey SerializablePaillierPubKey
}

func (poll *Poll) IsEqual(poll2 *Poll) bool {
	if poll.Origin != poll2.Origin {
		return false
	}
	if poll.Id != poll2.Id {
		return false
	}
	return true
}

type EncryptedVote struct {
	Origin string
	PollID uint32
	Vote   []byte
}

type Registry struct {
	Origin    string
	PublicKey SerializableRSAPubKey
}

// New votes cast
type VoteTx struct {
	ID        uint32
	Vote      *EncryptedVote
	Signature []byte
}

// New poll added, finder of block assigns the unique pollID
type PollTx struct {
	Poll      *Poll
	ID        uint32
	Signature []byte
}

// New users registered
type RegisterTx struct {
	ID       uint32
	Registry *Registry
}

// Results of the poll
type ResultTx struct {
	ID     uint32
	Result *Result
}

type Result struct {
	Count     int64
	PollId    uint32
	Timestamp time.Time
}

/******************************************************************************/

type GossipPacket struct {
	Rumor           *RumorMessage
	Status          *StatusPacket
	Private         *PrivateMessage
	Transaction     *Transaction
	MongerableBlock *MongerableBlock
}

type Transaction struct {
	Origin     string
	ID         uint32
	VoteTx     *VoteTx
	PollTx     *PollTx
	RegisterTx *RegisterTx
	ResultTx   *ResultTx
}

type MongerableBlock struct {
	Origin string
	ID     uint32
	Block  *Block
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
	GetID() uint32
	SetID(uint32)

	ToGossip() *GossipPacket
}

// Implement the MongerableMessage interface for RumorMessage
func (r *RumorMessage) GetOrigin() string       { return r.Origin }
func (r *RumorMessage) GetID() uint32           { return r.ID }
func (r *RumorMessage) SetID(id uint32)         { r.ID = id }
func (r *RumorMessage) ToGossip() *GossipPacket { return &GossipPacket{Rumor: r} }

// Implement the MongerableMessage interface for Transaction
func (t *Transaction) GetOrigin() string       { return t.Origin }
func (t *Transaction) GetID() uint32           { return t.ID }
func (t *Transaction) SetID(id uint32)         { t.ID = id }
func (t *Transaction) ToGossip() *GossipPacket { return &GossipPacket{Transaction: t} }

// Implement the MongerableMessage interface for Block
func (b *MongerableBlock) GetOrigin() string       { return b.Origin }
func (b *MongerableBlock) GetID() uint32           { return b.ID }
func (b *MongerableBlock) SetID(id uint32)         { b.ID = id }
func (b *MongerableBlock) ToGossip() *GossipPacket { return &GossipPacket{MongerableBlock: b} }

// Get MongerableMessage from GossipPacket
func (g *GossipPacket) ToMongerableMessage() MongerableMessage {
	if g.Rumor != nil {
		return g.Rumor
	} else if g.Transaction != nil {
		return g.Transaction
	} else if g.MongerableBlock != nil {
		return g.MongerableBlock
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
