package voting

import (
	"bitbucket.org/ustraca/crypto/paillier"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"fmt"
	"github.com/dedis/protobuf"
	. "github.com/lukasdeloose/decentralized-voting-system/project/blockchain"
	"github.com/lukasdeloose/decentralized-voting-system/project/constants"
	. "github.com/lukasdeloose/decentralized-voting-system/project/udp"
	. "github.com/lukasdeloose/decentralized-voting-system/project/utils"
	"math/big"
	"sync"
)

type VoteRumorer struct {
	name string
	nameHash [32]byte

	privateKey *rsa.PrivateKey

	polls map[uint32] *paillier.PrivateKey
	pollsMutex *sync.RWMutex

	uiIn chan *VotingMessage
	in chan *AddrGossipPacket
	publicOut chan *AddrGossipPacket

	blockchain *Blockchain
}

func NewVoteRumorer(name string, uiIn chan *VotingMessage, in chan *AddrGossipPacket, publicOut chan *AddrGossipPacket, blockchain *Blockchain) *VoteRumorer {
	return &VoteRumorer{
		name: name,
		nameHash: sha256.Sum256([]byte(name)),
		polls: make(map[uint32]*paillier.PrivateKey),
		pollsMutex: &sync.RWMutex{},
		uiIn: uiIn,
		in:   in,
		publicOut:  publicOut,
		blockchain: blockchain,
	}
}

func (v *VoteRumorer) Run() {
	v.registerName()

	go func(){
		for msg := range v.uiIn {
			if msg.NewVote != nil {
				go v.handleNewVote(msg.NewVote.Vote, msg.NewVote.Pollid)
			} else if msg.NewPoll != nil {
				go v.handleNewPoll(msg.NewPoll.Question, msg.NewPoll.Voters)
			} else if msg.CountRequest != nil {
				go v.countVotes(msg.CountRequest.Pollid)
			}
		}
	}()

	go func() {
		for packet := range v.in {
			if packet.Gossip.Transaction != nil {
				// Pass the transactions to the blockchain for storage
				go func() {
					v.blockchain.Transactions <- packet.Gossip.Transaction
				}()
			}
		}
	}()
}

func (v *VoteRumorer) Polls() []*PollTx {
	return v.blockchain.GetPolls()
}


func (v *VoteRumorer) countVotes(pollid uint32) {
	v.pollsMutex.Lock()
	privKey, exists := v.polls[pollid]
	if !exists {
		if constants.Debug {
			fmt.Printf("[DEBUG] You need the private key to count the votes")
		}
		return
	}

	// Get the votes for this poll from the blockchain
	// Remember that a vote is only registered on the blockchain if the
	// signature is checked, and if this user can actually vote, so we can
	// simply decrypt the votes, check if in (0, 1) and add them together
	votes := v.blockchain.RetrieveVotes(pollid)
	count := int64(0)
	for _, vote := range votes {
		// decrypt the vote with our private key
		voteBigInt := (&big.Int{}).SetBytes(vote.Vote)
		voteDecr := privKey.Decrypt(&paillier.Cypher{C: voteBigInt})
		if !voteDecr.IsInt64() || (voteDecr.Int64() != 0 && voteDecr.Int64() != 1){
			if constants.Debug {
				fmt.Printf("[DEBUG] Invalid vote! Will be ignored...\n")
			}
		} else {
			count += voteDecr.Int64()
		}
	}

	fmt.Printf("COUNTED VOTES FOR POLLID %v, COUNT: %v\n", pollid, count)

	// TODO Publish the votes by mongering them (with deadline at which votes were counted)
}


func (v *VoteRumorer) verifyPublishedVotes(pollid uint32) {
	// TODO
}


func (v *VoteRumorer) registerName() {
	privKey, _ := rsa.GenerateKey(rand.Reader, 128)

	registry := &Registry{
		Origin: v.name,
		PublicKey: SerializableRSAPubKey{
			N: privKey.N.Bytes(),
			E: privKey.E,
		},
	}

	tx := &Transaction{
		Origin:   v.name,
		ID:       0,
		RegisterTx: &RegisterTx{
			ID:       0,
			Registry: registry,
		},
	}

	// Let the public rumorer monger the transaction
	v.publicOut <- &AddrGossipPacket{
		Address: UDPAddr{},
		Gossip:  &GossipPacket{Transaction: tx},
	}

	fmt.Println("REGISTERED NAME AND PUBLIC KEY")

	// TODO WAIT FOR CONFIRMATION?
}


func (v *VoteRumorer) handleNewVote(vote bool, pollid uint32) {
	// Create a new transaction, this is mongerable
	votetx := v.createEncryptedVote(vote, pollid)
	if votetx== nil {
		return
	}

	tx := &Transaction {
		ID:   0,
		Origin: v.name,
		VoteTx: votetx,
	}

	// Let the public rumorer monger the transaction
	v.publicOut <- &AddrGossipPacket{
		Address: UDPAddr{},
		Gossip:  &GossipPacket{Transaction: tx},
	}

	fmt.Printf("VOTE %v FOR %v\n", vote, pollid)
}


func (v *VoteRumorer) handleNewPoll(question string, voters []string) {
	poll := v.createPoll(question, voters)
	if poll == nil {
		return
	}

	tx := &Transaction {
		ID: 0,
		Origin: v.name,
		PollTx: poll,
	}

	// Let the public rumorer monger the transaction
	v.publicOut <- &AddrGossipPacket{
		Address: UDPAddr{},
		Gossip:  &GossipPacket{Transaction: tx},
	}
	fmt.Printf("POLL: %v\n", question)
}


func (v *VoteRumorer) createEncryptedVote(vote bool, pollid uint32) *VoteTx {
	publicKey, exists := v.blockchain.PollKey(pollid)
	if !exists {
		if constants.Debug {
			fmt.Printf("[DEBUG] Public key for %v could not be found\n", pollid)
			return nil
		}
	}

	voteInt := big.NewInt(0)
	if vote{
		voteInt = big.NewInt(1)
	}

	voteCypher, _ := publicKey.Encrypt(voteInt, rand.Reader)

	encrVote := &EncryptedVote{
			Origin: v.name,
			PollID: 0,
			Vote:  voteCypher.C.Bytes(),
	}
	voteBytes, _ := protobuf.Encode(encrVote)

	if v.privateKey != nil {
		if constants.Debug {
			fmt.Printf("[DEBUG] First register your name and get a private key\n")
		}
		return nil
	}
	signature, _ := v.privateKey.Sign(rand.Reader, voteBytes, crypto.SHA256)

	return &VoteTx{
		ID:        0,
		Vote:      encrVote,
		Signature: signature,
	}
}


func (v *VoteRumorer) createPoll(question string, voters []string) *PollTx {
	// Create public private key pair for poll
	privKey := generateKey(128)
	poll := &Poll {
		Origin: v.name,
		Question: question,
		Voters:   voters,
		PublicKey:  SerializablePaillierPubKey{
			N: privKey.PublicKey.N.Bytes(),
			G: privKey.PublicKey.G.Bytes(),
		},
	}
	pollBytes, _ := protobuf.Encode(poll)

	if v.privateKey != nil {
		if constants.Debug {
			fmt.Printf("[DEBUG] First register your name and get a private key\n")
		}
		return nil
	}
	signature, _ := v.privateKey.Sign(rand.Reader, pollBytes, crypto.SHA256)

	return &PollTx{
		Poll:      poll,
		ID:        0,
		Signature: signature,
	}
}


func generateKey(bits int) *paillier.PrivateKey {
	var p, q *big.Int

	p, _ = rand.Prime(rand.Reader, bits/2)
	q, _ = rand.Prime(rand.Reader, bits/2)

	return paillier.CreatePrivateKey(p, q)
}
