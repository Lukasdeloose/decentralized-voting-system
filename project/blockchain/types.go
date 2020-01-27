package blockchain

import (
	"bitbucket.org/ustraca/crypto/paillier"
	. "github.com/lukasdeloose/decentralized-voting-system/project/utils"
	"time"
)

type BlockAck PrivateMessage

type Poll struct {
	Origin   string
	Question string
	Votes    []EncryptedVote
	Voters   []string // Hashes of Sciper numbers of people who are allowed to vote
}

type EncryptedVote struct {
	Origin string
	PollID uint32
	Vote   paillier.Cypher
}

// *** ID's of transactions are 0 (unconfirmed) until they are put on the blockchain *** //

// New votes cast
type VoteTx struct {
	ID   uint32
	Vote EncryptedVote
}

// New poll added, finder of block assigns the unique pollID
type PollTx struct {
	Poll Poll
	ID   uint32
}

type SciperTx struct {
	ID uint32
	// TODO Thomas
}

// New users registered
type RegisterTx struct {
	ID uint32
	// TODO Thomas
}

// Transactions that happened since last Block
type Transactions struct {
	Votes     []VoteTx
	Polls     []PollTx
	Registers []RegisterTx
	Scipers   [] []SciperTx
}

type Block struct {
	Index          int
	Timestamp      time.Time
	Transactions   Transactions
	PaillierPublic paillier.PublicKey
	Difficulty     int
	Nonce          string
	PrevHash       string
	Hash           string
}

type BlockMessage struct {
	Origin      string
	ID          uint32
	Confirmed   int
	Block       Block
	VectorClock *StatusPacket
}
