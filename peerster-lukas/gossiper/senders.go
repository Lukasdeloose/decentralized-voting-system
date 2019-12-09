package gossiper

import (
	"fmt"
	"github.com/dedis/protobuf"
	"github.com/lukasdeloose/Peerster/helpers"
	"github.com/lukasdeloose/Peerster/packets"
	"math"
	"net"
	"strings"
	"time"
)

func (gossiper *Gossiper) sendStatusPacket(addr string) {
	statusPacket := &packets.StatusPacket{Want: gossiper.vClock.createWant()}
	statusGossipPacket := &packets.GossipPacket{
		Status: statusPacket,
	}
	gossiper.sendNeighbour(statusGossipPacket, addr)
}

// knownPeers is passed so that we can leave out unreachable nodes when we recursively call this function
func (gossiper *Gossiper) sendRumorToPeers(gossipPacket *packets.GossipPacket, recursiveKnownPeers []string, coin bool) {
	R, found := helpers.GetRandomPeer(recursiveKnownPeers)
	if !found {
		return
	}

	if coin {
		//fmt.Println("FLIPPED COIN sending rumor to", R)
	}

	// Check if channel already exists
	gossiper.rumorChannelsLock.Lock()
	_, ok := gossiper.rumorChannels[R]
	if !ok {
		gossiper.rumorChannels[R] = make(chan *packets.GossipPacket)
	}
	gossiper.rumorChannelsLock.Unlock()

	gossiper.sendNeighbour(gossipPacket, R)
	//fmt.Println("MONGERING with", R)

	// Start the ticker
	ticker := time.NewTicker(10 * time.Second)
	select {
	case newGossipPacket := <-gossiper.rumorChannels[R]:
		ticker.Stop()
		go gossiper.executeStatus(newGossipPacket.Status, gossipPacket, R)

	case <-ticker.C: // timeout, resend message
		ticker.Stop()
		// remove timeout peer from temporary knownPeers
		newKnownPeers := helpers.Filter(recursiveKnownPeers, R)

		go gossiper.sendRumorToPeers(gossipPacket, newKnownPeers, false)
	}
}

func (gossiper *Gossiper) sendSimpleToPeers(gossipPacket *packets.GossipPacket, sender string) {

	for _, peer := range gossiper.KnownPeers {
		// Don't send back to original sender peer
		if peer == sender {
			continue
		}
		gossiper.sendNeighbour(gossipPacket, peer)
	}
}

func (gossiper *Gossiper) SendAntiEntropy() {
	for {
		time.Sleep(time.Duration(gossiper.AntiEntropy) * time.Second)
		gossiper.KnowPeersLock.RLock()
		peer, found := helpers.GetRandomPeer(gossiper.KnownPeers)
		gossiper.KnowPeersLock.RUnlock()
		if found {
			gossiper.sendStatusPacket(peer)
		}
	}
}

func (gossiper *Gossiper) SendRouteRumor() {
	// startup route rumor
	gossipPacket := gossiper.createRumorPacket("")
	go gossiper.sendRumorToPeers(gossipPacket, gossiper.KnownPeers, false)

	for {
		time.Sleep(time.Duration(gossiper.rtimer) * time.Second)
		gossipPacket := gossiper.createRumorPacket("")
		go gossiper.sendRumorToPeers(gossipPacket, gossiper.KnownPeers, false)

	}
}

func (gossiper *Gossiper) sendDataReply(gossipPacket *packets.GossipPacket, data []byte) {
	reply := &packets.DataReply{
		Origin:      gossiper.Name,
		Destination: gossipPacket.DataRequest.Origin,
		HopLimit:    helpers.HopLimit - 1,
		HashValue:   gossipPacket.DataRequest.HashValue,
		Data:        data,
	}
	newGossipPacket := &packets.GossipPacket{
		DataReply: reply,
	}

	gossiper.sendTo(newGossipPacket, gossipPacket.DataRequest.Origin)
}

// Sends a search request and waits for helpers.SearchTimeout (1s) for replies or until helpers.MatchThreshold (2) matches found
// Returns true is search was successful, then download one of the files
func (gossiper *Gossiper) sendSearchRequest(keywords string, budget int) bool {
	fullMatches := 0
	searchRequest := &packets.SearchRequest{
		Origin:   gossiper.Name,
		Budget:   uint64(budget - 1),
		Keywords: strings.Split(keywords, ","),
	}

	// Process locally, not necessary according to forum
	//results, found := gossiper.searchFile(searchRequest.Keywords)
	//if found {
	//	for _, result := range results {
	//		fullMatches += gossiper.processSearchResult(result, gossiper.Name)
	//	}
	//}

	gossiper.forwardSearchRequest(searchRequest, "")

	firstMatchFound := false
	var firstFullResult *packets.SearchResult
	var firstMatchOrigin string
	// Loop until timeout or more than MatchThreshold matches
	ticker := time.NewTicker(helpers.SearchTimeout * time.Second)
	for {
		if fullMatches == helpers.MatchThreshold {
			fmt.Println("SEARCH FINISHED")
			go gossiper.downloadSearchedFile(firstFullResult, firstMatchOrigin)
			return true
		}
		select {
		case <-ticker.C:
			return false
		case searchReply := <-gossiper.searchChannel:
			for _, result := range searchReply.Results {
				matches := gossiper.processSearchResult(result, searchReply.Origin)
				fullMatches += matches
				if matches > 0 && !firstMatchFound {
					firstFullResult = result
					firstMatchOrigin = searchReply.Origin
					firstMatchFound = true
				}
			}
		}
	}
}

// Distributes the remaining budget over the known neighbours, except the sender of the packet
func (gossiper *Gossiper) forwardSearchRequest(request *packets.SearchRequest, sender string) {
	budget := int(request.Budget)
	if budget == 0 {
		return
	}

	gossiper.KnowPeersLock.RLock()
	neighbours := helpers.Filter(gossiper.KnownPeers, sender)
	gossiper.KnowPeersLock.RUnlock()

	if len(neighbours) == 0 {
		return
	}

	// Send ceil(q) to r nodes and floor(q) to len(neighbours) - r
	q := float64(budget) / float64(len(neighbours))
	r := budget % (len(neighbours))

	if gossiper.debug {
		fmt.Println("Forwarding search request with budget", request.Budget)
		fmt.Println("q is", q, "r is", r, "for budget", request.Budget)
	}
	oldBudget := request.Budget

	request.Budget = uint64(math.Ceil(q))
	gossipPacket := &packets.GossipPacket{
		SearchRequest: request,
	}
	for i, neighbour := range neighbours {
		if i < r {
			if gossiper.debug {
				fmt.Println("sending searchRequest to neighbour", neighbour, "with budget", request.Budget, "for total budget", oldBudget)
			}

			gossiper.sendNeighbour(gossipPacket, neighbour)
		} else {
			if int(q) == 0 {
				return
			}
			request.Budget = uint64(q)
			if gossiper.debug {
				fmt.Println("sending searchRequest to neighbour", neighbour, "with budget", request.Budget, "for total budget", oldBudget)
			}
			gossiper.sendNeighbour(gossipPacket, neighbour)
		}
	}
}

func (gossiper *Gossiper) broadcast(gossipPacket *packets.GossipPacket) {
	for _, peer := range gossiper.KnownPeers {
		gossiper.sendRumorToPeers(gossipPacket, []string{peer}, false)
	}
}

// Sends a gossipPacket to a destination via nextHop, returns true if nextHop is known
func (gossiper *Gossiper) sendTo(gossipPacket *packets.GossipPacket, destination string) bool {
	if nextHop, known := gossiper.routingTable.GetNextHop(destination); known {
		gossiper.sendNeighbour(gossipPacket, nextHop)
		return true
	} else {
		if gossiper.debug {
			fmt.Println("sendTo: Next hop not known")
		}
	}
	return false
}

// Sends a packet to a directly known neighbour
func (gossiper *Gossiper) sendNeighbour(gossipPacket *packets.GossipPacket, destination string) {
	packetBytes, err := protobuf.Encode(gossipPacket)
	if err != nil {
		fmt.Println(err)
		return
	}
	udpPeerAddr, err := net.ResolveUDPAddr("udp4", destination)
	if err != nil {
		fmt.Println(err)
		return
	}
	_, err = gossiper.Connection.PeerConn.WriteToUDP(packetBytes, udpPeerAddr)
	if err != nil {
		fmt.Println(err)
		return
	}
}


