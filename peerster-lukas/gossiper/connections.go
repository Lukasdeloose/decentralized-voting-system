package gossiper

import (
	"github.com/lukasdeloose/Peerster/helpers"
	"net"
)

type Connection struct {
	PeerAddress   *net.UDPAddr
	PeerConn      *net.UDPConn
	ClientAddress *net.UDPAddr
	ClientConn    *net.UDPConn
}

func newConnection(gossipAddr, clientPort string) *Connection {
	// Create the peer server to listen
	udpPeerAddr, err := net.ResolveUDPAddr("udp4", gossipAddr)
	if err != nil {
		panic(err)
	}
	udpPeerConn, err := net.ListenUDP("udp4", udpPeerAddr)
	if err != nil {
		panic(err)
	}
	// Create connection with the client
	udpClientAddr, err := net.ResolveUDPAddr("udp4", helpers.Localhost+":"+clientPort)
	if err != nil {
		panic(err)
	}
	udpClientConn, err := net.ListenUDP("udp4", udpClientAddr)
	if err != nil {
		panic(err)
	}
	return &Connection{
		PeerAddress:   udpPeerAddr,
		PeerConn:      udpPeerConn,
		ClientAddress: udpClientAddr,
		ClientConn:    udpClientConn,
	}
}
