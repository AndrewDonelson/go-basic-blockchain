package vdf

import (
	"math/big"
	"testing"
)

// testDiscriminant returns a small discriminant, fast enough to use everywhere.
func testDiscriminant(t testing.TB) *big.Int {
	t.Helper()
	d, err := NewDiscriminant([]byte("gbb-class-group-tests"), 256)
	if err != nil {
		t.Fatalf("discriminant: %v", err)
	}
	return d
}

func testForm(t testing.TB, d *big.Int, seed string) Form {
	t.Helper()
	f, err := HashToForm([]byte(seed), d)
	if err != nil {
		t.Fatalf("hash to form %q: %v", seed, err)
	}
	return f
}

// -----------------------------------------------------------------------------
// Discriminant
// -----------------------------------------------------------------------------

// TestDiscriminantIsDeterministic is what makes the parameters verifiable: the
// whole no-trusted-setup argument rests on anyone being able to re-derive them.
func TestDiscriminantIsDeterministic(t *testing.T) {
	first, err := NewDiscriminant([]byte("seed"), 256)
	if err != nil {
		t.Fatalf("derive: %v", err)
	}
	for i := 0; i < 5; i++ {
		again, err := NewDiscriminant([]byte("seed"), 256)
		if err != nil {
			t.Fatalf("derive: %v", err)
		}
		if first.Cmp(again) != 0 {
			t.Fatal("the same seed produced two different discriminants; the " +
				"parameters could not be checked by anyone else")
		}
	}

	other, err := NewDiscriminant([]byte("different seed"), 256)
	if err != nil {
		t.Fatalf("derive: %v", err)
	}
	if first.Cmp(other) == 0 {
		t.Fatal("different seeds produced the same discriminant")
	}
}

// TestDiscriminantShape: D must be negative, prime in absolute value, and 1 mod 8
// -- the last is what makes HashToForm's mod-4a condition follow from mod-a.
func TestDiscriminantShape(t *testing.T) {
	for _, bits := range []int{128, 256, 512} {
		d, err := NewDiscriminant([]byte("shape"), bits)
		if err != nil {
			t.Fatalf("derive %d bits: %v", bits, err)
		}

		if d.Sign() >= 0 {
			t.Fatalf("discriminant is not negative: %s", d)
		}
		if got := new(big.Int).Mod(d, big.NewInt(8)); got.Cmp(big.NewInt(1)) != 0 {
			t.Fatalf("discriminant mod 8 = %s, want 1", got)
		}
		p := new(big.Int).Neg(d)
		if !p.ProbablyPrime(32) {
			t.Fatal("|discriminant| is not prime")
		}
		if p.BitLen() != bits {
			t.Fatalf("discriminant is %d bits, want %d", p.BitLen(), bits)
		}
		if err := ValidateDiscriminant(d); err != nil {
			t.Fatalf("a generated discriminant failed validation: %v", err)
		}
	}
}

func TestValidateDiscriminantRejectsBadInput(t *testing.T) {
	for _, bad := range []*big.Int{
		nil,
		big.NewInt(0),
		big.NewInt(7),                   // positive
		big.NewInt(-4),                  // not 1 mod 8
		new(big.Int).Neg(big.NewInt(9)), // -9 mod 8 = 7, not 1
	} {
		if err := ValidateDiscriminant(bad); err == nil {
			t.Fatalf("accepted an invalid discriminant: %v", bad)
		}
	}
}

func TestNewDiscriminantRejectsTinySizes(t *testing.T) {
	if _, err := NewDiscriminant([]byte("x"), 32); err == nil {
		t.Fatal("a 32-bit discriminant was accepted")
	}
}

// -----------------------------------------------------------------------------
// Group axioms
//
// These are the safety net for composition. Gauss composition is intricate, and
// a subtle error in it would not show up as a crash -- it would show up as a
// group that is not a group. Associativity in particular is very hard to satisfy
// by accident.
// -----------------------------------------------------------------------------

func TestIdentityIsAnIdentity(t *testing.T) {
	d := testDiscriminant(t)
	id := Identity(d)

	if id.Discriminant().Cmp(d) != 0 {
		t.Fatalf("the identity has the wrong discriminant")
	}

	for _, seed := range []string{"a", "b", "c", "hello", "world"} {
		f := testForm(t, d, seed)

		left, err := Compose(id, f)
		if err != nil {
			t.Fatalf("compose: %v", err)
		}
		right, err := Compose(f, id)
		if err != nil {
			t.Fatalf("compose: %v", err)
		}

		if !left.Equal(Reduce(f)) {
			t.Fatalf("identity * f != f for seed %q", seed)
		}
		if !right.Equal(Reduce(f)) {
			t.Fatalf("f * identity != f for seed %q", seed)
		}
	}
}

func TestCompositionIsAssociative(t *testing.T) {
	d := testDiscriminant(t)

	seeds := []string{"alpha", "beta", "gamma", "delta", "epsilon"}
	for i := range seeds {
		for j := range seeds {
			for k := range seeds {
				f := testForm(t, d, seeds[i])
				g := testForm(t, d, seeds[j])
				h := testForm(t, d, seeds[k])

				fg, err := Compose(f, g)
				if err != nil {
					t.Fatalf("compose: %v", err)
				}
				fgh, err := Compose(fg, h)
				if err != nil {
					t.Fatalf("compose: %v", err)
				}

				gh, err := Compose(g, h)
				if err != nil {
					t.Fatalf("compose: %v", err)
				}
				fGh, err := Compose(f, gh)
				if err != nil {
					t.Fatalf("compose: %v", err)
				}

				if !fgh.Equal(fGh) {
					t.Fatalf("(f*g)*h != f*(g*h) for %q,%q,%q -- composition is wrong",
						seeds[i], seeds[j], seeds[k])
				}
			}
		}
	}
}

func TestCompositionIsCommutative(t *testing.T) {
	d := testDiscriminant(t)

	for _, pair := range [][2]string{{"a", "b"}, {"x", "y"}, {"one", "two"}} {
		f := testForm(t, d, pair[0])
		g := testForm(t, d, pair[1])

		fg, err := Compose(f, g)
		if err != nil {
			t.Fatalf("compose: %v", err)
		}
		gf, err := Compose(g, f)
		if err != nil {
			t.Fatalf("compose: %v", err)
		}
		if !fg.Equal(gf) {
			t.Fatalf("f*g != g*f for %v; the class group is abelian", pair)
		}
	}
}

func TestInverseCancels(t *testing.T) {
	d := testDiscriminant(t)
	id := Identity(d)

	for _, seed := range []string{"p", "q", "r", "inverse-test"} {
		f := testForm(t, d, seed)
		inv := Inverse(f)

		product, err := Compose(f, inv)
		if err != nil {
			t.Fatalf("compose: %v", err)
		}
		if !product.Equal(id) {
			t.Fatalf("f * f^-1 != identity for seed %q\n got %v %v %v",
				seed, product.A, product.B, product.C)
		}
	}
}

// TestCompositionPreservesTheDiscriminant. Every element must stay in the same
// group; a composition that drifts is producing elements of a different group.
func TestCompositionPreservesTheDiscriminant(t *testing.T) {
	d := testDiscriminant(t)

	f := testForm(t, d, "preserve-1")
	g := testForm(t, d, "preserve-2")

	for i := 0; i < 20; i++ {
		var err error
		f, err = Compose(f, g)
		if err != nil {
			t.Fatalf("compose at %d: %v", i, err)
		}
		if f.Discriminant().Cmp(d) != 0 {
			t.Fatalf("discriminant drifted after %d compositions", i+1)
		}
		if !f.IsReduced() {
			t.Fatalf("composition returned an unreduced form at %d", i+1)
		}
	}
}

// -----------------------------------------------------------------------------
// Reduction
// -----------------------------------------------------------------------------

// TestReductionIsCanonical: equality is a field comparison, which is only valid
// because reduced forms are unique per class.
func TestReductionIsCanonical(t *testing.T) {
	d := testDiscriminant(t)
	f := testForm(t, d, "canonical")

	if !f.IsReduced() {
		t.Fatal("HashToForm returned an unreduced form")
	}

	// Reducing an already-reduced form changes nothing.
	if !Reduce(f).Equal(f) {
		t.Fatal("reduction is not idempotent")
	}

	// An equivalent but unreduced representation must reduce to the same form.
	// (a, b, c) -> (a, b + 2a, c + a + b) is the same class.
	shifted := Form{
		A: new(big.Int).Set(f.A),
		B: new(big.Int).Add(f.B, new(big.Int).Lsh(f.A, 1)),
		C: new(big.Int).Add(new(big.Int).Add(f.C, f.A), f.B),
	}
	if shifted.Discriminant().Cmp(d) != 0 {
		t.Fatal("the shifted form has a different discriminant; the test is wrong")
	}
	if !Reduce(shifted).Equal(f) {
		t.Fatal("two representations of one class reduced to different forms")
	}
}

func TestReducedFormsSatisfyTheirConditions(t *testing.T) {
	d := testDiscriminant(t)

	for _, seed := range []string{"r1", "r2", "r3", "r4", "r5"} {
		f := testForm(t, d, seed)
		if !f.IsReduced() {
			t.Fatalf("form for %q is not reduced: a=%v b=%v c=%v", seed, f.A, f.B, f.C)
		}

		absB := new(big.Int).Abs(f.B)
		if absB.Cmp(f.A) > 0 {
			t.Fatalf("|b| > a for %q", seed)
		}
		if f.A.Cmp(f.C) > 0 {
			t.Fatalf("a > c for %q", seed)
		}
	}
}

// -----------------------------------------------------------------------------
// Exponentiation
// -----------------------------------------------------------------------------

// TestExpMatchesRepeatedComposition is the check the VDF depends on: squaring T
// times must equal exponentiation by 2^T.
func TestExpMatchesRepeatedComposition(t *testing.T) {
	d := testDiscriminant(t)
	f := testForm(t, d, "exponent")

	// f^(2^8) by repeated squaring...
	squared := f.Clone()
	for i := 0; i < 8; i++ {
		var err error
		squared, err = Square(squared)
		if err != nil {
			t.Fatalf("square: %v", err)
		}
	}

	// ...must equal f^256 by square-and-multiply.
	exponent := new(big.Int).Lsh(big.NewInt(1), 8)
	powered, err := Exp(f, exponent, d)
	if err != nil {
		t.Fatalf("exp: %v", err)
	}

	if !squared.Equal(powered) {
		t.Fatal("repeated squaring and exponentiation disagree; the VDF's whole " +
			"evaluation rests on these being the same")
	}
}

func TestExpLaws(t *testing.T) {
	d := testDiscriminant(t)
	f := testForm(t, d, "laws")
	id := Identity(d)

	zero, err := Exp(f, big.NewInt(0), d)
	if err != nil {
		t.Fatalf("exp: %v", err)
	}
	if !zero.Equal(id) {
		t.Fatal("f^0 != identity")
	}

	first, err := Exp(f, big.NewInt(1), d)
	if err != nil {
		t.Fatalf("exp: %v", err)
	}
	if !first.Equal(Reduce(f)) {
		t.Fatal("f^1 != f")
	}

	// f^3 * f^5 == f^8
	three, _ := Exp(f, big.NewInt(3), d)
	five, _ := Exp(f, big.NewInt(5), d)
	eightPow, _ := Exp(f, big.NewInt(8), d)

	product, err := Compose(three, five)
	if err != nil {
		t.Fatalf("compose: %v", err)
	}
	if !product.Equal(eightPow) {
		t.Fatal("f^3 * f^5 != f^8")
	}

	if _, err := Exp(f, big.NewInt(-1), d); err == nil {
		t.Fatal("a negative exponent was accepted")
	}
}

// -----------------------------------------------------------------------------
// Hashing to the group
// -----------------------------------------------------------------------------

func TestHashToFormIsDeterministicAndDistinct(t *testing.T) {
	d := testDiscriminant(t)

	first := testForm(t, d, "stable")
	for i := 0; i < 5; i++ {
		again := testForm(t, d, "stable")
		if !first.Equal(again) {
			t.Fatal("HashToForm is not deterministic; a verifier could not " +
				"reconstruct the VDF input")
		}
	}

	seen := map[string]bool{}
	for _, seed := range []string{"a", "b", "c", "d", "e", "f", "g", "h"} {
		f := testForm(t, d, seed)
		key := f.A.String() + "|" + f.B.String()
		if seen[key] {
			t.Fatalf("two seeds mapped to the same element (%q)", seed)
		}
		seen[key] = true

		if f.Discriminant().Cmp(d) != 0 {
			t.Fatalf("HashToForm returned the wrong discriminant for %q", seed)
		}
	}
}

func TestHashToFormRejectsABadDiscriminant(t *testing.T) {
	if _, err := HashToForm([]byte("x"), big.NewInt(-4)); err == nil {
		t.Fatal("a bad discriminant was accepted")
	}
}

// -----------------------------------------------------------------------------
// Encoding
// -----------------------------------------------------------------------------

func TestFormEncodingRoundTrips(t *testing.T) {
	d := testDiscriminant(t)

	for _, seed := range []string{"enc1", "enc2", "enc3"} {
		f := testForm(t, d, seed)

		decoded, err := FormFromBytes(f.Bytes(), d)
		if err != nil {
			t.Fatalf("decode %q: %v", seed, err)
		}
		if !decoded.Equal(f) {
			t.Fatalf("round trip changed the form for %q", seed)
		}
	}

	// A negative b must survive: Bytes() drops the sign, so it is carried
	// explicitly.
	f := testForm(t, d, "negative-b")
	negated := Form{A: f.A, B: new(big.Int).Neg(f.B), C: f.C}
	if negated.B.Sign() < 0 {
		decoded, err := FormFromBytes(negated.Bytes(), d)
		if err != nil {
			t.Fatalf("decode negative b: %v", err)
		}
		if decoded.B.Sign() >= 0 {
			t.Fatal("the sign of b was lost in encoding")
		}
	}
}

func TestFormDecodingRejectsGarbage(t *testing.T) {
	d := testDiscriminant(t)

	for _, bad := range [][]byte{
		nil,
		{},
		{1, 2, 3},
		make([]byte, 8),
		{0, 0, 0, 200, 1, 2, 3}, // a-length beyond the buffer
	} {
		if _, err := FormFromBytes(bad, d); err == nil {
			t.Fatalf("decoded garbage: %v", bad)
		}
	}

	// A well-formed encoding whose values do not belong to this discriminant.
	wrong := Form{A: big.NewInt(2), B: big.NewInt(0), C: big.NewInt(1)}
	if _, err := FormFromBytes(wrong.Bytes(), d); err == nil {
		t.Fatal("decoded a form of the wrong discriminant")
	}

	// An unreduced encoding of a real class must also be refused: equality is a
	// field comparison, so two encodings of one element would break it.
	f := testForm(t, d, "unreduced")
	unreduced := Form{
		A: new(big.Int).Set(f.A),
		B: new(big.Int).Add(f.B, new(big.Int).Lsh(f.A, 1)),
		C: new(big.Int).Add(new(big.Int).Add(f.C, f.A), f.B),
	}
	if unreduced.IsReduced() {
		t.Skip("the shifted form happened to stay reduced")
	}
	if _, err := FormFromBytes(unreduced.Bytes(), d); err == nil {
		t.Fatal("decoded an unreduced form; equality would no longer be canonical")
	}
}
