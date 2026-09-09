package sdk

import (
	"encoding/json"
	"testing"
)

// TestCoinbaseWireFormatCarriesEveryField.
//
// A coinbase has TWO decode paths: Coinbase.UnmarshalJSON and the Coinbase branch
// of DecodeTransaction, which builds the struct field by field. They can drift,
// and they did -- the subsidy fields were added to one and not the other, so a
// reloaded block paid nothing and was refused as invalid despite having been
// valid when mined.
//
// This compares the two on a fully populated value, so a field added to the
// struct and forgotten in either path fails here.
func TestCoinbaseWireFormatCarriesEveryField(t *testing.T) {
	from := addressOnlyWallet("aaaa000000000000000000000000000000000000000000000000000000000000")
	to := addressOnlyWallet("bbbb000000000000000000000000000000000000000000000000000000000000")

	original, err := NewCoinbaseTransaction(from, to, NewConfig())
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	original.TokenCount = 12345
	original.SubsidyUnits = 5_000_000_00
	original.BlockHeight = 4321

	encoded, err := EncodeTransaction(original)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}

	// Path one: the codec used for block persistence.
	decoded, err := DecodeTransaction(encoded)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	viaCodec, ok := decoded.(*Coinbase)
	if !ok {
		t.Fatalf("decoded to %T, want *Coinbase", decoded)
	}

	// Path two: the type's own UnmarshalJSON.
	var viaUnmarshal Coinbase
	if err := json.Unmarshal(encoded, &viaUnmarshal); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	for name, got := range map[string][2]int64{
		"SubsidyUnits": {viaCodec.SubsidyUnits, viaUnmarshal.SubsidyUnits},
		"BlockHeight":  {viaCodec.BlockHeight, viaUnmarshal.BlockHeight},
		"TokenCount":   {viaCodec.TokenCount, viaUnmarshal.TokenCount},
	} {
		if got[0] != got[1] {
			t.Fatalf("%s differs between the two decode paths: codec=%d unmarshal=%d",
				name, got[0], got[1])
		}
	}

	if viaCodec.SubsidyUnits != original.SubsidyUnits {
		t.Fatalf("SubsidyUnits did not survive the round trip: %d, want %d",
			viaCodec.SubsidyUnits, original.SubsidyUnits)
	}
	if viaCodec.BlockHeight != original.BlockHeight {
		t.Fatalf("BlockHeight did not survive the round trip: %d, want %d",
			viaCodec.BlockHeight, original.BlockHeight)
	}
}
