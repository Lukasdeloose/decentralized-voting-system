package gossiper

import (
	. "github.com/lukasdeloose/decentralized-voting-system/project/privateRumorer"
	. "github.com/lukasdeloose/decentralized-voting-system/project/rumorer"
	. "github.com/lukasdeloose/decentralized-voting-system/project/utils"
	. "github.com/lukasdeloose/decentralized-voting-system/project/web"
)

type Gossiper struct {
	Dispatcher *Dispatcher

	WebServer *WebServer

	Rumorer *Rumorer

	PrivateRumorer *PrivateRumorer

	name string

	simple bool

	N int
	stubbornTimeout int
}

func NewGossiper(name string, peers *Set, simple bool, uiPort string, gossipAddr string,
	antiEntropy int, routeRumoringTimeout int, N int, stubbornTimeout int, hopLimit int) *Gossiper {
	// Create the dispatcher
	disp := NewDispatcher(uiPort, gossipAddr)

	// Create the rumorer
	rumorer := NewRumorer(name, peers, disp.RumorerGossipIn, disp.RumorerOut, disp.RumorerUIIn, antiEntropy)

	// Create the rumorer for private messages
	privateRumorer := NewPrivateRumorer(name, disp.PrivateRumorerGossipIn, disp.PrivateRumorerUIIn,
		disp.PrivateRumorerGossipOut, disp.RumorerUIIn, disp.PrivateRumorerLocalOut, routeRumoringTimeout, gossipAddr, hopLimit)

	// Create the webserver for interacting with the rumorer
	webServer := NewWebServer(rumorer, privateRumorer, uiPort)

	return &Gossiper{
		Dispatcher:     disp,
		WebServer:      webServer,
		Rumorer:        rumorer,
		PrivateRumorer: privateRumorer,
		simple:         simple,
		name:           name,
		N:				N,
		stubbornTimeout: stubbornTimeout,
	}
}

func (g *Gossiper) Run() {
	g.Dispatcher.Run()
	g.Rumorer.Run()
	if !g.simple {
		g.PrivateRumorer.Run()
	}
	if g.WebServer != nil {
		g.WebServer.Run()
	}
}
