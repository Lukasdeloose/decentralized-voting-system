package gossiper

import (
	"fmt"
	"github.com/dedis/protobuf"
	. "github.com/lukasdeloose/decentralized-voting-system/project/udp"
	. "github.com/lukasdeloose/decentralized-voting-system/project/utils"
	"log"
)

type Dispatcher struct {
	name string

	// To retrieve messages that have to be dispatched to different components of the program
	// and send messages coming from the components
	UIServer     *Server
	GossipServer *Server

	// To dispatch to the 'public' rumorer
	RumorerGossipIn chan *AddrGossipPacket
	RumorerUIIn     chan *Message
	// To dispatch from the 'public' rumorer
	RumorerOut chan *AddrGossipPacket
	// To dispatch public messages that have to be processed
	// by other parts of the application
	RumorerLocalOut chan MongerableMessage

	// To dispatch to the 'private' rumorer
	PrivateRumorerGossipIn chan *AddrGossipPacket
	PrivateRumorerUIIn     chan *Message
	// To dispatch from the 'private' rumorer
	PrivateRumorerGossipOut chan *AddrGossipPacket
	// P2P reply that are for the local node and should be handled
	// by other parts of the gossper
	PrivateRumorerLocalOut chan *AddrGossipPacket

	VoteRumorerUIIn chan *VotingMessage
	VoteRumorerIn   chan *AddrGossipPacket

	TransactionRumorerIn chan *Transaction

	BlockRumorerIn  chan *Block
	BlockRumorerOut chan *Block
}

func NewDispatcher(name string, uiPort string, gossipAddr string) *Dispatcher {
	return &Dispatcher{
		name:         name,
		UIServer:     NewServer("127.0.0.1:" + uiPort),
		GossipServer: NewServer(gossipAddr),

		RumorerGossipIn: make(chan *AddrGossipPacket),
		RumorerUIIn:     make(chan *Message),
		RumorerLocalOut: make(chan MongerableMessage),
		RumorerOut:      make(chan *AddrGossipPacket),

		PrivateRumorerGossipIn:  make(chan *AddrGossipPacket),
		PrivateRumorerUIIn:      make(chan *Message),
		PrivateRumorerGossipOut: make(chan *AddrGossipPacket),
		PrivateRumorerLocalOut:  make(chan *AddrGossipPacket),

		VoteRumorerUIIn: make(chan *VotingMessage),
		VoteRumorerIn:   make(chan *AddrGossipPacket),

		BlockRumorerIn:  make(chan *Block),
		BlockRumorerOut: make(chan *Block),

		TransactionRumorerIn: make(chan *Transaction),
	}
}

func (d *Dispatcher) Run() {
	d.UIServer.Run()
	d.GossipServer.Run()

	go func() {
		for pack := range d.UIServer.Ingress() {
			// Decode the packet
			msg := Message{}
			err := protobuf.Decode(pack.Data, &msg)
			if err != nil {
				panic(fmt.Sprintf("ERROR when decoding packet: %v", err))
			}

			// Dispatch client message
			d.dispatchFromClient(&msg)
		}
	}()

	go func() {
		for raw := range d.GossipServer.Ingress() {
			// Decode the packet
			packet := GossipPacket{}
			err := protobuf.Decode(raw.Data, &packet)
			if err != nil {
				panic(fmt.Sprintf("ERROR when decoding packet: %v", err))
			}

			// Dispatch gossip
			d.dispatchFromPeer(&AddrGossipPacket{raw.Addr, &packet})

		}
	}()

	go func() {
		for range d.PrivateRumorerLocalOut {
			// Process private messages for different parts of the application
		}
	}()

	go func() {
		for mongerable := range d.RumorerLocalOut {
			// Process public messages for different parts of the application
			if mongerable.ToGossip().Transaction != nil {
				d.VoteRumorerIn <- &AddrGossipPacket{Gossip: mongerable.ToGossip()}
				d.TransactionRumorerIn <- mongerable.ToGossip().Transaction
			} else if mongerable.ToGossip().Block != nil {
				d.BlockRumorerIn <- mongerable.ToGossip().Block
			}
		}
	}()

	go func() {
		for packet := range d.RumorerOut {
			bytes, err := protobuf.Encode(packet.Gossip)
			if err != nil {
				log.Fatalf("ERROR could not encode packet: %v", err)
			}
			d.GossipServer.Outgress() <- &RawPacket{packet.Address, bytes}
		}
	}()

	go func() {
		for packet := range d.PrivateRumorerGossipOut {
			bytes, err := protobuf.Encode(packet.Gossip)
			if err != nil {
				log.Fatalf("ERROR could not encode packet: %v", err)
			}
			d.GossipServer.Outgress() <- &RawPacket{packet.Address, bytes}
		}
	}()
}

func (d *Dispatcher) dispatchFromPeer(gossip *AddrGossipPacket) {
	if gossip.Gossip.ToMongerableMessage() != nil {
		d.RumorerGossipIn <- gossip
	}

	if gossip.Gossip.Rumor != nil {
		d.PrivateRumorerGossipIn <- gossip
	}

	if gossip.Gossip.Status != nil {
		d.RumorerGossipIn <- gossip
	}

	if gossip.Gossip.Private != nil {
		d.PrivateRumorerGossipIn <- gossip
	}
}

func (d *Dispatcher) dispatchFromClient(msg *Message) {
	if msg.Text != "" {
		if msg.Destination == nil {
			d.RumorerGossipIn <- &AddrGossipPacket{
				Address: UDPAddr{},
				Gossip: &GossipPacket{Rumor: &RumorMessage{
					Origin: d.name,
					ID:     0,
					Text:   msg.Text,
				}},
			}

		} else {
			d.PrivateRumorerUIIn <- msg
		}
	}

	if msg.Voting != nil {
		d.VoteRumorerUIIn <- msg.Voting
	}
}
