package transport

import (
	"context"
)

type streamableHTTPClientTransport struct{}

func NewStreamableHTTPClientTransport(serverURL string) (ClientTransport, error) {
	return &streamableHTTPClientTransport{}, nil
}

func (t *streamableHTTPClientTransport) Start() error {
	// TODO implement me
	panic("implement me")
}

func (t *streamableHTTPClientTransport) Send(ctx context.Context, msg Message) error {
	// TODO implement me
	panic("implement me")
}

func (t *streamableHTTPClientTransport) SetReceiver(receiver clientReceiver) {
	// TODO implement me
	panic("implement me")
}

func (t *streamableHTTPClientTransport) Close() error {
	// TODO implement me
	panic("implement me")
}
