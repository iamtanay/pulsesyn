package store

import (
	"encoding/json"
	"errors"
	"fmt"

	badger "github.com/dgraph-io/badger/v4"
)

// Sentinel errors returned by store operations.
var (
	ErrClaimNotFound      = errors.New("pulsesyn/store: claim not found")
	ErrValidatorNotFound  = errors.New("pulsesyn/store: validator record not found")
	ErrValidationNotFound = errors.New("pulsesyn/store: validation record not found")
	ErrVoteNotFound       = errors.New("pulsesyn/store: vote not found")
	ErrAlreadyExists      = errors.New("pulsesyn/store: record already exists")
	ErrInvalidRecord      = errors.New("pulsesyn/store: record is invalid")
)

// Store is the PulseSyn local node state store, backed by BadgerDB v4.
// All write transactions are serialized by BadgerDB internally. Reads
// are concurrent. Open and Close must be called from the same goroutine.
// See PulseSyn Protocol Specification v0.1, Phase 2. See DECISIONS.md: 2026-05-31.
type Store struct {
	db *badger.DB
}

// Open opens or creates the Store at the given directory path.
// The directory is created if it does not exist. The caller must call Close
// exactly once when finished.
func Open(dir string) (*Store, error) {
	opts := badger.DefaultOptions(dir)
	opts.Logger = nil
	db, err := badger.Open(opts)
	if err != nil {
		return nil, fmt.Errorf("store.Open: %w", err)
	}
	return &Store{db: db}, nil
}

// Close flushes and closes the BadgerDB instance. Must be called exactly once
// on node shutdown. Calling Close more than once returns an error.
func (s *Store) Close() error {
	if err := s.db.Close(); err != nil {
		return fmt.Errorf("store.Close: %w", err)
	}
	return nil
}

// RunGC triggers BadgerDB's value log garbage collection. Long-running nodes
// must call this periodically (e.g. every hour) to prevent unbounded growth
// of the BadgerDB value log. See DECISIONS.md: 2026-05-31.
func (s *Store) RunGC() error {
	err := s.db.RunValueLogGC(0.5)
	if err != nil && !errors.Is(err, badger.ErrNoRewrite) {
		return fmt.Errorf("store.RunGC: %w", err)
	}
	return nil
}

// marshal serializes v to JSON for storage in BadgerDB.
func marshal(v any) ([]byte, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("store.marshal: %w", err)
	}
	return b, nil
}

// unmarshal deserializes JSON bytes from BadgerDB into v.
func unmarshal(b []byte, v any) error {
	if err := json.Unmarshal(b, v); err != nil {
		return fmt.Errorf("store.unmarshal: %w", err)
	}
	return nil
}

// readValue fetches the raw value for key from a read-only transaction and
// passes it to fn. Returns ErrNotFound (caller's sentinel) if the key is absent.
func readValue(txn *badger.Txn, key []byte, notFound error) ([]byte, error) {
	item, err := txn.Get(key)
	if errors.Is(err, badger.ErrKeyNotFound) {
		return nil, notFound
	}
	if err != nil {
		return nil, fmt.Errorf("store.readValue: %w", err)
	}
	var val []byte
	if err := item.Value(func(v []byte) error {
		val = make([]byte, len(v))
		copy(val, v)
		return nil
	}); err != nil {
		return nil, fmt.Errorf("store.readValue: %w", err)
	}
	return val, nil
}
