package gossiper

import (
	"encoding/hex"
	"fmt"
	"github.com/dedis/protobuf"
	"github.com/lukasdeloose/Peerster/helpers"
	"github.com/lukasdeloose/Peerster/packets"
	"math/rand"
	"strings"
	"sync"
	"time"
)

type Gossiper struct {
	Connection        *Connection
	SequenceID        uint32 // ID of last sent message, starting from 1
	Simple            bool
	Name              string
	KnowPeersLock     sync.RWMutex
	KnownPeers        []string
	vClock            *VClock
	rumorChannels     map[string]chan *packets.GossipPacket
	rumorChannelsLock sync.RWMutex
	AntiEntropy       int
	routingTable      *RoutingTable
	rtimer            int
	filesLock         sync.RWMutex
	files             map[string]*File // Maps hash of MetaFile on the file
	fileDataLock      sync.RWMutex
	fileData          map[string][]byte // Map to keep all chunks indexed by their hash, for efficient retrieval
	fileChannelsLock  sync.RWMutex
	fileChannels      map[string]map[string]chan *packets.DataReply
	debug             bool
	searchChannel     chan *packets.SearchReply
	searchMap         *SearchMap
	searchRequestSet  *FileRequestsSet
	ackChannels       map[uint32]chan *packets.TLCAck //channel for every file being published
	ackChannelsLock   sync.RWMutex
	TLCClientChannel  chan *packets.BlockPublish
	hw3ex2            bool
	hw3ex3            bool
	hw3ex4            bool
	blocks            map[string]*packets.BlockPublish // Holds all the blocks for hw3ex4, to check if our confirmed is subset
	blockLock         sync.RWMutex
}

func NewGossiper(name, gossipAddr, clientPort, peers string, antiEntropy, rtimer int, simple, debug, hw3ex2, hw3ex3, hw3ex4 bool) *Gossiper {
	knownPeers := make([]string, 0)
	if peers != "" {
		knownPeers = strings.Split(peers, ",")
	}

	vClock := newVClock()
	connection := newConnection(gossipAddr, clientPort)

	gossiper := &Gossiper{
		SequenceID:       0,
		Connection:       connection,
		Simple:           simple,
		Name:             name,
		rumorChannels:    make(map[string]chan *packets.GossipPacket),
		vClock:           vClock,
		AntiEntropy:      antiEntropy,
		KnownPeers:       knownPeers,
		routingTable:     newRoutingTable(),
		rtimer:           rtimer,
		files:            make(map[string]*File),
		fileData:         make(map[string][]byte),
		fileChannels:     make(map[string]map[string]chan *packets.DataReply),
		debug:            debug,
		searchChannel:    make(chan *packets.SearchReply, helpers.SearchChannelSize),
		searchMap:        NewSearchMap(),
		searchRequestSet: NewFileRequestsSet(),
		ackChannels:      make(map[uint32]chan *packets.TLCAck),
		hw3ex2:           hw3ex2,
		hw3ex3:           hw3ex3 || hw3ex4, // All functions of hw3ex3 used for ex4
		hw3ex4:           hw3ex4,
		TLCClientChannel: make(chan *packets.BlockPublish, helpers.TLCBufferSize),
		blocks:           make(map[string]*packets.BlockPublish),
	}

	if gossiper.hw3ex3 {
		go gossiper.checkRoundAdvance()
	}
	return gossiper
}

//*** Exported functions ***//
func (gossiper *Gossiper) HandleClientGossip(message *packets.Message) {
	fmt.Println("CLIENT MESSAGE", message.Text)
	gossiper.KnowPeersLock.RLock()
	stringPeers := strings.Join(gossiper.KnownPeers, ",")
	gossiper.KnowPeersLock.RUnlock()
	fmt.Println("PEERS", stringPeers)

	if gossiper.Simple {
		if *message.Destination != "" {
			fmt.Println("ERROR: private message in simple mode")
			return
		}
		simpleMessage := packets.SimpleMessage{
			OriginalName:  gossiper.Name,
			RelayPeerAddr: gossiper.Connection.PeerAddress.String(),
			Contents:      message.Text,
		}
		gossipPacket := packets.GossipPacket{Simple: &simpleMessage}

		go gossiper.sendSimpleToPeers(&gossipPacket, "")
	} else {
		if len(gossiper.KnownPeers) < 1 {
			return
		}
		gossipPacket := gossiper.createRumorPacket(message.Text)
		go gossiper.sendRumorToPeers(gossipPacket, gossiper.KnownPeers, false)
	}
}

func (gossiper *Gossiper) HandleClientPrivate(message *packets.Message) {
	fmt.Println("CLIENT MESSAGE", message.Text, "dest", *message.Destination)

	pm := &packets.PrivateMessage{
		Origin:      gossiper.Name,
		ID:          0,
		Text:        message.Text,
		Destination: *message.Destination,
		HopLimit:    helpers.HopLimit,
	}
	gossipPacket := &packets.GossipPacket{
		Private: pm,
	}

	gossiper.handlePrivate(gossipPacket)
}

// Request META
func (gossiper *Gossiper) HandleClientRequest(message *packets.Message) {
	// Downloading from fileSearch
	if *message.Destination == "" {
		destPeers := gossiper.searchMap.getDestPeers(*message.File, hex.EncodeToString(*message.Request))
		if len(destPeers) == 0 {
			return
		}
		message.Destination = &destPeers[0]
	}

	if nextHop, known := gossiper.routingTable.GetNextHop(*message.Destination); known {
		// Create first packet
		requestMessage := &packets.DataRequest{
			Origin:      gossiper.Name,
			Destination: *message.Destination,
			HopLimit:    helpers.HopLimit - 1,
			HashValue:   *message.Request,
		}
		packet := &packets.GossipPacket{
			DataRequest: requestMessage,
		}

		// Create channel for this file
		c := make(chan *packets.DataReply)

		gossiper.fileChannelsLock.Lock()
		if _, ok := gossiper.fileChannels[requestMessage.Destination][hex.EncodeToString(requestMessage.HashValue)]; !ok {
			gossiper.fileChannels[requestMessage.Destination] = make(map[string]chan *packets.DataReply)
		}
		gossiper.fileChannels[requestMessage.Destination][hex.EncodeToString(requestMessage.HashValue)] = c
		gossiper.fileChannelsLock.Unlock()

		gossiper.sendNeighbour(packet, nextHop)
		fmt.Println("DOWNLOADING metafile of", *message.File, "from", requestMessage.Destination)

		received := false
		for !received {
			// Resend the Request every 5 seconds until answered
			ticker := time.NewTicker(5 * time.Second)
			select {
			case reply := <-c:
				// MetaFile received
				ticker.Stop()
				received = true
				gossiper.getFileFromMeta(reply, *message.File, c)
			case <-ticker.C:
				// timeout, resend message
				ticker.Stop()
				fmt.Println("DOWNLOADING metafile of", *message.File, "from", requestMessage.Destination)
				gossiper.sendNeighbour(packet, nextHop)
			}
		}
	} else {
		fmt.Println("HandleClientRequest: Next hop not known")
	}
}

func (gossiper *Gossiper) HandleClientSearch(message *packets.Message) {
	// New search, clean the channel (we assume no parallel clientSearch requests as said on forum)
	for len(gossiper.searchChannel) > 0 {
		<-gossiper.searchChannel
	}
	if message.Budget == 0 {
		// Budget was not specified by client
		found := false
		for budget := 2; budget <= 32 && !found; budget *= 2 {
			if gossiper.debug {
				fmt.Println("budget is", budget)
			}
			found = gossiper.sendSearchRequest(message.KeyWords, budget)
			fmt.Println("found is", found)
		}
	} else {
		gossiper.sendSearchRequest(message.KeyWords, message.Budget)
	}
}

func (gossiper *Gossiper) HandlePeer(buffer []byte, addr string) {

	gossipPacket := &packets.GossipPacket{}
	err := protobuf.Decode(buffer, gossipPacket)
	if err != nil {
		fmt.Println(err)
		return
	}

	// Add to list of known peers
	gossiper.KnowPeersLock.Lock()
	gossiper.KnownPeers = helpers.AppendIfMissing(gossiper.KnownPeers, addr)
	gossiper.KnowPeersLock.Unlock()

	// Check type of message
	switch helpers.TypeOfMessage(gossipPacket) {
	case packets.SIMPLE:
		gossiper.handleSimple(gossipPacket, addr)
	case packets.RUMOR:
		gossiper.handleRumor(gossipPacket, addr, false)
	case packets.STATUS:
		gossiper.handleStatus(gossipPacket, addr)
	case packets.PRIVATE:
		gossiper.handlePrivate(gossipPacket)
	case packets.DATAREQUEST:
		gossiper.handleDataRequest(gossipPacket)
	case packets.DATAREPLY:
		gossiper.handleDataReply(gossipPacket)
	case packets.SEARCHREQUEST:
		gossiper.handleSearchRequest(gossipPacket, addr)
	case packets.SEARCHREPLY:
		gossiper.handleSearchReply(gossipPacket)
	case packets.TLCMESSAGE:
		if gossiper.hw3ex3 {
			gossiper.handleTLCRumor(gossipPacket, addr)
		} else {
			gossiper.handleTLCMessage(gossipPacket, addr)
		}
	case packets.TLCACK:
		gossiper.handleTLCAck(gossipPacket)
	case 0:
		fmt.Println("Invalid gossipPacket")
	}
}

//*** Getters ***///
func (gossiper *Gossiper) GetPeers() []string {
	return gossiper.KnownPeers
}

func (gossiper *Gossiper) GetVClock() *VClock {
	return gossiper.vClock
}

func (gossiper *Gossiper) GetSearchMap() *SearchMap {
	return gossiper.searchMap
}

func (gossiper *Gossiper) GetRoutingTable() map[string]map[string]string {
	return gossiper.routingTable.table
}

//*** Internal functions ***//
func (gossiper *Gossiper) executeStatus(statusPacket *packets.StatusPacket, gossipOldPacket *packets.GossipPacket, addr string) {
	// Compare statusPacket with our vectorClock
	send, want := gossiper.vClock.compareWants(statusPacket)

	if send != nil {
		gossiper.sendRumorToPeers(send, []string{addr}, false)
		return
	} else if want == true {
		gossiper.sendStatusPacket(addr)
	} else if gossipOldPacket != nil {
		//fmt.Println("IN SYNC WITH", addr)
		// Neither has new messages, so flip coin
		rand.Seed(time.Now().UnixNano())
		heads := (rand.Int() % 2) == 0
		if heads {
			gossiper.KnowPeersLock.RLock()
			knownPeers := helpers.Filter(gossiper.KnownPeers, addr)
			gossiper.KnowPeersLock.RUnlock()
			// True to indicate that we flipped a coin (for output reasons)
			gossiper.sendRumorToPeers(gossipOldPacket, knownPeers, true)
		}
	}
}

func (gossiper *Gossiper) createRumorPacket(message string) *packets.GossipPacket {
	// Set want so other peers don't send your own messages to you
	gossiper.SequenceID++
	gossiper.vClock.increment(gossiper.Name, false)

	rumorMessage := &packets.RumorMessage{
		Origin: gossiper.Name,
		ID:     gossiper.SequenceID,
		Text:   message,
	}
	gossipPacket := &packets.GossipPacket{
		Rumor: rumorMessage,
	}
	// Add to own messages
	gossiper.vClock.addMessage(gossipPacket, false)

	return gossipPacket
}

func (gossiper *Gossiper) getFileFromMeta(reply *packets.DataReply, name string, c chan *packets.DataReply) {
	// Gets the entire file after receiving the MetaFile

	file := &File{
		name:     name,
		size:     0,
		metaFile: reply.Data,
		hash:     reply.HashValue,
	}

	// contains the full file data
	var fileData []byte
	size := 0
	chunkHashes := gossiper.getChunkHashes(file.metaFile)

	nextHop, ok := gossiper.routingTable.GetNextHop(reply.Origin)
	if !ok {
		fmt.Println("ERROR: route not found for packet we got metaFile from")
	}

	for i, hash := range chunkHashes {
		//fmt.Println(hex.EncodeToString(hash))
		requestMessage := &packets.DataRequest{
			Origin:      gossiper.Name,
			Destination: reply.Origin,
			HopLimit:    helpers.HopLimit - 1,
			HashValue:   hash,
		}
		packet := &packets.GossipPacket{
			DataRequest: requestMessage,
		}
		gossiper.fileChannelsLock.Lock()
		gossiper.fileChannels[requestMessage.Destination][hex.EncodeToString(requestMessage.HashValue)] = c
		gossiper.fileChannelsLock.Unlock()

		fmt.Println("DOWNLOADING", name, "chunk", i+1, "from", reply.Origin)
		gossiper.sendNeighbour(packet, nextHop)

		received := false
		for !received {
			ticker := time.NewTicker(5 * time.Second)
			select {
			case reply := <-c:
				// MetaFile received
				ticker.Stop()
				received = true
				// Append for full file
				fileData = append(fileData, reply.Data...)
				size = size + len(reply.Data)
				// Save so we can share it too
				gossiper.fileDataLock.Lock()
				gossiper.fileData[hex.EncodeToString(reply.HashValue)] = reply.Data
				gossiper.fileDataLock.Unlock()
			case <-ticker.C: // timeout, resend message
				ticker.Stop()
				fmt.Println("DOWNLOADING", name, "chunk", i+1, "from", reply.Origin)
				gossiper.sendNeighbour(packet, nextHop)

			}
		}
	}
	file.size = size
	gossiper.saveFile(name, fileData)
	gossiper.files[hex.EncodeToString(file.hash)] = file

}
