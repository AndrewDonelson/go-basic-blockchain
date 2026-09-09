// Package sdk is a software development kit for building blockchain applications.
// File sdk/utxo.go - the UTXO set: the authoritative account state
//
// Balances used to have two sources of truth that never agreed. Blockchain.GetBalance
// rescanned every block on every call, and each wallet also carried its own number
// in its encrypted store; NewBankTransaction checked affordability against the
// wallet's copy, which nothing kept in step with the chain. Neither prevented a
// double spend, because neither knew what had already been spent.
//
// This file replaces both with one set of unspent outputs derived deterministically
// from the blocks. Every node that applies the same blocks arrives at the same set.
//
// # Implicit inputs
//
// A Bank transaction here is {From, To, Amount} -- it does not carry an explicit
// list of inputs the way a Bitcoin transaction does. Rather than rewrite the
// transaction format (and with it the wire codec, the P2P protocol and the course
// material), inputs are selected DETERMINISTICALLY at apply time: the sender's
// outputs ordered by (block index, transaction ID, output index), consumed until
// the amount plus fee is covered.
//
// That ordering is what makes the model sound. Every node selects the same inputs
// in the same order, so every node produces the same UTXO set, and a spend can be
// checked against outputs that actually remain. The tradeoff is that a transaction
// cannot be validated in isolation -- you need the set as of its block. Explicit
// inputs would remove that constraint and are the natural next step.
//
// # Integer amounts
//
// Balances are held as int64 base units, not float64. Float arithmetic does not
// represent 0.1 exactly, so repeatedly crediting and debiting float balances
// accumulates error -- unacceptable in the component that is supposed to be
// authoritative about who owns what.
package sdk

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"sync"
)

const (
	// UnitsPerToken is the number of indivisible base units in one token.
	// Amounts are stored as int64 units so balance arithmetic is exact.
	UnitsPerToken = 100_000_000

	// maxCoinSelectionInputs bounds how many outputs one transaction may consume,
	// so a wallet holding a very large number of dust outputs cannot make block
	// assembly pathological.
	maxCoinSelectionInputs = 1000
)

var (
	// ErrInsufficientFunds is returned when a spender's unspent outputs do not
	// cover the amount plus the fee.
	ErrInsufficientFunds = errors.New("insufficient unspent outputs")

	// ErrNonceNotIncreasing is returned when a transaction reuses or goes back
	// below a nonce its sender has already had confirmed.
	ErrNonceNotIncreasing = errors.New("transaction nonce does not increase")

	// ErrOutputAlreadySpent is returned when a block tries to consume an output
	// that is not in the set -- a double spend.
	ErrOutputAlreadySpent = errors.New("output is already spent or does not exist")
)

// AmountToUnits converts a token amount to base units, rounding to the nearest
// unit. Values that are not finite become zero rather than a nonsense int64.
func AmountToUnits(amount float64) int64 {
	if math.IsNaN(amount) || math.IsInf(amount, 0) {
		return 0
	}
	return int64(math.Round(amount * UnitsPerToken))
}

// UnitsToAmount converts base units back to a token amount for display.
func UnitsToAmount(units int64) float64 {
	return float64(units) / UnitsPerToken
}

// Outpoint identifies one output of one transaction.
type Outpoint struct {
	TxID  string `json:"tx_id"`
	Index uint32 `json:"index"`
}

// String renders an outpoint as "txid:index".
func (o Outpoint) String() string {
	return fmt.Sprintf("%s:%d", o.TxID, o.Index)
}

// UTXO is a single unspent transaction output.
type UTXO struct {
	Outpoint   Outpoint `json:"outpoint"`
	Address    string   `json:"address"`
	Units      int64    `json:"units"`
	BlockIndex int64    `json:"block_index"`
	// Coinbase marks newly minted supply, which is worth distinguishing when
	// auditing where tokens came from.
	Coinbase bool `json:"coinbase"`
}

// Amount returns the output's value in tokens.
func (u *UTXO) Amount() float64 { return UnitsToAmount(u.Units) }

// BlockUndo records what applying one block changed, so it can be reverted.
//
// Reorganisation needs this: without an undo record, switching branches would
// require replaying the chain from genesis to rebuild the set.
type BlockUndo struct {
	BlockHash  string
	BlockIndex int64
	// Spent are the outputs consumed by this block, restored on revert.
	Spent []*UTXO
	// Created are the outputs added by this block, removed on revert.
	Created []Outpoint
	// Nonces holds each sender's highest confirmed nonce as it was *before* this
	// block, so a reorg restores it. Without this a rolled-back transaction
	// would leave its nonce recorded and the sender could never resubmit it on
	// the new branch.
	Nonces map[string]nonceRestore
	// Anchors holds each sidechain's anchor as it was before this block, for the
	// same reason: a rolled-back anchor must stop counting as committed, or the
	// publisher's next anchor fails contiguity against a commitment that is no
	// longer on the chain.
	Anchors []anchorRestore
	// Registrations holds the ids this block allocated, so a rollback releases
	// them. Leaving an id allocated after its registration was rolled back would
	// let the next registrant receive an id that a still-circulating anchor
	// refers to.
	Registrations []registrationRestore
	// Availability holds each challenge's state before this block, so a rollback
	// reopens what it closed. A challenge that stayed closed after the block
	// answering it was dropped would let a publisher clear an obligation with a
	// proof no longer on the chain.
	Availability []availabilityRestore
	// Bonds holds what this block slashed, per publisher.
	Bonds map[uint64]int64
}

// registrationRestore identifies an allocation to release on rollback.
type registrationRestore struct {
	Kind        uint32
	PublisherID uint64
	GameID      uint64
}

// nonceRestore remembers a sender's prior nonce, distinguishing "had none" from
// "had zero" -- a sender's first transaction may legitimately use nonce 0.
type nonceRestore struct {
	Nonce   uint64
	Existed bool
}

// UTXOSet is the set of unspent outputs, and the authoritative record of balances.
type UTXOSet struct {
	mu sync.RWMutex

	outputs   map[Outpoint]*UTXO
	byAddress map[string]map[Outpoint]struct{}
	undo      map[string]*BlockUndo
	// nonces is each sender's highest confirmed transaction nonce. A sender's
	// next transaction must exceed it, which is what stops an old transaction
	// being replayed onto the chain a second time.
	nonces map[string]uint64
	// applied tracks block hashes in application order, so double-application is
	// detectable and reverts can be checked against the tip.
	applied []string
	// anchors is the main chain's record of what each sidechain has committed.
	// It lives here because this is where block application and rollback already
	// happen, and an anchor that survived a reorg would be a commitment to a
	// history the chain no longer contains.
	anchors *AnchorLedger
	// availability tracks open challenges, for the same reason and in the same
	// place.
	availability *AvailabilityLedger
}

// NewUTXOSet creates an empty set.
func NewUTXOSet() *UTXOSet {
	set := &UTXOSet{
		outputs:   map[Outpoint]*UTXO{},
		byAddress: map[string]map[Outpoint]struct{}{},
		undo:      map[string]*BlockUndo{},
		nonces:    map[string]uint64{},
		anchors:   NewAnchorLedger(NewPublisherRegistry()),
	}
	set.availability = NewAvailabilityLedger(set.anchors)
	return set
}

// BalanceUnits returns an address's spendable balance in base units.
func (s *UTXOSet) BalanceUnits(address string) int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.balanceUnitsLocked(address)
}

func (s *UTXOSet) balanceUnitsLocked(address string) int64 {
	var total int64
	for outpoint := range s.byAddress[address] {
		if utxo, ok := s.outputs[outpoint]; ok {
			total += utxo.Units
		}
	}
	return total
}

// Balance returns an address's spendable balance in tokens.
func (s *UTXOSet) Balance(address string) float64 {
	return UnitsToAmount(s.BalanceUnits(address))
}

// OutputsFor returns an address's unspent outputs in deterministic order.
func (s *UTXOSet) OutputsFor(address string) []*UTXO {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.outputsForLocked(address)
}

// outputsForLocked returns the address's outputs sorted by (block index,
// transaction ID, output index).
//
// The ordering is the load-bearing part: it is what makes implicit input
// selection identical on every node. Map iteration order is random in Go, so
// selecting without sorting would give different nodes different UTXO sets from
// the same blocks.
func (s *UTXOSet) outputsForLocked(address string) []*UTXO {
	owned := s.byAddress[address]
	out := make([]*UTXO, 0, len(owned))
	for outpoint := range owned {
		if utxo, ok := s.outputs[outpoint]; ok {
			out = append(out, utxo)
		}
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].BlockIndex != out[j].BlockIndex {
			return out[i].BlockIndex < out[j].BlockIndex
		}
		if out[i].Outpoint.TxID != out[j].Outpoint.TxID {
			return out[i].Outpoint.TxID < out[j].Outpoint.TxID
		}
		return out[i].Outpoint.Index < out[j].Outpoint.Index
	})
	return out
}

// Size returns how many unspent outputs the set holds.
func (s *UTXOSet) Size() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.outputs)
}

// TotalUnits returns the sum of every unspent output: the circulating supply.
func (s *UTXOSet) TotalUnits() int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var total int64
	for _, utxo := range s.outputs {
		total += utxo.Units
	}
	return total
}

// Has reports whether an outpoint is unspent.
func (s *UTXOSet) Has(outpoint Outpoint) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.outputs[outpoint]
	return ok
}

// HasBlock reports whether a block has been applied to this set.
func (s *UTXOSet) HasBlock(hash string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.undo[hash]
	return ok
}

// addLocked inserts an output.
func (s *UTXOSet) addLocked(utxo *UTXO) {
	s.outputs[utxo.Outpoint] = utxo
	if s.byAddress[utxo.Address] == nil {
		s.byAddress[utxo.Address] = map[Outpoint]struct{}{}
	}
	s.byAddress[utxo.Address][utxo.Outpoint] = struct{}{}
}

// removeLocked deletes an output, returning it.
func (s *UTXOSet) removeLocked(outpoint Outpoint) (*UTXO, bool) {
	utxo, ok := s.outputs[outpoint]
	if !ok {
		return nil, false
	}

	delete(s.outputs, outpoint)
	if owned, ok := s.byAddress[utxo.Address]; ok {
		delete(owned, outpoint)
		if len(owned) == 0 {
			delete(s.byAddress, utxo.Address)
		}
	}
	return utxo, true
}

// feeSplit describes where transaction fees are paid.
type feeSplit struct {
	MinerAddress string
	MinerPercent float64
	DevAddress   string
	DevPercent   float64
}

// formatUnits renders base units as a token amount for error messages.
func formatUnits(units int64) string {
	return fmt.Sprintf("%.8f", UnitsToAmount(units))
}

// ApplyBlock applies every transaction in a block to the set.
//
// Either the whole block applies or nothing does: the changes are staged and only
// committed once every transaction has been processed. A partially applied block
// would leave the set describing a state no chain ever had.
func (s *UTXOSet) ApplyBlock(block *Block, split feeSplit) (*BlockUndo, error) {
	if block == nil {
		return nil, errors.New("block is nil")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.undo[block.Hash]; exists {
		return nil, fmt.Errorf("block %s has already been applied", block.Index.String())
	}

	blockIndex := block.Index.Int64()
	undo := &BlockUndo{
		BlockHash:  block.Hash,
		BlockIndex: blockIndex,
		Nonces:     map[string]nonceRestore{},
		Bonds:      map[uint64]int64{},
	}

	// Stage the changes so a failure part-way leaves the set untouched.
	staged := newStagedChanges(s)

	for _, tx := range block.Transactions {
		if tx == nil {
			return nil, errors.New("block contains a nil transaction")
		}
		if err := s.applyTransactionLocked(tx, blockIndex, split, staged, undo); err != nil {
			return nil, fmt.Errorf("block %s, transaction %s: %w",
				block.Index.String(), tx.GetID(), err)
		}
	}

	// Expiry is swept after the block's transactions, so a proof included in this
	// very block still counts. A publisher who answers on the deadline block
	// should not be slashed by an ordering accident.
	if err := s.expireChallengesLocked(blockIndex, staged, undo); err != nil {
		return nil, fmt.Errorf("block %s: %w", block.Index.String(), err)
	}

	staged.commit()
	s.undo[block.Hash] = undo
	s.applied = append(s.applied, block.Hash)
	return undo, nil
}

// applyTransactionLocked applies a single transaction.
func (s *UTXOSet) applyTransactionLocked(tx Transaction, blockIndex int64, split feeSplit, staged *stagedChanges, undo *BlockUndo) error {
	txID := tx.GetID()
	if txID == "" {
		return errors.New("transaction has no ID")
	}

	sender, recipient := transactionParties(tx)
	feeUnits := AmountToUnits(tx.GetFee())
	if feeUnits < 0 {
		return errors.New("transaction fee is negative")
	}

	// nextIndex numbers this transaction's outputs.
	var nextIndex uint32
	emit := func(address string, units int64, coinbase bool) {
		if units <= 0 || address == "" {
			return
		}
		utxo := &UTXO{
			Outpoint:   Outpoint{TxID: txID, Index: nextIndex},
			Address:    address,
			Units:      units,
			BlockIndex: blockIndex,
			Coinbase:   coinbase,
		}
		nextIndex++
		staged.add(utxo)
		undo.Created = append(undo.Created, utxo.Outpoint)
	}

	// A sender's nonce must strictly increase.
	//
	// This is what makes a transaction unrepeatable. Without it a transaction
	// already mined could be handed back to the chain -- on a fresh branch after
	// a reorg, or by any peer that kept a copy -- and applied a second time,
	// spending the sender's coins again. The nonce is covered by the signature,
	// so it cannot be edited to make an old transaction look new.
	//
	// A coinbase is exempt: it has no sender and mints rather than spends.
	// A coinbase is exempt from the nonce rule. Genesis mints and has no sender;
	// a subsidy is authorised by height rather than by a sender's sequence, and
	// the height already makes it unrepeatable -- one subsidy per block, checked
	// against the schedule.
	if _, isCoinbase := tx.(*Coinbase); !isCoinbase && sender != "" {
		nonce := tx.GetNonce()
		if last, seen := s.nonces[sender]; seen && nonce <= last {
			return fmt.Errorf("%w: %s already used nonce %d, this transaction uses %d",
				ErrNonceNotIncreasing, sender, last, nonce)
		}
		if undo != nil {
			// Record the prior value once per sender per block, so reverting a
			// block with several transactions from one sender restores the value
			// from before the block rather than from mid-block.
			if _, already := undo.Nonces[sender]; !already {
				prior, existed := s.nonces[sender]
				undo.Nonces[sender] = nonceRestore{Nonce: prior, Existed: existed}
			}
		}
		s.nonces[sender] = nonce
	}

	switch concrete := tx.(type) {
	case *Coinbase:
		if recipient == "" {
			return errors.New("coinbase transaction has no recipient")
		}

		// Genesis: the one place new supply comes into existence.
		if blockIndex == 0 {
			units := concrete.TokenCount * UnitsPerToken
			if units < 0 {
				return errors.New("coinbase token count is negative")
			}
			emit(recipient, units, true)
			return nil
		}

		// Every later block: the subsidy is a TRANSFER OUT OF THE RESERVE, never
		// newly minted supply.
		//
		// This is what keeps the supply fixed, and it is also what bounds the
		// damage if the amount check above it were ever wrong: a transfer cannot
		// create coins, so the worst case is that the reserve empties early. A
		// minting subsidy has no such floor -- get its amount check wrong and the
		// chain inflates without limit, which is exactly the hole that once let
		// any block mint the entire supply.
		if concrete.SubsidyUnits <= 0 {
			return fmt.Errorf("coinbase transaction %s in block %d pays nothing",
				txID, blockIndex)
		}
		if sender == "" {
			return fmt.Errorf("subsidy %s has no reserve to draw from", txID)
		}

		spent, totalIn, err := s.spendLocked(sender, concrete.SubsidyUnits, staged, undo)
		if err != nil {
			return fmt.Errorf("subsidy %s: %w", txID, err)
		}
		_ = spent

		emit(recipient, concrete.SubsidyUnits, true)
		if change := totalIn - concrete.SubsidyUnits; change > 0 {
			emit(sender, change, false)
		}
		return nil

	case *Bank:
		amountUnits := AmountToUnits(concrete.Amount)
		if amountUnits <= 0 {
			return errors.New("bank transaction amount must be positive")
		}
		if recipient == "" {
			return errors.New("bank transaction has no recipient")
		}

		spent, totalIn, err := s.spendLocked(sender, amountUnits+feeUnits, staged, undo)
		if err != nil {
			return err
		}

		emit(recipient, amountUnits, false)
		s.emitFees(feeUnits, split, emit)

		// Change back to the sender.
		if change := totalIn - amountUnits - feeUnits; change > 0 {
			emit(sender, change, false)
		}
		_ = spent
		return nil

	case *RegistrationTx:
		// The registration is applied before the bond is taken, for the same
		// reason the anchor is: a refused registration must leave the whole
		// transaction without effect rather than charging for a claim that was
		// not granted.
		if err := concrete.VerifyRegistrationSignature(s.anchors.Registry()); err != nil {
			return fmt.Errorf("registration %s: %w", txID, err)
		}

		registry := s.anchors.Registry()

		switch concrete.Kind {
		case RegistrationKindPublisher:
			publisherID, err := registry.RegisterPublisher(concrete.PublicKeyPEM,
				concrete.Name, concrete.BondUnits, blockIndex)
			if err != nil {
				return fmt.Errorf("registration %s: %w", txID, err)
			}
			if undo != nil {
				undo.Registrations = append(undo.Registrations, registrationRestore{
					Kind: RegistrationKindPublisher, PublisherID: publisherID,
				})
			}

			// The bond moves to an address with no private key, so only a
			// consensus rule can ever move it again.
			bondAddress := DerivePublisherBondAddress(publisherID)
			_, totalIn, err := s.spendLocked(sender, concrete.BondUnits+feeUnits, staged, undo)
			if err != nil {
				return fmt.Errorf("registration %s: %w", txID, err)
			}

			emit(bondAddress, concrete.BondUnits, false)
			s.emitFees(feeUnits, split, emit)
			if change := totalIn - concrete.BondUnits - feeUnits; change > 0 {
				emit(sender, change, false)
			}
			return nil

		case RegistrationKindGame:
			gameID, err := registry.RegisterGame(concrete.PublisherID, concrete.Name, blockIndex)
			if err != nil {
				return fmt.Errorf("registration %s: %w", txID, err)
			}
			if undo != nil {
				undo.Registrations = append(undo.Registrations, registrationRestore{
					Kind:        RegistrationKindGame,
					PublisherID: concrete.PublisherID,
					GameID:      gameID,
				})
			}

			if feeUnits == 0 {
				return nil
			}
			_, totalIn, err := s.spendLocked(sender, feeUnits, staged, undo)
			if err != nil {
				return err
			}
			s.emitFees(feeUnits, split, emit)
			if change := totalIn - feeUnits; change > 0 {
				emit(sender, change, false)
			}
			return nil

		default:
			return fmt.Errorf("registration %s: %w: unknown kind %d",
				txID, ErrRegistrationInvalid, concrete.Kind)
		}

	case *AvailabilityTx:
		switch concrete.Kind {
		case AvailabilityKindChallenge:
			// The challenger's stake goes to an address with no private key and
			// stays there until the challenge resolves, so neither side can
			// withdraw it while the question is open.
			restore, err := s.availability.openForUndo(concrete.Challenge, blockIndex)
			if err != nil {
				return fmt.Errorf("challenge %s: %w", txID, err)
			}
			undo.Availability = append(undo.Availability, restore)

			escrow := DeriveChallengeEscrowAddress(concrete.Challenge.ID())
			_, totalIn, err := s.spendLocked(sender,
				AvailabilityChallengeFeeUnits+feeUnits, staged, undo)
			if err != nil {
				return fmt.Errorf("challenge %s: %w", txID, err)
			}

			emit(escrow, AvailabilityChallengeFeeUnits, false)
			s.emitFees(feeUnits, split, emit)
			if change := totalIn - AvailabilityChallengeFeeUnits - feeUnits; change > 0 {
				emit(sender, change, false)
			}
			return nil

		case AvailabilityKindProof:
			restore, challenge, err := s.availability.answerForUndo(concrete.Proof)
			if err != nil {
				return fmt.Errorf("proof %s: %w", txID, err)
			}
			undo.Availability = append(undo.Availability, restore)

			// The stake goes to whoever answered, which is what makes a
			// frivolous challenge pay the person it inconvenienced.
			escrow := DeriveChallengeEscrowAddress(challenge.ID())
			staked, _, err := s.spendLocked(escrow, AvailabilityChallengeFeeUnits, staged, undo)
			if err != nil {
				return fmt.Errorf("proof %s: %w", txID, err)
			}
			_ = staked

			if sender != "" {
				emit(sender, AvailabilityChallengeFeeUnits, false)
			}

			if feeUnits > 0 {
				_, totalIn, err := s.spendLocked(sender, feeUnits, staged, undo)
				if err != nil {
					return err
				}
				s.emitFees(feeUnits, split, emit)
				if change := totalIn - feeUnits; change > 0 {
					emit(sender, change, false)
				}
			}
			return nil

		default:
			return fmt.Errorf("availability %s: %w: unknown kind %d",
				txID, ErrChallengeInvalid, concrete.Kind)
		}

	case *AnchorTx:
		// The anchor is the whole point of the transaction, so it is recorded
		// before the fee is taken: if the commitment is refused -- wrong
		// publisher, or a range that does not follow what is already anchored --
		// nothing about this transaction should take effect.
		restore, err := s.anchors.recordForUndo(concrete.Anchor, concrete.AnchorSignature, blockIndex)
		if err != nil {
			return fmt.Errorf("anchor %s: %w", txID, err)
		}
		if undo != nil {
			undo.Anchors = append(undo.Anchors, restore)
		}

		if feeUnits == 0 {
			return nil
		}

		_, totalIn, err := s.spendLocked(sender, feeUnits, staged, undo)
		if err != nil {
			return err
		}

		s.emitFees(feeUnits, split, emit)
		if change := totalIn - feeUnits; change > 0 {
			emit(sender, change, false)
		}
		return nil

	default:
		// Fee-only protocols: MESSAGE, PERSIST, CHAIN, P2P.
		if feeUnits == 0 {
			return nil
		}

		_, totalIn, err := s.spendLocked(sender, feeUnits, staged, undo)
		if err != nil {
			return err
		}

		s.emitFees(feeUnits, split, emit)
		if change := totalIn - feeUnits; change > 0 {
			emit(sender, change, false)
		}
		return nil
	}
}

// emitFees pays a transaction fee to the miner and developer addresses.
//
// Fees used to be subtracted from the sender and credited to nobody, so every
// transaction quietly destroyed value. Any remainder from the percentage split
// goes to the miner, so the units always balance exactly.
func (s *UTXOSet) emitFees(feeUnits int64, split feeSplit, emit func(string, int64, bool)) {
	if feeUnits <= 0 {
		return
	}

	devUnits := int64(float64(feeUnits) * split.DevPercent / 100.0)
	if devUnits < 0 {
		devUnits = 0
	}
	if devUnits > feeUnits {
		devUnits = feeUnits
	}
	minerUnits := feeUnits - devUnits

	if split.DevAddress != "" {
		emit(split.DevAddress, devUnits, false)
	} else {
		minerUnits = feeUnits
	}
	if split.MinerAddress != "" {
		emit(split.MinerAddress, minerUnits, false)
	}
}

// spendLocked consumes enough of an address's outputs to cover needUnits.
func (s *UTXOSet) spendLocked(address string, needUnits int64, staged *stagedChanges, undo *BlockUndo) ([]*UTXO, int64, error) {
	if address == "" {
		return nil, 0, errors.New("transaction has no sender")
	}
	if needUnits <= 0 {
		return nil, 0, nil
	}

	selected, total, err := staged.selectInputs(address, needUnits)
	if err != nil {
		return nil, total, err
	}

	for _, utxo := range selected {
		if !staged.remove(utxo.Outpoint) {
			// Selection came from the staged view, so this can only happen if the
			// same output were chosen twice -- a double spend inside one block.
			return nil, total, fmt.Errorf("%w: %s", ErrOutputAlreadySpent, utxo.Outpoint)
		}
		undo.Spent = append(undo.Spent, utxo)
	}

	return selected, total, nil
}

// RevertBlock undoes a block, restoring the set to its prior state.
func (s *UTXOSet) RevertBlock(blockHash string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	undo, ok := s.undo[blockHash]
	if !ok {
		return fmt.Errorf("no undo record for block %s", blockHash)
	}

	// Reverts must be in reverse application order, or an output created by a
	// later block could be resurrected by reverting an earlier one.
	if len(s.applied) == 0 || s.applied[len(s.applied)-1] != blockHash {
		return fmt.Errorf("block %s is not the most recently applied block", blockHash)
	}

	// Releasing allocated ids comes first because it is the only step that can
	// fail. Doing it after the outputs had already been rewritten would leave the
	// set half-reverted, describing a state no chain ever had -- the same failure
	// mode ApplyBlock stages its changes to avoid.
	//
	// Reverse order, and for a stricter reason than the others: ids come from a
	// counter, so they can only be released newest first.
	registry := s.anchors.Registry()
	for publisherID, units := range undo.Bonds {
		registry.restoreBond(publisherID, units)
	}
	for i := len(undo.Availability) - 1; i >= 0; i-- {
		s.availability.restore(undo.Availability[i])
	}
	for i := len(undo.Registrations) - 1; i >= 0; i-- {
		entry := undo.Registrations[i]
		var err error
		if entry.Kind == RegistrationKindPublisher {
			err = registry.unregisterPublisher(entry.PublisherID)
		} else {
			err = registry.unregisterGame(entry.PublisherID, entry.GameID)
		}
		if err != nil {
			return fmt.Errorf("reverting block %s: %w", blockHash, err)
		}
	}

	for _, outpoint := range undo.Created {
		s.removeLocked(outpoint)
	}
	for _, utxo := range undo.Spent {
		s.addLocked(utxo)
	}
	for address, prior := range undo.Nonces {
		if prior.Existed {
			s.nonces[address] = prior.Nonce
		} else {
			delete(s.nonces, address)
		}
	}
	// Reverse order: a block may carry several anchors for the same sidechain,
	// and only the earliest restore holds the state from before the block.
	for i := len(undo.Anchors) - 1; i >= 0; i-- {
		s.anchors.restore(undo.Anchors[i])
	}

	delete(s.undo, blockHash)
	s.applied = s.applied[:len(s.applied)-1]
	return nil
}

// LastNonce returns the highest confirmed nonce for an address, and whether the
// address has ever sent a transaction.
func (s *UTXOSet) LastNonce(address string) (uint64, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	nonce, ok := s.nonces[address]
	return nonce, ok
}

// NextNonce returns the lowest nonce an address may use next.
func (s *UTXOSet) NextNonce(address string) uint64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if nonce, ok := s.nonces[address]; ok {
		return nonce + 1
	}
	return 0
}

// Clone returns a deep copy, used to test a candidate branch without touching
// the live set.
func (s *UTXOSet) Clone() *UTXOSet {
	s.mu.RLock()
	defer s.mu.RUnlock()

	clone := NewUTXOSet()
	for outpoint, utxo := range s.outputs {
		copied := *utxo
		clone.outputs[outpoint] = &copied
		if clone.byAddress[utxo.Address] == nil {
			clone.byAddress[utxo.Address] = map[Outpoint]struct{}{}
		}
		clone.byAddress[utxo.Address][outpoint] = struct{}{}
	}
	for hash, undo := range s.undo {
		copied := *undo
		// The nonce map must be copied, not shared: a candidate branch tested
		// against a clone would otherwise write through into the live set.
		copied.Nonces = map[string]nonceRestore{}
		for address, prior := range undo.Nonces {
			copied.Nonces[address] = prior
		}
		clone.undo[hash] = &copied
	}
	for address, nonce := range s.nonces {
		clone.nonces[address] = nonce
	}
	clone.applied = append([]string{}, s.applied...)
	return clone
}

// stagedChanges buffers a block's additions and removals so the set is only
// mutated once the whole block has applied cleanly.
type stagedChanges struct {
	set     *UTXOSet
	added   map[Outpoint]*UTXO
	removed map[Outpoint]struct{}
}

func newStagedChanges(set *UTXOSet) *stagedChanges {
	return &stagedChanges{
		set:     set,
		added:   map[Outpoint]*UTXO{},
		removed: map[Outpoint]struct{}{},
	}
}

func (sc *stagedChanges) add(utxo *UTXO) { sc.added[utxo.Outpoint] = utxo }

// remove marks an output spent, reporting whether it was available.
func (sc *stagedChanges) remove(outpoint Outpoint) bool {
	if _, alreadyRemoved := sc.removed[outpoint]; alreadyRemoved {
		return false
	}
	if _, staged := sc.added[outpoint]; staged {
		delete(sc.added, outpoint)
		sc.removed[outpoint] = struct{}{}
		return true
	}
	if _, exists := sc.set.outputs[outpoint]; exists {
		sc.removed[outpoint] = struct{}{}
		return true
	}
	return false
}

// selectInputs chooses inputs from the staged view: committed outputs plus this
// block's own new outputs, minus what it has already spent.
//
// Including the block's own outputs is what lets a transaction spend one created
// earlier in the same block -- which the genesis block does, funding the miner
// from the coinbase alongside it.
func (sc *stagedChanges) selectInputs(address string, needUnits int64) ([]*UTXO, int64, error) {
	candidates := make([]*UTXO, 0, 8)

	for outpoint := range sc.set.byAddress[address] {
		if _, removed := sc.removed[outpoint]; removed {
			continue
		}
		if utxo, ok := sc.set.outputs[outpoint]; ok {
			candidates = append(candidates, utxo)
		}
	}
	for _, utxo := range sc.added {
		if utxo.Address == address {
			candidates = append(candidates, utxo)
		}
	}

	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].BlockIndex != candidates[j].BlockIndex {
			return candidates[i].BlockIndex < candidates[j].BlockIndex
		}
		if candidates[i].Outpoint.TxID != candidates[j].Outpoint.TxID {
			return candidates[i].Outpoint.TxID < candidates[j].Outpoint.TxID
		}
		return candidates[i].Outpoint.Index < candidates[j].Outpoint.Index
	})

	var (
		selected []*UTXO
		total    int64
	)
	for _, utxo := range candidates {
		selected = append(selected, utxo)
		total += utxo.Units

		if len(selected) > maxCoinSelectionInputs {
			return nil, total, fmt.Errorf("spending from %s would need more than %d inputs",
				address, maxCoinSelectionInputs)
		}
		if total >= needUnits {
			return selected, total, nil
		}
	}

	return nil, total, fmt.Errorf("%w: %s has %s, needs %s",
		ErrInsufficientFunds, address, formatUnits(total), formatUnits(needUnits))
}

// commit applies the staged changes to the set.
func (sc *stagedChanges) commit() {
	for outpoint := range sc.removed {
		sc.set.removeLocked(outpoint)
	}
	for _, utxo := range sc.added {
		sc.set.addLocked(utxo)
	}
}

// transactionParties returns a transaction's sender and recipient addresses.
func transactionParties(tx Transaction) (sender, recipient string) {
	if w := tx.GetSenderWallet(); w != nil {
		sender = w.GetAddress()
	}
	if w := tx.GetRecipientWallet(); w != nil {
		recipient = w.GetAddress()
	}
	return sender, recipient
}

// Anchors returns the main chain's sidechain anchor ledger.
//
// Publishers must be registered in its registry before their anchors will be
// accepted, which is what stops one publisher committing history under another's
// identity.
func (s *UTXOSet) Anchors() *AnchorLedger {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.anchors
}

// expireChallengesLocked closes every availability challenge whose deadline has
// passed, slashing the publisher's bond.
//
// This runs on every applied block rather than waiting for someone to submit a
// "you failed" transaction. Making a slash depend on a third party bothering to
// claim it would mean a publisher who withheld data escaped whenever nobody was
// watching, which is exactly the case the mechanism is for.
func (s *UTXOSet) expireChallengesLocked(blockIndex int64, staged *stagedChanges, undo *BlockUndo) error {
	if s.availability == nil {
		return nil
	}

	due := s.availability.dueChallenges(blockIndex)
	if len(due) == 0 {
		return nil
	}

	// Deterministic order: every node must produce identical outputs from the
	// same block, and a map's iteration order is not identical anywhere.
	sort.Slice(due, func(i, j int) bool {
		return due[i].ID() < due[j].ID()
	})

	registry := s.anchors.Registry()

	for _, challenge := range due {
		challengeID := challenge.ID()

		restore, expired, err := s.availability.expireForUndo(challengeID)
		if err != nil {
			return err
		}
		undo.Availability = append(undo.Availability, restore)

		// Synthetic outpoints: this movement belongs to the block rather than to
		// any transaction in it, but it still has to be addressable so a revert
		// can find it.
		txID := "expire:" + challengeID
		var nextIndex uint32
		emit := func(address string, units int64) {
			if units <= 0 || address == "" {
				return
			}
			utxo := &UTXO{
				Outpoint:   Outpoint{TxID: txID, Index: nextIndex},
				Address:    address,
				Units:      units,
				BlockIndex: blockIndex,
			}
			nextIndex++
			staged.add(utxo)
			undo.Created = append(undo.Created, utxo.Outpoint)
		}

		// The stake goes back to the challenger: they asked a fair question and
		// were not answered.
		escrow := DeriveChallengeEscrowAddress(challengeID)
		if _, _, err := s.spendLocked(escrow, AvailabilityChallengeFeeUnits, staged, undo); err != nil {
			return fmt.Errorf("expiring challenge %s: %w", challengeID, err)
		}
		emit(expired.Challenger, AvailabilityChallengeFeeUnits)

		// And the bond pays for the failure. The registry's figure is reduced
		// first so it cannot be slashed twice for coins that are no longer there,
		// and the coins move to match.
		taken, err := registry.slashBond(expired.PublisherID, AvailabilitySlashUnits)
		if err != nil {
			return fmt.Errorf("expiring challenge %s: %w", challengeID, err)
		}
		if taken <= 0 {
			continue
		}
		undo.Bonds[expired.PublisherID] += taken

		bondAddress := DerivePublisherBondAddress(expired.PublisherID)
		_, totalIn, err := s.spendLocked(bondAddress, taken, staged, undo)
		if err != nil {
			return fmt.Errorf("expiring challenge %s: %w", challengeID, err)
		}

		emit(expired.Challenger, taken)
		if change := totalIn - taken; change > 0 {
			emit(bondAddress, change)
		}
	}

	return nil
}

// Availability returns the main chain's availability challenge ledger.
func (s *UTXOSet) Availability() *AvailabilityLedger {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.availability
}
