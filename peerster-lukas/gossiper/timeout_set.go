package gossiper

import (
	"github.com/lukasdeloose/Peerster/helpers"
	"github.com/lukasdeloose/Peerster/packets"
	"strings"
	"sync"
	"time"
)

type RequestID struct {
	keywords string
	origin   string
}

func CreateRequestID(request *packets.SearchRequest) RequestID {
	return RequestID{
		keywords: strings.Join(request.Keywords, ","),
		origin:   request.Origin,
	}
}

type FileRequestsSet struct {
	requests    map[RequestID]time.Time //Set of current requests mapped on moment they were put in
	requestLock sync.Mutex
}

func NewFileRequestsSet() *FileRequestsSet {
	ts := &FileRequestsSet{requests: make(map[RequestID]time.Time)}
	go func() {
		for _ = range time.Tick(20 * time.Second) { //Clean up every 10 seconds so does not get too big
			ts.requestLock.Lock()
			for request, timePut := range ts.requests {
				if time.Now().Sub(timePut).Seconds() > helpers.DuplicateSearchTimeout {
					delete(ts.requests, request)
				}
			}
			ts.requestLock.Unlock()
		}
	}()
	return ts
}

func (frs *FileRequestsSet) Len() int {
	return len(frs.requests)
}

// Returns true if duplicate within timeout, else put in set
// Also updates time
func (frs *FileRequestsSet) isDuplicate(request RequestID) bool {
	timeout := false
	frs.requestLock.Lock()
	defer frs.requestLock.Unlock()

	lastTime, ok := frs.requests[request]
	if ok {
		if time.Now().Sub(lastTime).Seconds() < helpers.DuplicateSearchTimeout {
			timeout = true
		}
	}
	frs.requests[request] = time.Now()
	return timeout
}
