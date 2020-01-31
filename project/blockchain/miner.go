package blockchain

import (
	"bitbucket.org/ustraca/crypto/paillier"
	"fmt"
	"github.com/lukasdeloose/decentralized-voting-system/project/udp"
	. "github.com/lukasdeloose/decentralized-voting-system/project/utils"
	"time"
)

const numTxBeforeMine = 2
const numTxBeforeGossip = 1
const secondsPerBlock = 10 * time.Second

type Miner struct {
	blockchain       *Blockchain
	forkedBlockchain *Blockchain
	difficulty       int
	transActionsIn   chan *Transaction
	blocksIn         chan *Block
	blocksOut        chan *AddrGossipPacket
	stopMining       chan uint32 // ID of block where to stop mining for
	fork             bool
	mining           bool
	name             string
}

func NewMiner(name string, blockchain *Blockchain, transActionsIn chan *Transaction, blockIn chan *Block, blocksOut chan *AddrGossipPacket) *Miner {
	return &Miner{
		blockchain:     blockchain,
		difficulty:     1,
		transActionsIn: transActionsIn,
		blocksIn:       blockIn,
		blocksOut:      blocksOut,
		stopMining:     make(chan uint32, 10),
		mining:         false,
		name:           name,
	}
}

func (miner Miner) Run() {
	go miner.listenTransactions()
	go miner.listenBlocks()
}

func (miner Miner) listenTransactions() {
	for tx := range miner.transActionsIn {
		fmt.Println("transaction arrived in miner")
		miner.blockchain.addUnconfirmedTransaction(*tx)
		numTrans := len(miner.blockchain.unconfirmedTransactions.Polls) + len(miner.blockchain.unconfirmedTransactions.Registers) + len(miner.blockchain.unconfirmedTransactions.Votes)
		if numTrans > numTxBeforeMine && !miner.mining {
			miner.mining = true
			fmt.Println("Generating block ")
			go miner.generateBlock()
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
	prevTime := miner.blockchain.Blocks[miner.blockchain.length()-10].Timestamp
	timeDiff := time.Now().Sub(prevTime)
	if timeDiff/10 < 10*secondsPerBlock {
		miner.difficulty++
	}
}

// Take the current unconfirmed transactions and try to mine new block from these
func (miner Miner) generateBlock() {
	newBlock := &Block{
		ID:             uint32(len(miner.blockchain.Blocks)),
		Timestamp:      time.Now(),
		PaillierPublic: paillier.PublicKey{},
		Difficulty:     miner.difficulty,
		PrevHash:       miner.blockchain.Blocks[len(miner.blockchain.Blocks)-1].Hash,
	}
	miner.checkTransactions(miner.blockchain.unconfirmedTransactions)

	// Start mining until block found, or received from other peer
	newBlock = miner.mine(newBlock)
	if newBlock == nil {
		fmt.Println("Stopped mining, other person found block")
		return
	}
	miner.blocksOut <- &AddrGossipPacket{
		Address: udp.UDPAddr{},
		Gossip: &GossipPacket{
			MongerableBlock: &MongerableBlock{
				Origin: miner.name,
				ID:     0,
				Block:  newBlock,
			},
		},
	}
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

func (miner Miner) mine(newBlock *Block) *Block {
	for i := 0; ; i++ {
		for len(miner.stopMining) > 0 {
			if <-miner.stopMining >= newBlock.ID {
				return nil
			}
		}
		hex := fmt.Sprintf("%x", i)
		newBlock.Nonce = hex
		if !hashValid(calculateHash(newBlock), newBlock.Difficulty) {
			fmt.Println(calculateHash(newBlock), " do more work!")
		} else {
			fmt.Println(calculateHash(newBlock), " work done!")
			newBlock.Hash = calculateHash(newBlock)
			miner.mining = false
			break
		}
	}
	return newBlock
}
