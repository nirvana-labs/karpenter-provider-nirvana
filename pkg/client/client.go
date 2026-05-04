package client

import (
	"context"
	"time"

	nirvana "github.com/nirvana-labs/nirvana-go"
	"github.com/nirvana-labs/nirvana-go/lib"
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

func (c *Client) WaitForOperation(ctx context.Context, operationID string) error {
	return lib.NewOperationWaiter().
		WithTimeout(15 * time.Minute).
		WithPollInterval(5 * time.Second).
		Wait(ctx, &c.sdk, operationID)
}
