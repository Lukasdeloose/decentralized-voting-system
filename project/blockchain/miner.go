package blockchain

import (
	"bitbucket.org/ustraca/crypto/paillier"
	"fmt"
	"sync"
	"time"
)

const numTxBeforeMine = 5
const numTxBeforeGossip = 1

type Miner struct {
	blockchain       []Block
	difficulty       int
	newTransactions  Transactions // unconfirmed transactions
	transactionsLock sync.RWMutex
	transActionsIn   chan Transactions
	blocksIn         chan Block
	stopMining       chan int // ID of block where to stop mining for
}

func newMiner() Miner {
	return Miner{
		blockchain:       make([]Block, 0),
		difficulty:       1,
		newTransactions:  Transactions{},
		transactionsLock: sync.RWMutex{},
		transActionsIn:   make(chan Transactions),
		blocksIn:         make(chan Block),
		stopMining:       make(chan int, 10),
	}
}

func (miner Miner) Run() {
	go miner.listenTransactions()
	go miner.listenBlocks()
}

func (miner Miner) listenTransactions() {
	for tx := range miner.transActionsIn {
		miner.transactionsLock.Lock()
		if tx.Polls != nil {
			miner.newTransactions.Polls = append(miner.newTransactions.Polls, tx.Polls...)
		}
		if tx.Votes != nil {
			miner.newTransactions.Votes = append(miner.newTransactions.Votes, tx.Votes...)
		}
		if tx.Registers != nil {
			miner.newTransactions.Registers = append(miner.newTransactions.Registers, tx.Registers...)
		}
		numTrans := len(miner.newTransactions.Polls) + len(miner.newTransactions.Registers) + len(miner.newTransactions.Votes)
		if numTrans > numTxBeforeMine {
			miner.generateBlock()
		}
	}
}

func (miner Miner) listenBlocks() {
	for block := range miner.blocksIn {
		if miner.nextValidBlock(block) {
			miner.stopMining <- block.Index
			miner.blockchain = append(miner.blockchain, block)
			miner.removeConfirmedTx(block.Transactions)
		}
	}
}

func (miner Miner) nextValidBlock(block Block) bool {
	if !block.isValid() {
		return false
	}
	if _, ok := miner.checkTransactions(block.Transactions); !ok {
		return false
	}
	return true
}

func (miner Miner) removeConfirmedTx(tx Transactions) {
	// Keep only votes that are not confirmed
	newVotes := miner.newTransactions.Votes[:0]
	for _, unconfirmedVote := range miner.newTransactions.Votes {
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

	// Polls
	newPolls := miner.newTransactions.Polls[:0]
	for _, unconfirmedPoll := range miner.newTransactions.Polls {
		found := false
		for _, confirmedPoll := range tx.Polls {
			if confirmedPoll.Poll.isEqual(unconfirmedPoll.Poll) {
				found = true
			}
		}
		if !found {
			newPolls = append(newPolls, unconfirmedPoll)
		}
	}

	// TODO: registers
}

// Calculates the difficulty (amount of 0's necessary for the hashing problem) for the PoW algorithm
func (miner Miner) calculateDifficulty() int {
	// TODO
	return 2
}

// Take the current unconfirmed transactions and try to mine new block from these
func (miner Miner) generateBlock() {
	newBlock := Block{
		Index:          len(miner.blockchain),
		Timestamp:      time.Now(),
		PaillierPublic: paillier.PublicKey{},
		Difficulty:     miner.calculateDifficulty(),
		PrevHash:       miner.blockchain[len(miner.blockchain)-1].Hash,
	}
	miner.checkTransactions(miner.newTransactions)

	// Start mining until block found, or received from other peer
	miner.mine(newBlock)
}

// Checks if the transactions are valid.
// If all are valid, return same Transactions and True
// Remove invalid Transactions and return False otherwise
func (miner Miner) checkTransactions(transactions Transactions) (Transactions, bool) {
	// TODO: check votes
	// TODO: check polls
	// TODO: check registers
	return transactions, true
}

func (miner Miner) mine(newBlock Block) Block {
	for i := 0; ; i++ {
		for index := range miner.stopMining {
			if index >= newBlock.Index {
				break
			}
		}
		hex := fmt.Sprintf("%x", i)
		newBlock.Nonce = hex
		if !hashValid(newBlock.calculateHash(), newBlock.Difficulty) {
			fmt.Println(newBlock.calculateHash(), " do more work!")
			time.Sleep(time.Second)
			continue
		} else {
			fmt.Println(newBlock.calculateHash(), " work done!")
			newBlock.Hash = newBlock.calculateHash()
			break
		}
	}
	return newBlock
}
