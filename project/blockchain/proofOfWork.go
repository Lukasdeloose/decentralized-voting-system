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

func (block Block) toString() string {
	// TODO
}

func isHashValid(hash string, difficulty int) bool {
	prefix := strings.Repeat("0", difficulty)
	return strings.HasPrefix(hash, prefix)
}

func generateBlock(oldBlock Block, transactions Transactions) Block {
	var newBlock Block
	// TODO
	return newBlock
}
