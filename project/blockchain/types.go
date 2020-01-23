package blockchain

import (
	"bitbucket.org/ustraca/crypto/paillier"
	. "github.com/lukasdeloose/decentralized-voting-system/project/utils"
)

var Blockchain []Block

// New votes cast
type VoteTx struct {

}

// New users registered
type RegisterTx struct {

}

// New poll added
type PollTx struct {

}


// Transactions that happened since last Block
type Transactions struct {
	 votes []VoteTx
	 registers []RegisterTx
	 polls []PollTx
}

type Block struct {
	Index          int
	Timestamp      string
	Scipers        []uint32 // Sciper numbers of all registered citizens, generally 6 digits. Should only be added by the 'government'
	PaillierPublic paillier.PublicKey
	Hash           string
	PrevHash       string
	Difficulty     int
	Nonce          string
}

type BlockMessage struct {
	Origin      string
	ID          uint32
	Confirmed   int
	Block       Block
	VectorClock *StatusPacket
}

type BlockAck PrivateMessage
