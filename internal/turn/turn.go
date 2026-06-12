package turn

import (
	"context"
	"errors"
)

type Creds struct {
	URLs       []string
	Username   string
	Credential string
	ExpiresAt  uint64
}

type Provider interface {
	Fetch(ctx context.Context, joinLink string) (Creds, error)
}

var (
	ErrAuthExpired = errors.New("provider auth expired")
	ErrCallGone    = errors.New("call cannot be joined")
)
