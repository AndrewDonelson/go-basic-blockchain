// Package client is a Go client for a node's application-facing API.
//
// It is the other half of the endpoints in sdk/api_handlers_sidechain.go: an
// application builds and signs transactions with the sdk package -- a REGISTER
// for its publisher and games, an ANCHOR committing its sidechain -- and uses
// this package to hand them to a node and to read back what consensus made of
// them.
//
//	c, err := client.New("https://node.example:8100", apiKey)
//	id, err := c.SubmitTransaction(ctx, registration)
//	// ...once the registration is mined:
//	publisher, err := c.FindPublisherByKey(ctx, identity.PublicPEM)
//	anchors, err := c.Anchors(ctx, sdk.SidechainID{PublisherID: publisher.PublisherID, GameID: 1})
//
// The reply types are the sdk's own (sdk.PublisherInfo, sdk.AnchorInfo,
// sdk.TransactionSubmission). The client deliberately keeps no copies of them:
// a client with its own definitions is one renamed field away from silently
// decoding zeroes.
package client

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/AndrewDonelson/go-basic-blockchain/sdk"
	"github.com/AndrewDonelson/go-basic-blockchain/sidechain"
)

// The implementation is sidechain.Client, which applications that must not link
// the node use directly; this package adds submission of the node's own
// transaction types.

// ErrNotFound is returned when the node answers 404: no such publisher, no
// publisher with that key, or no such sidechain.
var ErrNotFound = sidechain.ErrNotFound

// APIError is a non-success reply from the node.
type APIError = sidechain.APIError

// Option configures a Client.
type Option = sidechain.Option

// WithHTTPClient replaces the HTTP client, for custom TLS, proxies or
// timeouts. The default has a 30-second timeout.
func WithHTTPClient(httpClient *http.Client) Option { return sidechain.WithHTTPClient(httpClient) }

// WithAPIKeyHeader sets the header the API key is sent in, for a node
// configured with API_KEY_HEADER. The default is Authorization.
func WithAPIKeyHeader(name string) Option { return sidechain.WithAPIKeyHeader(name) }

// Client talks to one node.
type Client struct {
	*sidechain.Client
}

// New creates a client for the node at baseURL (scheme and host, optionally a
// path prefix; "/v1" is added by the client).
func New(baseURL, apiKey string, opts ...Option) (*Client, error) {
	c, err := sidechain.NewClient(baseURL, apiKey, opts...)
	if err != nil {
		return nil, err
	}
	return &Client{Client: c}, nil
}

// Submit sends a signed transaction and returns the node's full reply. A
// transaction the node already knows is a success, reported with Known set.
func (c *Client) Submit(ctx context.Context, tx sdk.Transaction) (sdk.TransactionSubmission, error) {
	if tx == nil {
		return sdk.TransactionSubmission{}, errors.New("transaction is nil")
	}
	body, err := sdk.EncodeTransaction(tx)
	if err != nil {
		return sdk.TransactionSubmission{}, fmt.Errorf("encode transaction: %w", err)
	}
	return c.SubmitRaw(ctx, body)
}

// SubmitTransaction sends a signed transaction and returns its id.
//
// Being queued is not being mined: the id is how the application follows the
// transaction (Transaction / WaitConfirmed) or, for a registration, the signal
// to start polling FindPublisherByKey.
func (c *Client) SubmitTransaction(ctx context.Context, tx sdk.Transaction) (string, error) {
	reply, err := c.Submit(ctx, tx)
	if err != nil {
		return "", err
	}
	return reply.ID, nil
}
