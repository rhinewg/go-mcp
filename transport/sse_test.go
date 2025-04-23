package transport

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/ThinkInAIXYZ/go-mcp/pkg"
)

func TestSSE(t *testing.T) {
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
	serverURL := fmt.Sprintf("http://%s/sse", serverAddr)

	if svr, err = NewSSEServerTransport(serverAddr); err != nil {
		t.Fatalf("NewSSEServerTransport failed: %v", err)
	}

	if client, err = NewSSEClientTransport(serverURL); err != nil {
		t.Fatalf("NewSSEClientTransport failed: %v", err)
	}

	testTransport(t, client, svr)
}

func TestSSEHandler(t *testing.T) {
	var (
		messageURL = "/message"
		port       int

		err    error
		svr    ServerTransport
		client ClientTransport
	)

	// Get an available port
	port, err = getAvailablePort()
	if err != nil {
		t.Fatalf("Failed to get available port: %v", err)
	}

	serverAddr := fmt.Sprintf("http://127.0.0.1:%d", port)
	serverURL := fmt.Sprintf("%s/sse", serverAddr)

	svr, handler, err := NewSSEServerTransportAndHandler(fmt.Sprintf("%s%s", serverAddr, messageURL))
	if err != nil {
		t.Fatalf("NewSSEServerTransport failed: %v", err)
	}

	// 设置 HTTP 路由
	http.Handle("/sse", handler.HandleSSE())
	http.Handle(messageURL, handler.HandleMessage())

	errCh := make(chan error, 1)
	go func() {
		if e := http.ListenAndServe(fmt.Sprintf(":%d", port), nil); e != nil {
			log.Fatalf("Failed to start HTTP server: %v", e)
		}
	}()

	// Use select to handle potential errors
	select {
	case err = <-errCh:
		t.Fatalf("http.ListenAndServe() failed: %v", err)
	case <-time.After(time.Second):
		// Server started normally
	}

	if client, err = NewSSEClientTransport(serverURL); err != nil {
		t.Fatalf("NewSSEClientTransport failed: %v", err)
	}

	testTransport(t, client, svr)
}

// getAvailablePort returns a port that is available for use
func getAvailablePort() (int, error) {
	addr, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, fmt.Errorf("failed to get available port: %v", err)
	}
	defer func() {
		if err = addr.Close(); err != nil {
			fmt.Println(err)
		}
	}()

	port := addr.Addr().(*net.TCPAddr).Port
	return port, nil
}

func Test_joinPath(t *testing.T) {
	type args struct {
		u    *url.URL
		elem []string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "1",
			args: args{
				u: func() *url.URL {
					uri, err := url.Parse("https://google.com/api/v1")
					if err != nil {
						panic(err)
					}
					return uri
				}(),
				elem: []string{"/test"},
			},
			want: "https://google.com/api/v1/test",
		},
		{
			name: "2",
			args: args{
				u: func() *url.URL {
					uri, err := url.Parse("/api/v1")
					if err != nil {
						panic(err)
					}
					return uri
				}(),
				elem: []string{"/test"},
			},
			want: "/api/v1/test",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			joinPath(tt.args.u, tt.args.elem...)
			if got := tt.args.u.String(); got != tt.want {
				t.Errorf("joinPath() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_sseClientTransport_handleSSEEvent(t1 *testing.T) {
	type fields struct {
		serverURL *url.URL
		logger    pkg.Logger
	}
	type args struct {
		event string
		data  string
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   string
	}{
		{
			name: "1",
			fields: fields{
				serverURL: func() *url.URL {
					uri, err := url.Parse("https://api.baidu.com/mcp")
					if err != nil {
						panic(err)
					}
					return uri
				}(),
				logger: pkg.DefaultLogger,
			},
			args: args{
				event: "endpoint",
				data:  "/sse/messages",
			},
			want: "https://api.baidu.com/sse/messages",
		},
		{
			name: "2",
			fields: fields{
				serverURL: func() *url.URL {
					uri, err := url.Parse("https://api.baidu.com/mcp")
					if err != nil {
						panic(err)
					}
					return uri
				}(),
				logger: pkg.DefaultLogger,
			},
			args: args{
				event: "endpoint",
				data:  "https://api.google.com/sse/messages",
			},
			want: "https://api.google.com/sse/messages",
		},
	}
	for _, tt := range tests {
		t1.Run(tt.name, func(t1 *testing.T) {
			t := &sseClientTransport{
				serverURL:    tt.fields.serverURL,
				logger:       tt.fields.logger,
				endpointChan: make(chan struct{}),
			}
			t.handleSSEEvent(tt.args.event, tt.args.data)
			if t.messageEndpoint.String() != tt.want {
				t1.Errorf("handleSSEEvent() = %v, want %v", t.messageEndpoint.String(), tt.want)
			}
		})
	}
}
