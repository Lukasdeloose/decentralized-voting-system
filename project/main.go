package main

import (
	. "github.com/lukasdeloose/decentralized-voting-system/project/constants"
	. "github.com/lukasdeloose/decentralized-voting-system/project/gossiper"
	. "github.com/lukasdeloose/decentralized-voting-system/project/udp"
	. "github.com/lukasdeloose/decentralized-voting-system/project/utils"
	"log"
	"math/rand"
	"time"

	"flag"
	"strings"
)

var (
	// The variables below will be filled in by the CLI arguments
	uiPort        string
	gossipAddr    string
	name          string
	peers         string
	antiEntropy   int
	routeRumoring int

	debug         bool
	N             int
	stubbornTimeout int
	hopLimit	 int
)

func main() {
	// Load command line arguments
	flag.StringVar(&uiPort, "UIPort", "8080", "port for the UI client (default '8080'")
	flag.StringVar(&gossipAddr, "gossipAddr", "127.0.0.1:5000",
		"ip:port for the gossiper (default '127.0.0.1:5000")
	flag.StringVar(&name, "name", "", "name of the gossiper")
	flag.StringVar(&peers, "peers", "", "comma seperated list of peers in the from ip:port")
	flag.BoolVar(&debug, "debug", false, "print debug information")
	flag.IntVar(&antiEntropy, "antiEntropy", 10, "Timeout for running anti entropy")
	flag.IntVar(&routeRumoring, "rtimer", 0, "Timeout in seconds to send route rumors. 0 (default) "+
		"means disable sending route rumors.")
	flag.IntVar(&N, "N", -1, "Total number of peers in the network")
	flag.IntVar(&stubbornTimeout, "stubbornTimeout", 5, "Timeout for resending txn BlockPublish")
	flag.IntVar(&hopLimit, "hopLimit", 10, "HopLimit for point to point messages")
	flag.Parse()

	// Seed random generator
	rand.Seed(time.Now().UTC().UnixNano())

	// Parse the arguments
	if name == "" {
		log.Fatal("Please provide your name with the '-name' flag")
	}
	peersSet := NewSet()
	for _, peer := range strings.Split(peers, ",") {
		if peer != "" {
			peersSet.Add(UDPAddr{Addr: peer})
		}
	}

	// Set constants
	Debug = debug
	HW1 = true
	HW2 = true

	// Initialize and run gossiper
	goss := NewGossiper(name, peersSet, uiPort, gossipAddr, antiEntropy, routeRumoring, N, stubbornTimeout, hopLimit)
	goss.Run()

	// Wait forever
	select {}
}

// TODO: indicate confirmation of origin in GUI

