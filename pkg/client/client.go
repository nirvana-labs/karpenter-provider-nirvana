package client

import (
	"context"
	"fmt"
	"time"

	nirvana "github.com/nirvana-labs/nirvana-go"
	"github.com/nirvana-labs/nirvana-go/operations"
	"github.com/nirvana-labs/nirvana-go/option"
)

// Client wraps the Nirvana SDK client and adds NKS-specific operations.
type Client struct {
	sdk nirvana.Client

	// PollInterval is the interval between operation status checks.
	PollInterval time.Duration
	// MaxPollDuration is the maximum time to wait for an operation to complete.
	MaxPollDuration time.Duration
}

// New creates a new Nirvana API client.
func New(apiKey string, opts ...option.RequestOption) *Client {
	sdk := nirvana.NewClient(
		append([]option.RequestOption{option.WithAPIKey(apiKey)}, opts...)...,
	)
	return &Client{
		sdk:             sdk,
		PollInterval:    10 * time.Second,
		MaxPollDuration: 30 * time.Minute,
	}
}

// WaitForOperation polls an operation until it reaches a terminal state.
func (c *Client) WaitForOperation(ctx context.Context, operationID string) (*operations.Operation, error) {
	deadline := time.Now().Add(c.MaxPollDuration)
	ticker := time.NewTicker(c.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			if time.Now().After(deadline) {
				return nil, fmt.Errorf("operation %s timed out after %s", operationID, c.MaxPollDuration)
			}

			op, err := c.sdk.Operations.Get(ctx, operationID)
			if err != nil {
				return nil, fmt.Errorf("getting operation %s: %w", operationID, err)
			}

			switch op.Status {
			case operations.OperationStatusDone:
				return op, nil
			case operations.OperationStatusFailed:
				return op, fmt.Errorf("operation %s failed", operationID)
			case operations.OperationStatusPending, operations.OperationStatusRunning:
				continue
			default:
				return op, fmt.Errorf("operation %s has unexpected status: %s", operationID, op.Status)
			}
		}
	}
}
