package blockchain

import (
	"bitbucket.org/ustraca/crypto/paillier"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	. "github.com/lukasdeloose/decentralized-voting-system/project/utils"
	"strings"
	"sync"
	"time"
)

// Blockchain implementation

type Blockchain struct {
	Transactions            chan *Transaction
	unconfirmedTransactions Transactions
	TransactionsLock        sync.RWMutex

	Blocks         []*Block
	nextRegisterId uint32
	nextVoteId     uint32
	nextPollId     uint32
	nextResultId   uint32

	difficulty int

	Registry []*RegisterTx
	Votes    map[uint32][]*VoteTx // Votes by pollID
	Polls    []*PollTx
	Results  map[uint32]*ResultTx // Results by pollID
	mutex    *sync.RWMutex
}

func NewBlockChain() *Blockchain {
	Blocks := make([]*Block, 1)
	Blocks[0] = &Block{
		ID:           0,
		Timestamp:    time.Now(),
		Transactions: Transactions{},
		Difficulty:   1,
		Nonce:        "",
		PrevHash:     "0",
	} // Genesis block
	Blocks[0].Hash = calculateHash(Blocks[0])
	return &Blockchain{
		Transactions:            make(chan *Transaction),
		Registry:                make([]*RegisterTx, 0),
		Votes:                   make(map[uint32][]*VoteTx),
		Polls:                   make([]*PollTx, 0),
		Results:                 make(map[uint32]*ResultTx),
		unconfirmedTransactions: Transactions{},
		Blocks:                  Blocks,
		difficulty:              1,
		mutex:                   &sync.RWMutex{},
	}
}

func (b *Blockchain) GetPoll(pollId uint32) *PollTx {
	b.mutex.RLock()
	defer b.mutex.RUnlock()

	for _, poll := range b.Polls {
		if poll.ID == pollId {
			return poll
		}
	}
	return nil
}

func (b *Blockchain) GetVotes(pollId uint32) []*VoteTx {
	b.mutex.RLock()
	defer b.mutex.RUnlock()

	return b.Votes[pollId]

}

func (b *Blockchain) GetResult(pollId uint32) *ResultTx {
	b.mutex.RLock()
	defer b.mutex.RUnlock()

	return b.Results[pollId]
}

func (b *Blockchain) lastBlock() *Block {
	return b.Blocks[len(b.Blocks)-1]
}

func (b *Blockchain) length() int {
	return len(b.Blocks)
}

func (b *Blockchain) addUnconfirmedTransactions(tx Transactions) {
	b.TransactionsLock.Lock()
	defer b.TransactionsLock.Unlock()

	if tx.Polls != nil {
		b.unconfirmedTransactions.Polls = append(b.unconfirmedTransactions.Polls, tx.Polls...)
	}
	if tx.Votes != nil {
		b.unconfirmedTransactions.Votes = append(b.unconfirmedTransactions.Votes, tx.Votes...)
	}
	if tx.Registers != nil {
		b.unconfirmedTransactions.Registers = append(b.unconfirmedTransactions.Registers, tx.Registers...)
	}
	if tx.Results != nil {
		b.unconfirmedTransactions.Results = append(b.unconfirmedTransactions.Results, tx.Results...)
	}
}

func (b *Blockchain) removeConfirmedTx(tx Transactions) {
	b.TransactionsLock.Lock()
	defer b.TransactionsLock.Unlock()

	// Keep only votes that are not confirmed
	newVotes := b.unconfirmedTransactions.Votes[:0]
	for _, unconfirmedVote := range b.unconfirmedTransactions.Votes {
		found := false
		for _, confirmedVote := range tx.Votes {
			if confirmedVote.Vote == unconfirmedVote.Vote {
				found = true
			}
		}
		if !found {
			newVotes = append(newVotes, unconfirmedVote)
		}
	}
	b.unconfirmedTransactions.Votes = newVotes

	// Polls
	// TODO: check based on ID?
	newPolls := b.unconfirmedTransactions.Polls[:0]
	for _, unconfirmedPoll := range b.unconfirmedTransactions.Polls {
		found := false
		for _, confirmedPoll := range tx.Polls {
			if confirmedPoll.Poll.IsEqual(unconfirmedPoll.Poll) {
				found = true
			}
		}
		if !found {
			newPolls = append(newPolls, unconfirmedPoll)
		}
	}
	b.unconfirmedTransactions.Polls = newPolls

	// TODO: registers and results
}

func (b *Blockchain) GetPolls() []*PollTx {
	b.mutex.RLock()
	defer b.mutex.RUnlock()

	res := make([]*PollTx, len(b.Polls))
	i := 0
	for _, poll := range b.Polls {
		res[i] = poll
		i += 1
	}
	return res
}

func (b *Blockchain) AddTransaction(t *Transaction) {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	if t.PollTx != nil {
		b.Polls = append(b.Polls, t.PollTx)
		fmt.Printf("BLOCKCHAIN ADD polltx %v %v\n", t.PollTx.ID, t.PollTx.Poll.Question)
	} else if t.VoteTx != nil {
		b.Votes[t.VoteTx.Vote.PollID] = append(b.Votes[t.VoteTx.Vote.PollID], t.VoteTx)
		fmt.Printf("BLOCKCHAIN ADD votetx for %v\n", t.VoteTx.Vote.PollID)
	} else if t.RegisterTx != nil {
		b.Registry = append(b.Registry, t.RegisterTx)
		fmt.Printf("BLOCKCHAIN ADD registerTx for %v\n", t.RegisterTx.Registry.Origin)
	} else if t.ResultTx != nil {
		b.Results[t.ResultTx.Result.PollId] = t.ResultTx
		fmt.Printf("BLOCKCHAIN ADD resultTx for poll %v\n", t.ResultTx.Result.PollId)
	}
}

func (b *Blockchain) addTransactions(t Transactions) {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	for _, vote := range t.Votes {
		b.Votes[vote.Vote.PollID] = append(b.Votes[vote.Vote.PollID], &vote)
	}

	for _, poll := range t.Polls {
		b.Polls = append(b.Polls, &poll)
		b.Votes[poll.ID] = make([]*VoteTx, 0)
	}

	for _, register := range t.Registers {
		b.Registry = append(b.Registry, &register)
	}

	for _, result := range t.Results {
		b.Results[result.Result.PollId] = &result
	}
}

func (b *Blockchain) PollKey(pollid uint32) (paillier.PublicKey, bool) {
	b.mutex.RLock()
	defer b.mutex.RUnlock()

	for _, poll := range b.Polls {
		if poll.ID == pollid {
			return poll.Poll.PublicKey.ToPaillier(), true
		}
	}

	return paillier.PublicKey{}, false
}

func (b *Blockchain) RegistryKey(origin string) (rsa.PublicKey, bool) {
	b.mutex.RLock()
	defer b.mutex.RUnlock()

	for _, reg := range b.Registry {
		if reg.Registry.Origin == origin {
			return reg.Registry.PublicKey.ToRSA(), true
		}
	}

	return rsa.PublicKey{}, false
}

func (b *Blockchain) RetrieveVotes(pollid uint32) []*EncryptedVote {
	b.mutex.RLock()
	defer b.mutex.RUnlock()

	votes := make([]*EncryptedVote, 0)
	for _, vote := range b.Votes[pollid] {
		votes = append(votes, vote.Vote)
	}

	return votes
}

func (b *Blockchain) pollValid(pollTx PollTx) bool {
	// TODO: check signature
	// Check if ID is unique, in known polls and this transaction
	nextPollId := b.nextPollId
	if pollTx.ID != nextPollId {
		return false
	}
	nextPollId++

	if pollTx.Poll.Question == "" {
		return false
	}

	if pollTx.Poll.Deadline.Before(time.Now()) {
		return false
	}
	return true
}

func (b *Blockchain) voteValid(voteTx VoteTx) bool {
	// TODO: check signature
	// Check if ID is unique, in known polls and this transaction
	nextVoteId := b.nextVoteId
	if voteTx.ID != nextVoteId {
		return false
	}
	nextVoteId++

	// Poll exists
	if voteTx.Vote.PollID >= b.nextPollId {
		return false
	}
	return true
}

func (b *Blockchain) registerValid(registerTx RegisterTx) bool {
	// TODO
	return true
}

func calculateHash(block *Block) string {
	record := block.ToString()
	h := sha256.New()
	h.Write([]byte(record))
	hashed := h.Sum(nil)
	return hex.EncodeToString(hashed)
}

// When receiving a new block from another peer, this function checks if it is valid:
// - Hash is correct and starts with necessary amount of zeros
func hashesValid(block *Block) bool {
	if !hashValid(block.Hash, block.Difficulty) {
		return false
	}
	if block.Hash != calculateHash(block) {
		return false
	}
	return true
}

// Hash starts with necessary amount of 0's
func hashValid(hash string, difficulty int) bool {
	prefix := strings.Repeat("0", difficulty)
	return strings.HasPrefix(hash, prefix)
}
