package blockchain

import (
	"bitbucket.org/ustraca/crypto/paillier"
	"fmt"
	. "github.com/lukasdeloose/decentralized-voting-system/project/utils"
	"time"
)

const numTxBeforeMine = 5
const numTxBeforeGossip = 1
const secondsPerBlock = 10 * time.Second

type Miner struct {
	blockchain       *Blockchain
	forkedBlockchain *Blockchain
	difficulty       int
	transActionsIn   chan Transactions
	blocksIn         chan *Block
	stopMining       chan uint32 // ID of block where to stop mining for
	fork             bool
}

func NewMiner(blockIn chan *Block) *Miner {
	return &Miner{
		blockchain:     NewBlockChain(),
		difficulty:     1,
		transActionsIn: make(chan Transactions),
		blocksIn:       blockIn,
		stopMining:     make(chan uint32, 10),
	}
}

func (miner Miner) Run() {
	go miner.listenTransactions()
	go miner.listenBlocks()
}

func (miner Miner) listenTransactions() {
	for tx := range miner.transActionsIn {
		miner.blockchain.addUnconfirmedTransactions(tx)
		numTrans := len(miner.blockchain.unconfirmedTransactions.Polls) + len(miner.blockchain.unconfirmedTransactions.Registers) + len(miner.blockchain.unconfirmedTransactions.Votes)
		if numTrans > numTxBeforeMine {
			miner.generateBlock()
		}
	}
}

func (miner Miner) handleFork(block *Block) {
	if block.ID == uint32(len(miner.blockchain.Blocks)) {
		// Next block
		if block.PrevHash == miner.blockchain.lastBlock().Hash {
			miner.stopMining <- block.ID
			miner.blockchain.Blocks = append(miner.blockchain.Blocks, block)
			miner.blockchain.addTransactions(block.Transactions)
			miner.blockchain.removeConfirmedTx(block.Transactions)
			return
		}
	}
	if block.ID == uint32(len(miner.forkedBlockchain.Blocks)) {
		if block.PrevHash == miner.forkedBlockchain.lastBlock().Hash {
			miner.forkedBlockchain.Blocks = append(miner.forkedBlockchain.Blocks, block)
			miner.forkedBlockchain.addTransactions(block.Transactions)
			miner.forkedBlockchain.removeConfirmedTx(block.Transactions)
			return
		}
	}
	if len(miner.forkedBlockchain.Blocks) > len(miner.blockchain.Blocks) {
		temp := miner.blockchain
		miner.blockchain = miner.forkedBlockchain
		miner.forkedBlockchain = temp
	}
	if len(miner.blockchain.Blocks)-len(miner.forkedBlockchain.Blocks) > 4 {
		miner.forkedBlockchain = nil
		miner.fork = false
	}
}

func (miner Miner) listenBlocks() {
	for block := range miner.blocksIn {
		if !miner.validBlock(block) {
			return
		}
		if miner.fork == true {
			miner.handleFork(block)
		}

		if block.ID == uint32(len(miner.blockchain.Blocks)) {
			// Next block
			if block.PrevHash == miner.blockchain.lastBlock().Hash {
				miner.stopMining <- block.ID
				miner.blockchain.Blocks = append(miner.blockchain.Blocks, block)
				//miner.blockchain.addTransactions(block.Transactions)
				miner.blockchain.removeConfirmedTx(block.Transactions)
				return
			}
		}
		if block.ID == uint32(len(miner.blockchain.Blocks))-1 {
			if block.PrevHash == miner.blockchain.Blocks[len(miner.blockchain.Blocks)-1].Hash {
				miner.fork = true
				miner.forkedBlockchain = miner.blockchain
				miner.forkedBlockchain.Blocks = append(miner.blockchain.Blocks, block)
				//miner.blockchain.addTransactions(block.Transactions)
				miner.forkedBlockchain.removeConfirmedTx(block.Transactions)
				return
			}
		}
	}
}

func (miner Miner) validBlock(block *Block) bool {
	if !hashesValid(block) {
		return false
	}
	if _, ok := miner.checkTransactions(block.Transactions); !ok {
		return false
	}
	return true
}

// Calculates the difficulty (amount of 0's necessary for the hashing problem) for the PoW algorithm
func (miner Miner) adaptDifficulty() {
	// TODO
}

// Take the current unconfirmed transactions and try to mine new block from these
func (miner Miner) generateBlock() {
	newBlock := Block{
		ID:             uint32(len(miner.blockchain.Blocks)),
		Timestamp:      time.Now(),
		PaillierPublic: paillier.PublicKey{},
		Difficulty:     miner.difficulty,
		PrevHash:       miner.blockchain.Blocks[len(miner.blockchain.Blocks)-1].Hash,
	}
	miner.checkTransactions(miner.blockchain.unconfirmedTransactions)

	// Start mining until block found, or received from other peer
	miner.mine(&newBlock)
}

// Checks if the transactions are valid.
// If all are valid, return same Transactions and True
// Remove invalid Transactions and return False otherwise
func (miner Miner) checkTransactions(transactions Transactions) (Transactions, bool) {
	valid := true
	i := 0
	for _, pollTx := range transactions.Polls {
		if !miner.blockchain.pollValid(pollTx) {
			valid = false
		} else {
			transactions.Polls[i] = pollTx
			i++
		}
	}
	transactions.Polls = transactions.Polls[:i]

	i = 0
	for _, voteTx := range transactions.Votes {
		if !miner.blockchain.voteValid(voteTx) {
			valid = false
		} else {
			transactions.Votes[i] = voteTx
			i++
		}
	}
	transactions.Votes = transactions.Votes[:i]

	i = 0
	for _, registerTx := range transactions.Registers {
		if !miner.blockchain.registerValid(registerTx) {
			valid = false
		} else {
			transactions.Registers[i] = registerTx
			i++
		}
	}
	transactions.Registers = transactions.Registers[:i]

	return transactions, valid
}

func (miner Miner) mine(newBlock *Block) Block {
	for i := 0; ; i++ {
		for index := range miner.stopMining {
			if index >= newBlock.ID {
				break
			}
		}
		hex := fmt.Sprintf("%x", i)
		newBlock.Nonce = hex
		if !hashValid(calculateHash(newBlock), newBlock.Difficulty) {
			fmt.Println(calculateHash(newBlock), " do more work!")
			time.Sleep(time.Second)
			continue
		} else {
			fmt.Println(calculateHash(newBlock), " work done!")
			newBlock.Hash = calculateHash(newBlock)
			break
		}
	}
	return *newBlock
}
