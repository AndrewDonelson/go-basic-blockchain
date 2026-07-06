package sdk

import "testing"

func TestPEMEncodeDecodeAndAccessors(t *testing.T) {
	v := NewVault()
	if v == nil || v.Key == nil || v.Pem == nil {
		t.Fatal("expected NewVault to initialize key and pem")
	}

	pemObj := NewPEM(v.Key)
	if pemObj == nil {
		t.Fatal("expected NewPEM to return value")
	}

	if pemObj.GetPrivate() == "" || pemObj.GetPublic() == "" {
		t.Fatal("expected non-empty PEM values")
	}

	priv, pub := pemObj.Decode(pemObj.GetPrivate(), pemObj.GetPublic())
	if priv == nil || pub == nil {
		t.Fatal("expected decode to reconstruct private/public key")
	}

	if got := string(pemObj.AsBytes(pemObj.GetPrivate())); got == "" {
		t.Fatal("expected AsBytes to return data")
	}
}

func TestVaultRestoreKeyFromPEM(t *testing.T) {
	v := NewVault()
	if v == nil || v.Pem == nil {
		t.Fatal("expected NewVault to initialize pem")
	}

	v.Key = nil
	if err := v.RestoreKeyFromPEM(); err != nil {
		t.Fatalf("expected restore to succeed, got: %v", err)
	}
	if v.Key == nil {
		t.Fatal("expected key to be restored")
	}
}

func TestVaultRestoreKeyFromPEMErrors(t *testing.T) {
	v := &Vault{}
	if err := v.RestoreKeyFromPEM(); err == nil {
		t.Fatal("expected error for empty pem/key")
	}

	v.Pem = &PEM{PrivateKey: "not-a-pem"}
	if err := v.RestoreKeyFromPEM(); err == nil {
		t.Fatal("expected error for malformed pem")
	}
}

func TestNewVaultWithDataAndDataMethods(t *testing.T) {
	v := NewVaultWithData("name", []string{"a", "b"}, 5.5)
	if v == nil {
		t.Fatal("expected NewVaultWithData to return value")
	}

	if v.Name != "name" || v.Balance != 5.5 {
		t.Fatal("expected fields initialized from constructor")
	}

	if got, _ := v.GetData("name"); got != "name" {
		t.Fatalf("expected name data, got %v", got)
	}
	if got, _ := v.GetData("balance"); got == nil {
		t.Fatal("expected balance data")
	}

	if err := v.SetData("custom", 42); err != nil {
		t.Fatalf("expected SetData success: %v", err)
	}
	if got, _ := v.GetData("custom"); got != 42 {
		t.Fatalf("expected custom data 42, got %v", got)
	}

	var nilVault *Vault
	if err := nilVault.SetData("x", 1); err != nil {
		t.Fatalf("nil vault SetData should be tolerated: %v", err)
	}
}

func TestVaultPEMHelpers(t *testing.T) {
	v := NewVault()
	if v == nil || v.Pem == nil {
		t.Fatal("expected initialized vault")
	}

	if v.PrivatePEM() == "" {
		t.Fatal("expected private pem")
	}
	if v.PublicPEM() == "" {
		t.Fatal("expected public pem")
	}
}
