package blockchain

import (
	"bitbucket.org/ustraca/crypto/paillier"
	"crypto/rsa"
	"fmt"
	. "github.com/lukasdeloose/decentralized-voting-system/project/utils"
	"sync"
)

// DUMMY Blockchain implementation

type id struct {
	Origin string
	Id uint32
}

type Blockchain struct {
	Transactions chan *Transaction

	Registry map[id]*RegisterTx
	Votes map[id]*VoteTx
	Polls map[id]*PollTx
	mutex *sync.RWMutex
}


func NewBlockChain() *Blockchain {
	return &Blockchain{
		Transactions: make(chan *Transaction),
		Registry: make(map[id]*RegisterTx),
		Votes: make(map[id]*VoteTx),
		Polls: make(map[id]*PollTx),
		mutex: &sync.RWMutex{},
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
		b.Polls[id{t.Origin, t.ID}] = t.PollTx
		fmt.Printf("BLOCKCHAIN ADD polltx %v %v\n", t.PollTx.ID, t.PollTx.Poll.Question)
	} else if t.VoteTx != nil {
		b.Votes[id{t.Origin, t.ID}] = t.VoteTx
		fmt.Printf("BLOCKCHAIN ADD votetx for %v\n", t.VoteTx.Vote.PollID)
	} else if t.RegisterTx != nil {
		b.Registry[id{t.Origin, t.ID}] = t.RegisterTx
		fmt.Printf("BLOCKCHAIN ADD registerTx for %v\n", t.RegisterTx.Registry.Origin)
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
	for _, vote := range b.Votes {
		if vote.Vote.PollID == pollid {
			votes = append(votes, vote.Vote)
		}
	}

	return votes
}
