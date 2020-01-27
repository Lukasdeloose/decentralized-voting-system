package blockchain

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
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
func hashValid(hash string, difficulty int) bool {
	prefix := strings.Repeat("0", difficulty)
	return strings.HasPrefix(hash, prefix)
}



