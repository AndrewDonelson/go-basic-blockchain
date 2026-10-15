// File sidechain/key_test.go
package sidechain

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"testing"
)

func TestKeyRoundTripsAndSigns(t *testing.T) {
	k, err := GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	back, err := ParseKeyPEM(k.PrivatePEM())
	if err != nil {
		t.Fatal(err)
	}
	if back.Address() != k.Address() || len(k.Address()) != 64 {
		t.Fatalf("address %q / %q", k.Address(), back.Address())
	}
	if got, err := AddressOf(k.PublicPEM()); err != nil || got != k.Address() {
		t.Fatalf("AddressOf: %q %v", got, err)
	}
	sig, err := back.Sign([]byte("hello"))
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifySignature(k.PublicPEM(), []byte("hello"), sig); err != nil {
		t.Fatal(err)
	}
	if VerifySignature(k.PublicPEM(), []byte("hellp"), sig) == nil {
		t.Fatal("a signature over other bytes must not verify")
	}

	// PKCS #8 is accepted too; other curves are not.
	raw, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	der, _ := x509.MarshalPKCS8PrivateKey(raw)
	if _, err := ParseKeyPEM(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})); err != nil {
		t.Fatalf("PKCS8: %v", err)
	}
	p384, _ := ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	der, _ = x509.MarshalECPrivateKey(p384)
	if _, err := ParseKeyPEM(pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: der})); err == nil {
		t.Fatal("a P-384 key must be refused")
	}
	if _, err := ParseKeyPEM([]byte("nope")); err == nil {
		t.Fatal("garbage must be refused")
	}
}

func TestAccountNextNonce(t *testing.T) {
	for _, tc := range []struct {
		a    AccountInfo
		want uint64
	}{
		{AccountInfo{}, 0},
		{AccountInfo{HasNonce: true, LastNonce: 4}, 5},
		{AccountInfo{HasNonce: true, LastNonce: 4, HasPending: true, PendingNonce: 6}, 7},
		{AccountInfo{HasPending: true, PendingNonce: 0}, 1},
	} {
		if got := tc.a.NextNonce(); got != tc.want {
			t.Errorf("%+v: NextNonce %d, want %d", tc.a, got, tc.want)
		}
	}
}
