package main

import (
	"flag"
	"fmt"
	"github.com/dedis/protobuf"
	. "github.com/lukasdeloose/decentralized-voting-system/project/utils"
	"log"
	"net"
	"strings"
)

var (
	UIPort  string
	msg     string
	dest    string
	vote    bool
	pollid  int
	question string
	voters string
	count bool
)

func main() {
	// Load command line arguments
	flag.StringVar(&UIPort, "UIPort", "8080", "port for the UI client (default '8080'")
	flag.StringVar(&msg, "msg", "", "message to be sent; if the -dest flag is present, "+
		"this is a private message, otherwise it’s a rumor message")
	flag.StringVar(&dest, "dest", "", "destination for the private message; can be omitted")
	flag.BoolVar(&vote, "vote", false, "Your vote for poll 'pollid'")
	flag.IntVar(&pollid, "pollid", -1, "The poll you want to vote for")
	flag.StringVar(&question, "question", "", "The question you want to create a poll for")
	flag.StringVar(&voters, "voters", "", "The people that are allowed to vote for your question, as a" +
		"comma seperated list of ciphers")
	flag.BoolVar(&count, "count", false, "Use this command to count the votes for pollid")
	flag.Parse()

	// TODO Check if valid command

	// Send message to the Gossiper
	addr := "127.0.0.1" + ":" + UIPort
	SendMsg(msg, dest, addr)
}

func SendMsg(msg, dest, addr string) {
	// Set up UDP socket
	remoteAddr, err := net.ResolveUDPAddr("udp", addr)
	conn, err := net.DialUDP("udp", nil, remoteAddr)
	if err != nil {
		panic(fmt.Sprintf("ERROR: %v", err))
	}
	// Close connection after message is sent
	defer conn.Close()

	// Encode the message
	message := Message{Text: msg}
	if dest != "" {
		message.Destination = &dest
	}

	// Add vote
	if pollid >= 0 {
		message.Voting = &VotingMessage{
			NewVote: &NewVote{
				Pollid: uint32(pollid),
				Vote:   vote,
			},
		}
	}

	if count {
		if pollid > 0 {
			message.Voting.CountRequest = &CountRequest{Pollid: uint32(pollid)}
		} else {
			log.Fatalf("Please provide the id of the poll you want to count")
		}
	}

	voterStrings := make([]string, 0)
	for _, voter := range strings.Split(voters, ",") {
		if voter != "" {
			voterStrings = append(voterStrings, voter)
		}
	}

	if question != "" {
		message.Voting = &VotingMessage{
			NewPoll: &NewPoll{
				Question: question,
				Voters:   voterStrings,
			},
		}
	}

	packetBytes, err := protobuf.Encode(&message)
	if err != nil {
		fmt.Printf("ERROR: Could not serialize message\n")
		fmt.Println(err)
	}

	// Write the bytes to the UDP socket
	_, err = conn.Write(packetBytes)
	if err != nil {
		fmt.Printf("ERROR: %v\n", err)
	}
}
