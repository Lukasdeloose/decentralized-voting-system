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
	Voters   []string // Hashes of Sciper numbers of people who are allowed to vote
}

func (poll Poll) isEqual(poll2 Poll) bool {
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
}

func (tx Transactions) toString() string {
	str := ""
	for _, vote := range tx.Votes {
		str += string(vote.ID) + vote.Vote.Origin + string(vote.Vote.PollID) + vote.Vote.Vote.String()
	}
	for _, poll := range tx.Polls {
		str += string(poll.ID) + poll.Poll.Origin + poll.Poll.Question
		for _, voter := range poll.Poll.Voters {
			str += voter
		}
	}
	// TODO: Registers
	return str
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
