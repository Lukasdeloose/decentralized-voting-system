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
	transactions, valid := miner.checkTransactionsCreate(miner.blockchain.unconfirmedTransactions)
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
	id := miner.blockchain.nextPollId
	for _, pollTx := range transactions.Polls {
		if !miner.blockchain.pollValid(pollTx, id) {
			valid = false
			fmt.Println("Poll invalid")
		} else {
			id++
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

func (miner Miner) checkTransactionsCreate(transactions Transactions) (Transactions, bool) {
	valid := true
	i := 0
	nextPollId := miner.blockchain.nextPollId
	for _, pollTx := range transactions.Polls {
		if !miner.blockchain.pollValid(pollTx, 0) {
			valid = false
			fmt.Println("Poll invalid")
		} else {
			pollTx.ID = nextPollId
			nextPollId++
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

/*
func (m *Miner) verifyPollTransaction(tx *PollTx) bool {
	// Verify the signature
	pubKey := m.blockchain.GetPublicKey(tx.Poll.Origin)
	if pubKey == nil {
		fmt.Printf("INVALIDE POLLTX: cannot find origin\n")
		return false
	}

	pollBytes, _ := protobuf.Encode(tx.Poll)
	hash := sha256.Sum256(pollBytes)
	err := rsa.VerifyPSS(pubKey, crypto.SHA256, hash[:], tx.Signature, nil)
	if err != nil {
		fmt.Printf("INVALID POLLTX: invalid signature\n")
		return false
	}

	for _, poll :=  range m.blockchain.Polls {
		if poll.ID == tx.ID {
			fmt.Printf("INVALID POLLTX: id already exists\n") // TODO +1 check?
			return false
		}
	}
	return true
}


func (m *Miner) verifyVoteTransaction(tx *VoteTx) bool {
	pubKey := m.blockchain.GetPublicKey(tx.Vote.Origin)
	if pubKey == nil {
		fmt.Printf("INVALIDE VOTETX: cannot find origin\n")
		return false
	}

	pollBytes, _ := protobuf.Encode(tx.Vote)
	hash := sha256.Sum256(pollBytes)
	err := rsa.VerifyPSS(pubKey, crypto.SHA256, hash[:], tx.Signature, nil)
	if err != nil {
		fmt.Printf("INVALID VOTETX: invalid signature\n")
		return false
	}

	for _, votes :=  range m.blockchain.Votes {
		for _, vote := range votes {
			if vote.ID == tx.ID {
				fmt.Printf("INVALID VOTETX: id already exists\n") // TODO +1 check?
				return false
			}
		}
	}

	var poll *PollTx
	for _, p := range m.blockchain.Polls {
		if p.ID == tx.Vote.PollID {
			poll = p
			break
		}
	}
	if poll == nil {
		fmt.Printf("INVALID VOTETX: poll does not exist\n") // TODO necessary?
		return false
	}

	allowed := false
	for _, voter := range poll.Poll.Voters {
		if voter == m.name {
			allowed = true
			break
		}
	}
	if !allowed {
		fmt.Printf("INVALID VOTETX: you are not allowed to vote\n")
		return false
	}

	return true
}

func (m *Miner) verifyRegistration(reg *RegisterTx) bool {
	exists := false
	for _, r := range m.blockchain.Registry {
		if r.ID == reg.ID {
			exists = true
			break
		}
	}
	if exists {
		fmt.Printf("INVALIDE REGISTERTX: id already exists\n")
		return false
	}

	originExists := false
	for _, r := range m.blockchain.Registry {
		if r.Registry.Origin == reg.Registry.Origin {
			originExists = true
			break
		}
	}
	if originExists {
		fmt.Printf("INVALID REGISTERTX: origin already exists\n")
		return false
	}

	return true
}

func (m *Miner) verifyResults(res *ResultTx) bool {
	exists := false
	for _, r := range m.blockchain.Results {
		if r.ID == res.ID {
			exists = true
			break
		}
	}
	if exists {
		fmt.Printf("INVALID RESULT: id already exists\n")
		return false
	}

	poll := m.blockchain.GetPoll(res.Result.PollId)
	if poll == nil {
		fmt.Printf("INVALID RESULT: poll does not exist\n")
		return false
	}

	votes := m.blockchain.GetVotes(res.Result.PollId)
	ourCount := &big.Int{}
	for _, vote := range votes {
		ourCount = big.Int{}.Add(ourCount, big.Int{}.SetBytes(vote.Vote.Vote))
	}

	// TODO: this verification always returns false, skip it...
	return true

	encrCount, err := poll.Poll.PublicKey.ToPaillier().Encrypt(big.NewInt(res.Result.Count), rand.Reader)
	if err != nil {
		fmt.Printf("INVALID RESULT: could not encrypt votes\n")
		return false
	}
	if ourCount.Cmp(encrCount.C) != 0 {
		fmt.Printf("INVALID RESULT: count is incorrect")
		return false
	}

	return true

}*/
