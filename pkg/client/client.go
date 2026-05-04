package client

import (
	"context"
	"fmt"
	"time"

	nirvana "github.com/nirvana-labs/nirvana-go"
	"github.com/nirvana-labs/nirvana-go/lib"
	"github.com/nirvana-labs/nirvana-go/operations"
	"github.com/nirvana-labs/nirvana-go/option"
)

type Client struct {
	sdk nirvana.Client
}

func New(apiKey string, opts ...option.RequestOption) *Client {
	sdk := nirvana.NewClient(
		append([]option.RequestOption{option.WithAPIKey(apiKey)}, opts...)...,
	)
	return &Client{sdk: sdk}
}

func (c *Client) SDKClient() *nirvana.Client {
	return &c.sdk
}

func (c *Client) WaitForOperation(ctx context.Context, operationID string) (*operations.Operation, error) {
	if err := lib.NewOperationWaiter().
		WithTimeout(15 * time.Minute).
		WithPollInterval(5 * time.Second).
		Wait(ctx, &c.sdk, operationID); err != nil {
		return nil, err
	}

	op, err := c.sdk.Operations.Get(ctx, operationID)
	if err != nil {
		return nil, fmt.Errorf("fetching completed operation %s: %w", operationID, err)
	}
	return op, nil
}
