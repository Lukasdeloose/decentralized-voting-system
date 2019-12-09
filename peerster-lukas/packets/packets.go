package packets

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
)

const SIMPLE = 1
const RUMOR = 2
const STATUS = 3
const PRIVATE = 4
const DATAREQUEST = 5
const DATAREPLY = 6
const SEARCHREQUEST = 7
const SEARCHREPLY = 8
const TLCMESSAGE = 9
const TLCACK = 10

type PeerStatus struct {
	Identifier string
	NextID     uint32
}

type Message struct {
	Text        string
	Destination *string
	File        *string
	Request     *[]byte
	KeyWords    string
	Budget      int
}

type SimpleMessage struct {
	OriginalName  string
	RelayPeerAddr string
	Contents      string
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

type StatusPacket struct {
	Want []PeerStatus
}

type DataRequest struct {
	Origin      string
	Destination string
	HopLimit    uint32
	HashValue   []byte
}

type DataReply struct {
	Origin      string
	Destination string
	HopLimit    uint32
	HashValue   []byte // Represents hash of chunk or MetaHash
	Data        []byte
}

type SearchRequest struct {
	Origin   string
	Budget   uint64
	Keywords []string
}

type SearchResult struct {
	FileName     string
	MetafileHash []byte
	ChunkMap     []uint64
	ChunkCount   uint64
}

type SearchReply struct {
	Origin      string
	Destination string
	HopLimit    uint32
	Results     []*SearchResult
}

type TxPublish struct {
	Name         string
	Size         int64 // Size in bytes
	MetafileHash []byte
}

type BlockPublish struct {
	PrevHash    [32]byte
	Transaction TxPublish
}

func (b *BlockPublish) Hash() (out [32]byte) {
	h := sha256.New()
	h.Write(b.PrevHash[:])
	th := b.Transaction.Hash()
	h.Write(th[:])
	copy(out[:], h.Sum(nil))
	return
}
func (t *TxPublish) Hash() (out [32]byte) {
	h := sha256.New()
	err := binary.Write(h, binary.LittleEndian, uint32(len(t.Name)))
	if err != nil {
		fmt.Println(err)
		return
	}
	h.Write([]byte(t.Name))
	h.Write(t.MetafileHash)
	copy(out[:], h.Sum(nil))
	return
}

type TLCMessage struct {
	Origin      string
	ID          uint32
	Confirmed   int
	TxBlock     BlockPublish
	VectorClock *StatusPacket
	Fitness     float32
}

type TLCAck PrivateMessage

type GossipPacket struct {
	Simple        *SimpleMessage
	Rumor         *RumorMessage
	Status        *StatusPacket
	Private       *PrivateMessage
	DataRequest   *DataRequest
	DataReply     *DataReply
	SearchRequest *SearchRequest
	SearchReply   *SearchReply
	TLCMessage    *TLCMessage
	Ack           *TLCAck
}

type GUIMessage struct {
	Rumor         *RumorMessage
	MessageNumber int // In order to show in order in frontend
}
