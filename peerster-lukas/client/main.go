package main

import (
	"encoding/hex"
	"os"

	//"bufio"
	"flag"
	"fmt"
	"github.com/dedis/protobuf"
	"github.com/lukasdeloose/Peerster/packets"
	"net"

	//"net"
)

func wrongArgumentError() {
	fmt.Println("ERROR (bad argument combination)")
	os.Exit(1)
}

func wrongRequestError() {
	fmt.Println(" ERROR (Unable to decode hex hash)")
	os.Exit(1)
}

func main() {
	port := flag.String("UIPort", "8080", "port for the UI client")
	msg := flag.String("msg", "", "message to be sent")
	dest := flag.String("dest", "", "destination for the private message; can be omitted")
	file := flag.String("file", "", "file to be indexed by the gossiper")
	request := flag.String("request", "", "request a chunk or metaFile of this hash")
	keywords := flag.String("keywords", "", "keywords to search files")
	budget := flag.Int("budget", 0, "searchRequest budget")

	flag.Parse()

	// Create the peer server to listen
	udpGossiperAddr, err := net.ResolveUDPAddr("udp4", "127.0.0.1:"+*port)
	if err != nil {
		fmt.Println("Error resolving UDP peer address")
		panic(err)
	}
	udpGossiperConn, err := net.ListenPacket("udp4", ":0")
	if err != nil {
		fmt.Println("Error listening on UDP address")
		panic(err)
	}

	if *msg != "" && *request != "" {
		wrongArgumentError()
	}
	if *msg != "" && *request != "" {
		wrongArgumentError()
	}
	if *file != "" && *dest != "" && *request == "" {
		wrongArgumentError()
	}
	if *budget != 0 && *keywords == "" {
		wrongArgumentError()
	}


	var requestBytes []byte
	if request != nil {
		requestBytes, err = hex.DecodeString(*request)
		if err != nil {
			fmt.Println(err)
			wrongRequestError()
		}
	}
	message := packets.Message{Text: *msg, Destination: dest, File: file, Request: &requestBytes, KeyWords: *keywords, Budget: *budget}

	packetBytes, err := protobuf.Encode(&message)
	if err != nil {
		fmt.Println("error while encoding the gossipPacket")
		panic(err)
	}

	_, err = udpGossiperConn.WriteTo(packetBytes, udpGossiperAddr)
	if err != nil {
		fmt.Println("error while sending packet")
		panic(err)
	}
}
