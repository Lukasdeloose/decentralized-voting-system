package blockchain

import (
	"crypto/rand"
	"crypto/rsa"
	"fmt"
	. "github.com/lukasdeloose/decentralized-voting-system/project/utils"
	"os"
	"sync"
	"testing"
	"time"
)


func TestBlockchain(t *testing.T) {
	if _, err := os.Stat("blocks.proto"); !os.IsNotExist(err) {
		os.Remove("blocks.proto")
	}

	chain := NewBlockChain()
	chain.Run()

	privKeys := make([]*rsa.PrivateKey, 0)
	for i := 0; i < 400; i++ {
		privKey, _ := rsa.GenerateKey(rand.Reader, 128)
		privKeys = append(privKeys, privKey)
	}

	for i := 0; i < 400; i++ {
		go func(i int) {
			chain.Transactions <- &Transaction{
				Origin:     fmt.Sprintf("Tester%v", i),
				ID:         1,
				RegisterTx: &RegisterTx{
					ID:       1,
					Registry: &Registry{
						Origin:    fmt.Sprintf("Tester%v", i),
						PublicKey: SerializableRSAPubKey{
							N: privKeys[i].N.Bytes(),
							E: privKeys[i].E,
						},
					},
				},
			}
		}(i)
	}

	time.Sleep(10 * time.Second)

	wg := sync.WaitGroup{}
	for i := 0; i < 400; i++ {
		wg.Add(1)
		go func(i int) {
			pubKey, exists := chain.RegistryKey(fmt.Sprintf("Tester%v", i))
			if !exists || pubKey.N.Cmp(privKeys[i].PublicKey.N) != 0 || pubKey.E != privKeys[i].PublicKey.E {
				t.Errorf("Tester%v could not be found in blockhain after registering\n", i)
			}
			wg.Done()
		}(i)
	}
	wg.Wait()

}


