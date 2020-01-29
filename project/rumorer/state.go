package rumorer

import (
	"fmt"
	. "github.com/lukasdeloose/decentralized-voting-system/project/constants"
	. "github.com/lukasdeloose/decentralized-voting-system/project/udp"
	. "github.com/lukasdeloose/decentralized-voting-system/project/utils"
	"sync"
)

type State struct {
	// Vector clock data structure that encodes the state
	state      map[string]uint32
	stateMutex *sync.RWMutex

	// All accepted messages, indexed by Origin
	messages      map[string]map[uint32] MongerableMessage

	// Outgoing communication channel to send StatePackets
	out chan *AddrGossipPacket
}

func NewState(out chan *AddrGossipPacket) *State {
	return &State{
		state:         make(map[string]uint32),
		stateMutex:    &sync.RWMutex{},
		messages:      make(map[string]map[uint32] MongerableMessage),
		out:           out,
	}
}

func (s *State) Messages() []*RumorMessage {
	s.stateMutex.RLock()
	defer s.stateMutex.RUnlock()

	// Return messages as list (not map)
	res := make([]*RumorMessage, 0)
	for _, msgs := range s.messages {
		for _, v := range msgs {
			gossip := v.ToGossip()
			if gossip.Rumor != nil && gossip.Rumor.Text != "" { // don't include the route rumors and tlc messages
				res = append(res, gossip.Rumor)
			}
		}
	}
	return res
}

func (s *State) Message(origin string, id uint32) (rumor MongerableMessage) {
	// Return message
	s.stateMutex.RLock()
	defer s.stateMutex.RUnlock()

	return s.messages[origin][id]
}

func (s *State) Compare(msg *StatusPacket) (iHave *PeerStatus, youHave *PeerStatus) {
	// Returns a I have, that he doesn't have, or the other way around if I can't give him any more messages
	s.stateMutex.RLock()
	defer s.stateMutex.RUnlock()

	iHave, youHave = nil, nil

	// Keep a set of the origins the other peer knows about
	origins := make(map[string]bool)

	// Iterate over status of the other peer
	for _, entry := range msg.Want {
		// Add the origin, to the set of origins the other peer knows about
		origins[entry.Identifier] = true

		nextID, exists := s.state[entry.Identifier]
		if !exists {
			nextID = 1
		}
		if nextID > entry.NextID {
			// I am in front
			iHave = &PeerStatus{
				Identifier: entry.Identifier,
				NextID:     entry.NextID, // This is the ID of the message I should send
			}
			if Debug {
				fmt.Printf("[DEBUG] CompareStatus: I AM IN FRONT\n")
			}
			return
		}
	}

	// origins that the peer does not now about: send the message with ID 1
	for origin, _ := range s.state {
		if !origins[origin] {
			if s.state[origin] > 1 {
				iHave = &PeerStatus{
					Identifier: origin,
					NextID:     1, // This is the ID of the message I have
				}
				if Debug {
					fmt.Printf("[DEBUG] CompareStatus: I AM IN FRONT\n")
				}
				return
			}
		}
	}

	for _, entry := range msg.Want {
		nextID, exists := s.state[entry.Identifier]
		if !exists {
			nextID = 1
		}
		if nextID < entry.NextID {
			// He is in front
			youHave = &PeerStatus{
				Identifier: entry.Identifier,
				NextID:     nextID, // This is the ID of the message I want
			}
			if Debug {
				fmt.Printf("[DEBUG] CompareStatus: HE IS IN FRONT\n")
			}
			return
		}
	}
	return
}

func (s *State) Update(msg MongerableMessage) (res []MongerableMessage) {
	s.stateMutex.Lock()
	defer s.stateMutex.Unlock()

	curr, exists := s.state[msg.GetOrigin()]
	if !exists {
		curr = 1
	}
	if curr <= msg.GetID() {
		// Save the message
		if _, ok := s.messages[msg.GetOrigin()]; !ok {
			s.messages[msg.GetOrigin()] = make(map[uint32] MongerableMessage)
		}
		s.messages[msg.GetOrigin()][msg.GetID()] = msg

		// Update the vector clock
		msgs := make([]MongerableMessage, 0)
		ok := true
		var m MongerableMessage
		for ok {
			m, ok = s.messages[msg.GetOrigin()][curr]
			if ok {
				msgs = append(msgs, m)
			}
			curr += 1
		}
		s.state[msg.GetOrigin()] = curr-1

		res = msgs
	} else {
		res = make([]MongerableMessage, 0)
	}
	return res
}

func (s *State) ToStatusPacket() *StatusPacket{
	s.stateMutex.RLock()
	defer s.stateMutex.RUnlock()

	// Construct StatusPacket
	want := make([]PeerStatus, 0)
	for origin, next := range s.state {
		want = append(want, PeerStatus{origin, next})
	}
	return &StatusPacket{Want: want}
}

func (s *State) Send(addr UDPAddr) {
	// Send it on the outgoing communication channel
	gossip := GossipPacket{Status: s.ToStatusPacket()}
	s.out <- &AddrGossipPacket{addr, &gossip}
}
