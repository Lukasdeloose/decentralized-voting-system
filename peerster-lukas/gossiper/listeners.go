package gossiper

import (
	"fmt"
	"github.com/dedis/protobuf"
	"github.com/lukasdeloose/Peerster/helpers"
	"github.com/lukasdeloose/Peerster/packets"
)

func (gossiper *Gossiper) ListenPeer() {
	for {
		buffer := make([]byte, helpers.MaxBufferSize)
		n, addr, err := gossiper.Connection.PeerConn.ReadFromUDP(buffer)
		if err != nil {
			panic(err)
		}
		// Go routine to increase performance
		go gossiper.HandlePeer(buffer[:n], addr.String())
	}
}

func (gossiper *Gossiper) ListenClientCLI() {
	for {
		buffer := make([]byte, helpers.MaxBufferSize)
		n, _, err := gossiper.Connection.ClientConn.ReadFromUDP(buffer)
		if err != nil {
			fmt.Println(err)
			continue
		}

		message := &packets.Message{}
		err = protobuf.Decode(buffer[:n], message)
		if err != nil {
			fmt.Println(err)
			continue
		}

		switch helpers.ClientCase(message) {
		case helpers.GOSSIP:
			go gossiper.HandleClientGossip(message)
		case helpers.PRIVATE:
			go gossiper.HandleClientPrivate(message)
		case helpers.FILE:
			go gossiper.ShareFile(*message.File)
		case helpers.REQUEST:
			go gossiper.HandleClientRequest(message)
		case helpers.SEARCH:
			fmt.Println("client search")
			go gossiper.HandleClientSearch(message)
		case 0:
			fmt.Println("Wrong input combination by client")
		}
	}
}

