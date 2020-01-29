package blockchain

import (
	"encoding/hex"
	"github.com/Roasbeef/go-go-gadget-paillier"
	. "github.com/lukasdeloose/decentralized-voting-system/project/utils"
	"time"
)

type BlockAck PrivateMessage


// Transactions that happened since last Block
type Transactions struct {
	Votes     []VoteTx
	Polls     []PollTx
	Registers []RegisterTx
}

func (tx Transactions) toString() string {
	str := ""
	for _, vote := range tx.Votes {
		str += string(vote.ID) + vote.Vote.Origin + string(vote.Vote.PollID) + hex.EncodeToString(vote.Vote.Vote)
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
