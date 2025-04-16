package transport

import (
	"context"
)

type streamableHTTPServerTransport struct{}

func NewStreamableHTTPServerTransport(serverURL string) (ServerTransport, error) {
	return &streamableHTTPServerTransport{}, nil
}

func (t *streamableHTTPServerTransport) Run() error {
	// TODO implement me
	panic("implement me")
}

func (t *streamableHTTPServerTransport) Send(ctx context.Context, sessionID string, msg Message) error {
	// TODO implement me
	panic("implement me")
}

func (t *streamableHTTPServerTransport) SetReceiver(receiver serverReceiver) {
	// TODO implement me
	panic("implement me")
}

func (t *streamableHTTPServerTransport) SetSessionManager(manager sessionManager) {
	// TODO implement me
	panic("implement me")
}

func (t *streamableHTTPServerTransport) Shutdown(userCtx context.Context, serverCtx context.Context) error {
	// TODO implement me
	panic("implement me")
}
