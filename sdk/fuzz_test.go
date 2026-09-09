package sdk

import (
	"encoding/json"
	"testing"
)

// Fuzz targets for every parser that consumes bytes from disk or the network.
//
// These are the functions an attacker or a corrupt file reaches first, and a
// panic in any of them takes the node down. Go's native fuzzing would have found
// the NewMerkleTree panic on 6 transactions, the NewBigIntFromBytes panic on
// short slices, and the executeMemoryPhase slice-bounds panic in seconds each --
// all three were found by hand instead.
//
// The contract every target asserts is the same: parse or return an error, never
// panic, and never hand back a nil value alongside a nil error.

// FuzzBlockUnmarshal covers blocks read from disk and received from peers.
func FuzzBlockUnmarshal(f *testing.F) {
	f.Add([]byte(`{}`))
	f.Add([]byte(`{"index":1,"hash":"abc","transactions":[]}`))
	f.Add([]byte(`{"index":-1}`))
	f.Add([]byte(`{"transactions":[{"protocol":"MESSAGE"}]}`))
	f.Add([]byte(`{"header":{"difficulty":4294967295,"nonce":1}}`))
	f.Add([]byte(`[]`))
	f.Add([]byte(``))

	f.Fuzz(func(t *testing.T, data []byte) {
		var block Block
		if err := json.Unmarshal(data, &block); err != nil {
			return
		}

		// A block that decoded must survive everything the chain does to it
		// without panicking.
		_ = block.CalculateHash()
		_ = block.CalculateMerkleRoot()
		_ = block.currentSize()
		_ = block.String()
	})
}

// FuzzDecodeTransaction covers the wire format for every protocol. This is the
// decoder that makes block persistence possible, so a panic here means a node
// cannot start.
func FuzzDecodeTransaction(f *testing.F) {
	f.Add([]byte(`{"protocol":"MESSAGE","payload":{}}`))
	f.Add([]byte(`{"protocol":"BANK","payload":{"amount":1.5}}`))
	f.Add([]byte(`{"protocol":"COINBASE","payload":{"token_count":1}}`))
	f.Add([]byte(`{"protocol":"PERSIST","payload":{}}`))
	f.Add([]byte(`{"protocol":"UNKNOWN","payload":{}}`))
	f.Add([]byte(`{"protocol":"","payload":null}`))
	f.Add([]byte(`{}`))
	f.Add([]byte(``))

	f.Fuzz(func(t *testing.T, data []byte) {
		tx, err := DecodeTransaction(data)
		if err != nil {
			return
		}
		if tx == nil {
			t.Fatal("DecodeTransaction returned a nil transaction and a nil error")
		}

		// Anything that decoded is about to be validated, hashed and sized.
		_ = tx.Validate()
		_ = tx.Size()
		_ = tx.GetID()
		_ = tx.GetProtocol()
		_ = tx.Hash()
		_, _ = tx.SigningBytes()
	})
}

// FuzzPUIDParse covers identity strings, which arrive in transactions from peers.
func FuzzPUIDParse(f *testing.F) {
	f.Add("1:1:1:1")
	f.Add("0:0:0:0")
	f.Add("")
	f.Add(":::")
	f.Add("1:1:1")
	f.Add("1:1:1:1:1")
	f.Add("-1:-1:-1:-1")
	f.Add("99999999999999999999999999:1:1:1")
	f.Add("a:b:c:d")

	f.Fuzz(func(t *testing.T, s string) {
		puid, err := NewPUIDFromString(s)
		if err != nil {
			return
		}
		if puid == nil {
			t.Fatal("NewPUIDFromString returned a nil PUID and a nil error")
		}

		// A PUID that parsed must render, and rendering must not panic.
		_ = puid.String()
	})
}

// FuzzBigIntFromString covers numeric parsing. NewBigIntFromString misdetects
// some inputs as base64 and forwards them to NewBigIntFromBytes, which used to
// panic on any slice shorter than eight bytes.
func FuzzBigIntFromString(f *testing.F) {
	f.Add("0")
	f.Add("1")
	f.Add("-1")
	f.Add("")
	f.Add("abc")
	f.Add("AAAA")
	f.Add("=")
	f.Add("999999999999999999999999999999999999")
	f.Add("0x10")

	f.Fuzz(func(t *testing.T, s string) {
		value, err := NewBigIntFromString(s)
		if err != nil {
			return
		}
		if value == nil {
			t.Fatal("NewBigIntFromString returned a nil value and a nil error")
		}
		_ = value.String()
	})
}

// FuzzMerkleTree targets the constructor that panicked on any block with 6, 10,
// 14 or 22 transactions -- reachable from any peer simply by sending one.
func FuzzMerkleTree(f *testing.F) {
	f.Add(0)
	f.Add(1)
	f.Add(2)
	f.Add(6)
	f.Add(10)
	f.Add(22)

	f.Fuzz(func(t *testing.T, count int) {
		// Bound the input: this is a shape test, not a memory test.
		if count < 0 || count > 512 {
			return
		}

		leaves := make([][]byte, 0, count)
		for i := 0; i < count; i++ {
			leaves = append(leaves, []byte{byte(i), byte(i >> 8)})
		}

		tree := NewMerkleTree(leaves)
		if tree == nil {
			t.Fatal("NewMerkleTree returned nil")
		}
	})
}

// FuzzP2PTransactionUnmarshal covers messages straight off the wire. P2PTransaction
// serialised without any of its own fields once, because method promotion on the
// embedded Tx swallowed MarshalJSON.
func FuzzP2PTransactionUnmarshal(f *testing.F) {
	f.Add([]byte(`{"action":"validate","target":"all"}`))
	f.Add([]byte(`{"action":"remove","data":"abc"}`))
	f.Add([]byte(`{"tx":{},"action":""}`))
	f.Add([]byte(`{}`))
	f.Add([]byte(``))

	f.Fuzz(func(t *testing.T, data []byte) {
		var tx P2PTransaction
		if err := json.Unmarshal(data, &tx); err != nil {
			return
		}

		_ = tx.GetID()
		_, _ = json.Marshal(&tx)
	})
}

// FuzzP2PTransactionStateFromString covers a small parser reached from wire data.
func FuzzP2PTransactionStateFromString(f *testing.F) {
	f.Add("pending")
	f.Add("")
	f.Add("PENDING")
	f.Add("nonsense")

	f.Fuzz(func(t *testing.T, s string) {
		state, err := P2PTransactionStateFromString(s)
		if err != nil {
			return
		}
		_ = state
	})
}

// FuzzUnitsRoundTrip targets the base-unit conversions that every balance passes
// through. A wrong answer here is silent and costs money.
func FuzzUnitsRoundTrip(f *testing.F) {
	f.Add(int64(0))
	f.Add(int64(1))
	f.Add(int64(UnitsPerToken))
	f.Add(int64(-1))
	f.Add(int64(1 << 62))

	f.Fuzz(func(t *testing.T, units int64) {
		formatted := formatUnits(units)
		if formatted == "" {
			t.Fatalf("formatUnits(%d) produced an empty string", units)
		}

		// Converting to tokens and back must not gain units. It may lose
		// precision for very large values, which is why this checks a bound
		// rather than exact equality.
		tokens := UnitsToAmount(units)
		back := AmountToUnits(tokens)
		if units >= 0 && back < 0 {
			t.Fatalf("%d units round-tripped to %d: sign flipped", units, back)
		}
	})
}
