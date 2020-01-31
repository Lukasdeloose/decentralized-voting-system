package web

import (
	"github.com/gorilla/mux"
	. "github.com/lukasdeloose/decentralized-voting-system/project/blockchain"
	. "github.com/lukasdeloose/decentralized-voting-system/project/privateRumorer"
	. "github.com/lukasdeloose/decentralized-voting-system/project/rumorer"
	. "github.com/lukasdeloose/decentralized-voting-system/project/voting"
	"net/http"
	"time"
)

// Server that handles the HTTP requests from the GUI running on localhost:8080
type WebServer struct {
	rumorer        *Rumorer
	privateRumorer *PrivateRumorer
	voteRumorer *VoteRumorer
	blockchain *Blockchain


	router *mux.Router
	server *http.Server

	uiPort string
}

func NewWebServer(rumorer *Rumorer, privateRumorer *PrivateRumorer, voteRumorer *VoteRumorer, blockchain *Blockchain, uiPort string) (ws *WebServer) {
	ws = &WebServer{}
	ws.uiPort = uiPort
	ws.rumorer = rumorer
	ws.privateRumorer = privateRumorer
	ws.voteRumorer = voteRumorer
	ws.blockchain = blockchain
	ws.router = mux.NewRouter()

	// Serve api calls
	ws.router.HandleFunc("/id", ws.handleGetNodeID).Methods("GET")
	ws.router.HandleFunc("/message", ws.handleGetMessages).Methods("GET")
	ws.router.HandleFunc("/node", ws.handleGetPeers).Methods("GET")
	ws.router.HandleFunc("/message", ws.handlePostMessages).Methods("POST")
	ws.router.HandleFunc("/node", ws.handlePostPeers).Methods("POST")
	ws.router.HandleFunc("/dsdv", ws.handleGetOrigins).Methods("GET")
	ws.router.HandleFunc("/private/{origin}", ws.handleGetPrivate).Methods("GET")
	ws.router.HandleFunc("/private/{origin}", ws.handlePostPrivate).Methods("POST")

	// Voting
	ws.router.HandleFunc("/voting/polls", ws.handleGetPolls).Methods("GET")
	ws.router.HandleFunc("/voting/poll/{pollId}/vote", ws.handlePostVote).Methods("POST")
	ws.router.HandleFunc("/voting/poll/{pollId}/count", ws.handlePostCount).Methods("POST")
	ws.router.HandleFunc("/voting/polls", ws.handlePostPolls).Methods("POST")

	// Serve static files (Note: relative path from Peerster root)
	ws.router.PathPrefix("/").Handler(http.StripPrefix("/", http.FileServer(http.Dir("web/assets"))))

	ws.server = &http.Server{
		Handler: ws.router,
		Addr:    "127.0.0.1:" + ws.uiPort,
		// Good practice: enforce timeouts for servers you create!
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}

	return ws
}

func (ws *WebServer) Run() {
	// Ignore errors when starting GUI
	go ws.server.ListenAndServe()
}
