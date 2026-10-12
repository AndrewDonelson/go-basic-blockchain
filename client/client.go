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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/AndrewDonelson/go-basic-blockchain/sdk"
)

// apiPrefix is the versioned mount every endpoint here lives under.
const apiPrefix = "/v1"

// maxResponseBytes bounds how much of a reply is read. A node is trusted to
// answer honestly about the chain, not to be well-behaved about memory.
const maxResponseBytes = 8 << 20

// anchorPageSize is how many anchors are requested per page while listing.
const anchorPageSize = 1000

// ErrNotFound is returned when the node answers 404: no such publisher, no
// publisher with that key, or no such sidechain.
var ErrNotFound = errors.New("not found")

// APIError is a non-success reply from the node.
//
// Reason is set when the node rejected a transaction (422), and carries the
// node's own explanation -- a bad signature, a stale nonce, insufficient funds.
type APIError struct {
	StatusCode int
	Message    string
	Reason     string
}

// Error implements error.
func (e *APIError) Error() string {
	switch {
	case e.Reason != "":
		return fmt.Sprintf("node rejected the request (%d): %s", e.StatusCode, e.Reason)
	case e.Message != "":
		return fmt.Sprintf("node returned %d: %s", e.StatusCode, e.Message)
	default:
		return fmt.Sprintf("node returned %d", e.StatusCode)
	}
}

// Is lets errors.Is(err, ErrNotFound) match a 404.
func (e *APIError) Is(target error) bool {
	return target == ErrNotFound && e.StatusCode == http.StatusNotFound
}

// Client talks to one node.
type Client struct {
	baseURL    *url.URL
	apiKey     string
	header     string
	httpClient *http.Client
}

// Option configures a Client.
type Option func(*Client)

// WithHTTPClient replaces the HTTP client, for custom TLS, proxies or
// timeouts. The default has a 30-second timeout.
func WithHTTPClient(httpClient *http.Client) Option {
	return func(c *Client) {
		if httpClient != nil {
			c.httpClient = httpClient
		}
	}
}

// WithAPIKeyHeader sets the header the API key is sent in, for a node
// configured with API_KEY_HEADER. The default is Authorization. The value is
// always sent as "Bearer <key>", which is the form the node parses whatever the
// header is called.
func WithAPIKeyHeader(name string) Option {
	return func(c *Client) {
		if strings.TrimSpace(name) != "" {
			c.header = name
		}
	}
}

// New creates a client for the node at baseURL (scheme and host, optionally a
// path prefix; "/v1" is added by the client).
func New(baseURL, apiKey string, opts ...Option) (*Client, error) {
	parsed, err := url.Parse(strings.TrimRight(strings.TrimSpace(baseURL), "/"))
	if err != nil {
		return nil, fmt.Errorf("invalid base URL: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, fmt.Errorf("invalid base URL %q: the scheme must be http or https", baseURL)
	}
	if parsed.Host == "" {
		return nil, fmt.Errorf("invalid base URL %q: no host", baseURL)
	}
	if strings.TrimSpace(apiKey) == "" {
		return nil, errors.New("an API key is required: every application endpoint is authenticated")
	}

	c := &Client{
		baseURL:    parsed,
		apiKey:     apiKey,
		header:     "Authorization",
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
	for _, opt := range opts {
		opt(c)
	}
	return c, nil
}

// Submit sends a signed transaction and returns the node's full reply.
//
// A transaction the node already knows is a success, reported with Known set:
// an application that lost the reply to its first attempt can retry without the
// retry looking like a failure.
func (c *Client) Submit(ctx context.Context, tx sdk.Transaction) (sdk.TransactionSubmission, error) {
	if tx == nil {
		return sdk.TransactionSubmission{}, errors.New("transaction is nil")
	}
	body, err := sdk.EncodeTransaction(tx)
	if err != nil {
		return sdk.TransactionSubmission{}, fmt.Errorf("encode transaction: %w", err)
	}

	var reply sdk.TransactionSubmission
	status, raw, err := c.do(ctx, http.MethodPost, apiPrefix+"/transactions", nil, body)
	if err != nil {
		return sdk.TransactionSubmission{}, err
	}

	switch status {
	case http.StatusOK, http.StatusAccepted:
		if err := json.Unmarshal(raw, &reply); err != nil {
			return sdk.TransactionSubmission{}, fmt.Errorf("decode submission reply: %w", err)
		}
		return reply, nil
	case http.StatusUnprocessableEntity:
		// A rejection carries a submission body with the node's reason.
		if err := json.Unmarshal(raw, &reply); err == nil && reply.Reason != "" {
			return reply, &APIError{StatusCode: status, Reason: reply.Reason}
		}
	}
	return sdk.TransactionSubmission{}, apiError(status, raw)
}

// SubmitTransaction sends a signed transaction and returns its id.
//
// Being queued is not being mined: the id is how the application follows the
// transaction (GET /v1/blockchain/transactions/{id}) or, for a registration,
// the signal to start polling FindPublisherByKey.
func (c *Client) SubmitTransaction(ctx context.Context, tx sdk.Transaction) (string, error) {
	reply, err := c.Submit(ctx, tx)
	if err != nil {
		return "", err
	}
	return reply.ID, nil
}

// Publisher returns a registered publisher and its games. A publisher that is
// not registered is ErrNotFound.
func (c *Client) Publisher(ctx context.Context, publisherID uint64) (*sdk.PublisherInfo, error) {
	if publisherID == 0 {
		return nil, fmt.Errorf("%w: publisher id must not be zero", sdk.ErrInvalidSidechainID)
	}
	return c.getPublisher(ctx, apiPrefix+"/publishers/"+strconv.FormatUint(publisherID, 10), nil)
}

// FindPublisherByKey returns the publisher registered with an identity key,
// given the key's PEM. A key with no publisher is ErrNotFound -- which, right
// after submitting a registration, means "not mined yet".
func (c *Client) FindPublisherByKey(ctx context.Context, publicKeyPEM string) (*sdk.PublisherInfo, error) {
	fingerprint, err := sdk.PublisherKeyFingerprint(publicKeyPEM)
	if err != nil {
		return nil, err
	}
	return c.FindPublisherByFingerprint(ctx, fingerprint)
}

// FindPublisherByFingerprint is FindPublisherByKey for a caller that already
// has the fingerprint (sdk.PublisherKeyFingerprint, or a PeerIdentity's NodeID).
func (c *Client) FindPublisherByFingerprint(ctx context.Context, fingerprint string) (*sdk.PublisherInfo, error) {
	query := url.Values{"key": []string{strings.ToLower(strings.TrimSpace(fingerprint))}}
	return c.getPublisher(ctx, apiPrefix+"/publishers", query)
}

// Anchors returns every anchor recorded on chain for a sidechain, oldest first.
// A sidechain whose publisher or game is not registered is ErrNotFound; a
// registered one with nothing anchored yet returns an empty slice.
func (c *Client) Anchors(ctx context.Context, id sdk.SidechainID) ([]sdk.AnchorInfo, error) {
	if err := id.Validate(); err != nil {
		return nil, err
	}

	path := fmt.Sprintf("%s/sidechains/%d/%d/anchors", apiPrefix, id.PublisherID, id.GameID)
	anchors := make([]sdk.AnchorInfo, 0)

	for page := 1; ; page++ {
		query := url.Values{
			"page":  []string{strconv.Itoa(page)},
			"limit": []string{strconv.Itoa(anchorPageSize)},
		}
		status, raw, err := c.do(ctx, http.MethodGet, path, query, nil)
		if err != nil {
			return nil, err
		}
		if status != http.StatusOK {
			return nil, apiError(status, raw)
		}

		var list sdk.AnchorList
		if err := json.Unmarshal(raw, &list); err != nil {
			return nil, fmt.Errorf("decode anchors: %w", err)
		}
		anchors = append(anchors, list.Anchors...)

		// Stop on a short page as well as on the total, so a node that clamps
		// the limit lower than requested still terminates the loop.
		if len(list.Anchors) == 0 || len(anchors) >= list.Total {
			return anchors, nil
		}
	}
}

// AnchorCovering returns the on-chain anchor that commits a sidechain height,
// already converted into the sdk.SidechainAnchor that sdk.VerifyRecord takes.
// A height no anchor covers yet is ErrNotFound.
func (c *Client) AnchorCovering(ctx context.Context, id sdk.SidechainID, height uint64) (sdk.SidechainAnchor, error) {
	anchors, err := c.Anchors(ctx, id)
	if err != nil {
		return sdk.SidechainAnchor{}, err
	}
	for _, info := range anchors {
		if info.Covers(height) {
			return info.Anchor()
		}
	}
	return sdk.SidechainAnchor{}, fmt.Errorf("%w: no anchor for %s covers height %d",
		ErrNotFound, id, height)
}

// getPublisher fetches and decodes a publisher reply.
func (c *Client) getPublisher(ctx context.Context, path string, query url.Values) (*sdk.PublisherInfo, error) {
	status, raw, err := c.do(ctx, http.MethodGet, path, query, nil)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, apiError(status, raw)
	}

	var info sdk.PublisherInfo
	if err := json.Unmarshal(raw, &info); err != nil {
		return nil, fmt.Errorf("decode publisher: %w", err)
	}
	return &info, nil
}

// do performs one request and returns the status and body.
func (c *Client) do(ctx context.Context, method, path string, query url.Values, body []byte) (int, []byte, error) {
	target := *c.baseURL
	target.Path = strings.TrimRight(target.Path, "/") + path
	target.RawQuery = query.Encode()

	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, target.String(), reader)
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set(c.header, "Bearer "+c.apiKey)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return 0, nil, fmt.Errorf("read reply: %w", err)
	}
	return resp.StatusCode, raw, nil
}

// apiError builds an APIError from the node's error envelope.
func apiError(status int, raw []byte) error {
	var envelope sdk.ErrorResponse
	if err := json.Unmarshal(raw, &envelope); err == nil && envelope.Message != "" {
		return &APIError{StatusCode: status, Message: envelope.Message}
	}
	return &APIError{StatusCode: status, Message: strings.TrimSpace(string(raw))}
}
