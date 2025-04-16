package tests

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/ThinkInAIXYZ/go-mcp/client"
	"github.com/ThinkInAIXYZ/go-mcp/protocol"
	"github.com/ThinkInAIXYZ/go-mcp/transport"
)

func test(t *testing.T, runServer func() error, transportClient transport.ClientTransport) {
	errCh := make(chan error, 1)
	go func() {
		errCh <- runServer()
	}()

	// Use select to handle potential errors
	select {
	case err := <-errCh:
		t.Fatalf("server.Run() failed: %v", err)
	case <-time.After(time.Second * 5):
		// Server started normally
	}

	// Create MCP client using transport
	mcpClient, err := client.NewClient(transportClient, client.WithClientInfo(protocol.Implementation{
		Name:    "Example MCP Client",
		Version: "1.0.0",
	}))
	if err != nil {
		t.Fatalf("Failed to create MCP client: %v", err)
	}
	defer func() {
		if err = mcpClient.Close(); err != nil {
			t.Fatalf("Failed to close MCP client: %v", err)
			return
		}
	}()

	// List available tools
	toolsResult, err := mcpClient.ListTools(context.Background())
	if err != nil {
		t.Fatalf("Failed to list tools: %v", err)
	}
	bytes, _ := json.Marshal(toolsResult)
	fmt.Printf("Available tools: %s\n", bytes)

	// Call tool
	callResult, err := mcpClient.CallTool(
		context.Background(),
		protocol.NewCallToolRequest("current time", map[string]interface{}{
			"timezone": "UTC",
		}))
	if err != nil {
		t.Fatalf("Failed to call tool: %v", err)
	}
	bytes, _ = json.Marshal(callResult)
	fmt.Printf("Tool call result: %s\n", bytes)
}

func compileMockStdioServerTr() (string, error) {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	mockServerTrPath := filepath.Join(os.TempDir(), "mock_server_tr_"+strconv.Itoa(r.Int()))

	cmd := exec.Command("go", "build", "-o", mockServerTrPath, "../examples/everything/main.go")

	if output, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("compilation failed: %v\nOutput: %s", err, output)
	}

	return mockServerTrPath, nil
}
