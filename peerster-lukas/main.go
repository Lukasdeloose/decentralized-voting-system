package main

import (
	"flag"
	"fmt"
	"github.com/lukasdeloose/Peerster/gossiper"
	"github.com/lukasdeloose/Peerster/gui"
	"github.com/lukasdeloose/Peerster/helpers"
	"os"
)

// TODO: private messages only show when normal message send
// TODO: more channels instead of locks
// TODO: file request concurrency
// TODO: embed locks in structs

func main() {
	// Get the command line arguments

	clientPort := flag.String("UIPort", "8080", "port for the UI client")
	gossipAddr := flag.String("gossipAddr", "127.0.0.1:5000", "ip:port for the gossiper")
	name := flag.String("name", "", "name of the gossiper")
	peers := flag.String("peers", "", "comma separated list of peers of the form of ip:port")
	simpleFlag := flag.Bool("simple", false, "run gossiper in simple broadcast mode")
	debug := flag.Bool("debug", false, "run gossiper in debug mode")
	antiEntropy := flag.Int("antiEntropy", 10, "Use the given timeout in seconds for anti-entropy. The default anti-entropy duration is 10 seconds.")
	rtimer := flag.Int("rtimer", 0, "Timeout in seconds to send route rumors. 0 (default) means disable sending route rumors")
	hw3ex2 := flag.Bool("hw3ex2", false, "run gossiper in hw3ex2 mode")
	hw3ex3 := flag.Bool("hw3ex3", false, "run gossiper in hw3ex3 mode")
	hw3ex4 := flag.Bool("hw3ex4", false, "run gossiper in hw3ex4 mode")
	N := flag.Int("N", 0, "number of peers in network")
	stubbornTimeout := flag.Int("stubbornTimeout", 5, "timeout in seconds")
	hopLimit := flag.Int("hopLimit", 10, "hop limit for TLCAck")

	flag.Parse()

	helpers.StubbornTimeout = *stubbornTimeout
	helpers.HopLimit = uint32(*hopLimit)
	helpers.Nodes = *N

	if *hw3ex3 && *hw3ex2 {
		fmt.Println("ERROR, can't be in hw3ex and hw3ex2 mode at the same time")
		os.Exit(1)
	}


	g := gossiper.NewGossiper(*name, *gossipAddr, *clientPort, *peers, *antiEntropy, *rtimer, *simpleFlag, *debug, *hw3ex2, *hw3ex3, *hw3ex4)

	go g.ListenPeer()
	go g.ListenClientCLI()
	if *antiEntropy > 0 {
		go g.SendAntiEntropy()
	}
	if *rtimer > 0 {
		go g.SendRouteRumor()
	}
	go gui.StartGUI(*clientPort, g)

	// Block
	finished := make(chan bool)
	<-finished
}
