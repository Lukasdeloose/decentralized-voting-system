package blockchain

import (
	"bitbucket.org/ustraca/crypto/paillier"
	"crypto/rsa"
	"fmt"
	. "github.com/lukasdeloose/decentralized-voting-system/project/utils"
	"sync"
	"time"
)

// DUMMY Blockchain implementation

type id struct {
	Origin string
	Id     uint32
}

type Blockchain struct {
	Transactions chan *Transaction

	Blocks         []Block
	nextRegisterId uint32
	nextVoteId     uint32
	nextPollId     uint32

	difficulty int

	Registry []*RegisterTx
	Votes    map[uint32][]*VoteTx // votes by pollID
	Polls    []*PollTx

	mutex *sync.RWMutex
}

func NewBlockChain() *Blockchain {
	Blocks := make([]Block, 1)
	Blocks[0] = Block{
		Index:        0,
		Timestamp:    time.Now(),
		Transactions: Transactions{},
		Difficulty:   1,
		Nonce:        "",
		PrevHash:     "0",
	} // Genesis block
	Blocks[0].Hash = Blocks[0].calculateHash()
	return &Blockchain{
		Transactions: make(chan *Transaction),
		Registry:     make([]*RegisterTx, 0),
		Votes:        make(map[uint32][]*VoteTx),
		Polls:        make([]*PollTx, 0),
		Blocks:       Blocks,
		difficulty:   1,
		mutex:        &sync.RWMutex{},
	}
}

func (b *Blockchain) Run() {
	go func() {
		for t := range b.Transactions {
			b.AddTransaction(t)
		}
	}()
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
		// TODO: make slice when poll added
		b.Votes[t.VoteTx.Vote.PollID] = append(b.Votes[t.VoteTx.Vote.PollID], t.VoteTx)
		fmt.Printf("BLOCKCHAIN ADD votetx for %v\n", t.VoteTx.Vote.PollID)
	} else if t.RegisterTx != nil {
		b.Registry = append(b.Registry, t.RegisterTx)
		fmt.Printf("BLOCKCHAIN ADD registerTx for %v\n", t.RegisterTx.Registry.Origin)
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
