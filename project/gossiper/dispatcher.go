package gossiper

import (
	"fmt"
	"github.com/dedis/protobuf"
	. "github.com/lukasdeloose/decentralized-voting-system/project/udp"
	. "github.com/lukasdeloose/decentralized-voting-system/project/utils"
	"log"
)

type Dispatcher struct {
	// To retrieve messages that have to be dispatched to different components of the program
	// and send messages coming from the components
	UIServer     *Server
	GossipServer *Server

	// To dispatch to the 'public' rumorer
	RumorerGossipIn chan *AddrGossipPacket
	RumorerUIIn     chan *Message
	// To dispatch from the 'public' rumorer
	RumorerOut chan *AddrGossipPacket

	// To dispatch to the 'private' rumorer
	PrivateRumorerGossipIn chan *AddrGossipPacket
	PrivateRumorerUIIn     chan *Message
	// To dispatch from the 'private' rumorer
	PrivateRumorerGossipOut chan *AddrGossipPacket
	// P2P reply that are for the local node and should be handled
	// by other parts of the gossper
	PrivateRumorerLocalOut chan *AddrGossipPacket
}

func NewDispatcher(uiPort string, gossipAddr string) *Dispatcher {
	return &Dispatcher{
		UIServer:     NewServer("127.0.0.1:" + uiPort),
		GossipServer: NewServer(gossipAddr),

		RumorerGossipIn: make(chan *AddrGossipPacket, 1024),
		RumorerUIIn:     make(chan *Message, 1024),
		RumorerOut:      make(chan *AddrGossipPacket, 1024),

		PrivateRumorerGossipIn:  make(chan *AddrGossipPacket, 1024),
		PrivateRumorerUIIn:      make(chan *Message, 1024),
		PrivateRumorerGossipOut: make(chan *AddrGossipPacket, 1024),
		PrivateRumorerLocalOut:  make(chan *AddrGossipPacket, 1024),
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
	if gossip.Gossip.Rumor != nil {
		if gossip.Gossip.Rumor.Text != "" {
			fmt.Printf("RUMOR origin %v from %v ID %v contents %v\n", gossip.Gossip.Rumor.Origin, gossip.Address, gossip.Gossip.Rumor.ID, gossip.Gossip.Rumor.Text)
		}
		d.PrivateRumorerGossipIn <- gossip

		d.RumorerGossipIn <- gossip

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
			d.RumorerUIIn <- msg
		} else {
			d.PrivateRumorerUIIn <- msg
		}
	}
}
