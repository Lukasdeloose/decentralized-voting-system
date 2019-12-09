package gossiper

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"github.com/lukasdeloose/Peerster/helpers"
	"github.com/lukasdeloose/Peerster/packets"
	"strings"
)

func (gossiper *Gossiper) handleSimple(gossipPacket *packets.GossipPacket, addr string) {
	fmt.Println("SIMPLE MESSAGE origin", gossipPacket.Simple.OriginalName, "from", addr, "contents", gossipPacket.Simple.Contents)
	stringPeers := strings.Join(gossiper.KnownPeers, ",")
	fmt.Println("PEERS", stringPeers)

	// Change relay address to own address
	gossipPacket.Simple.RelayPeerAddr = gossiper.Connection.PeerAddress.String()

	gossiper.sendSimpleToPeers(gossipPacket, addr)
}

func (gossiper *Gossiper) handleRumor(gossipPacket *packets.GossipPacket, addr string, tlc bool) {

	if tlc {
		// Easy fix to work with previous implementations
		gossipPacket.Rumor = &packets.RumorMessage{
			Origin: gossipPacket.TLCMessage.Origin,
			ID:     gossipPacket.TLCMessage.ID,
		}
		if gossipPacket.TLCMessage.Confirmed != -1 {
			tx := gossipPacket.TLCMessage.TxBlock.Transaction
			gossipPacket.Rumor.Text = fmt.Sprint("CONFIRMED GOSSIP origin ", gossiper.Name, " ID ", gossipPacket.TLCMessage.Confirmed, " file name ", tx.Name, " size ", tx.Size, " metahash ", hex.EncodeToString(tx.MetafileHash))
		}
	}
	origin := gossipPacket.Rumor.Origin
	id := gossipPacket.Rumor.ID

	//gossiper.KnowPeersLock.RLock()
	//stringPeers := strings.Join(gossiper.KnownPeers, ",")
	//gossiper.KnowPeersLock.RUnlock()
	//fmt.Println("PEERS", stringPeers)

	if !gossiper.vClock.addMessage(gossipPacket, tlc) {
		// Already have the message, send status and return
		gossiper.sendStatusPacket(addr)
		return
	}

	fmt.Println("RUMOR origin", gossipPacket.Rumor.Origin, "from", addr, "ID", gossipPacket.Rumor.ID, "contents", gossipPacket.Rumor.Text)

	// Update Want
	sendToPeers := false
	nextID := gossiper.vClock.getNextPacketID(origin)

	if nextID == id || (nextID == 0 && id == 1) {
		// This was the next packet we needed (or the first of this peer)
		gossiper.vClock.increment(origin, false)
		sendToPeers = true

		// Check if we already have the next packets (to support out-of-order receiving)
		gossiper.vClock.updateNextPacketID(origin, gossiper.hw3ex3)

	} else if nextID < id {
		// The nextID is lower, we miss some messages
		// Don't increment want, send statusPacket back to receive missing message
		sendToPeers = true
	} else {
		// The nextID is higher, we already got the message
		// Discard it and send statusPacket back to ack
		gossiper.sendStatusPacket(addr)
		return
	}

	// Don't put yourself in routing table
	if gossiper.Name != origin {
		gossiper.routingTable.update(origin, id, gossipPacket.Rumor.Text, addr)
	}

	gossiper.sendStatusPacket(addr)
	gossiper.KnowPeersLock.RLock()
	knownPeers := helpers.Filter(gossiper.KnownPeers, addr)
	gossiper.KnowPeersLock.RUnlock()

	if tlc {
		// Set rumor to nil so receiver gets tlc
		gossipPacket.Rumor = nil
	}

	if sendToPeers {
		go gossiper.sendRumorToPeers(gossipPacket, knownPeers, false)
	}

}

func (gossiper *Gossiper) handleStatus(gossipPacket *packets.GossipPacket, addr string) {
	output := ""
	for _, want := range gossipPacket.Status.Want {
		output += "peer " + want.Identifier + " nextID " + fmt.Sprint(want.NextID) + " "
	}

	//fmt.Println("STATUS from", addr, output)
	//gossiper.KnowPeersLock.RLock()
	//stringPeers := strings.Join(gossiper.KnownPeers, ",")
	//fmt.Println("PEERS", stringPeers)
	//gossiper.KnowPeersLock.RUnlock()

	// Send to channel of this peer (if existing)
	if gossiper.rumorChannels[addr] != nil {
		select {
		case gossiper.rumorChannels[addr] <- gossipPacket:
		default:
			// anti-entropy or old ack
			gossiper.executeStatus(gossipPacket.Status, nil, addr)
		}
	} else {
		// No channel yet for this peer, so definitely anti-entropy
		gossiper.executeStatus(gossipPacket.Status, nil, addr)
	}
}

func (gossiper *Gossiper) handlePrivate(gossipPacket *packets.GossipPacket) {
	if gossipPacket.Private.Destination == gossiper.Name {
		fmt.Println("PRIVATE origin", gossipPacket.Private.Origin, "hop-limit", gossipPacket.Private.HopLimit, "contents", gossipPacket.Private.Text)
		gossiper.vClock.addPrivateMessage(gossipPacket, gossiper.Name)
		return
	}
	// Add own messages to gui
	if gossipPacket.Private.Origin == gossiper.Name {
		gossiper.vClock.addPrivateMessage(gossipPacket, gossiper.Name)
	}

	if gossipPacket.Private.HopLimit <= 0 {
		return
	}

	gossipPacket.Private.HopLimit--

	gossiper.sendTo(gossipPacket, gossipPacket.Private.Destination)

}

func (gossiper *Gossiper) handleDataRequest(gossipPacket *packets.GossipPacket) {
	fmt.Println("Handling dataRequest")
	if gossipPacket.DataRequest.Destination == gossiper.Name {
		data := gossiper.retrieveData(gossipPacket.DataRequest.HashValue)
		gossiper.sendDataReply(gossipPacket, data)
		return
	}
	if gossipPacket.DataRequest.HopLimit <= 0 {
		return
	}

	gossipPacket.DataRequest.HopLimit--

	gossiper.sendTo(gossipPacket, gossipPacket.DataRequest.Destination)

}

func (gossiper *Gossiper) handleDataReply(gossipPacket *packets.GossipPacket) {
	fmt.Println("Handling dataReply")
	reply := gossipPacket.DataReply
	if reply.Destination == gossiper.Name {
		if reply.Data == nil {
			fmt.Println("other peer did not have the data")
			return
		}

		// check if payload is correct
		h := sha256.New()
		h.Write(reply.Data)
		hash := hex.EncodeToString(h.Sum(nil))
		if hex.EncodeToString(reply.HashValue) != hash {
			fmt.Println("INCORRECT DATA")
			return
		}
		gossiper.fileChannelsLock.RLock()
		if fileChannel, ok := gossiper.fileChannels[reply.Origin][hash]; ok {
			fileChannel <- gossipPacket.DataReply
		}
		gossiper.fileChannelsLock.RUnlock()

		return
	}
	if gossipPacket.DataReply.HopLimit <= 0 {
		return
	}

	gossipPacket.DataReply.HopLimit--
	gossiper.sendTo(gossipPacket, gossipPacket.DataReply.Destination)

}

func (gossiper *Gossiper) handleSearchRequest(gossipPacket *packets.GossipPacket, sender string) {

	request := gossipPacket.SearchRequest

	if gossiper.debug {
		fmt.Println("handling searchRequest")
		fmt.Println("budget is", request.Budget, "Origin is", request.Origin)
	}

	if gossiper.searchRequestSet.isDuplicate(CreateRequestID(request)) {
		if gossiper.debug {
			fmt.Println("Duplicate searchRequest")
		}
		return
	} else {
		if gossiper.debug {
			fmt.Println("Not duplicate")
		}
	}

	results, found := gossiper.searchFile(request.Keywords)

	if found {
		if gossiper.debug {
			fmt.Println("found files!", results)
		}

		reply := &packets.SearchReply{
			Origin:      gossiper.Name,
			Destination: request.Origin,
			HopLimit:    helpers.HopLimit - 1,
			Results:     results,
		}

		newGossipPacket := &packets.GossipPacket{
			SearchReply: reply,
		}
		gossiper.sendTo(newGossipPacket, request.Origin)
	}

	request.Budget--
	gossiper.forwardSearchRequest(request, sender)
}

// returns if channel is full (assume channel is big enough and clientSearch is done)
func (gossiper *Gossiper) handleSearchReply(gossipPacket *packets.GossipPacket) {
	searchReply := gossipPacket.SearchReply

	if gossiper.debug {
		fmt.Println("Handling searchReply")
		fmt.Println(" Destination is", searchReply.Destination, "hoplimit is", searchReply.HopLimit)
	}

	if searchReply.Destination != gossiper.Name {
		// Forward
		searchReply.HopLimit--
		gossiper.sendTo(gossipPacket, searchReply.Destination)
		if gossiper.debug {
			fmt.Println("hoplimit", gossipPacket.SearchReply.HopLimit)
		}
		return
	}
	gossiper.searchChannel <- searchReply
}

func (gossiper *Gossiper) handleTLCMessage(gossipPacket *packets.GossipPacket, sender string) {
	tlcMessage := gossipPacket.TLCMessage

	tx := tlcMessage.TxBlock.Transaction
	if tlcMessage.Confirmed == -1 {
		fmt.Println("UNCONFIRMED GOSSIP origin", tlcMessage.Origin, "ID", tlcMessage.ID, "file name", tx.Name, "size", tx.Size, "metahash", hex.EncodeToString(tx.MetafileHash))
	} else {
		fmt.Println("CONFIRMED GOSSIP origin", gossiper.Name, "ID", tlcMessage.Confirmed, "file name", tx.Name, "size", tx.Size, "metahash", hex.EncodeToString(tx.MetafileHash))
	}

	// Rumonger to other nodes
	go gossiper.handleRumor(gossipPacket, sender, true)

	// Ack message
	ack := &packets.TLCAck{Origin: gossiper.Name, ID: tlcMessage.ID, Destination: tlcMessage.Origin, HopLimit: helpers.HopLimit - 1}

	newGossipPacket := &packets.GossipPacket{Ack: ack}

	gossiper.sendTo(newGossipPacket, tlcMessage.Origin)
	fmt.Println("SENDING ACK origin", tlcMessage.Origin, "ID", tlcMessage.ID)
}

func (gossiper *Gossiper) handleTLCAck(gossipPacket *packets.GossipPacket) {
	if gossipPacket.Ack.Destination != gossiper.Name {
		if gossipPacket.Ack.HopLimit <= 0 {
			return
		}
		gossipPacket.Ack.HopLimit--
		gossiper.sendTo(gossipPacket, gossipPacket.Ack.Destination)
		return
	}
	gossiper.ackChannelsLock.RLock()
	select {
	case gossiper.ackChannels[gossipPacket.Ack.ID] <- gossipPacket.Ack:
	default:
	}
	gossiper.ackChannelsLock.RUnlock()
}

func (gossiper *Gossiper) handleTLCRumor(gossipPacket *packets.GossipPacket, addr string) {
	tlcMessage := gossipPacket.TLCMessage
	confirmed := false // To know if we have to advance vClock
	origin := gossipPacket.TLCMessage.Origin
	id := gossipPacket.TLCMessage.ID
	text := ""
	tx := gossipPacket.TLCMessage.TxBlock.Transaction

	if gossipPacket.TLCMessage.Confirmed != -1 {
		confirmed = true
		// Just for GUI
		text = fmt.Sprint("CONFIRMED GOSSIP origin ", origin, " ID ", id, " file name ", tx.Name, " size ", tx.Size, " metahash ", hex.EncodeToString(tx.MetafileHash))

	} else {
		text = fmt.Sprint("UNCONFIRMED GOSSIP origin ", origin, " ID ", id, " file name ", tx.Name, " size ", tx.Size, " metahash ", hex.EncodeToString(tx.MetafileHash))
	}

	fmt.Println(text)

	gossiper.KnowPeersLock.RLock()
	knownPeers := helpers.Filter(gossiper.KnownPeers, addr)
	gossiper.KnowPeersLock.RUnlock()

	gossiper.vClock.lock(origin)
	defer gossiper.vClock.Unlock(origin)

	nextID := gossiper.vClock.getNextPacketID(origin)

	if !gossiper.vClock.addMessage(gossipPacket, true) {
		// Already have the message, send status, ack if necessary and return
		gossiper.sendStatusPacket(addr)
		if !confirmed && gossiper.vClock.compareRound(origin, confirmed, true) == helpers.SAMEROUND {
			gossiper.ackMessage(gossipPacket.TLCMessage)
		}
		return
	}

	if gossiper.hw3ex4 {
		// Add message block to map
		if confirmed {
			byteArray := tx.Hash()
			gossiper.blockLock.Lock()
			gossiper.blocks[string(byteArray[:32])] = &tlcMessage.TxBlock
			gossiper.blockLock.Unlock()
		}
	}

	if nextID < id {
		// The nextID is lower, we miss some messages
		// TLC: just save the message, will be handled through vClock.updateNextPacketID
		go gossiper.sendRumorToPeers(gossipPacket, knownPeers, false) // new rumour
		gossiper.sendStatusPacket(addr)
		return
	}

	fmt.Println("RUMOR origin", tlcMessage.Origin, "from", addr, "ID", tlcMessage.ID, "contents", text)

	//fmt.Println(text)

	// Don't put yourself in routing table
	if gossiper.Name != origin {
		gossiper.routingTable.update(origin, id, text, addr)
	}

	gossiper.vClock.roundLock.Lock()
	switch gossiper.vClock.compareRound(tlcMessage.Origin, confirmed, false) {
	case helpers.OLDROUND:
		// Nothing special
	case helpers.SAMEROUND:
		if !confirmed {
			gossiper.ackMessage(tlcMessage)
		} else {
			gossiper.vClock.confirmedRumorsChan <- tlcMessage
		}
	case helpers.FUTUREROUND:
		// If confirmation, send it through the channel. Unconfirmed gossips will be rebroadcasted, no need to ack
		if confirmed {
			gossiper.vClock.TLCChannelsLock.RLock()
			gossiper.vClock.TLCChannels[tlcMessage.Origin] <- gossipPacket.TLCMessage
			gossiper.vClock.TLCChannelsLock.RUnlock()
		}
	}
	// Check if we already have the next packets (to support out-of-order receiving)
	gossiper.vClock.increment(origin, !confirmed)
	gossiper.vClock.updateNextPacketID(origin, gossiper.hw3ex3)
	gossiper.vClock.roundLock.Unlock()

	return
}
