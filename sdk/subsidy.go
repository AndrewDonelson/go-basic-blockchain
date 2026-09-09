// Package sdk is a software development kit for building blockchain applications.
// File sdk/subsidy.go - The block subsidy and the emission reserve it is paid from.
package sdk

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
)

// THE SUBSIDY IS PAID FROM A RESERVE, NOT MINTED.
//
// The obvious way to pay miners is to let each block create new coins, as Bitcoin
// does. This chain does not, for two reasons.
//
// The supply is fixed and publishers hold it. A platform token whose quantity
// changes under its holders is a different product from one whose quantity does
// not, and predictability is worth more here than the elegance of an emission
// curve. TokenCount is the whole supply, for ever.
//
// And it bounds the damage from a mistake. A minting subsidy is only as safe as
// the check on its amount: get that wrong and the chain inflates without limit,
// which is exactly the hole that let any block mint 33,554,432 tokens before it
// was closed. A subsidy that moves coins out of a reserve cannot create any,
// whatever the schedule says -- the reserve runs dry and that is the end of it.
// The amount check is still there; it is simply no longer the only thing standing
// between the chain and unbounded inflation.

const (
	// defaultInitialSubsidy is the reward for the first halving era, in tokens.
	defaultInitialSubsidy = 50.0

	// defaultSubsidyHalvingInterval is how many blocks each era lasts.
	//
	// The geometric series 50 * interval * (1 + 1/2 + 1/4 + ...) converges to
	// 50 * interval * 2, so 210,000 emits 21,000,000 tokens over the life of the
	// chain -- comfortably inside TokenCount, leaving the remainder for the
	// genesis allocation.
	defaultSubsidyHalvingInterval = 210000

	// defaultReserveAllocationPCT is the share of TokenCount held back to pay
	// subsidies. The remainder is the genesis allocation.
	//
	// 63% of 33,554,432 is about 21.1M, which covers the 21M the default schedule
	// emits over the life of the chain with a little to spare.
	defaultReserveAllocationPCT = 63.0

	// maxSubsidyHalvings bounds the shift in the schedule. Past this the reward
	// is zero anyway; the bound exists so a huge height cannot shift by more than
	// the width of the type.
	maxSubsidyHalvings = 63
)

// ErrSubsidyAmount is returned when a block claims the wrong subsidy.
var ErrSubsidyAmount = errors.New("block subsidy does not match the schedule")

// DeriveReserveAddress returns the emission reserve's address.
//
// THE RESERVE HAS NO PRIVATE KEY, DELIBERATELY.
//
// It is the hash of a domain string rather than of a public key, so no keypair
// produces it and nobody can sign a transaction spending from it. The only way
// coins leave the reserve is the subsidy, whose amount consensus fixes by height
// and whose recipient is the block's miner.
//
// The alternative -- a real wallet holding the reserve -- puts the entire
// remaining emission behind one private key. Whoever held it could drain the
// reserve with an ordinary transfer, and on a chain where publishers are relying
// on the token that is not a risk worth carrying for the convenience of a
// signature nobody needs.
func DeriveReserveAddress(chainName string) string {
	digest := sha256.Sum256([]byte("gbb/emission/reserve/v1|" + chainName))
	return hex.EncodeToString(digest[:])
}

// subsidyHalvingInterval returns the configured era length.
func (bc *Blockchain) subsidyHalvingInterval() int64 {
	if bc.cfg != nil && bc.cfg.SubsidyHalvingInterval > 0 {
		return int64(bc.cfg.SubsidyHalvingInterval)
	}
	return defaultSubsidyHalvingInterval
}

// initialSubsidy returns the first-era reward in base units.
func (bc *Blockchain) initialSubsidy() int64 {
	if bc.cfg != nil && bc.cfg.InitialBlockSubsidy > 0 {
		return AmountToUnits(bc.cfg.InitialBlockSubsidy)
	}
	return AmountToUnits(defaultInitialSubsidy)
}

// BlockSubsidyUnits returns the subsidy a block at the given height may claim.
//
// It is a pure function of height, so every node computes the same value for the
// same block -- which is what lets a verifier check the amount rather than trust
// it. Genesis claims nothing: its allocation is the reserve itself.
func (bc *Blockchain) BlockSubsidyUnits(height int64) int64 {
	if height <= 0 {
		return 0
	}

	interval := bc.subsidyHalvingInterval()
	if interval <= 0 {
		return 0
	}

	halvings := height / interval
	// Bounded above by maxSubsidyHalvings and below by the height check at the
	// top, so the conversion cannot wrap and the shift cannot exceed the width of
	// the type -- which is the reason the bound exists at all.
	if halvings < 0 || halvings >= maxSubsidyHalvings {
		return 0
	}

	return bc.initialSubsidy() >> uint(halvings) //nolint:gosec // bounded immediately above
}

// reserveAddress returns the address the subsidy is paid from.
func (bc *Blockchain) reserveAddress() string {
	if bc.cfg == nil {
		return ""
	}
	return bc.cfg.ReserveAddress
}

// SubsidyFor reports what a block at this height may pay, and what the reserve
// can actually cover.
//
// The two differ once the reserve is nearly empty: the schedule keeps asking for
// more than is left. The payable amount is the smaller, and it reaches zero when
// the reserve is exhausted -- from then on miners are paid by transaction fees
// alone, which is the steady state the schedule exists to bridge to.
func (bc *Blockchain) SubsidyFor(height int64) (scheduled, payable int64) {
	scheduled = bc.BlockSubsidyUnits(height)
	if scheduled <= 0 {
		return 0, 0
	}

	reserve := bc.reserveAddress()
	if reserve == "" {
		return scheduled, 0
	}

	available := bc.GetBalanceUnits(reserve)
	if available < scheduled {
		return scheduled, available
	}
	return scheduled, scheduled
}

// buildSubsidyTransaction creates the coinbase paying a block's subsidy.
//
// It returns nil when there is nothing to pay, which is a normal condition rather
// than an error: the reserve is finite by design.
func (bc *Blockchain) buildSubsidyTransaction(height int64, minerAddress string) (*Coinbase, error) {
	if minerAddress == "" {
		return nil, nil
	}

	_, payable := bc.SubsidyFor(height)
	if payable <= 0 {
		return nil, nil
	}

	reserve := bc.reserveAddress()
	if reserve == "" {
		return nil, nil
	}

	// Wallet stubs carrying only addresses. A subsidy is authorised by the
	// consensus rules -- the amount is fixed by height and the source is the
	// reserve -- so there is no key to sign with and none is needed. That is the
	// same reason a coinbase has no signature in any chain.
	from := addressOnlyWallet(reserve)
	to := addressOnlyWallet(minerAddress)

	tx, err := NewCoinbaseTransaction(from, to, bc.cfg)
	if err != nil {
		return nil, fmt.Errorf("build subsidy transaction: %w", err)
	}

	tx.Fee = 0
	tx.SubsidyUnits = payable
	tx.BlockHeight = height
	tx.SetStatus(StatusConfirmed)

	return tx, nil
}

// addressOnlyWallet builds a wallet stub that carries an address and nothing
// else.
//
// The PUID is present because transaction construction requires one, not because
// the reserve has an identity. It is empty rather than invented: the asset ID
// inside a transaction's own PUID is random, so two subsidies still get distinct
// transaction IDs even though both parties look the same.
func addressOnlyWallet(address string) *Wallet {
	return &Wallet{Address: address, ID: NewPUIDEmpty()}
}

// validateSubsidyLocked checks a block's subsidy claim against the schedule.
//
// A block may carry at most one subsidy, it must be the first transaction, and
// the amount must be exactly what the height allows. Without the amount check a
// miner would simply write a larger number; without the position rule the
// "at most one" check would have to scan, and a canonical position makes the
// block's shape unambiguous for everything downstream.
func (bc *Blockchain) validateSubsidyLocked(block *Block) error {
	if block == nil {
		return errors.New("block is nil")
	}

	height := block.Index.Int64()
	if height == 0 {
		return nil // genesis is allocation, not subsidy
	}

	var subsidies int
	for i, tx := range block.Transactions {
		coinbase, ok := tx.(*Coinbase)
		if !ok {
			continue
		}
		subsidies++

		if subsidies > 1 {
			return fmt.Errorf("block %s carries %d subsidies; at most one is allowed",
				block.Index.String(), subsidies)
		}
		if i != 0 {
			return fmt.Errorf("block %s puts its subsidy at position %d; it must be first",
				block.Index.String(), i)
		}

		scheduled := bc.BlockSubsidyUnits(height)
		if coinbase.SubsidyUnits != scheduled {
			return fmt.Errorf("%w: block %s claims %s but height %d allows %s",
				ErrSubsidyAmount, block.Index.String(),
				formatUnits(coinbase.SubsidyUnits), height, formatUnits(scheduled))
		}
		if coinbase.BlockHeight != height {
			return fmt.Errorf("subsidy in block %s declares height %d",
				block.Index.String(), coinbase.BlockHeight)
		}
		if sender, _ := transactionParties(tx); sender != bc.reserveAddress() {
			return fmt.Errorf("subsidy in block %s is not drawn from the reserve",
				block.Index.String())
		}
	}

	return nil
}

// TotalEmission returns how much the schedule will pay out over the chain's life.
//
// Useful for sizing the reserve: a reserve smaller than this simply runs out
// early, which is a policy choice rather than a fault.
func (bc *Blockchain) TotalEmission() *big.Int {
	total := new(big.Int)
	interval := bc.subsidyHalvingInterval()
	initial := bc.initialSubsidy()

	for halving := int64(0); halving < maxSubsidyHalvings; halving++ {
		reward := initial >> uint(halving)
		if reward == 0 {
			break
		}
		total.Add(total, new(big.Int).Mul(big.NewInt(reward), big.NewInt(interval)))
	}
	return total
}
