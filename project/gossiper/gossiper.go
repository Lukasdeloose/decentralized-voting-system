package gossiper

import (
	. "github.com/lukasdeloose/decentralized-voting-system/project/confirmationRumorer"
	. "github.com/lukasdeloose/decentralized-voting-system/project/constants"
	. "github.com/lukasdeloose/decentralized-voting-system/project/privateRumorer"
	. "github.com/lukasdeloose/decentralized-voting-system/project/files"
	. "github.com/lukasdeloose/decentralized-voting-system/project/rumorer"
	. "github.com/lukasdeloose/decentralized-voting-system/project/search"
	. "github.com/lukasdeloose/decentralized-voting-system/project/utils"
	. "github.com/lukasdeloose/decentralized-voting-system/project/web"
)

type Gossiper struct {
	Dispatcher *Dispatcher

	WebServer *WebServer

	Rumorer GenericRumorer

	PrivateRumorer *PrivateRumorer

	FileHandler *FileHandler

	Searcher *Searcher

	ConfirmationRumorer *ConfirmationRumorer

	name string

	simple bool

	N int
	stubbornTimeout int
}

func NewGossiper(name string, peers *Set, simple bool, uiPort string, gossipAddr string,
	antiEntropy int, routeRumoringTimeout int, N int, stubbornTimeout int, hopLimit int) *Gossiper {
	// Create the dispatcher
	disp := NewDispatcher(uiPort, gossipAddr)

	// Create the simple/normal rumorer
	var rumorer GenericRumorer
	if simple {
		rumorer = NewSimpleRumorer(gossipAddr, name, peers, disp.RumorerGossipIn, disp.RumorerOut, disp.RumorerUIIn)
	} else {
		rumorer = NewRumorer(name, peers, disp.RumorerGossipIn, disp.RumorerOut, disp.RumorerUIIn, antiEntropy)
	}

	// Create the rumorer for private messages
	privateRumorer := NewPrivateRumorer(name, disp.PrivateRumorerGossipIn, disp.PrivateRumorerUIIn,
		disp.PrivateRumorerGossipOut, disp.RumorerUIIn, disp.PrivateRumorerLocalOut, routeRumoringTimeout, gossipAddr, hopLimit)

	fileHandler := NewFileHandler(name, disp.FileHandlerIn, disp.FileHandlerUIIn, disp.PrivateRumorerGossipIn, hopLimit)
	files, filesMutex := fileHandler.Files()

	searcher := NewSearcher(name, peers, disp.SearchHandlerIn, disp.SearchHandlerOut, disp.SearchHandlerUIIn,
		disp.PrivateRumorerGossipIn, fileHandler.SearchDownloadIn(), files, filesMutex, hopLimit)

	confirmationRumorer := NewConfirmationRumorer(name, fileHandler.ConfFileOut(), rumorer.TLCOut(), disp.ConfRumorerP2PIn, disp.PrivateRumorerGossipIn, rumorer.TLCIn(), N, hopLimit, stubbornTimeout)

	// Create the webserver for interacting with the rumorer
	webServer := NewWebServer(rumorer, privateRumorer, fileHandler, searcher, uiPort)

	return &Gossiper{
		Dispatcher:     disp,
		WebServer:      webServer,
		Rumorer:        rumorer,
		PrivateRumorer: privateRumorer,
		FileHandler:    fileHandler,
		Searcher:       searcher,
		ConfirmationRumorer: confirmationRumorer,
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
		g.FileHandler.Run()
		g.Searcher.Run()
		if HW3EX2 || HW3EX3 {
			g.ConfirmationRumorer.Run()
		}
	}
	if g.WebServer != nil {
		g.WebServer.Run()
	}
}
