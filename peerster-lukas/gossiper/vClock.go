package gossiper

import (
	"encoding/hex"
	"fmt"
	"github.com/lukasdeloose/Peerster/helpers"
	"github.com/lukasdeloose/Peerster/packets"
	"sync"
)

type VClock struct {
	WantMapLock         sync.RWMutex
	WantMap             map[string][]uint32 //1st entry is total, 2nd is round peer is in
	MessagesLock        sync.RWMutex
	Messages            map[string]map[uint32]packets.GossipPacket
	PrivateMessagesLock sync.RWMutex
	PrivateMessages     map[string][]packets.PrivateMessage
	GuiMessageLock      sync.RWMutex
	GuiMessages         []packets.GUIMessage                // Add timestamp so we can display messages in order
	TLCChannels         map[string]chan *packets.TLCMessage //channel that holds tx that needs to be acked for every peer
	TLCChannelsLock     sync.RWMutex
	confirmedRumorsChan chan *packets.TLCMessage
	roundLock           sync.Mutex
	myRound             uint32
	sendThisRound       chan bool
	roundAdvance        chan uint32
	TLCPeerLocks        map[string]*sync.Mutex
	committedBlockChain []*packets.BlockPublish
	uncommittedBlock    *packets.BlockPublish
}

func newVClock() *VClock {
	return &VClock{
		WantMapLock:         sync.RWMutex{},
		WantMap:             make(map[string][]uint32),
		MessagesLock:        sync.RWMutex{},
		Messages:            make(map[string]map[uint32]packets.GossipPacket),
		PrivateMessagesLock: sync.RWMutex{},
		PrivateMessages:     make(map[string][]packets.PrivateMessage),
		GuiMessageLock:      sync.RWMutex{},
		TLCChannels:         make(map[string]chan *packets.TLCMessage),
		confirmedRumorsChan: make(chan *packets.TLCMessage, helpers.Nodes), //Can only have one confirmation per peer
		myRound:             0,
		roundAdvance:        make(chan uint32, 2),
		sendThisRound:       make(chan bool),
		TLCPeerLocks:        make(map[string]*sync.Mutex),
		committedBlockChain: make([]*packets.BlockPublish, 0), //Committed history
	}
}

// Round increments the round the other peer is in (hw3ex3)
func (v *VClock) increment(peerID string, round bool) map[string][]uint32 {
	v.WantMapLock.Lock()
	if _, ok := v.WantMap[peerID]; !ok {
		// First time, create want
		v.WantMap[peerID] = make([]uint32, 2)
		v.WantMap[peerID][0] = 1
	}
	v.WantMap[peerID][0]++
	if round {
		v.WantMap[peerID][1]++
	}
	v.WantMapLock.Unlock()
	return v.WantMap
}

// Tries to add the message, if message already present, return False, else return True
func (v *VClock) addMessage(gossipPacket *packets.GossipPacket, tlc bool) bool {
	var origin, text string
	var id uint32

	if tlc {
		origin = gossipPacket.TLCMessage.Origin
		id = gossipPacket.TLCMessage.ID
		if gossipPacket.TLCMessage.Confirmed != -1 {
			tx := gossipPacket.TLCMessage.TxBlock.Transaction
			text = fmt.Sprint("CONFIRMED GOSSIP origin ", gossipPacket.TLCMessage.Origin, " ID ", gossipPacket.TLCMessage.Confirmed, " file name ", tx.Name, " size ", tx.Size, " metahash ", hex.EncodeToString(tx.MetafileHash))
		}
	} else {
		origin = gossipPacket.Rumor.Origin
		id = gossipPacket.Rumor.ID
		text = gossipPacket.Rumor.Text
	}

	v.MessagesLock.Lock()
	defer v.MessagesLock.Unlock()
	_, peerKnown := v.Messages[origin]
	if !peerKnown {
		// Create new entry in map
		v.Messages[origin] = make(map[uint32]packets.GossipPacket)
		v.TLCChannelsLock.Lock()
		v.TLCChannels[origin] = make(chan *packets.TLCMessage, helpers.TLCBufferSize)
		v.TLCChannelsLock.Unlock()
	} else {
		_, messageKnown := v.Messages[origin][id]
		if messageKnown {
			// We already have the message
			return false
		}
	}
	v.Messages[origin][id] = *gossipPacket

	if tlc {
		// Create new so we don't have to create rumorfield in gossipPacket
		gossipPacket = &packets.GossipPacket{Rumor: &packets.RumorMessage{
			Origin: origin,
			ID:     id,
			Text:   text,
		}}
	}
	if gossipPacket.Rumor.Text != "" {
		v.addGuiMessage(gossipPacket)
	}
	return true
}

func (v *VClock) getNextPacketID(peerID string) uint32 {
	v.WantMapLock.RLock()
	defer v.WantMapLock.RUnlock()
	if _, know := v.WantMap[peerID]; !know {
		return 0
	}
	return v.WantMap[peerID][0]
}

func (v *VClock) getNextRound(peerID string) uint32 {
	v.WantMapLock.RLock()
	defer v.WantMapLock.RUnlock()
	if _, know := v.WantMap[peerID]; !know {
		return 0
	}
	return v.WantMap[peerID][1]
}

func (v *VClock) GetMyRound() uint32 {
	return v.myRound
}

// Starts from an ID and check if we already have other messages
// Returns first ID where no message is present
func (v *VClock) updateNextPacketID(peerID string, hw3ex3 bool) {
	v.WantMapLock.RLock()
	j := v.WantMap[peerID][0]
	v.WantMapLock.RUnlock()
	for ; ; j++ {
		v.MessagesLock.RLock()
		message, ok := v.Messages[peerID][j]
		v.MessagesLock.RUnlock()
		if ok {
			if hw3ex3 {
				// We already have the next packet
				updateRound := false
				if message.TLCMessage != nil {
					// We have the nex tlc message (can be confirmed or unconfirmed)
					if message.TLCMessage.Confirmed == -1 {
						// We update only when it's an unconfirmed message
						updateRound = true
					} else {
						// Confirmed message, check round and send to right channel
						switch v.compareRound(peerID, true, false) {
						case helpers.OLDROUND:
							// Nothing special
						case helpers.SAMEROUND:
							v.confirmedRumorsChan <- message.TLCMessage
						case helpers.FUTUREROUND:
							// Send confirmation through channel
							v.TLCChannelsLock.RLock()
							v.TLCChannels[message.TLCMessage.Origin] <- message.TLCMessage
							v.TLCChannelsLock.RUnlock()
						}
					}
				}
				v.increment(peerID, updateRound)
			} else {
				v.increment(peerID, false)
			}
		} else {
			// We don't have the next packet, break
			break
		}
	}
}

func (v *VClock) addGuiMessage(gossipPacket *packets.GossipPacket) {
	v.GuiMessageLock.Lock()
	v.GuiMessages = append(v.GuiMessages, packets.GUIMessage{
		Rumor:         gossipPacket.Rumor,
		MessageNumber: len(v.GuiMessages)})
	v.GuiMessageLock.Unlock()
}

func (v *VClock) addPrivateMessage(gossipPacket *packets.GossipPacket, name string) {
	v.PrivateMessagesLock.Lock()
	defer v.PrivateMessagesLock.Unlock()
	var otherPeer string
	if gossipPacket.Private.Origin == name {
		otherPeer = gossipPacket.Private.Destination // we sent the message
	} else {
		otherPeer = gossipPacket.Private.Origin // we received the message
	}
	if otherPeer == "" {
		return
	}

	if _, ok := v.PrivateMessages[otherPeer]; !ok {
		v.PrivateMessages[otherPeer] = make([]packets.PrivateMessage, 0)
	}
	v.PrivateMessages[otherPeer] = append(v.PrivateMessages[otherPeer], *gossipPacket.Private)
}

func (v *VClock) createWant() []packets.PeerStatus {
	want := make([]packets.PeerStatus, 0)
	v.WantMapLock.RLock()
	defer v.WantMapLock.RUnlock()
	for peerID, packetID := range v.WantMap {
		want = append(want, packets.PeerStatus{
			Identifier: peerID,
			NextID:     packetID[0],
		})
	}
	return want
}

func (v *VClock) compareWants(packet *packets.StatusPacket) (*packets.GossipPacket, bool) {
	// Compare the statusPacket with our vClock
	// return gossipPackets that other peer needs, and a bool indicating if we want messages from the other peer
	want := false

	otherPeerMap := helpers.CreateMapFromSlice(packet.Want)

	defer v.WantMapLock.RUnlock()
	v.WantMapLock.RLock()
	for peerID, nextPacketID := range v.WantMap {
		if _, knowsPeer := otherPeerMap[peerID]; !knowsPeer {
			// Other peer doesn't know this peer
			// Send the first message
			v.MessagesLock.RLock()
			newGossipPacket, ok := v.Messages[peerID][1]
			v.MessagesLock.RUnlock()
			if ok {
				return &newGossipPacket, false
			}
			//fmt.Println("We don't have the first message")
		}
		if nextPacketID[0] > otherPeerMap[peerID] {
			// We (A, sender) have new messages for the peer (1)
			v.MessagesLock.RLock()
			newGossipPacket, ok := v.Messages[peerID][otherPeerMap[peerID]]
			v.MessagesLock.RUnlock()
			if ok {
				return &newGossipPacket, false
			} else {
				// inconsistensies when re-entering with the same node ID
				// Could reset want to value of missing message

				// This shouldn't happen, otherwise we have inconsistencies
				//fmt.Println(nextPacketID)
				//fmt.Println(otherPeerMap[peerID])
				//fmt.Println(v.Messages[peerID])
				//fmt.Println("ERROR: We don't have this message")
			}
		} else if nextPacketID[0] < otherPeerMap[peerID] {
			// Peer (R) has new messages for us (2)
			// Can't send controlPacket directly because we first have to check if peer needs messages from us
			want = true
		}
	}
	// Check if peer knows peers that we don't
	for peerID := range otherPeerMap {
		if _, knowsPeer := otherPeerMap[peerID]; !knowsPeer {
			// We don't know this peer
			return nil, want
		}
	}
	return nil, want
}

func (v *VClock) compareRound(peer string, confirmed, old bool) int {
	// If old message, round will already have advanced, but actually still the same round
	v.WantMapLock.Lock()
	if _, ok := v.WantMap[peer]; !ok {
		// First time, create want
		v.WantMap[peer] = make([]uint32, 2)
		v.WantMap[peer][0] = 1
		v.WantMap[peer][1] = 0

	}
	otherRound := v.WantMap[peer][1]
	v.WantMapLock.Unlock()

	myRound := v.myRound

	// Round advances from the moment we get an unconfirmed message of the peer
	// It's a confirmation, or old message, so we think he is already in the next round
	if confirmed || old {
		otherRound--
	}

	if otherRound < myRound {
		// Old round, don't ack!
		return helpers.OLDROUND
	}
	if otherRound == myRound {
		// Correct round, ack!
		return helpers.SAMEROUND
	}
	// Message from the future, just save and don't ack!
	return helpers.FUTUREROUND
}

func (v *VClock) advanceRound() {
	v.myRound++
}

func (v *VClock) lock(peer string) {
	if _, ok := v.TLCPeerLocks[peer]; !ok {
		v.TLCPeerLocks[peer] = &sync.Mutex{}
	}
	v.TLCPeerLocks[peer].Lock()
}

func (v *VClock) Unlock(peer string) {
	v.TLCPeerLocks[peer].Unlock()
}
