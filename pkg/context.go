package pkg

import (
	"context"
	"time"
)

type CancelShieldContext struct {
	context.Context
}

func (v CancelShieldContext) Deadline() (deadline time.Time, ok bool) {
	return
}

func (v CancelShieldContext) Done() <-chan struct{} {
	return nil
}

func (v CancelShieldContext) Err() error {
	return nil
}
