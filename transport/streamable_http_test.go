package transport

import (
	"fmt"
	"testing"
)

func TestStreamableHTTP(t *testing.T) {
	var (
		err    error
		svr    ServerTransport
		client ClientTransport
	)

	// Get an available port
	port, err := getAvailablePort()
	if err != nil {
		t.Fatalf("Failed to get available port: %v", err)
	}

	serverAddr := fmt.Sprintf("127.0.0.1:%d", port)
	serverURL := fmt.Sprintf("http://%s/mcp", serverAddr)

	svr = NewStreamableHTTPServerTransport(serverAddr)

	if client, err = NewStreamableHTTPClientTransport(serverURL); err != nil {
		t.Fatalf("NewStreamableHTTPClientTransport failed: %v", err)
	}

	testTransport(t, client, svr)
}
