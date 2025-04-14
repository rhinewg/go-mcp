package transport

import (
	"fmt"
	"io"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
)

type mock struct {
	reader *io.PipeReader
	writer *io.PipeWriter
	closer io.Closer
}

func (m *mock) Write(p []byte) (n int, err error) {
	return m.writer.Write(p)
}

func (m *mock) Close() error {
	if err := m.writer.Close(); err != nil {
		return err
	}
	if err := m.reader.Close(); err != nil {
		return err
	}
	if err := m.closer.Close(); err != nil {
		return err
	}
	return nil
}

func TestStdioTransport(t *testing.T) {
	var (
		err    error
		server *stdioServerTransport
		client *stdioClientTransport
	)

	mockServerTrPath := filepath.Join(os.TempDir(), "mock_server_tr_"+strconv.Itoa(rand.Int()))
	if err = compileMockStdioServerTr(mockServerTrPath); err != nil {
		t.Fatalf("Failed to compile mock server: %v", err)
	}

	defer func(name string) {
		if err = os.Remove(name); err != nil {
			fmt.Printf("Failed to remove mock server: %v\n", err)
		}
	}(mockServerTrPath)

	clientT, err := NewStdioClientTransport(mockServerTrPath, []string{})
	if err != nil {
		t.Fatalf("NewStdioClientTransport failed: %v", err)
	}

	client = clientT.(*stdioClientTransport)
	server = NewStdioServerTransport().(*stdioServerTransport)

	// Create pipes for communication
	reader1, writer1 := io.Pipe()
	reader2, writer2 := io.Pipe()

	// Set up the communication channels
	server.reader = reader2
	server.writer = writer1
	client.reader = reader1
	client.writer = &mock{
		reader: reader1,
		writer: writer2,
		closer: client.writer,
	}

	testTransport(t, client, server)
}

func compileMockStdioServerTr(outputPath string) error {
	cmd := exec.Command("go", "build", "-o", outputPath, "../testdata/mock_block_server.go")

	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("compilation failed: %v\nOutput: %s", err, output)
	}

	return nil
}
