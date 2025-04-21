package transport

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/ThinkInAIXYZ/go-mcp/pkg"
)

type StreamableHTTPServerTransportOption func(*streamableHTTPServerTransport)

func WithStreamableHTTPServerTransportOptionLogger(logger pkg.Logger) StreamableHTTPServerTransportOption {
	return func(t *streamableHTTPServerTransport) {
		t.logger = logger
	}
}

func WithStreamableHTTPServerTransportOptionEndpoint(endpoint string) StreamableHTTPServerTransportOption {
	return func(t *streamableHTTPServerTransport) {
		t.mcpEndpoint = endpoint
	}
}

type StreamableHTTPServerTransportAndHandlerOption func(*streamableHTTPServerTransport)

func WithStreamableHTTPServerTransportAndHandlerOptionLogger(logger pkg.Logger) StreamableHTTPServerTransportAndHandlerOption {
	return func(t *streamableHTTPServerTransport) {
		t.logger = logger
	}
}

type streamableHTTPServerTransport struct {
	// ctx is the context that controls the lifecycle of the server
	ctx    context.Context
	cancel context.CancelFunc

	httpSvr *http.Server

	inFlySend sync.WaitGroup

	receiver serverReceiver

	sessionManager sessionManager

	// options
	logger      pkg.Logger
	mcpEndpoint string // The single MCP endpoint path
}

type StreamableHTTPHandler struct {
	transport *streamableHTTPServerTransport
}

// HandleMCP handles incoming MCP requests
func (h *StreamableHTTPHandler) HandleMCP() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.transport.handleMCPEndpoint(w, r)
	})
}

// NewStreamableHTTPServerTransportAndHandler returns transport without starting the HTTP server,
// and returns a Handler for users to start their own HTTP server externally
// eg:
// transport, handler, _ := NewStreamableHTTPServerTransportAndHandler()
// http.Handle("/mcp", handler.HandleMCP())
// http.ListenAndServe(":8080", nil)
func NewStreamableHTTPServerTransportAndHandler(
	opts ...StreamableHTTPServerTransportAndHandlerOption,
) (ServerTransport, *StreamableHTTPHandler, error) {
	ctx, cancel := context.WithCancel(context.Background())

	t := &streamableHTTPServerTransport{
		ctx:    ctx,
		cancel: cancel,
		logger: pkg.DefaultLogger,
	}

	for _, opt := range opts {
		opt(t)
	}

	return t, &StreamableHTTPHandler{transport: t}, nil
}

func NewStreamableHTTPServerTransport(addr string, opts ...StreamableHTTPServerTransportOption) (ServerTransport, error) {
	ctx, cancel := context.WithCancel(context.Background())

	t := &streamableHTTPServerTransport{
		ctx:         ctx,
		cancel:      cancel,
		logger:      pkg.DefaultLogger,
		mcpEndpoint: "/mcp", // Default MCP endpoint
	}

	for _, opt := range opts {
		opt(t)
	}

	mux := http.NewServeMux()
	mux.HandleFunc(t.mcpEndpoint, t.handleMCPEndpoint)

	t.httpSvr = &http.Server{
		Addr:        addr,
		Handler:     mux,
		IdleTimeout: time.Minute,
	}

	return t, nil
}

func (t *streamableHTTPServerTransport) Run() error {
	if t.httpSvr == nil {
		<-t.ctx.Done()
		return nil
	}

	if err := t.httpSvr.ListenAndServe(); err != nil {
		return fmt.Errorf("failed to start HTTP server: %w", err)
	}
	return nil
}

func (t *streamableHTTPServerTransport) Send(ctx context.Context, sessionID string, msg Message) error {
	t.inFlySend.Add(1)
	defer t.inFlySend.Done()

	select {
	case <-t.ctx.Done():
		return t.ctx.Err()
	default:
		return t.sessionManager.EnqueueMessage(ctx, sessionID, msg)
	}
}

func (t *streamableHTTPServerTransport) SetReceiver(receiver serverReceiver) {
	t.receiver = receiver
}

func (t *streamableHTTPServerTransport) SetSessionManager(manager sessionManager) {
	t.sessionManager = manager
}

func (t *streamableHTTPServerTransport) handleMCPEndpoint(w http.ResponseWriter, r *http.Request) {
	defer pkg.RecoverWithFunc(func(_ any) {
		t.writeError(w, http.StatusInternalServerError, "Internal server error")
	})

	switch r.Method {
	case http.MethodPost:
		t.handlePost(w, r)
	case http.MethodGet:
		t.handleGet(w, r)
	case http.MethodDelete:
		t.handleDelete(w, r)
	default:
		t.writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

func (t *streamableHTTPServerTransport) handlePost(w http.ResponseWriter, r *http.Request) {
	// Validate Accept header
	accept := r.Header.Get("Accept")
	if accept == "" {
		t.writeError(w, http.StatusBadRequest, "Missing Accept header")
		return
	}

	// Read and process the message
	bs, err := io.ReadAll(r.Body)
	if err != nil {
		t.writeError(w, http.StatusBadRequest, fmt.Sprintf("Invalid request: %v", err))
		return
	}

	// Disconnection SHOULD NOT be interpreted as the client cancelling its request.
	// To cancel, the client SHOULD explicitly send an MCP CancelledNotification.
	ctx := pkg.CancelShieldContext{Context: r.Context()}
	outputMsgCh, err := t.receiver.Receive(ctx, r.Header.Get(sessionIDHeader), bs)
	if err != nil {
		if errors.Is(err, pkg.ErrSessionClosed) {
			t.writeError(w, http.StatusNotFound, fmt.Sprintf("Failed to receive: %v", err))
			return
		}
		t.writeError(w, http.StatusBadRequest, fmt.Sprintf("Failed to receive: %v", err))
		return
	}

	if outputMsgCh == nil {
		w.WriteHeader(http.StatusAccepted)
		w.Header().Set("Content-Type", "application/json")
		return
	}

	if !strings.Contains(r.Header.Get("Accept"), "text/event-stream") {
		t.writeError(w, http.StatusBadRequest, "Must accept text/event-stream")
		return
	}

	if msg := <-outputMsgCh; len(msg) != 0 {
		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "application/json")
		// if { // 需要想办法获取到sessionid
		// 	w.Header().Set(sessionIDHeader, "")
		// }

		if _, err = w.Write(msg); err != nil {
			t.logger.Errorf("streamableHTTPServerTransport post write: %+v", err)
			return
		}
	}
}

func (t *streamableHTTPServerTransport) handleGet(w http.ResponseWriter, r *http.Request) {
	defer pkg.RecoverWithFunc(func(_ any) {
		t.writeError(w, http.StatusInternalServerError, "Internal server error")
	})

	if !strings.Contains(r.Header.Get("Accept"), "text/event-stream") {
		t.writeError(w, http.StatusBadRequest, "Must accept text/event-stream")
		return
	}

	// Set headers for SSE
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	// Create flush-supporting writer
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	sessionID := r.Header.Get(sessionIDHeader)
	if sessionID == "" {
		t.writeError(w, http.StatusBadRequest, "Missing Session ID")
		return
	}

	for {
		msg, err := t.sessionManager.DequeueMessage(r.Context(), sessionID)
		if err != nil {
			if !errors.Is(err, pkg.ErrSendEOF) {
				t.logger.Errorf("SSE stream getMessageForSend: %v, sessionID=%s", err, sessionID)
			}
			return
		}

		t.logger.Debugf("Sending message: %s", string(msg))

		if _, err = fmt.Fprintf(w, "data: %s\n\n", msg); err != nil {
			t.logger.Errorf("Failed to write message: %v", err)
			continue
		}
		flusher.Flush()
	}
}

func (t *streamableHTTPServerTransport) handleDelete(w http.ResponseWriter, r *http.Request) {
	sessionID := r.Header.Get("Mcp-Session-Id")
	if sessionID == "" {
		t.writeError(w, http.StatusBadRequest, "Missing session ID")
		return
	}

	t.sessionManager.CloseSession(sessionID)
	w.WriteHeader(http.StatusOK)
}

func (t *streamableHTTPServerTransport) writeError(w http.ResponseWriter, code int, message string) {
	t.logger.Errorf("streamableHTTPServerTransport Error: code: %d, message: %s", code, message)

	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(code)
	if _, err := w.Write([]byte(message)); err != nil {
		t.logger.Errorf("streamableHTTPServerTransport writeError: %v", err)
	}
}

func (t *streamableHTTPServerTransport) Shutdown(userCtx context.Context, serverCtx context.Context) error {
	shutdownFunc := func() {
		<-serverCtx.Done()

		t.cancel()

		t.inFlySend.Wait()

		t.sessionManager.CloseAllSessions()
	}

	if t.httpSvr == nil {
		shutdownFunc()
		return nil
	}

	t.httpSvr.RegisterOnShutdown(shutdownFunc)

	if err := t.httpSvr.Shutdown(userCtx); err != nil {
		return fmt.Errorf("failed to shutdown HTTP server: %w", err)
	}

	return nil
}
