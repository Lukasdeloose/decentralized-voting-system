package gui

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"github.com/lukasdeloose/Peerster/gossiper"
	"github.com/lukasdeloose/Peerster/helpers"
	"github.com/lukasdeloose/Peerster/packets"
	"net/http"
	"strconv"
)

type idJSON struct {
	Name string `json:"name"`
}

type roundJSON struct {
	Round string `json:"round"`
}

func StartGUI(port string, gsp *gossiper.Gossiper) {
	r := mux.NewRouter()
	r.HandleFunc("/message", messageHandler(gsp))
	r.HandleFunc("/node", nodeHandler(gsp))
	r.HandleFunc("/id", idHandler(gsp))
	r.HandleFunc("/round", roundHandler(gsp))
	r.HandleFunc("/route", routeHandler(gsp))
	r.HandleFunc("/share", shareFileHandler(gsp))
	r.HandleFunc("/download", downloadFileHandler(gsp))
	r.HandleFunc("/search", searchFileHandler(gsp))
	r.PathPrefix("/").Handler(http.StripPrefix("/", http.FileServer(http.Dir("gui"))))

	for {
		err := http.ListenAndServe(":"+port, r)
		if err != nil {
			panic(err)
		}
	}
}

func messageHandler(gsp *gossiper.Gossiper) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "GET":

			type Messages struct {
				Rumor   []packets.GUIMessage                `json:"Rumor"`
				Private map[string][]packets.PrivateMessage `json:"Private"`
				//File    string `json:"fileName"`
			}

			rumorList := gsp.GetVClock().GuiMessages
			privateList := gsp.GetVClock().PrivateMessages
			messages := Messages{Rumor: rumorList, Private: privateList,}
			err := json.NewEncoder(w).Encode(messages)
			if err != nil {
				panic(err)
			}

		case "POST":
			decoder := json.NewDecoder(r.Body)
			var data struct {
				Message     string `json:"message"`
				Destination string `json:"destination"`
			}
			err := decoder.Decode(&data)
			if err != nil {
				fmt.Println(err)
				w.WriteHeader(500)
			}
			if data.Destination == "" {
				// Not a private message
				message := packets.Message{Text: data.Message}
				gsp.HandleClientGossip(&message)
			} else {
				// Private message
				message := packets.Message{Text: data.Message, Destination: &data.Destination}
				gsp.HandleClientPrivate(&message)
			}
			w.WriteHeader(200)

		default:
			_, _ = fmt.Fprintf(w, "Sorry, only GET and POST methods are supported.")
		}
	})
}

func nodeHandler(gsp *gossiper.Gossiper) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "GET":
			nodeList := gsp.GetPeers()
			err := json.NewEncoder(w).Encode(nodeList)
			if err != nil {
				panic(err)
			}

		case "POST":
			decoder := json.NewDecoder(r.Body)
			var data struct{ PeerID string `json:"peerID"` }
			err := decoder.Decode(&data)
			if err != nil {
				fmt.Println(err)
				w.WriteHeader(500)
			}
			gsp.KnownPeers = helpers.AppendIfMissing(gsp.KnownPeers, data.PeerID)
			w.WriteHeader(200)
		default:
			_, _ = fmt.Fprintf(w, "Sorry, only GET and POST methods are supported.")
		}
	})
}

func routeHandler(gsp *gossiper.Gossiper) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "GET":
			routes := gsp.GetRoutingTable()
			err := json.NewEncoder(w).Encode(routes)
			if err != nil {
				panic(err)
			}
		default:
			_, _ = fmt.Fprintf(w, "Sorry, only GET methods are supported.")
		}
	})
}

func idHandler(gsp *gossiper.Gossiper) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "GET":
			response := idJSON{Name: gsp.Name}
			err := json.NewEncoder(w).Encode(response)
			if err != nil {
				panic(err)
			}
		default:
			_, _ = fmt.Fprintf(w, "Sorry, only GET and POST methods are supported.")
		}
	})
}

func roundHandler(gsp *gossiper.Gossiper) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "GET":
			response := roundJSON{Round: fmt.Sprint(gsp.GetVClock().GetMyRound())}
			err := json.NewEncoder(w).Encode(response)
			if err != nil {
				panic(err)
			}
		default:
			_, _ = fmt.Fprintf(w, "Sorry, only GET and POST methods are supported.")
		}
	})
}

func shareFileHandler(gsp *gossiper.Gossiper) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "POST":
			decoder := json.NewDecoder(r.Body)
			var data struct{ File string `json:"file"` }
			err := decoder.Decode(&data)
			if err != nil {
				fmt.Println(err)
				w.WriteHeader(500)
			}
			gsp.ShareFile(data.File)
			w.WriteHeader(200)
		default:
			_, _ = fmt.Fprintf(w, "Sorry, only POST methods are supported.")
		}
	})
}

func downloadFileHandler(gsp *gossiper.Gossiper) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "POST":
			decoder := json.NewDecoder(r.Body)
			var data struct {
				Destination string `json:"destination"`
				Hash        string `json:"hash"`
				FileName    string `json:"fileName"`
			}
			err := decoder.Decode(&data)
			if err != nil {
				fmt.Println(err)
				w.WriteHeader(500)
			}
			hashByte, _ := hex.DecodeString(data.Hash)
			message := &packets.Message{Destination: &data.Destination, Request: &hashByte, File: &data.FileName}

			gsp.HandleClientRequest(message)

			w.WriteHeader(200)
		default:
			_, _ = fmt.Fprintf(w, "Sorry, only POST methods are supported.")
		}
	})
}

func searchFileHandler(gsp *gossiper.Gossiper) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "GET":

			err := json.NewEncoder(w).Encode(gsp.GetSearchMap().GetGui())
			if err != nil {
				panic(err)
			}

		case "POST":
			decoder := json.NewDecoder(r.Body)
			var data struct {
				Keywords string `json:"keywords"`
				Budget   string `json:"budget"`
			}

			err := decoder.Decode(&data)
			if err != nil {
				fmt.Println(err)
				w.WriteHeader(500)
			}
			budget, _ := strconv.Atoi(data.Budget)
			fmt.Println(budget)
			message := &packets.Message{KeyWords: data.Keywords, Budget: budget}
			gsp.HandleClientSearch(message)

			w.WriteHeader(200)
		default:
			_, _ = fmt.Fprintf(w, "Sorry, only POST methods are supported.")
		}
	})
}
