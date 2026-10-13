// File sidechain/client.go - A client for a node's application-facing API.
package sidechain

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
)

// Client talks to one node's application endpoints: submit a signed
// transaction, read publishers and anchors, and look up accounts and
// transactions. It is the dependency-free counterpart of the node's own client
// package, which wraps it.
//
//	c, err := sidechain.NewClient("https://node.example:8100", apiKey)
//	acct, err := c.Account(ctx, wallet.Address())
//	tx, err := sidechain.NewAnchorTransaction(wallet, anchor, sig, sidechain.TxOptions{Nonce: acct.NextNonce()})
//	_, err = c.Submit(ctx, tx)
//	info, err := c.WaitConfirmed(ctx, tx.ID(), time.Second)

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

// BaseURL is the node's address as configured.
func (c *Client) BaseURL() string { return c.baseURL.String() }

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

// NewClient creates a client for the node at baseURL (scheme and host, optionally a
// path prefix; "/v1" is added by the client).
func NewClient(baseURL, apiKey string, opts ...Option) (*Client, error) {
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

// Submit sends a signed transaction (anything whose JSON is the node's wire
// form: a *Transaction from this package, or a node sdk.Transaction) and returns
// the node's full reply.
//
// A transaction the node already knows is a success, reported with Known set:
// an application that lost the reply to its first attempt can retry without the
// retry looking like a failure.
func (c *Client) Submit(ctx context.Context, tx json.Marshaler) (TransactionSubmission, error) {
	if tx == nil {
		return TransactionSubmission{}, errors.New("transaction is nil")
	}
	body, err := tx.MarshalJSON()
	if err != nil {
		return TransactionSubmission{}, fmt.Errorf("encode transaction: %w", err)
	}
	return c.SubmitRaw(ctx, body)
}

// SubmitRaw sends an already-encoded transaction.
func (c *Client) SubmitRaw(ctx context.Context, body []byte) (TransactionSubmission, error) {
	var reply TransactionSubmission
	status, raw, err := c.do(ctx, http.MethodPost, apiPrefix+"/transactions", nil, body)
	if err != nil {
		return TransactionSubmission{}, err
	}

	switch status {
	case http.StatusOK, http.StatusAccepted:
		if err := json.Unmarshal(raw, &reply); err != nil {
			return TransactionSubmission{}, fmt.Errorf("decode submission reply: %w", err)
		}
		return reply, nil
	case http.StatusUnprocessableEntity:
		// A rejection carries a submission body with the node's reason.
		if err := json.Unmarshal(raw, &reply); err == nil && reply.Reason != "" {
			return reply, &APIError{StatusCode: status, Reason: reply.Reason}
		}
	}
	return TransactionSubmission{}, apiError(status, raw)
}

// Transaction reports whether a transaction is pending or mined, and where. An
// id this node doesn't know is ErrNotFound.
func (c *Client) Transaction(ctx context.Context, id string) (*TransactionInfo, error) {
	status, raw, err := c.do(ctx, http.MethodGet, apiPrefix+"/transactions/"+url.PathEscape(id), nil, nil)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, apiError(status, raw)
	}
	var info TransactionInfo
	if err := json.Unmarshal(raw, &info); err != nil {
		return nil, fmt.Errorf("decode transaction status: %w", err)
	}
	return &info, nil
}

// WaitConfirmed polls Transaction every interval until the transaction is in a
// block, ctx ends, or the node stops knowing it (dropped from the mempool:
// ErrNotFound, so the caller can rebuild and resubmit).
func (c *Client) WaitConfirmed(ctx context.Context, id string, interval time.Duration) (*TransactionInfo, error) {
	if interval <= 0 {
		interval = time.Second
	}
	for {
		info, err := c.Transaction(ctx, id)
		if err != nil {
			return nil, err
		}
		if info.Status == SubmissionConfirmed {
			return info, nil
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(interval):
		}
	}
}

// Account returns an address's spendable balance and nonces.
func (c *Client) Account(ctx context.Context, address string) (*AccountInfo, error) {
	status, raw, err := c.do(ctx, http.MethodGet, apiPrefix+"/accounts/"+url.PathEscape(address), nil, nil)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, apiError(status, raw)
	}
	var info AccountInfo
	if err := json.Unmarshal(raw, &info); err != nil {
		return nil, fmt.Errorf("decode account: %w", err)
	}
	return &info, nil
}

// Publisher returns a registered publisher and its games. A publisher that is
// not registered is ErrNotFound.
func (c *Client) Publisher(ctx context.Context, publisherID uint64) (*PublisherInfo, error) {
	if publisherID == 0 {
		return nil, fmt.Errorf("%w: publisher id must not be zero", ErrInvalidSidechainID)
	}
	return c.getPublisher(ctx, apiPrefix+"/publishers/"+strconv.FormatUint(publisherID, 10), nil)
}

// FindPublisherByKey returns the publisher registered with an identity key,
// given the key's PEM. A key with no publisher is ErrNotFound -- which, right
// after submitting a registration, means "not mined yet".
func (c *Client) FindPublisherByKey(ctx context.Context, publicKeyPEM string) (*PublisherInfo, error) {
	fingerprint, err := AddressOf(publicKeyPEM)
	if err != nil {
		return nil, err
	}
	return c.FindPublisherByFingerprint(ctx, fingerprint)
}

// FindPublisherByFingerprint is FindPublisherByKey for a caller that already
// has the fingerprint (AddressOf, Key.Address, or a node PeerIdentity's NodeID).
func (c *Client) FindPublisherByFingerprint(ctx context.Context, fingerprint string) (*PublisherInfo, error) {
	query := url.Values{"key": []string{strings.ToLower(strings.TrimSpace(fingerprint))}}
	return c.getPublisher(ctx, apiPrefix+"/publishers", query)
}

// Anchors returns every anchor recorded on chain for a sidechain, oldest first.
// A sidechain whose publisher or game is not registered is ErrNotFound; a
// registered one with nothing anchored yet returns an empty slice.
func (c *Client) Anchors(ctx context.Context, id ID) ([]AnchorInfo, error) {
	if err := id.Validate(); err != nil {
		return nil, err
	}

	path := fmt.Sprintf("%s/sidechains/%d/%d/anchors", apiPrefix, id.PublisherID, id.GameID)
	anchors := make([]AnchorInfo, 0)

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

		var list AnchorList
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
// already converted into the Anchor that VerifyRecord takes.
// A height no anchor covers yet is ErrNotFound.
func (c *Client) AnchorCovering(ctx context.Context, id ID, height uint64) (Anchor, error) {
	anchors, err := c.Anchors(ctx, id)
	if err != nil {
		return Anchor{}, err
	}
	for _, info := range anchors {
		if info.Covers(height) {
			return info.Anchor()
		}
	}
	return Anchor{}, fmt.Errorf("%w: no anchor for %s covers height %d",
		ErrNotFound, id, height)
}

// getPublisher fetches and decodes a publisher reply.
func (c *Client) getPublisher(ctx context.Context, path string, query url.Values) (*PublisherInfo, error) {
	status, raw, err := c.do(ctx, http.MethodGet, path, query, nil)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, apiError(status, raw)
	}

	var info PublisherInfo
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
	var envelope struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal(raw, &envelope); err == nil && envelope.Message != "" {
		return &APIError{StatusCode: status, Message: envelope.Message}
	}
	return &APIError{StatusCode: status, Message: strings.TrimSpace(string(raw))}
}
