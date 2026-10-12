// Package sdk is a software development kit for building blockchain applications.
// File sdk/sidechain_filestore.go - Durable storage for sidechains an application writes.
package sdk

import (
	"bufio"
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// WHY A STORE AT ALL
//
// A Sidechain lives in memory, and the publisher's node is the only party that
// holds its blocks. Lose them and the anchors on the main chain still prove that
// a history existed -- but nobody, the publisher included, can say what it was,
// and every availability challenge against it will expire and slash the bond.
// Persisting the blocks is therefore not an optimisation; it is part of what the
// publisher signed up for when they registered.
//
// Applications with a database should keep blocks there and use
// RestoreSidechain on start-up. SidechainStore is for everyone else, and the
// file implementation is deliberately the simplest thing that is durable.

// ErrSidechainStoreCorrupt is returned when a store's contents cannot be read
// back as a valid chain.
var ErrSidechainStoreCorrupt = errors.New("sidechain store is corrupt")

// SidechainStore persists sidechain blocks and the anchored watermark.
//
// Implementations must refuse a block that does not extend what they already
// hold. A store that accepted anything would let the persisted history drift
// from the in-memory one, and the drift would surface only at the next restart
// -- as a chain that fails RestoreSidechain, long after the bad write.
type SidechainStore interface {
	// AppendBlock durably records a block. It must follow the stored tip.
	AppendBlock(block *SidechainBlock) error
	// MarkAnchored durably records that a sidechain is anchored through height.
	MarkAnchored(id SidechainID, height uint64) error
	// Load rebuilds a sidechain from storage, re-validating every block. A
	// sidechain with nothing stored loads as an empty chain.
	Load(id SidechainID) (*Sidechain, error)
	// IDs lists the sidechains the store holds.
	IDs() ([]SidechainID, error)
}

// sidechainFileExtension names a sidechain's log file.
const sidechainFileExtension = ".jsonl"

// sidechainLogRecord is one line of a sidechain's log.
//
// Exactly one field is set. A block line appends a block; an anchored line moves
// the watermark. Keeping both in one append-only file means a single fsync
// orders them, so a restart can never see a watermark above blocks it has not
// got.
type sidechainLogRecord struct {
	Block           *SidechainBlock `json:"block,omitempty"`
	AnchoredThrough *uint64         `json:"anchored_through,omitempty"`
}

// fileChainState is the store's cached view of one sidechain's file.
type fileChainState struct {
	hasTip   bool
	height   uint64
	hash     []byte
	anchored *uint64
}

// FileSidechainStore keeps each sidechain in its own append-only JSON Lines file
// under one directory, named by the sidechain's canonical hex id.
//
// Every write is followed by an fsync before it returns, so a block the caller
// was told is stored survives a power cut. Writes are append-only, so the worst
// a crash can leave behind is one torn final line; it is detected and cut off
// when the file is next read, and everything before it is intact. A malformed
// line anywhere else is not a crash artefact, and is refused as corruption
// rather than silently skipped.
//
// It is safe for concurrent use within one process. Two processes writing the
// same directory are not supported -- a sidechain has one writer by design, and
// that writer is one node.
type FileSidechainStore struct {
	mu     sync.Mutex
	dir    string
	states map[string]*fileChainState
}

// OpenFileSidechainStore opens (creating if needed) a store rooted at dir.
func OpenFileSidechainStore(dir string) (*FileSidechainStore, error) {
	if strings.TrimSpace(dir) == "" {
		return nil, errors.New("sidechain store directory is empty")
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("create sidechain store directory: %w", err)
	}
	return &FileSidechainStore{dir: dir, states: map[string]*fileChainState{}}, nil
}

// path returns the file holding a sidechain.
//
// The name is the id's fixed-width hex encoding, so it can contain nothing but
// hex digits -- there is no input from which a path separator or ".." could
// reach the filesystem.
func (s *FileSidechainStore) path(id SidechainID) string {
	return filepath.Join(s.dir, id.Hex()+sidechainFileExtension)
}

// AppendBlock validates a block, checks it follows the stored tip, and appends
// it durably.
func (s *FileSidechainStore) AppendBlock(block *SidechainBlock) error {
	if block == nil {
		return fmt.Errorf("%w: block is nil", ErrInvalidSidechainBlock)
	}
	if err := block.Validate(); err != nil {
		return err
	}

	id := block.SidechainID()

	s.mu.Lock()
	defer s.mu.Unlock()

	state, err := s.stateLocked(id)
	if err != nil {
		return err
	}

	switch {
	case !state.hasTip:
		if block.Header.Height != 0 {
			return fmt.Errorf("%w: the store holds nothing for %s, so the first block "+
				"must be height 0, not %d", ErrSidechainNotContiguous, id, block.Header.Height)
		}
	case block.Header.Height != state.height+1:
		return fmt.Errorf("%w: height %d does not follow the stored tip %d",
			ErrSidechainNotContiguous, block.Header.Height, state.height)
	case !bytes.Equal(block.Header.PreviousHash, state.hash):
		return fmt.Errorf("%w: previous hash does not match the stored tip",
			ErrSidechainNotContiguous)
	}

	if err := s.appendLocked(id, sidechainLogRecord{Block: block}); err != nil {
		return err
	}

	state.hasTip = true
	state.height = block.Header.Height
	state.hash = append([]byte{}, block.Hash...)
	return nil
}

// MarkAnchored durably moves a sidechain's anchored watermark forward.
//
// The same rules as Sidechain.MarkAnchored apply: forward only, never past the
// stored tip, and repeating the current height is a no-op that writes nothing.
func (s *FileSidechainStore) MarkAnchored(id SidechainID, height uint64) error {
	if err := id.Validate(); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	state, err := s.stateLocked(id)
	if err != nil {
		return err
	}

	if !state.hasTip || height > state.height {
		return fmt.Errorf("%w: cannot mark height %d anchored; the store does not reach it",
			ErrAnchorNotContiguous, height)
	}
	if state.anchored != nil {
		if height < *state.anchored {
			return fmt.Errorf("%w: height %d is below the anchored height %d",
				ErrAnchorNotContiguous, height, *state.anchored)
		}
		if height == *state.anchored {
			return nil
		}
	}

	h := height
	if err := s.appendLocked(id, sidechainLogRecord{AnchoredThrough: &h}); err != nil {
		return err
	}
	state.anchored = &h
	return nil
}

// Load reads a sidechain back and rebuilds it through RestoreSidechain, so every
// block is re-validated exactly as if it had come from anywhere else.
func (s *FileSidechainStore) Load(id SidechainID) (*Sidechain, error) {
	if err := id.Validate(); err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	blocks, anchored, err := s.readLocked(id)
	if err != nil {
		return nil, err
	}

	chain, err := RestoreSidechain(id, blocks, anchored)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrSidechainStoreCorrupt, err)
	}

	s.states[id.String()] = stateFromBlocks(blocks, anchored)
	return chain, nil
}

// IDs lists the sidechains with a file in the store, in ascending order.
func (s *FileSidechainStore) IDs() ([]SidechainID, error) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, fmt.Errorf("list sidechain store: %w", err)
	}

	var ids []SidechainID
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, sidechainFileExtension) {
			continue
		}
		raw, err := hex.DecodeString(strings.TrimSuffix(name, sidechainFileExtension))
		if err != nil {
			continue // not a file this store wrote
		}
		id, err := SidechainIDFromBytes(raw)
		if err != nil {
			continue
		}
		ids = append(ids, id)
	}

	sort.Slice(ids, func(i, j int) bool {
		if ids[i].PublisherID != ids[j].PublisherID {
			return ids[i].PublisherID < ids[j].PublisherID
		}
		return ids[i].GameID < ids[j].GameID
	})
	return ids, nil
}

// stateLocked returns the cached view of a sidechain's file, reading and fully
// validating the file the first time it is needed.
//
// Validating on first touch rather than trusting the last line is what makes
// "follows the stored tip" mean something: the tip it compares against is the
// tip of a chain that has been checked end to end.
func (s *FileSidechainStore) stateLocked(id SidechainID) (*fileChainState, error) {
	if state, ok := s.states[id.String()]; ok {
		return state, nil
	}

	blocks, anchored, err := s.readLocked(id)
	if err != nil {
		return nil, err
	}
	if _, err := RestoreSidechain(id, blocks, anchored); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrSidechainStoreCorrupt, err)
	}

	state := stateFromBlocks(blocks, anchored)
	s.states[id.String()] = state
	return state, nil
}

// stateFromBlocks summarises a validated chain for the cache.
func stateFromBlocks(blocks []*SidechainBlock, anchored *uint64) *fileChainState {
	state := &fileChainState{anchored: anchored}
	if len(blocks) > 0 {
		tip := blocks[len(blocks)-1]
		state.hasTip = true
		state.height = tip.Header.Height
		state.hash = append([]byte{}, tip.Hash...)
	}
	return state
}

// readLocked parses a sidechain's file, cutting off a torn final line.
//
// A line without its terminating newline can only be a write that a crash
// interrupted -- every complete write ends in one -- so it is the record the
// caller was never told succeeded, and it is truncated away. A complete line
// that does not parse cannot be explained that way and is reported as
// corruption.
func (s *FileSidechainStore) readLocked(id SidechainID) ([]*SidechainBlock, *uint64, error) {
	path := s.path(id)

	file, err := os.Open(path) //nolint:gosec // path is built from a hex id, see path()
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, fmt.Errorf("open sidechain store file: %w", err)
	}
	defer func() { _ = file.Close() }()

	var (
		blocks   []*SidechainBlock
		anchored *uint64
		offset   int64
		lineNo   int
	)

	reader := bufio.NewReader(file)
	for {
		line, readErr := reader.ReadBytes('\n')
		if readErr != nil && !errors.Is(readErr, io.EOF) {
			return nil, nil, fmt.Errorf("read sidechain store file: %w", readErr)
		}

		if len(line) > 0 && line[len(line)-1] != '\n' {
			// A torn tail: drop it so the next append starts on a clean line.
			if err := truncateAndSync(path, offset); err != nil {
				return nil, nil, err
			}
			break
		}

		if len(line) > 0 {
			lineNo++
			var record sidechainLogRecord
			if err := json.Unmarshal(line, &record); err != nil {
				return nil, nil, fmt.Errorf("%w: %s line %d: %w",
					ErrSidechainStoreCorrupt, filepath.Base(path), lineNo, err)
			}
			switch {
			case record.Block != nil && record.AnchoredThrough == nil:
				blocks = append(blocks, record.Block)
			case record.AnchoredThrough != nil && record.Block == nil:
				anchored = record.AnchoredThrough
			default:
				return nil, nil, fmt.Errorf("%w: %s line %d is neither a block nor a watermark",
					ErrSidechainStoreCorrupt, filepath.Base(path), lineNo)
			}
			offset += int64(len(line))
		}

		if errors.Is(readErr, io.EOF) {
			break
		}
	}

	return blocks, anchored, nil
}

// appendLocked writes one record as a line and syncs it to disk.
//
// If anything goes wrong mid-write the cached state is dropped, so the next
// operation re-reads the file -- and cuts off whatever partial line the failed
// write left -- instead of trusting a tip that may not be on disk.
func (s *FileSidechainStore) appendLocked(id SidechainID, record sidechainLogRecord) (err error) {
	line, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("encode sidechain store record: %w", err)
	}
	line = append(line, '\n')

	path := s.path(id)
	_, statErr := os.Stat(path)
	created := errors.Is(statErr, os.ErrNotExist)

	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600) //nolint:gosec // path is built from a hex id
	if err != nil {
		return fmt.Errorf("open sidechain store file: %w", err)
	}
	defer func() {
		if closeErr := file.Close(); err == nil && closeErr != nil {
			err = fmt.Errorf("close sidechain store file: %w", closeErr)
		}
		if err != nil {
			delete(s.states, id.String())
		}
	}()

	if _, err = file.Write(line); err != nil {
		return fmt.Errorf("write sidechain store file: %w", err)
	}
	if err = file.Sync(); err != nil {
		return fmt.Errorf("sync sidechain store file: %w", err)
	}

	// A new file's directory entry must reach the disk too, or a crash can
	// leave a synced file that no directory points to.
	if created {
		syncDir(s.dir)
	}
	return nil
}

// truncateAndSync cuts a file to size and makes the cut durable.
func truncateAndSync(path string, size int64) error {
	file, err := os.OpenFile(path, os.O_WRONLY, 0o600) //nolint:gosec // path is built from a hex id
	if err != nil {
		return fmt.Errorf("open sidechain store file for repair: %w", err)
	}
	defer func() { _ = file.Close() }()

	if err := file.Truncate(size); err != nil {
		return fmt.Errorf("truncate torn sidechain store record: %w", err)
	}
	if err := file.Sync(); err != nil {
		return fmt.Errorf("sync repaired sidechain store file: %w", err)
	}
	return nil
}

// syncDir fsyncs a directory so a newly created entry in it is durable.
//
// Some platforms (Windows among them) cannot open a directory for syncing; on
// those the call is a best effort, since there is nothing further to be done.
func syncDir(dir string) {
	handle, err := os.Open(dir) //nolint:gosec // the store's own directory
	if err != nil {
		return
	}
	defer func() { _ = handle.Close() }()
	_ = handle.Sync()
}
