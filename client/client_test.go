package client

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/AndrewDonelson/go-basic-blockchain/sdk"
)

const testAPIKey = "69a082ff3996745bd4b48bcc92d5bb40ff97115896183f1cb53a3409f818b15f"

// sharedNode is one real node, served in-process for the whole package: the
// same router, middleware and handlers a running node uses, over a fresh chain
// in a temporary directory. It is shared because creating a chain mines its
// genesis block, which takes seconds; the tests keep to their own keys and
// ids, so sharing it costs them nothing.
var sharedNode struct {
	bc     *sdk.Blockchain
	server *httptest.Server
}

func TestMain(m *testing.M) {
	os.Exit(runWithNode(m))
}

func runWithNode(m *testing.M) int {
	// The node fails closed without credentials, exactly as in production.
	_ = os.Setenv("BLOCKCHAIN_API_KEY", testAPIKey)
	_ = os.Setenv("BLOCKCHAIN_SERVER_SEED", "5f4dcc3b5aa765d61d8327deb882cf995f4dcc3b5aa765d61d8327deb882cf99")

	dir, err := os.MkdirTemp("", "gbb-client-test-")
	if err != nil {
		fmt.Fprintln(os.Stderr, "temp dir:", err)
		return 1
	}
	defer func() { _ = os.RemoveAll(dir) }()

	cfg := sdk.NewConfig()
	cfg.DataPath = dir
	if err := sdk.NewLocalStorage(cfg.DataPath); err != nil {
		fmt.Fprintln(os.Stderr, "NewLocalStorage:", err)
		return 1
	}
	bc := sdk.NewBlockchain(cfg)
	if bc == nil {
		fmt.Fprintln(os.Stderr, "NewBlockchain returned nil")
		return 1
	}
	api, err := sdk.NewAPIWithError(bc)
	if err != nil {
		fmt.Fprintln(os.Stderr, "NewAPIWithError:", err)
		return 1
	}
	server := httptest.NewServer(api.Handler())
	defer server.Close()

	sharedNode.bc = bc
	sharedNode.server = server
	return m.Run()
}

// newNode returns the shared node and a client for it.
func newNode(t *testing.T) (*sdk.Blockchain, *Client) {
	t.Helper()
	c, err := New(sharedNode.server.URL, testAPIKey)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return sharedNode.bc, c
}

// signedMessage builds a signed, zero-fee MESSAGE transaction from a fresh key.
//
// It is assembled from the canonical wire form rather than from an sdk.Wallet:
// a wallet encrypts itself with production scrypt parameters on creation, which
// costs seconds and a gigabyte per test for nothing this package is testing.
// Building it this way also shows that the wire format is all a client in
// another language would need.
func signedMessage(t *testing.T, text string) *sdk.Message {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	publicDER, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		t.Fatalf("MarshalPKIXPublicKey: %v", err)
	}
	privateDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatalf("MarshalECPrivateKey: %v", err)
	}
	publicPEM := string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: publicDER}))
	privatePEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: privateDER})
	sum := sha256.Sum256(publicDER)
	address := hex.EncodeToString(sum[:])

	wire, err := json.Marshal(map[string]interface{}{
		"id":              "1:1:1:" + strconv.FormatInt(time.Now().UnixNano(), 10),
		"time":            time.Now().UTC(),
		"version":         sdk.TransactionVersion,
		"protocol":        sdk.MessageProtocolID,
		"from":            address,
		"to":              address,
		"from_public_key": publicPEM,
		"to_public_key":   publicPEM,
		"fee":             0,
		"status":          "pending",
		"nonce":           1,
		"message":         text,
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	decoded, err := sdk.DecodeTransaction(wire)
	if err != nil {
		t.Fatalf("DecodeTransaction: %v", err)
	}
	message, ok := decoded.(*sdk.Message)
	if !ok {
		t.Fatalf("decoded %T, want *sdk.Message", decoded)
	}
	message.Signature, err = message.Sign(privatePEM)
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}
	return message
}

func TestNewValidatesItsArguments(t *testing.T) {
	for _, tc := range []struct{ name, url, key string }{
		{"no scheme", "localhost:8100", testAPIKey},
		{"wrong scheme", "ftp://localhost", testAPIKey},
		{"no host", "http://", testAPIKey},
		{"no key", "http://localhost:8100", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := New(tc.url, tc.key); err == nil {
				t.Fatal("expected an error")
			}
		})
	}
}

func TestSubmitTransactionAgainstARealNode(t *testing.T) {
	bc, c := newNode(t)
	ctx := context.Background()

	tx := signedMessage(t, "hello")
	id, err := c.SubmitTransaction(ctx, tx)
	if err != nil {
		t.Fatalf("SubmitTransaction: %v", err)
	}
	if id != tx.GetID() || !bc.HasTransactionID(id) {
		t.Fatalf("returned id %q, node knows it: %v", id, bc.HasTransactionID(id))
	}

	// Retrying is safe.
	reply, err := c.Submit(ctx, tx)
	if err != nil {
		t.Fatalf("resubmission: %v", err)
	}
	if !reply.Known || reply.Status != sdk.SubmissionPending {
		t.Fatalf("unexpected resubmission reply %+v", reply)
	}

	// A rejection carries the node's reason.
	tampered := signedMessage(t, "original")
	tampered.Message = "edited after signing"
	_, err = c.SubmitTransaction(ctx, tampered)
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusUnprocessableEntity || apiErr.Reason == "" {
		t.Fatalf("expected a 422 with a reason, got %v", err)
	}
}

func TestWrongAPIKeyIsReported(t *testing.T) {
	_, good := newNode(t)
	c, err := New(good.baseURL.String(), "00")
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	_, err = c.Publisher(context.Background(), 1)
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected a 401, got %v", err)
	}
}

// The whole application flow, with consensus stood in for by the registry and
// ledger the node's own block application writes to.
func TestPublisherAndAnchorLookupsAgainstARealNode(t *testing.T) {
	bc, c := newNode(t)
	ctx := context.Background()
	ledger := bc.UTXOSet().Anchors()

	identity, err := sdk.NewPeerIdentity()
	if err != nil {
		t.Fatalf("NewPeerIdentity: %v", err)
	}

	// Before the registration confirms, the key finds nothing.
	if _, err := c.FindPublisherByKey(ctx, identity.PublicPEM); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound before registering, got %v", err)
	}

	publisherID, err := ledger.Registry().RegisterPublisher(identity.PublicPEM, "Studio",
		sdk.MinPublisherBondUnits, 1)
	if err != nil {
		t.Fatalf("RegisterPublisher: %v", err)
	}
	gameID, err := ledger.Registry().RegisterGame(publisherID, "Game", 1)
	if err != nil {
		t.Fatalf("RegisterGame: %v", err)
	}
	id := sdk.SidechainID{PublisherID: publisherID, GameID: gameID}

	found, err := c.FindPublisherByKey(ctx, identity.PublicPEM)
	if err != nil {
		t.Fatalf("FindPublisherByKey: %v", err)
	}
	if found.PublisherID != publisherID || len(found.GamesNamed("Game")) != 1 {
		t.Fatalf("unexpected publisher %+v", found)
	}
	byFingerprint, err := c.FindPublisherByFingerprint(ctx, identity.NodeID)
	if err != nil || byFingerprint.PublisherID != publisherID {
		t.Fatalf("FindPublisherByFingerprint: %+v, %v", byFingerprint, err)
	}
	byID, err := c.Publisher(ctx, publisherID)
	if err != nil || byID.KeyFingerprint != identity.NodeID {
		t.Fatalf("Publisher: %+v, %v", byID, err)
	}
	if _, err := c.Publisher(ctx, publisherID+1); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound for an unregistered publisher, got %v", err)
	}

	if _, err := c.Anchors(ctx, sdk.SidechainID{PublisherID: publisherID, GameID: 9}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound for an unregistered game, got %v", err)
	}
	anchors, err := c.Anchors(ctx, id)
	if err != nil || len(anchors) != 0 {
		t.Fatalf("expected no anchors yet, got %v, %v", anchors, err)
	}

	// The publisher writes blocks and anchors them twice.
	chain, err := sdk.NewSidechain(id)
	if err != nil {
		t.Fatalf("NewSidechain: %v", err)
	}
	appendBlocks := func(n int) {
		for i := 0; i < n; i++ {
			block, err := sdk.NewSidechainBlock(id, chain.Tip(),
				[][]byte{[]byte("event-a"), []byte("event-b"), []byte("event-c")}, int64(chain.Len()+1))
			if err != nil {
				t.Fatalf("NewSidechainBlock: %v", err)
			}
			if err := chain.Append(block); err != nil {
				t.Fatalf("Append: %v", err)
			}
		}
	}
	for round := int64(1); round <= 2; round++ {
		appendBlocks(3)
		anchor, err := sdk.BuildAnchor(chain)
		if err != nil {
			t.Fatalf("BuildAnchor: %v", err)
		}
		signature, err := sdk.SignAnchor(identity, *anchor)
		if err != nil {
			t.Fatalf("SignAnchor: %v", err)
		}
		if err := ledger.Record(*anchor, signature, round); err != nil {
			t.Fatalf("Record: %v", err)
		}
		if err := chain.MarkAnchored(anchor.ToHeight); err != nil {
			t.Fatalf("MarkAnchored: %v", err)
		}
	}

	anchors, err = c.Anchors(ctx, id)
	if err != nil {
		t.Fatalf("Anchors: %v", err)
	}
	if len(anchors) != 2 || anchors[0].ToHeight != 2 || anchors[1].FromHeight != 3 ||
		anchors[1].MainBlockIndex != 2 {
		t.Fatalf("unexpected anchors %+v", anchors)
	}

	// A third party holding one payload and its proof verifies it against the
	// anchor it read from the node.
	anchor, err := c.AnchorCovering(ctx, id, 4)
	if err != nil {
		t.Fatalf("AnchorCovering: %v", err)
	}
	proof, err := sdk.ProveRecord(chain, anchor, 4, 1)
	if err != nil {
		t.Fatalf("ProveRecord: %v", err)
	}
	if err := sdk.VerifyRecord([]byte("event-b"), proof, anchor); err != nil {
		t.Fatalf("VerifyRecord: %v", err)
	}
	if _, err := c.AnchorCovering(ctx, id, 99); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound for an unanchored height, got %v", err)
	}
}
