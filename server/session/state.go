package session

import (
	"sync/atomic"
	"time"

	cmap "github.com/orcaman/concurrent-map/v2"

	"github.com/ThinkInAIXYZ/go-mcp/pkg"
	"github.com/ThinkInAIXYZ/go-mcp/protocol"
)

type State struct {
	LastActiveAt time.Time

	MessageChan chan []byte

	RequestID int64

	ReqID2respChan cmap.ConcurrentMap[string, chan *protocol.JSONRPCResponse]

	// cache client initialize request info
	ClientInfo         *protocol.Implementation
	ClientCapabilities *protocol.ClientCapabilities

	// subscribed resources
	SubscribedResources cmap.ConcurrentMap[string, struct{}]

	ReceiveInitRequest atomic.Value
	Ready              atomic.Value
}

func NewState() *State {
	return &State{
		LastActiveAt:        time.Now(),
		MessageChan:         make(chan []byte, 64),
		ReqID2respChan:      cmap.New[chan *protocol.JSONRPCResponse](),
		SubscribedResources: cmap.New[struct{}](),
		ReceiveInitRequest:  *pkg.NewBoolAtomic(),
		Ready:               *pkg.NewBoolAtomic(),
	}
}
