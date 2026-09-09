// Package vdf implements a Wesolowski verifiable delay function over the class
// group of an imaginary quadratic field.
//
// # WHY A CLASS GROUP, AND NOT AN RSA GROUP
//
// A Wesolowski VDF needs a group whose order nobody knows. The security claim is
// that computing x^(2^T) takes T sequential squarings; anyone who knows the group
// order n can instead compute e = 2^T mod n and get the same answer with a single
// exponentiation. That is not a marginal speed-up, it is a complete break.
//
// The obvious choice, (Z/NZ)* for an RSA modulus N = p*q, has order (p-1)(q-1).
// Whoever generated N knows it. In a proof-of-work chain that party can produce
// the "delay" output instantly and mine as fast as they like while everyone else
// waits, so the parameters need either a multi-party ceremony nobody is around to
// run for a locally-installed SDK, or a modulus whose factors are asserted to
// have been destroyed -- an assertion no user can check.
//
// The class group of a negative prime discriminant has no such secret. Its order,
// the class number h(D), is believed hard to compute for large |D|, and crucially
// it is unknown to whoever chose D as well: picking the discriminant reveals
// nothing about the order. The discriminant here is derived deterministically
// from a public seed (see NewDiscriminant), so anyone can re-derive it and
// confirm no one had a chance to plant a trapdoor. There is no setup to trust
// because there is no secret to hold.
//
// The cost is arithmetic: elements are binary quadratic forms and composition is
// considerably more work than multiplying integers mod N. That is an
// implementation expense, not a weakening of the security argument, which is the
// right way round for a chain whose whole point is that no participant is
// privileged.
package vdf

import (
	"crypto/sha512"
	"encoding/binary"
	"errors"
	"fmt"
	"math/big"
)

// Form is a binary quadratic form ax^2 + bxy + cy^2, an element of the class
// group of discriminant D = b^2 - 4ac.
//
// Group elements are represented by their reduced form, which is unique per
// class -- that is what makes equality a byte comparison rather than a search.
type Form struct {
	A *big.Int
	B *big.Int
	C *big.Int
}

var (
	one   = big.NewInt(1)
	two   = big.NewInt(2)
	four  = big.NewInt(4)
	eight = big.NewInt(8)
)

// ErrBadDiscriminant is returned when a discriminant is unusable.
var ErrBadDiscriminant = errors.New("discriminant must be negative and congruent to 1 mod 8")

// NewDiscriminant derives a discriminant deterministically from a seed.
//
// D = -p for a prime p congruent to 7 mod 8, so D is congruent to 1 mod 8. That
// congruence is what lets HashToForm build a form for any odd prime coefficient:
// with D = 1 mod 8, every odd b satisfies b^2 = 1 = D (mod 8), so the mod-4a
// condition on the form reduces to the mod-a condition alone.
//
// Deriving from a seed rather than from randomness is the point. Anyone can
// re-run this and confirm the parameters were not chosen to have a convenient
// structure -- there is nothing up the sleeve, and unlike an RSA modulus there is
// no secret whose destruction has to be taken on faith.
func NewDiscriminant(seed []byte, bits int) (*big.Int, error) {
	if bits < 64 {
		return nil, fmt.Errorf("discriminant must be at least 64 bits, got %d", bits)
	}

	for counter := uint64(0); counter < 1<<24; counter++ {
		candidate := expandToBits(seed, counter, bits)

		// Top bit set so the value really has the requested size, and 7 mod 8.
		candidate.SetBit(candidate, bits-1, 1)
		candidate.Mod(candidate, new(big.Int).Lsh(one, uint(bits)))
		candidate.SetBit(candidate, bits-1, 1)

		remainder := new(big.Int).Mod(candidate, eight)
		candidate.Add(candidate, new(big.Int).Sub(big.NewInt(7), remainder))

		if candidate.ProbablyPrime(64) {
			return new(big.Int).Neg(candidate), nil
		}
	}

	return nil, errors.New("no suitable discriminant found")
}

// expandToBits stretches a seed and counter into an integer of the requested size.
func expandToBits(seed []byte, counter uint64, bits int) *big.Int {
	needed := (bits + 7) / 8
	out := make([]byte, 0, needed+sha512.Size)

	var block uint64
	for len(out) < needed {
		h := sha512.New()
		h.Write([]byte("gbb/vdf/discriminant/v1"))
		h.Write(seed)
		var buf [16]byte
		binary.BigEndian.PutUint64(buf[0:8], counter)
		binary.BigEndian.PutUint64(buf[8:16], block)
		h.Write(buf[:])
		out = append(out, h.Sum(nil)...)
		block++
	}

	return new(big.Int).SetBytes(out[:needed])
}

// ValidateDiscriminant checks a discriminant is of the form this package needs.
func ValidateDiscriminant(d *big.Int) error {
	if d == nil || d.Sign() >= 0 {
		return ErrBadDiscriminant
	}
	// -d mod 8 == 7  <=>  d mod 8 == 1
	mod := new(big.Int).Mod(d, eight)
	if mod.Cmp(one) != 0 {
		return ErrBadDiscriminant
	}
	return nil
}

// Identity returns the principal form, the group's identity element.
//
// For D congruent to 1 mod 4 this is (1, 1, (1-D)/4).
func Identity(d *big.Int) Form {
	c := new(big.Int).Sub(one, d)
	c.Div(c, four)
	return Form{A: big.NewInt(1), B: big.NewInt(1), C: c}
}

// Discriminant returns b^2 - 4ac.
func (f Form) Discriminant() *big.Int {
	bb := new(big.Int).Mul(f.B, f.B)
	ac := new(big.Int).Mul(f.A, f.C)
	ac.Mul(ac, four)
	return bb.Sub(bb, ac)
}

// Equal reports whether two forms are the same group element.
//
// Reduced forms are canonical, so this is exact rather than a heuristic --
// provided both sides are reduced, which every operation here guarantees.
func (f Form) Equal(g Form) bool {
	return f.A.Cmp(g.A) == 0 && f.B.Cmp(g.B) == 0 && f.C.Cmp(g.C) == 0
}

// Clone returns a deep copy, so callers cannot alias a form's internals.
func (f Form) Clone() Form {
	return Form{
		A: new(big.Int).Set(f.A),
		B: new(big.Int).Set(f.B),
		C: new(big.Int).Set(f.C),
	}
}

// normalize brings b into the interval (-a, a].
func normalize(f Form) Form {
	// r = floor((a - b) / 2a)
	numerator := new(big.Int).Sub(f.A, f.B)
	denominator := new(big.Int).Lsh(f.A, 1)
	r := new(big.Int).Div(numerator, denominator)
	if new(big.Int).Mod(numerator, denominator).Sign() < 0 {
		// big.Int.Div already floors toward negative infinity for positive
		// divisors, so this branch is defensive rather than expected.
		r.Sub(r, one)
	}

	// b' = b + 2ra ; c' = a r^2 + b r + c
	newB := new(big.Int).Mul(r, f.A)
	newB.Lsh(newB, 1)
	newB.Add(newB, f.B)

	ar2 := new(big.Int).Mul(r, r)
	ar2.Mul(ar2, f.A)
	br := new(big.Int).Mul(f.B, r)
	newC := new(big.Int).Add(ar2, br)
	newC.Add(newC, f.C)

	return Form{A: new(big.Int).Set(f.A), B: newB, C: newC}
}

// Reduce returns the unique reduced form equivalent to f.
//
// Reduced means -a < b <= a <= c, with b >= 0 when a == c. Uniqueness is what
// makes Equal a comparison instead of a search, and it is why every operation
// returns a reduced form rather than leaving it to the caller.
func Reduce(f Form) Form {
	g := normalize(f)

	for g.A.Cmp(g.C) > 0 || (g.A.Cmp(g.C) == 0 && g.B.Sign() < 0) {
		// (a, b, c) -> (c, -b, a), then renormalize.
		g = normalize(Form{
			A: new(big.Int).Set(g.C),
			B: new(big.Int).Neg(g.B),
			C: new(big.Int).Set(g.A),
		})
	}

	// The normalize step leaves b in (-a, a]; the only remaining case is a == c
	// with b negative, which the loop above has already handled.
	return g
}

// IsReduced reports whether f satisfies the reduction conditions.
func (f Form) IsReduced() bool {
	absB := new(big.Int).Abs(f.B)
	if absB.Cmp(f.A) > 0 || f.A.Cmp(f.C) > 0 {
		return false
	}
	if (absB.Cmp(f.A) == 0 || f.A.Cmp(f.C) == 0) && f.B.Sign() < 0 {
		return false
	}
	return true
}

// solveMod solves a*x = b (mod m), returning x and the modulus of the solution
// set. It reports whether a solution exists.
func solveMod(a, b, m *big.Int) (*big.Int, *big.Int, bool) {
	g := new(big.Int)
	d := new(big.Int)
	e := new(big.Int)
	g.GCD(d, e, new(big.Int).Mod(a, m), m)

	if new(big.Int).Mod(b, g).Sign() != 0 {
		return nil, nil, false
	}

	q := new(big.Int).Div(b, g)
	modulus := new(big.Int).Div(m, g)

	x := new(big.Int).Mul(q, d)
	x.Mod(x, modulus)

	return x, modulus, true
}

// Compose returns the product of two forms of the same discriminant.
//
// This is Gauss composition in the formulation used by the reference class-group
// implementations: it handles forms whose leading coefficients share a factor,
// which the textbook "united forms" shortcut does not, and which exponentiation
// hits constantly.
func Compose(f1, f2 Form) (Form, error) {
	a1, b1, c1 := f1.A, f1.B, f1.C
	a2, b2, c2 := f2.A, f2.B, f2.C

	g := new(big.Int).Add(b2, b1)
	g.Rsh(g, 1) // (b1 + b2) / 2
	h := new(big.Int).Sub(b2, b1)
	h.Rsh(h, 1) // (b2 - b1) / 2

	w := new(big.Int).GCD(nil, nil, new(big.Int).Abs(a1), new(big.Int).Abs(a2))
	w.GCD(nil, nil, w, new(big.Int).Abs(g))
	if w.Sign() == 0 {
		return Form{}, errors.New("degenerate forms: gcd is zero")
	}

	j := new(big.Int).Set(w)
	s := new(big.Int).Div(a1, w)
	t := new(big.Int).Div(a2, w)
	u := new(big.Int).Div(g, w)

	// Solve t*u*k = h*u + s*c1 (mod s*t), then refine modulo s.
	st := new(big.Int).Mul(s, t)
	rhs := new(big.Int).Mul(h, u)
	rhs.Add(rhs, new(big.Int).Mul(s, c1))

	kTemp, constantFactor, ok := solveMod(new(big.Int).Mul(t, u), rhs, st)
	if !ok {
		return Form{}, errors.New("composition has no solution in the first congruence")
	}

	rhs2 := new(big.Int).Mul(t, kTemp)
	rhs2.Sub(h, rhs2)
	n, _, ok := solveMod(new(big.Int).Mul(t, constantFactor), rhs2, s)
	if !ok {
		return Form{}, errors.New("composition has no solution in the second congruence")
	}

	k := new(big.Int).Mul(constantFactor, n)
	k.Add(k, kTemp)

	// l = (t*k - h) / s
	l := new(big.Int).Mul(t, k)
	l.Sub(l, h)
	l.Div(l, s)

	// m = (t*u*k - h*u - c1*s) / (s*t)
	m := new(big.Int).Mul(t, u)
	m.Mul(m, k)
	m.Sub(m, new(big.Int).Mul(h, u))
	m.Sub(m, new(big.Int).Mul(c1, s))
	m.Div(m, st)

	// A = s*t ; B = j*u - (k*t + l*s) ; C = k*l - j*m
	newA := new(big.Int).Mul(s, t)

	newB := new(big.Int).Mul(j, u)
	kt := new(big.Int).Mul(k, t)
	ls := new(big.Int).Mul(l, s)
	newB.Sub(newB, new(big.Int).Add(kt, ls))

	newC := new(big.Int).Mul(k, l)
	newC.Sub(newC, new(big.Int).Mul(j, m))

	_ = c2 // c2 is implied by the discriminant; kept for symmetry of the inputs.

	return Reduce(Form{A: newA, B: newB, C: newC}), nil
}

// Square returns f composed with itself.
//
// Squaring is specialised rather than routed through Compose because the VDF does
// almost nothing else: evaluating a delay of T is T squarings, and proving it is
// T more. Composing a form with itself collapses most of the general algorithm --
// with a1 == a2 and b1 == b2 the difference h is zero and s equals t, so the
// second modular congruence becomes 0*x = 0 and disappears, leaving one solve
// instead of two.
//
// Derivation, from the same formulas Compose uses, with w = gcd(a, b), s = a/w
// and u = b/w:
//
//	solve   u*k = c   (mod s)
//	A = s^2
//	B = b - 2ks
//	C = k^2 - w*(u*k - c)/s
//
// This is the same group element Compose(f, f) produces -- not an approximation
// of it -- and TestSquareMatchesComposition checks that on every element the
// tests can reach, including the degenerate ones. If the congruence has no
// solution the general path runs instead, so a case this derivation does not
// cover cannot produce a wrong answer, only a slower one.
func Square(f Form) (Form, error) {
	w := new(big.Int).GCD(nil, nil, new(big.Int).Abs(f.A), new(big.Int).Abs(f.B))
	if w.Sign() == 0 {
		return Compose(f, f)
	}

	s := new(big.Int).Div(f.A, w)
	u := new(big.Int).Div(f.B, w)

	if s.Sign() == 0 {
		return Compose(f, f)
	}

	k, _, ok := solveMod(u, f.C, s)
	if !ok {
		return Compose(f, f)
	}

	// m = (u*k - c) / s, exact because u*k = c (mod s).
	m := new(big.Int).Mul(u, k)
	m.Sub(m, f.C)
	quotient, remainder := new(big.Int).QuoRem(m, s, new(big.Int))
	if remainder.Sign() != 0 {
		return Compose(f, f)
	}

	newA := new(big.Int).Mul(s, s)

	newB := new(big.Int).Mul(k, s)
	newB.Lsh(newB, 1)
	newB.Sub(f.B, newB)

	newC := new(big.Int).Mul(k, k)
	newC.Sub(newC, new(big.Int).Mul(w, quotient))

	return Reduce(Form{A: newA, B: newB, C: newC}), nil
}

// Inverse returns the inverse of f, which is (a, -b, c).
func Inverse(f Form) Form {
	return Reduce(Form{
		A: new(big.Int).Set(f.A),
		B: new(big.Int).Neg(f.B),
		C: new(big.Int).Set(f.C),
	})
}

// Exp returns f raised to a non-negative exponent by square-and-multiply.
func Exp(f Form, exponent *big.Int, d *big.Int) (Form, error) {
	if exponent.Sign() < 0 {
		return Form{}, errors.New("negative exponents are not supported")
	}

	result := Identity(d)
	base := f.Clone()

	for i := 0; i < exponent.BitLen(); i++ {
		if exponent.Bit(i) == 1 {
			var err error
			result, err = Compose(result, base)
			if err != nil {
				return Form{}, err
			}
		}
		var err error
		base, err = Square(base)
		if err != nil {
			return Form{}, err
		}
	}

	return result, nil
}

// HashToForm maps a seed to a group element of the given discriminant.
//
// It searches for an odd prime a for which D is a quadratic residue, takes b as a
// square root of D modulo a, and forces b odd. Because D is 1 mod 8, b^2 = D
// holds modulo 8 for any odd b, so b^2 = D (mod 4a) follows from the mod-a
// condition -- which is what makes c = (b^2 - D)/4a an integer.
//
// Choosing the element by hashing rather than by fixing a generator matters: the
// VDF's input has to be outside the prover's control, and an element whose
// discrete log to some published base were known would let the prover shortcut
// the delay.
func HashToForm(seed []byte, d *big.Int) (Form, error) {
	if err := ValidateDiscriminant(d); err != nil {
		return Form{}, err
	}

	for counter := uint64(0); counter < 1<<20; counter++ {
		h := sha512.New()
		h.Write([]byte("gbb/vdf/hashtoform/v1"))
		h.Write(seed)
		var buf [8]byte
		binary.BigEndian.PutUint64(buf[:], counter)
		h.Write(buf[:])

		// A modest candidate: the element only needs to be unpredictable, and a
		// small leading coefficient keeps composition cheap.
		a := new(big.Int).SetBytes(h.Sum(nil)[:8])
		a.SetBit(a, 0, 1) // odd
		if a.Cmp(two) <= 0 {
			continue
		}
		if !a.ProbablyPrime(32) {
			continue
		}
		if big.Jacobi(d, a) != 1 {
			continue
		}

		dModA := new(big.Int).Mod(d, a)
		b := new(big.Int).ModSqrt(dModA, a)
		if b == nil {
			continue
		}
		if b.Bit(0) == 0 {
			b.Sub(a, b) // a is odd, so this flips the parity
		}

		// c = (b^2 - D) / 4a
		bb := new(big.Int).Mul(b, b)
		bb.Sub(bb, d)
		denominator := new(big.Int).Lsh(a, 2)
		c, remainder := new(big.Int).QuoRem(bb, denominator, new(big.Int))
		if remainder.Sign() != 0 {
			continue
		}

		form := Reduce(Form{A: a, B: b, C: c})
		if form.Discriminant().Cmp(d) != 0 {
			continue
		}
		return form, nil
	}

	return Form{}, errors.New("could not map the seed to a group element")
}

// Bytes returns a deterministic encoding of a reduced form.
//
// a and b determine the form given the discriminant, so c is not encoded; it is
// recomputed on decode. Lengths are prefixed because the two values vary in size
// and a bare concatenation would be ambiguous -- the same hazard the block header
// serialisation had to fix.
func (f Form) Bytes() []byte {
	aBytes := f.A.Bytes()
	bBytes := f.B.Bytes()

	// A coefficient is bounded by the discriminant, which is a few hundred bytes
	// at most, so these can never overflow. The check is here because a silently
	// truncated length would make two different forms encode identically -- and
	// the encoding feeds the Fiat-Shamir challenge, where a collision means two
	// statements share a challenge.
	assertEncodableLength(len(aBytes))
	assertEncodableLength(len(bBytes))

	out := make([]byte, 0, 10+len(aBytes)+len(bBytes))
	var length [4]byte

	binary.BigEndian.PutUint32(length[:], uint32(len(aBytes))) //nolint:gosec // bounded by assertEncodableLength above
	out = append(out, length[:]...)
	out = append(out, aBytes...)

	// The sign of b is carried explicitly: Bytes() drops it.
	if f.B.Sign() < 0 {
		out = append(out, 1)
	} else {
		out = append(out, 0)
	}
	binary.BigEndian.PutUint32(length[:], uint32(len(bBytes))) //nolint:gosec // bounded by assertEncodableLength above
	out = append(out, length[:]...)
	out = append(out, bBytes...)

	return out
}

// assertEncodableLength guards the 32-bit length prefixes used by Bytes.
func assertEncodableLength(n int) {
	if n < 0 || int64(n) > int64(^uint32(0)) {
		panic("vdf: value too large for a 32-bit length prefix")
	}
}

// FormFromBytes decodes a form encoded by Bytes, recomputing c from the
// discriminant.
func FormFromBytes(data []byte, d *big.Int) (Form, error) {
	if len(data) < 9 {
		return Form{}, errors.New("encoded form is too short")
	}

	offset := 0
	aLen := int(binary.BigEndian.Uint32(data[offset:]))
	offset += 4
	if aLen < 0 || offset+aLen > len(data) {
		return Form{}, errors.New("encoded form has a bad a-length")
	}
	a := new(big.Int).SetBytes(data[offset : offset+aLen])
	offset += aLen

	if offset >= len(data) {
		return Form{}, errors.New("encoded form is truncated before b")
	}
	negative := data[offset] == 1
	offset++

	if offset+4 > len(data) {
		return Form{}, errors.New("encoded form is truncated before the b-length")
	}
	bLen := int(binary.BigEndian.Uint32(data[offset:]))
	offset += 4
	if bLen < 0 || offset+bLen > len(data) {
		return Form{}, errors.New("encoded form has a bad b-length")
	}
	b := new(big.Int).SetBytes(data[offset : offset+bLen])
	if negative {
		b.Neg(b)
	}

	if a.Sign() <= 0 {
		return Form{}, errors.New("encoded form has a non-positive leading coefficient")
	}

	// c = (b^2 - D) / 4a, and it must divide exactly or the encoding is not a
	// form of this discriminant.
	bb := new(big.Int).Mul(b, b)
	bb.Sub(bb, d)
	denominator := new(big.Int).Lsh(a, 2)
	c, remainder := new(big.Int).QuoRem(bb, denominator, new(big.Int))
	if remainder.Sign() != 0 {
		return Form{}, errors.New("encoded form does not have the expected discriminant")
	}

	form := Form{A: a, B: b, C: c}

	// Reject anything that is not already reduced.
	//
	// Equality here is a field comparison, which is only sound because reduced
	// forms are canonical. Accepting an unreduced encoding would mean two
	// different byte strings denote the same group element -- so a prover could
	// hand over a proof that verifies against one encoding and not another, and
	// anything that hashes the encoding would disagree with anything that
	// compares the element.
	if !form.IsReduced() {
		return Form{}, errors.New("encoded form is not reduced")
	}

	return form, nil
}
