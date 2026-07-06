package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestResolveAPIURLFromValues(t *testing.T) {
	cases := []struct {
		name string
		url  string
		host string
		want string
	}{
		{name: "explicit url", url: "https://api.example.com", host: ":8100", want: "https://api.example.com"},
		{name: "host with colon", host: ":8100", want: "http://localhost:8100"},
		{name: "host with scheme", host: "http://node.local:8100", want: "http://node.local:8100"},
		{name: "host without scheme", host: "node.local:8100", want: "http://node.local:8100"},
		{name: "default", want: "http://localhost:8100"},
	}

	for _, tc := range cases {
		if got := resolveAPIURLFromValues(tc.url, tc.host); got != tc.want {
			t.Fatalf("%s: got %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestResolveAPIKeyFromValue(t *testing.T) {
	if got := resolveAPIKeyFromValue("abc"); got != "abc" {
		t.Fatalf("expected explicit key, got %q", got)
	}
	if got := resolveAPIKeyFromValue(""); got == "" {
		t.Fatal("expected default api key")
	}
}

func TestStatusSummary(t *testing.T) {
	info := map[string]interface{}{
		"block_count": 3.0,
		"difficulty":  2.0,
		"block_time":  20.0,
		"latest_block": map[string]interface{}{
			"hash":  "abc",
			"index": 3.0,
		},
	}

	lines := statusSummary(info, true)
	joined := strings.Join(lines, "\n")
	for _, want := range []string{"Connected: true", "Blocks: 3", "Difficulty: 2", "Block Time: 20 seconds", "Latest Hash: abc", "Latest Block Index: 3"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("expected summary to contain %q; got %q", want, joined)
		}
	}
}

func TestResolveAPIURLAndKeyFromEnv(t *testing.T) {
	oldURL, hadURL := os.LookupEnv("BLOCKCHAIN_API_URL")
	oldHost, hadHost := os.LookupEnv("API_HOSTNAME")
	oldKey, hadKey := os.LookupEnv("BLOCKCHAIN_API_KEY")
	defer func() {
		if hadURL {
			_ = os.Setenv("BLOCKCHAIN_API_URL", oldURL)
		} else {
			_ = os.Unsetenv("BLOCKCHAIN_API_URL")
		}
		if hadHost {
			_ = os.Setenv("API_HOSTNAME", oldHost)
		} else {
			_ = os.Unsetenv("API_HOSTNAME")
		}
		if hadKey {
			_ = os.Setenv("BLOCKCHAIN_API_KEY", oldKey)
		} else {
			_ = os.Unsetenv("BLOCKCHAIN_API_KEY")
		}
	}()

	_ = os.Setenv("BLOCKCHAIN_API_URL", "")
	_ = os.Setenv("API_HOSTNAME", ":9999")
	if got := resolveAPIURL(); got != "http://localhost:9999" {
		t.Fatalf("resolveAPIURL()=%q", got)
	}

	_ = os.Setenv("BLOCKCHAIN_API_KEY", "custom-key")
	if got := resolveAPIKey(); got != "custom-key" {
		t.Fatalf("resolveAPIKey()=%q", got)
	}
}

func TestBlockchainClientConnectAndRequestMethods(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/health":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("ok"))
		case r.URL.Path == "/blockchain":
			if r.Header.Get("Authorization") != "Bearer test-key" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			_, _ = w.Write([]byte(`{"block_count":3}`))
		case r.URL.Path == "/blockchain/blocks":
			_, _ = w.Write([]byte(`[{"index":1,"hash":"h1"}]`))
		case r.URL.Path == "/blockchain/blocks/1":
			_, _ = w.Write([]byte(`{"index":1,"hash":"h1"}`))
		case r.URL.Path == "/blockchain/wallets":
			_, _ = w.Write([]byte(`{"wallets":[{"id":"w1"}]}`))
		case r.URL.Path == "/blockchain/wallets/w1":
			_, _ = w.Write([]byte(`{"id":"w1"}`))
		case r.URL.Path == "/blockchain/transactions":
			_, _ = w.Write([]byte(`{"transactions":[{"id":"t1"}]}`))
		case r.URL.Path == "/blockchain/transactions/t1":
			_, _ = w.Write([]byte(`{"id":"t1"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()

	bc := &BlockchainClient{apiURL: ts.URL, httpClient: ts.Client(), apiKey: "test-key"}

	if err := bc.Connect(); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	if !bc.IsConnected() {
		t.Fatal("expected IsConnected=true")
	}

	if _, err := bc.makeRequest("GET", "/missing"); err == nil {
		t.Fatal("expected makeRequest error for non-200")
	}

	info, err := bc.GetBlockchainInfo()
	if err != nil || info["block_count"].(float64) != 3 {
		t.Fatalf("unexpected blockchain info: info=%v err=%v", info, err)
	}

	blocks, err := bc.GetBlocks(1, 10)
	if err != nil || len(blocks) != 1 {
		t.Fatalf("unexpected blocks: blocks=%v err=%v", blocks, err)
	}

	block, err := bc.GetBlock(1)
	if err != nil || block["hash"] != "h1" {
		t.Fatalf("unexpected block: block=%v err=%v", block, err)
	}

	wallets, err := bc.GetWallets(1, 10)
	if err != nil || wallets["wallets"] == nil {
		t.Fatalf("unexpected wallets: wallets=%v err=%v", wallets, err)
	}

	wallet, err := bc.GetWallet("w1")
	if err != nil || wallet["id"] != "w1" {
		t.Fatalf("unexpected wallet: wallet=%v err=%v", wallet, err)
	}

	txs, err := bc.GetTransactions(1, 10)
	if err != nil || txs["transactions"] == nil {
		t.Fatalf("unexpected txs: txs=%v err=%v", txs, err)
	}

	tx, err := bc.GetTransaction("t1")
	if err != nil || tx["id"] != "t1" {
		t.Fatalf("unexpected tx: tx=%v err=%v", tx, err)
	}
}

func TestBlockchainClientConnectFailureAndInvalidJSON(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/health":
			w.WriteHeader(http.StatusServiceUnavailable)
		case "/blockchain":
			_, _ = w.Write([]byte("not-json"))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()

	bc := &BlockchainClient{apiURL: ts.URL, httpClient: ts.Client(), apiKey: "k"}
	if err := bc.Connect(); err == nil {
		t.Fatal("expected connect failure on non-200 health")
	}
	if bc.IsConnected() {
		t.Fatal("expected IsConnected=false")
	}

	if _, err := bc.GetBlockchainInfo(); err == nil {
		t.Fatal("expected JSON unmarshal error")
	}
}

func TestPrettyPrintMap(t *testing.T) {
	originalStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe create failed: %v", err)
	}
	os.Stdout = w

	prettyPrintMap(map[string]interface{}{
		"top": map[string]interface{}{"a": 1},
		"arr": []interface{}{map[string]interface{}{"b": 2}, "x"},
	}, "")

	_ = w.Close()
	os.Stdout = originalStdout

	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read output failed: %v", err)
	}
	s := string(out)
	for _, want := range []string{"top:", "a: 1", "arr: [2 items]", "b: 2"} {
		if !strings.Contains(s, want) {
			t.Fatalf("expected output to contain %q, got %q", want, s)
		}
	}
}
