package transport

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/ThinkInAIXYZ/go-mcp/pkg"
)

const sessionIDHeader = "Mcp-Session-Id"

const eventIDHeader = "Last-Event-ID"

type StreamableHTTPClientTransportOption func(*streamableHTTPClientTransport)

func WithStreamableHTTPClientOptionReceiveTimeout(timeout time.Duration) StreamableHTTPClientTransportOption {
	return func(t *streamableHTTPClientTransport) {
		t.receiveTimeout = timeout
	}
}

func WithStreamableHTTPClientOptionHTTPClient(client *http.Client) StreamableHTTPClientTransportOption {
	return func(t *streamableHTTPClientTransport) {
		t.client = client
	}
}

func WithStreamableHTTPClientOptionLogger(log pkg.Logger) StreamableHTTPClientTransportOption {
	return func(t *streamableHTTPClientTransport) {
		t.logger = log
	}
}

type streamableHTTPClientTransport struct {
	ctx    context.Context
	cancel context.CancelFunc

	serverURL   *url.URL
	receiver    clientReceiver
	sessionID   *pkg.AtomicString
	lastEventID *pkg.AtomicString

	// options
	logger         pkg.Logger
	receiveTimeout time.Duration
	client         *http.Client
}

func NewStreamableHTTPClientTransport(serverURL string, opts ...StreamableHTTPClientTransportOption) (ClientTransport, error) {
	parsedURL, err := url.Parse(serverURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse server URL: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	t := &streamableHTTPClientTransport{
		ctx:            ctx,
		cancel:         cancel,
		serverURL:      parsedURL,
		sessionID:      pkg.NewAtomicString(),
		lastEventID:    pkg.NewAtomicString(),
		logger:         pkg.DefaultLogger,
		receiveTimeout: time.Second * 30,
		client:         http.DefaultClient,
	}

	for _, opt := range opts {
		opt(t)
	}

	return t, nil
}

func (t *streamableHTTPClientTransport) Start() error {
	// Start a GET stream for server-initiated messages
	go func() {
		defer pkg.Recover()

		t.startSSEStream()
	}()
	return nil
}

func (t *streamableHTTPClientTransport) Send(ctx context.Context, msg Message) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, t.serverURL.String(), bytes.NewReader(msg))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")

	if sessionID := t.sessionID.Load(); sessionID != "" {
		req.Header.Set(sessionIDHeader, sessionID)
	}

	resp, err := t.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}
	defer resp.Body.Close()

	// Handle session ID if provided in response
	respSessionID := resp.Header.Get(sessionIDHeader)
	localSessionID := t.sessionID.Load()
	if respSessionID != "" {
		if localSessionID != "" && respSessionID != localSessionID {
			return fmt.Errorf("failed to send message: session ID does not match")
		}
		if localSessionID == "" {
			t.sessionID.Store(respSessionID)
		}
	}

	// Handle different response types
	switch resp.Header.Get("Content-Type") {
	case "text/event-stream":
		go t.handleSSEStream(resp.Body)
		return nil
	case "application/json":
		// Handle immediate JSON response
		if resp.StatusCode == http.StatusAccepted {
			return nil // For notifications and responses
		}
		// Process JSON response
		if err = t.receiver.Receive(ctx, msg); err != nil {
			return fmt.Errorf("failed to process response: %w", err)
		}
		return nil
	default:
		return fmt.Errorf("unexpected content type: %s", resp.Header.Get("Content-Type"))
	}
}

func (t *streamableHTTPClientTransport) startSSEStream() {
	for {
		select {
		case <-t.ctx.Done():
			return
		default:
			req, err := http.NewRequestWithContext(t.ctx, http.MethodGet, t.serverURL.String(), nil)
			if err != nil {
				t.logger.Errorf("failed to create SSE request: %v", err)
				time.Sleep(time.Second)
				continue
			}

			req.Header.Set("Accept", "text/event-stream")
			if lastEventID := t.lastEventID.Load(); lastEventID != "" {
				req.Header.Set(eventIDHeader, lastEventID)
			}
			if sessionID := t.sessionID.Load(); sessionID != "" {
				req.Header.Set(sessionIDHeader, sessionID)
			}

			resp, err := t.client.Do(req)
			if err != nil {
				t.logger.Errorf("failed to connect to SSE stream: %v", err)
				time.Sleep(time.Second)
				continue
			}

			if resp.StatusCode == http.StatusMethodNotAllowed {
				resp.Body.Close()
				t.logger.Debugf("server does not support SSE streaming")
				return
			}

			t.handleSSEStream(resp.Body)
		}
	}
}

func (t *streamableHTTPClientTransport) handleSSEStream(reader io.ReadCloser) {
	defer reader.Close()

	go func() {
		defer pkg.Recover()

		<-t.ctx.Done()

		if err := reader.Close(); err != nil {
			t.logger.Errorf("failed to close SSE stream body: %+v", err)
			return
		}
	}()

	br := bufio.NewReader(reader)
	var _, data, id string

	for {
		line, err := br.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				// Process any pending event before exit
				if data != "" {
					t.processSSEEvent(data)
				}
				break
			}
			select {
			case <-t.ctx.Done():
				return
			default:
				t.logger.Errorf("SSE stream error: %v", err)
				return
			}
		}

		line = strings.TrimRight(line, "\r\n")

		if line == "" {
			// Empty line means end of event
			if data != "" {
				t.processSSEEvent(data)
				_, data, id = "", "", ""
			}
			continue
		}

		switch {
		case strings.HasPrefix(line, "event:"):
			_ = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		case strings.HasPrefix(line, "data:"):
			data = strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		case strings.HasPrefix(line, "id:"):
			id = strings.TrimSpace(strings.TrimPrefix(line, "id:"))
			t.lastEventID.Store(id)
		}
	}
}

func (t *streamableHTTPClientTransport) processSSEEvent(data string) {
	ctx, cancel := context.WithTimeout(t.ctx, t.receiveTimeout)
	defer cancel()

	if err := t.receiver.Receive(ctx, []byte(data)); err != nil {
		t.logger.Errorf("Error processing SSE event: %v", err)
	}
}

func (t *streamableHTTPClientTransport) SetReceiver(receiver clientReceiver) {
	t.receiver = receiver
}

func (t *streamableHTTPClientTransport) Close() error {
	t.cancel()

	if sessionID := t.sessionID.Load(); sessionID != "" {
		req, err := http.NewRequest(http.MethodDelete, t.serverURL.String(), nil)
		if err != nil {
			return err
		}
		req.Header.Set(sessionIDHeader, sessionID)
		resp, err := t.client.Do(req)
		if err != nil {
			return fmt.Errorf("failed to send message: %w", err)
		}
		defer resp.Body.Close()
	}

	return nil
}
