package rumorer

import (
	. "github.com/lukasdeloose/decentralized-voting-system/project/constants"
	. "github.com/lukasdeloose/decentralized-voting-system/project/udp"
	. "github.com/lukasdeloose/decentralized-voting-system/project/utils"
	"log"
	"math/rand"
	"sync"

	"fmt"
	"time"
)

const ACKTIMEOUT = 2

type msgID struct {
	origin string
	id     uint32
}

type Rumorer struct {
	name  string
	peers *Set

	mongeringWith      map[UDPAddr]MongerableMessage
	mongeringWithMutex *sync.RWMutex

	// ID the next message created by this peer will get
	id      uint32
	idMutex *sync.RWMutex

	// The rumorer communicates through these channels
	in       chan *AddrGossipPacket
	out      chan *AddrGossipPacket
	localOut chan MongerableMessage
	uiIn     chan *Message

	// State of this peer, this contains the vector clock and messages
	state *State

	// Channels used to acknowledge a rumor
	ackChans      map[UDPAddr]map[msgID]chan bool
	ackChansMutex *sync.RWMutex

	// Timeout for waiting for ack for a rumor
	timeout time.Duration

	// Interval between anti-entropy runs
	antiEntropyTimout time.Duration
}

func NewRumorer(name string, peers *Set,
	in chan *AddrGossipPacket, out chan *AddrGossipPacket, localOut chan MongerableMessage, uiIn chan *Message, antiEntropy int) *Rumorer {

	return &Rumorer{
		name:               name,
		peers:              peers,
		mongeringWith:      make(map[UDPAddr]MongerableMessage),
		mongeringWithMutex: &sync.RWMutex{},
		id:                 1,
		idMutex:            &sync.RWMutex{},
		in:                 in,
		out:                out,
		localOut:           localOut,
		uiIn:               uiIn,
		state:              NewState(out),
		ackChans:           make(map[UDPAddr]map[msgID]chan bool),
		ackChansMutex:      &sync.RWMutex{},
		timeout:            time.Second * ACKTIMEOUT,
		antiEntropyTimout:  time.Second * time.Duration(antiEntropy),
	}
}

func (r *Rumorer) Name() string {
	return r.name
}

func (r *Rumorer) Messages() []*RumorMessage {
	return r.state.Messages()
}

func (r *Rumorer) Peers() []UDPAddr {
	return r.peers.Data()
}

func (r *Rumorer) AddPeer(peer UDPAddr) {
	r.peers.Add(peer)
}

func (r *Rumorer) UIIn() chan *Message {
	return r.uiIn
}

func (r *Rumorer) Run() {
	go r.runPeer()

	if r.antiEntropyTimout != 0 {
		go r.runAntiEntropy()
	}
}

func (r *Rumorer) runPeer() {
	// Wait for and process incoming packets from other peers
	for packet := range r.in {
		go func() {
			gossip := packet.Gossip
			address := packet.Address

			// Dispatch packet according to type
			mongerableMsg := gossip.ToMongerableMessage()

			if mongerableMsg != nil {
				// Expand peers list
				if address.String() != "" {
					r.peers.Add(address)
				} else {
					// The message is ours: if it does not have an ID yet: give it one
					if mongerableMsg.GetID() == 0 {
						r.idMutex.Lock()
						mongerableMsg.SetID(r.id)
						r.id += 1
						r.idMutex.Unlock()
					}
				}

				if gossip.Rumor != nil {
					// Print logging info
					r.printRumor(gossip.Rumor, address)
				} else if gossip.Transaction != nil {
					r.printTx(gossip.Transaction)
				} else if gossip.MongerableBlock != nil {
					r.printBlock(gossip.MongerableBlock)
				}

				// Handle the message
				r.handleRumor(mongerableMsg, address, false)

			} else if gossip.Status != nil {
				// Expand peers list
				r.peers.Add(address)

				// Print logging info
				r.printStatus(gossip.Status, address)

				// Handle the message
				r.handleStatus(gossip.Status, address)

			} // Ignore SimpleMessage
		}()
	}
}

func (r *Rumorer) runAntiEntropy() {
	for {
		// Run anti-entropy every `antiEntropyTimout` seconds
		go func() {
			if Debug {
				fmt.Printf("[DEBUG] running antientropy\n")
			}
			// Send StatusPacket to a random peer
			randPeer, ok := r.peers.Rand()
			if ok {
				r.state.Send(randPeer)
			}
		}()

		timer := time.NewTicker(r.antiEntropyTimout)
		<-timer.C
	}
}

func (r *Rumorer) startMongering(msg MongerableMessage, except UDPAddr, coinFlip bool) {
	if coinFlip {
		// Flip a coin: heads -> don't start mongering
		if rand.Int()%2 == 0 {
			if Debug {
				fmt.Println("[DEBUG] FLIPPED COIN: nope")
			}
			return
		}
	}

	ok, first := false, true
	for !ok {
		// Select random peer
		randPeer, okRand := r.peers.RandExcept(except)
		if okRand {
			if coinFlip && first { // Only print FLIPPED COIN the first try, the coin was only flipped once...
				if HW1 || Debug {
					fmt.Printf("FLIPPED COIN sending rumor to %v\n", randPeer)
				}
				first = false
			}

			// Start mongering with this peer
			r.mongeringWithMutex.Lock()
			r.mongeringWith[randPeer] = msg
			if HW1 {
				fmt.Printf("MONGERING with %v\n", randPeer)
			}
			r.mongeringWithMutex.Unlock()

			ok = r.sendRumorWait(msg, randPeer)
			if !ok {
				// Not mongering with this peer so delete it from the set
				r.mongeringWithMutex.Lock()
				delete(r.mongeringWith, randPeer)
				r.mongeringWithMutex.Unlock()
			}
		} else {
			// No peers to select from: simply don't monger
			return
		}
	}
}

func (r *Rumorer) handleRumor(msg MongerableMessage, sender UDPAddr, forceResend bool) {
	// Update peer state, and check if the message was a message we were looking for
	newMsgs := r.state.Update(msg)

	// Dispatch (async) the newMsgs for processing
	go func() {
		for _, msg := range newMsgs {
			if Debug {
				fmt.Printf("[DEBUG] DISPATCHING MONGERABLE ID %v ORIGIN %v\n", msg.GetID(), msg.GetOrigin())
			}
			r.localOut <- msg
		}
	}()

	// If the message didn't come from the client: acknowledge the message
	if sender.String() != "" {
		r.state.Send(sender)
	}

	if len(newMsgs) > 0 || forceResend {
		if Debug && len(newMsgs) > 0 {
			fmt.Printf("[DEBUG] MONGERABLE MESSAGE accepted\n")
		}
		if Debug && forceResend {
			fmt.Printf("[DEBUG] RESENDING MONGERABLE MESSAGE\n")
		}

		// Start mongering the message
		r.startMongering(msg, sender, false) // except sender
	}
}

func (r *Rumorer) handleStatus(msg *StatusPacket, sender UDPAddr) {
	// Check if a rumor is waiting to be acknowledged
	r.ackChansMutex.RLock()
	if _, exists := r.ackChans[sender]; exists {
		for msgid, c := range r.ackChans[sender] {
			// Check if this msgid is acknowledged by msg
			for _, status := range msg.Want {
				if status.Identifier == msgid.origin && msgid.id < status.NextID {
					c <- true
				}
			}
		}
	}
	r.ackChansMutex.RUnlock()

	// Compare the received state to our state
	iHave, youHave := r.state.Compare(msg)

	if iHave != nil {
		// I have a message he wants: send this message
		toSend := r.state.Message(iHave.Identifier, iHave.NextID)
		if HW1 || HW2 {
			// We're actually not, but this is needed as output to indicate that we sent a rumor message
			fmt.Printf("MONGERING with %v\n", sender)
		}
		if toSend == nil {
			log.Fatalf("TOSEND IS NIL for ID %v ORIGIN %v MYORIGIN %v\n"+
				"NEXTID: %v, MSGS: %v\n", iHave.NextID, iHave.Identifier, r.name, r.state.state[iHave.Identifier], r.state.messages[iHave.Identifier])
		}
		r.send(toSend.ToGossip(), sender)

	} else if youHave != nil {
		// He has a message I need: request it by sending my state
		r.state.Send(sender)
	} else {
		// We are in sync
		if HW1 {
			fmt.Printf("IN SYNC WITH %v\n", sender)
		}

		// Check if we were mongering with a peer
		r.mongeringWithMutex.Lock()
		if rumor, exists := r.mongeringWith[sender]; exists {
			delete(r.mongeringWith, sender)
			r.mongeringWithMutex.Unlock()
			// If we we're at one point mongering with this peer: flip a coin to start mongering again
			r.startMongering(rumor, sender, true)
		} else {
			r.mongeringWithMutex.Unlock()
		}
	}
}

func (r *Rumorer) sendRumorWait(msg MongerableMessage, to UDPAddr) bool {
	// Create ack channel
	r.ackChansMutex.Lock()
	if _, exists := r.ackChans[to]; !exists {
		r.ackChans[to] = make(map[msgID]chan bool)
	}
	ackChan := make(chan bool, 64)
	r.ackChans[to][msgID{msg.GetOrigin(), msg.GetID()}] = ackChan
	r.ackChansMutex.Unlock()

	// send rumor to peer
	r.send(msg.ToGossip(), to)

	// start timer for timeout on ack
	timer := time.NewTicker(r.timeout)
	defer timer.Stop()

	select {
	case <-timer.C:
		if Debug {
			fmt.Printf("[DEBUG] Timeout when waiting for status\n")
		}
		// Timed out
		// Delete ack channel
		r.ackChansMutex.Lock()
		delete(r.ackChans[to], msgID{msg.GetOrigin(), msg.GetID()})
		r.ackChansMutex.Unlock()
		return false

	case <-ackChan:
		if Debug {
			fmt.Printf("[DEBUG] Packet confirmed\n")
		}
		// Status received
		// Delete ack channel
		r.ackChansMutex.Lock()
		delete(r.ackChans[to], msgID{msg.GetOrigin(), msg.GetID()})
		r.ackChansMutex.Unlock()
		return true
	}
}

func (r *Rumorer) send(packet *GossipPacket, addr UDPAddr) {
	// Send gossip packet to addr
	r.out <- &AddrGossipPacket{addr, packet}
}

func (r *Rumorer) printStatus(msg *StatusPacket, address UDPAddr) {
	if HW1 {
		toPrint := ""
		toPrint += fmt.Sprintf("STATUS from %v ", address)
		for _, entry := range msg.Want {
			toPrint += fmt.Sprintf("peer %v nextID %v ", entry.Identifier, entry.NextID)
		}
		toPrint += fmt.Sprintf("\n")
		toPrint += fmt.Sprintf("PEERS %v\n", r.peers)
		fmt.Printf(toPrint)
	}
}

func (r *Rumorer) printRumor(msg *RumorMessage, address UDPAddr) {
	if address.String() == "" {
		fmt.Printf("CLIENT MESSAGE %v\n", msg.Text)
	} else {
		if msg.Text != "" && (HW1 || HW2) {
			fmt.Printf("RUMOR origin %v from %v ID %v contents %v\n",
				msg.Origin, address, msg.ID, msg.Text)
		}
	}

	if msg.Text != "" && HW1 {
		fmt.Printf("PEERS %v\n", r.peers)
	}
}

func (r *Rumorer) printTx(t *Transaction) {
	if t.PollTx != nil {
		fmt.Printf("POLL TRANSACTION ID %v ORIGIN %v\n", t.ID, t.Origin)
	} else if t.VoteTx != nil {
		fmt.Printf("VOTE TRANSACTION ID %v ORIGIN %v\n", t.ID, t.Origin)
	} else if t.RegisterTx != nil {
		fmt.Printf("REGISTER TRANSACTION ID %v ORIGIN %v\n", t.ID, t.Origin)
	}
}

func (r *Rumorer) printBlock(b *MongerableBlock) {
	fmt.Printf("NEW BLOCK RECEIVED FROM %v with ID=%v\n", b.Origin, b.Block.ID)
}
