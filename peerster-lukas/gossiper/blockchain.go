package gossiper

import (
	"encoding/hex"
	"fmt"
	"github.com/lukasdeloose/Peerster/helpers"
	"github.com/lukasdeloose/Peerster/packets"
	"math/rand"
	"strings"
	"time"
)

func (gossiper *Gossiper) publishHw3ex2(blockPublish packets.BlockPublish) {
	gossiper.SequenceID++
	gossiper.vClock.increment(gossiper.Name, false)
	tlcMessage := &packets.TLCMessage{
		Origin:      gossiper.Name,
		ID:          gossiper.SequenceID,
		Confirmed:   -1,
		TxBlock:     blockPublish,
		VectorClock: nil,
		Fitness:     0,
	}
	gossipPacket := &packets.GossipPacket{TLCMessage: tlcMessage}

	gossiper.vClock.addMessage(gossipPacket, true)

	// Create channel to receive Acks
	ackChannel := make(chan *packets.TLCAck, helpers.Nodes)
	gossiper.ackChannelsLock.Lock()
	gossiper.ackChannels[tlcMessage.ID] = ackChannel
	gossiper.ackChannelsLock.Unlock()

	// Map to keep track of who already Acked
	ackMap := make(map[string]bool)

	witnesses := make([]string, 0, (helpers.Nodes/2)+1)
	witnesses = append(witnesses, gossiper.Name)

	if gossiper.debug {
		fmt.Println("length of witnesses before ", len(witnesses))
	}

	// start rumongering
	go gossiper.sendRumorToPeers(gossipPacket, gossiper.KnownPeers, false)

	// Ticker to countdown
	ticker := time.NewTicker(time.Duration(helpers.StubbornTimeout) * time.Second)
	for len(witnesses) < helpers.Nodes/2 {
		select {
		case ack := <-ackChannel:
			if _, duplicate := ackMap[ack.Origin]; !duplicate {
				// New ack
				witnesses = append(witnesses, ack.Origin)
			}
		case <-ticker.C:
			go gossiper.sendRumorToPeers(gossipPacket, gossiper.KnownPeers, false)
		default:
		}
	}
	if gossiper.debug {
		fmt.Println("length of witnesses after ", len(witnesses))
	}

	// Have majority ack, send confirmation
	gossiper.confirm(tlcMessage, witnesses)
}

func (gossiper *Gossiper) publishHw3ex3(blockPublish packets.BlockPublish, buffered bool, fitness float32) {
	myRound := gossiper.vClock.myRound // save, because could get incremented

	committedLength := len(gossiper.vClock.committedBlockChain)

	if committedLength > 0 && blockPublish.PrevHash == [32]byte{} {
		if gossiper.vClock.uncommittedBlock == nil {
			blockPublish.PrevHash = gossiper.vClock.committedBlockChain[committedLength-1].Hash()
		} else {
			blockPublish.PrevHash = gossiper.vClock.uncommittedBlock.Hash()
		}
	}

	if !buffered {
		select {
		case gossiper.vClock.sendThisRound <- true:
		// Listening, we did not send yet
		default:
			// Already sent this round
			fmt.Println("We already sent this round")
			gossiper.TLCClientChannel <- &blockPublish
			return
		}
	}

	if buffered {
		gossiper.vClock.sendThisRound <- true // Blocking send
	}

	gossiper.SequenceID++
	gossiper.vClock.increment(gossiper.Name, true)

	newFitness := float32(0)
	if fitness == float32(0) {
		newFitness = rand.Float32()
	} else {
		newFitness = fitness
	}

	tlcMessage := &packets.TLCMessage{
		Origin:      gossiper.Name,
		ID:          gossiper.SequenceID,
		Confirmed:   -1,
		TxBlock:     blockPublish,
		VectorClock: &packets.StatusPacket{Want: gossiper.vClock.createWant()},
		Fitness:     newFitness,
	}
	gossipPacket := &packets.GossipPacket{TLCMessage: tlcMessage}

	gossiper.vClock.addMessage(gossipPacket, true)

	// Create channel to receive Acks
	ackChannel := make(chan *packets.TLCAck, helpers.Nodes)
	gossiper.ackChannelsLock.Lock()
	gossiper.ackChannels[tlcMessage.ID] = ackChannel
	gossiper.ackChannelsLock.Unlock()

	// Map to keep track of who already Acked
	ackMap := make(map[string]bool)

	witnesses := make([]string, 0, (helpers.Nodes/2)+1)
	witnesses = append(witnesses, gossiper.Name)

	// start rumongering

	go gossiper.sendRumorToPeers(gossipPacket, gossiper.KnownPeers, false)

	// Ticker to countdown
	ticker := time.NewTicker(time.Duration(helpers.StubbornTimeout) * time.Second)
	for len(witnesses) <= helpers.Nodes/2 {
		//fmt.Println("in for loop, witnesses", len(witnesses))
		select {
		case roundAdvanced := <-gossiper.vClock.roundAdvance:
			// Check, because could also be from previous round
			//fmt.Println("Received round advance")
			if roundAdvanced == myRound {
				// Round advanced, stop waiting for ack's
				fmt.Println("Round advanced, stop waiting for ack's")
				return
			}
		case ack := <-ackChannel:
			if _, duplicate := ackMap[ack.Origin]; !duplicate {
				// New ack
				witnesses = append(witnesses, ack.Origin)
				ackMap[ack.Origin] = true
			}
		case <-ticker.C:
			fmt.Println("Resending publish")
			go gossiper.sendRumorToPeers(gossipPacket, gossiper.KnownPeers, false)
		}
	}
	// Have majority ack, send confirmation
	gossiper.confirm(tlcMessage, witnesses)

}

func (gossiper *Gossiper) confirm(tlcMessage *packets.TLCMessage, witnesses []string) {
	gossiper.SequenceID++
	gossiper.vClock.increment(gossiper.Name, false)

	tlcConfirm := &packets.TLCMessage{
		Origin:      tlcMessage.Origin,
		ID:          gossiper.SequenceID,
		Confirmed:   int(tlcMessage.ID),
		TxBlock:     tlcMessage.TxBlock,
		VectorClock: &packets.StatusPacket{Want: gossiper.vClock.createWant()},
		Fitness:     tlcMessage.Fitness,
	}

	fmt.Println("RE-BROADCAST ID", tlcMessage.ID, "WITNESSES", strings.Trim(strings.Join(strings.Fields(fmt.Sprint(witnesses)), ","), "[]"))

	gossipPacket := &packets.GossipPacket{TLCMessage: tlcConfirm}

	gossiper.vClock.addMessage(gossipPacket, true)

	go gossiper.sendRumorToPeers(gossipPacket, gossiper.KnownPeers, false)

	if gossiper.hw3ex3 {
		// Send to confirmation channel
		gossiper.vClock.confirmedRumorsChan <- tlcConfirm
	}
}

func (gossiper *Gossiper) canAck(tlcMessage *packets.TLCMessage) bool {
	if len(gossiper.vClock.committedBlockChain) == 0 {
		return true
	}

	// Check if name not taken
	for _, blockPublish := range gossiper.vClock.committedBlockChain {
		if blockPublish.Transaction.Name == tlcMessage.TxBlock.Transaction.Name {
			return false
		}
	}

	// Check if our confirmed is subset of other_block
	prevHash := tlcMessage.TxBlock.PrevHash
	lastCommit := gossiper.vClock.committedBlockChain[len(gossiper.vClock.committedBlockChain)-1]
	for i := 0; i < len(gossiper.vClock.committedBlockChain); i++ {
		prevBlock := gossiper.blocks[string(prevHash[:32])]
		if prevBlock.Hash() == lastCommit.Hash() {
			return true
		}
		prevHash = prevBlock.PrevHash
	}
	return false
}

func (gossiper *Gossiper) ackMessage(tlcMessage *packets.TLCMessage) {
	if gossiper.hw3ex4 {
		if !gossiper.canAck(tlcMessage) {
			fmt.Println("NOT ALLOWED TO ACK")
			return
		}
	}

	ack := &packets.TLCAck{Origin: gossiper.Name, ID: tlcMessage.ID, Destination: tlcMessage.Origin, HopLimit: helpers.HopLimit - 1}

	newGossipPacket := &packets.GossipPacket{Ack: ack}

	gossiper.sendTo(newGossipPacket, tlcMessage.Origin)
	fmt.Println("SENDING ACK origin", tlcMessage.Origin, "ID", tlcMessage.ID)
}

// Go routine that waits for new confirmations on channel
func (gossiper *Gossiper) checkRoundAdvance() {
	blockchainGossips := make(map[uint32][]*packets.TLCMessage) // Maps round on the confirmed gossips
	sendThisRound := false
	for {
		if !sendThisRound {
			//fmt.Println("Wait for sendThisRound")
			sendThisRound = <-gossiper.vClock.sendThisRound // Wait for sending this round
			blockchainGossips[gossiper.vClock.myRound] = make([]*packets.TLCMessage, 0)
		}
		sendThisRound = true
		confirmedGossip := <-gossiper.vClock.confirmedRumorsChan // Blocking receive
		blockchainGossips[gossiper.vClock.myRound] = append(blockchainGossips[gossiper.vClock.myRound], confirmedGossip)
		if len(blockchainGossips[gossiper.vClock.myRound]) > helpers.Nodes/2 {
			// Let publish now that we're advancing, so it stops waiting for ack's
			gossiper.vClock.roundAdvance <- gossiper.vClock.myRound
			// Lock, to keep sure state is consistent while advancing (e.g. no messages are sent through channel while advancing)
			gossiper.vClock.roundLock.Lock()
			gossiper.advanceRoundPrint(blockchainGossips[gossiper.vClock.myRound])
			gossiper.advanceRound()
			gossiper.vClock.roundLock.Unlock()
			// Publish buffered client message
			if gossiper.hw3ex4 {
				if gossiper.vClock.myRound%3 != 0 {
					highestFit, _ := gossiper.getHighestFit(blockchainGossips[gossiper.vClock.myRound-1])
					go gossiper.publishHw3ex3(highestFit.TxBlock, true, highestFit.Fitness)
				} else {
					block, consensus := gossiper.checkConsensus(blockchainGossips)
					if consensus {
						go gossiper.consensus(block)
					} else {
						go gossiper.noConsensus(blockchainGossips[gossiper.vClock.myRound-2])
					}
					select {
					case blockPublish := <-gossiper.TLCClientChannel:
						go gossiper.publishHw3ex3(*blockPublish, true, 0)
					default:
					}
				}
			} else {
				select {
				case blockPublish := <-gossiper.TLCClientChannel:
					go gossiper.publishHw3ex3(*blockPublish, true, 0)
				default:
				}
			}
			sendThisRound = false
		}
	}
}

func (gossiper *Gossiper) consensus(message *packets.TLCMessage) {
	gossiper.vClock.committedBlockChain = append(gossiper.vClock.committedBlockChain, &message.TxBlock)
	gossiper.vClock.uncommittedBlock = nil
	output := fmt.Sprint("CONSENSUS ON QSC round ", gossiper.vClock.myRound/3, " message origin ", message.Origin, " ID ", message.ID, " file names ")
	for _, block := range gossiper.vClock.committedBlockChain {
		output += fmt.Sprint(block.Transaction.Name, " ")
	}
	output += fmt.Sprint(" size ", message.TxBlock.Transaction.Size, " metahash ", hex.EncodeToString(message.TxBlock.Transaction.MetafileHash))
	fmt.Println(output)
}

func (gossiper *Gossiper) noConsensus(messages []*packets.TLCMessage) {
	highestFit, _ := gossiper.getHighestFit(messages)
	fmt.Println("No consensus, using ", highestFit.TxBlock.Transaction.Name)
	gossiper.vClock.uncommittedBlock = &highestFit.TxBlock
}

// Return highest fit and it's index
func (gossiper *Gossiper) getHighestFit(confirmedGossips []*packets.TLCMessage) (*packets.TLCMessage, int) {
	maxFit := float32(0)
	var maxGossip *packets.TLCMessage
	index := 0
	for i, gossip := range confirmedGossips {
		if gossip.Fitness > maxFit {
			maxGossip = gossip
			maxFit = gossip.Fitness
			index = i
		}
	}
	return maxGossip, index
}

func (gossiper *Gossiper) checkConsensus(blockchainGossips map[uint32][]*packets.TLCMessage) (*packets.TLCMessage, bool) {
	sRound := gossiper.vClock.myRound - 3
	for {
		highestFit_s, highestFitIndex_s := gossiper.getHighestFit(blockchainGossips[sRound])
		if helpers.ContainsTx(blockchainGossips[sRound+1], highestFit_s) {
			if helpers.ContainsTx(blockchainGossips[sRound+2], highestFit_s) {
				// Consensus reached
				return highestFit_s, true
			}
		}
		if len(blockchainGossips[sRound]) > 0 {
			// Delete this entry and look at next highest fit
			blockchainGossips[sRound] = helpers.DeleteTx(blockchainGossips[sRound], highestFitIndex_s)
		} else {
			return nil, false
		}
	}
}

func (gossiper *Gossiper) advanceRoundPrint(confirmedGossips []*packets.TLCMessage) {
	output := fmt.Sprint("ADVANCING TO round ", gossiper.vClock.myRound+1, " BASED ON CONFIRMED MESSAGES ")
	for i, confirmation := range confirmedGossips {
		output += fmt.Sprint("origin", i+1, " ", confirmation.Origin, " ID", i+1, " ", confirmation.Confirmed, " ")
	}
	fmt.Println(output)
}

func (gossiper *Gossiper) advanceRound() {
	// Clean channel for any old confirmations
	for len(gossiper.vClock.confirmedRumorsChan) > 0 {
		<-gossiper.vClock.confirmedRumorsChan
	}
	// Advance vClock
	gossiper.vClock.advanceRound()
	// Now check for messages that are waiting in the channels
	gossiper.vClock.TLCChannelsLock.RLock()
	defer gossiper.vClock.TLCChannelsLock.RUnlock()
	// Get confirmations of this round
	for _, TLCChannel := range gossiper.vClock.TLCChannels {
		// This has to be a confirmation message
		select {
		case TLCMessage := <-TLCChannel:
			if TLCMessage.Confirmed == -1 {
				fmt.Println("ERROR, message is no confirmation")
				fmt.Println("Origin", TLCMessage.Origin, "ID", TLCMessage.ID, "confirmation", TLCMessage.Confirmed)
				continue
			}
			gossiper.vClock.confirmedRumorsChan <- TLCMessage
		default:
		}
	}
}
