package session

import (
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	cmap "github.com/orcaman/concurrent-map/v2"

	"github.com/ThinkInAIXYZ/go-mcp/pkg"
	"github.com/ThinkInAIXYZ/go-mcp/protocol"
)

// State 定义了会话的状态和数据
type State struct {
	// Session ID
	ID string
	// Session 最后活跃时间
	LastActiveAt time.Time

	// 会话的消息通道，用于发送消息到客户端
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
		ID:                  uuid.New().String(),
		LastActiveAt:        time.Now(),
		MessageChan:         make(chan []byte, 64),
		ReqID2respChan:      cmap.New[chan *protocol.JSONRPCResponse](),
		SubscribedResources: cmap.New[struct{}](),
		ReceiveInitRequest:  *pkg.NewBoolAtomic(),
		Ready:               *pkg.NewBoolAtomic(),
	}
}
