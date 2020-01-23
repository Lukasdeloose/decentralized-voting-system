package blockchain

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

func (block Block) calculateHash() string {
	record := block.toString()
	h := sha256.New()
	h.Write([]byte(record))
	hashed := h.Sum(nil)
	return hex.EncodeToString(hashed)
}

// When receiving a new block from another peer, this function checks if it is valid:
// - Transactions inside are valid (vote hasn't been cast yet, user hasn't registered yet,...)
// - Hash is correct and starts with necessary amount of zeros
func (block Block) isValid() bool {
	// TODO
	return true
}

// Convert the fields of the block to a string representation, allowing us to hash it
func (block Block) toString() string {
	// TODO
	return ""
}

// Hash starts with necessary amount of 0's
func isHashValid(hash string, difficulty int) bool {
	prefix := strings.Repeat("0", difficulty)
	return strings.HasPrefix(hash, prefix)
}

// Take the current unconfirmed transactions and try to mine new block from these
func generateBlock(oldBlock Block, transactions Transactions) Block {
	var newBlock Block
	// TODO, add transactions to block
	newBlock = mine(newBlock)
	return newBlock
}

func mine(newBlock Block) Block {
	for i := 0; ; i++ {
		hex := fmt.Sprintf("%x", i)
		newBlock.Nonce = hex
		if !isHashValid(newBlock.calculateHash(), newBlock.Difficulty) {
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
