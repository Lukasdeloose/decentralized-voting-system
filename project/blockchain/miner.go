package blockchain

import (
	"bitbucket.org/ustraca/crypto/paillier"
	"fmt"
	"time"
)

type Miner struct {
	blockchain []Block
	difficulty int
 	newTransactions Transactions // unconfirmed transactions
}

// Calculates the difficulty (amount of 0's necessary for the hashing problem) for the PoW algorithm
func (miner Miner) calculateDifficulty() int {
	// TODO
	return 2
}

// Take the current unconfirmed transactions and try to mine new block from these
func (miner Miner) generateBlock() Block {
	newBlock := Block{
		Index:          len(miner.blockchain),
		Timestamp:      time.Now(),
		PaillierPublic: paillier.PublicKey{},
		Difficulty:     miner.calculateDifficulty(),
		PrevHash:       miner.blockchain[len(miner.blockchain)-1].Hash,
	}
	miner.checkTransactions(miner.newTransactions)
	newBlock = mine(newBlock)
	return newBlock
}

// Checks if the transactions are valid.
// If all are valid, return same Transactions and True
// Remove invalid Transactions and return False otherwise
func (miner Miner) checkTransactions (transactions Transactions) (Transactions, bool) {
	return transactions, true
}

func mine(newBlock Block) Block {
	for i := 0; ; i++ {
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
