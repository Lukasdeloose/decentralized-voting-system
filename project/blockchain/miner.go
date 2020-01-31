package blockchain

import (
	"bitbucket.org/ustraca/crypto/paillier"
	"fmt"
	"github.com/lukasdeloose/decentralized-voting-system/project/udp"
	. "github.com/lukasdeloose/decentralized-voting-system/project/utils"
	"time"
)

const numTxBeforeMine = 1
const numTxBeforeGossip = 1
const secondsPerBlock = 10 * time.Second
const initialDifficulty = 3

type Miner struct {
	blockchain       *Blockchain
	forkedBlockchain *Blockchain
	difficulty       int
	transActionsIn   chan *Transaction
	blocksIn         chan *Block
	blocksOut        chan *AddrGossipPacket
	stopMining       chan uint32 // ID of block where to stop mining for
	fork             bool
	mining           bool // To make sure that we don't start mining multiple times
	name             string
}

func NewMiner(name string, blockchain *Blockchain, transActionsIn chan *Transaction, blockIn chan *Block, blocksOut chan *AddrGossipPacket) *Miner {
	return &Miner{
		blockchain:     blockchain,
		difficulty:     initialDifficulty,
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
		miner.blockchain.addUnconfirmedTransaction(*tx)
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
		}
	} else if block.ID == uint32(len(miner.forkedBlockchain.Blocks)) {
		if block.PrevHash == miner.forkedBlockchain.lastBlock().Hash {
			miner.forkedBlockchain.Blocks = append(miner.forkedBlockchain.Blocks, block)
			miner.forkedBlockchain.addTransactions(block.Transactions)
			miner.forkedBlockchain.removeConfirmedTx(block.Transactions)
		}
	} else if len(miner.forkedBlockchain.Blocks) > len(miner.blockchain.Blocks) {
		temp := miner.blockchain
		miner.blockchain = miner.forkedBlockchain
		miner.forkedBlockchain = temp
	} else if len(miner.blockchain.Blocks)-len(miner.forkedBlockchain.Blocks) > 4 {
		miner.forkedBlockchain = nil
		miner.fork = false
	}
}

func (miner Miner) listenBlocks() {
	for block := range miner.blocksIn {
		fmt.Println("Received block from", block.Origin, "with id", block.ID, "in miner")
		if !miner.validBlock(block) {
			fmt.Println("Block not valid")
		} else if miner.fork == true {
			fmt.Println("handling fork")
			miner.handleFork(block)
		} else if block.ID == uint32(len(miner.blockchain.Blocks)) {
			fmt.Println("It's the next block")
			// Next block
			if block.PrevHash == miner.blockchain.lastBlock().Hash {
				fmt.Println("Hash is correct! Adding block")
				miner.stopMining <- block.ID
				fmt.Println("Transactions are:", block.Transactions)
				miner.blockchain.Blocks = append(miner.blockchain.Blocks, block)
				miner.blockchain.addTransactions(block.Transactions)
				miner.blockchain.removeConfirmedTx(block.Transactions)
			}
		} else if block.ID == uint32(len(miner.blockchain.Blocks))-1 {
			if block.PrevHash == miner.blockchain.Blocks[len(miner.blockchain.Blocks)-1].Hash {
				fmt.Println("Fork detected")
				miner.fork = true
				miner.forkedBlockchain = miner.blockchain
				miner.forkedBlockchain.Blocks = append(miner.blockchain.Blocks, block)
				miner.forkedBlockchain.addTransactions(block.Transactions)
				miner.forkedBlockchain.removeConfirmedTx(block.Transactions)
			}
		}
	}
}

func (miner Miner) validBlock(block *Block) bool {
	if !hashesValid(block) {
		fmt.Println("Invalid hashes")
		return false
	}
	if _, ok := miner.checkTransactions(block.Transactions); !ok {
		fmt.Println("Invalid transactions")
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
		Origin:         miner.name,
		PaillierPublic: paillier.PublicKey{},
		Difficulty:     miner.difficulty,
		PrevHash:       miner.blockchain.Blocks[len(miner.blockchain.Blocks)-1].Hash,
	}
	transactions, valid := miner.checkTransactions(miner.blockchain.unconfirmedTransactions)
	fmt.Println("Transactions are valid?", valid)
	newBlock.Transactions = transactions
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
			fmt.Println("Poll invalid")
		} else {
			transactions.Polls[i] = pollTx
			i++
		}
	}
	transactions.Polls = transactions.Polls[:i]

	i = 0
	for _, voteTx := range transactions.Votes {
		if !miner.blockchain.voteValid(voteTx) {
			fmt.Println("Invalid vote")
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
			fmt.Println("Invalid register")
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
				miner.mining = false
				fmt.Println("Setting mining to false")
				return nil
			}
		}
		hex := fmt.Sprintf("%x", i)
		newBlock.Nonce = hex
		if !hashValid(calculateHash(newBlock), newBlock.Difficulty) {
			fmt.Println(calculateHash(newBlock), " do more work!")
			//time.Sleep(time.Second / 10)
		} else {
			fmt.Println(calculateHash(newBlock), " work done!")
			newBlock.Hash = calculateHash(newBlock)
			newBlock.Timestamp = time.Now()
			miner.mining = false
			fmt.Println("Setting mining to false")
			break
		}
	}
	return newBlock
}
