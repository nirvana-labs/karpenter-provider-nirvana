package client

import (
	nirvana "github.com/nirvana-labs/nirvana-go"
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
