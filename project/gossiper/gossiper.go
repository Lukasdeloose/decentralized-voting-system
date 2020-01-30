package gossiper

import (
	. "github.com/lukasdeloose/decentralized-voting-system/project/blockchain"
	. "github.com/lukasdeloose/decentralized-voting-system/project/privateRumorer"
	. "github.com/lukasdeloose/decentralized-voting-system/project/rumorer"
	. "github.com/lukasdeloose/decentralized-voting-system/project/utils"
	. "github.com/lukasdeloose/decentralized-voting-system/project/voting"
	. "github.com/lukasdeloose/decentralized-voting-system/project/web"
)

type Gossiper struct {
	Dispatcher *Dispatcher

	WebServer *WebServer

	Rumorer *Rumorer

	PrivateRumorer *PrivateRumorer

	VoteRumorer *VoteRumorer

	Blockchain *Blockchain

	name string

	miner *Miner

	N int
	stubbornTimeout int
}

func NewGossiper(name string, peers *Set, uiPort string, gossipAddr string,
	antiEntropy int, routeRumoringTimeout int, N int, stubbornTimeout int, hopLimit int) *Gossiper {
	// Create the dispatcher
	disp := NewDispatcher(name, uiPort, gossipAddr)

	// Create the rumorer
	rumorer := NewRumorer(name, peers, disp.RumorerGossipIn, disp.RumorerOut, disp.RumorerLocalOut, disp.RumorerUIIn, antiEntropy)

	// Create the rumorer for private messages
	privateRumorer := NewPrivateRumorer(name, disp.PrivateRumorerGossipIn, disp.PrivateRumorerUIIn,
		disp.PrivateRumorerGossipOut, disp.RumorerUIIn, disp.PrivateRumorerLocalOut, routeRumoringTimeout, gossipAddr, hopLimit)

	// Create the blockchain miner
	blockchain := NewBlockChain()

	voteRumorer := NewVoteRumorer(name, disp.VoteRumorerUIIn, disp.VoteRumorerIn, disp.RumorerGossipIn, blockchain)

	miner := NewMiner(disp.BlockRumorerIn, disp.RumorerGossipIn)

	// Create the webserver for interacting with the rumorer
	webServer := NewWebServer(rumorer, privateRumorer, voteRumorer, blockchain, uiPort)


	return &Gossiper{
		Dispatcher:     disp,
		WebServer:      webServer,
		Rumorer:        rumorer,
		PrivateRumorer: privateRumorer,
		VoteRumorer:    voteRumorer,
		Blockchain:     blockchain,
		miner: 			miner,
		name:           name,
		N:				N,
		stubbornTimeout: stubbornTimeout,
	}
}

func (g *Gossiper) Run() {
	g.Dispatcher.Run()
	g.Rumorer.Run()
	g.PrivateRumorer.Run()
	g.VoteRumorer.Run()
	g.Blockchain.Run()
	g.miner.Run()

	if g.WebServer != nil {
		g.WebServer.Run()
	}
}
