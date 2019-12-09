package helpers

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"github.com/lukasdeloose/Peerster/packets"
	"math/rand"
	"reflect"
)

func ClientCase(message *packets.Message) int {
	if hex.EncodeToString(*message.Request) != "" {
		return REQUEST
	}
	if *message.File != "" {
		return FILE
	}
	if message.Text != "" {
		if *message.Destination != "" {
			return PRIVATE
		}
		return GOSSIP
	}
	if message.KeyWords != "" {
		return SEARCH
	}
	return 0
}

func HashToString(data []byte) string {
	h := sha256.New()
	h.Write(data)
	hash := h.Sum(nil)
	h.Reset()
	return hex.EncodeToString(hash)
}

func TypeOfMessage(gossipPacket *packets.GossipPacket) int {
	if gossipPacket.Simple != nil {
		return packets.SIMPLE
	}
	if gossipPacket.Rumor != nil {
		return packets.RUMOR
	}
	if gossipPacket.Status != nil {
		return packets.STATUS
	}
	if gossipPacket.Private != nil {
		return packets.PRIVATE
	}
	if gossipPacket.DataRequest != nil {
		return packets.DATAREQUEST
	}
	if gossipPacket.DataReply != nil {
		return packets.DATAREPLY
	}
	if gossipPacket.SearchRequest != nil {
		return packets.SEARCHREQUEST
	}
	if gossipPacket.SearchReply != nil {
		return packets.SEARCHREPLY
	}
	if gossipPacket.TLCMessage != nil {
		return packets.TLCMESSAGE
	}
	if gossipPacket.Ack != nil {
		return packets.TLCACK
	}
	return 0
}

func Filter(slice []string, toRemove string) []string {
	// Copy so we don't change original slice
	toCut := make([]string, len(slice))
	copy(toCut, slice)

	for i, peer := range toCut {
		if peer == toRemove {
			if i != len(toCut)-1 {
				return append(toCut[:i], toCut[i+1:]...)
			} else {
				return toCut[:i]
			}
		}
	}
	return toCut
}

func AppendIfMissing(peers []string, newPeer string) []string {
	for _, peer := range peers {
		if peer == newPeer {
			return peers
		}
	}
	return append(peers, newPeer)
}

func GetRandomPeer(Peers []string) (string, bool) {
	if len(Peers) < 1 {
		return "", false
	}

	// Pick random receiver from known peers
	randomPeer := Peers[rand.Intn(len(Peers))]
	return randomPeer, true
}

func CreateMapFromSlice(slice []packets.PeerStatus) map[string]uint32 {
	result := make(map[string]uint32)
	for _, peerStatus := range slice {
		result[peerStatus.Identifier] = peerStatus.NextID
	}
	return result
}

func ContainsTx(s []*packets.TLCMessage, highestFit *packets.TLCMessage) (bool) {
	for _, a := range s {
		if reflect.DeepEqual(a.TxBlock, highestFit.TxBlock) {
			return true
		}
	}
	return false
}

func DeleteTx(s []*packets.TLCMessage, i int) []*packets.TLCMessage {
	s[len(s)-1], s[i] = s[i], s[len(s)-1]
	return s[:len(s)-1]
}

func Check(e error) bool {
	if e != nil {
		fmt.Println(e)
		return false
	}
	return true
}
