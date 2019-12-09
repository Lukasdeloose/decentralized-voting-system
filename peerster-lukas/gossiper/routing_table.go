package gossiper

import (
	"fmt"
	"strings"
	"sync"
)

type RoutingTable struct {
	table       map[string]map[string]string
	max         map[string]uint32 //Keep the most recent ID to still support out-of-order receiving of messages
	routingLock sync.RWMutex
}

func newRoutingTable() *RoutingTable {
	return &RoutingTable{
		table: make(map[string]map[string]string),
		max:   make(map[string]uint32),
	}
}

func (r *RoutingTable) addEntry(peerID string) {
	// Already locked in update function
	r.table[peerID] = make(map[string]string)
	r.max[peerID] = 0
}

func (r *RoutingTable) update(peerID string, messageID uint32, text, addr string) {
	addrSplit := strings.Split(addr, ":")

	r.routingLock.Lock()
	defer r.routingLock.Unlock()
	if _, ok := r.table[peerID]; !ok {
		r.addEntry(peerID)
	}
	if messageID > r.max[peerID] {
		r.max[peerID] = messageID
		r.table[peerID][addrSplit[0]] = addrSplit[1]
		if text != "" {
			fmt.Println("DSDV", peerID, addrSplit[0]+":"+addrSplit[1])
		}
	}
}

func (r *RoutingTable) GetNextHop(entry string) (string, bool) {
	r.routingLock.RLock()
	defer r.routingLock.RUnlock()
	if nextHop, ok := r.table[entry]; ok {
		var nextHopAddr string
		for host, port := range nextHop {
			nextHopAddr = host + ":" + port
		}
		return nextHopAddr, true
	}
	return "", false
}
