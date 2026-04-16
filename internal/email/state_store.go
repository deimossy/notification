package email

import (
	"context"
	"time"
)

type AcquireState int

const (
	AcquireStateAcquired AcquireState = iota
	AcquireStateAlreadyProcessed
	AcquireStateInProgress
)

type MessageStateStore interface {
	Acquire(ctx context.Context, messageID string, lockTTL time.Duration) (AcquireState, error)
	MarkProcessed(ctx context.Context, messageID string) error
	MarkFailed(ctx context.Context, messageID, failureReason string) error
}
