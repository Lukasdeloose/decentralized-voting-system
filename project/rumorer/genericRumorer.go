package rumorer

import (
	. "github.com/lukasdeloose/decentralized-voting-system/project/confirmationRumorer"
	. "github.com/lukasdeloose/decentralized-voting-system/project/udp"
	. "github.com/lukasdeloose/decentralized-voting-system/project/utils"
)

// Interface that defines all functions the simple/normal rumorer must implement
type GenericRumorer interface {
	Run()

	// Needed for the webserver
	Name() string
	Messages() []*RumorMessage
	Peers() []UDPAddr
	AddPeer(addr UDPAddr)
	UIIn() chan *Message
	TLCIn() chan *TLCMessageWithReplyChan
	TLCOut() chan *TLCMessage
}
