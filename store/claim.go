package store

import (
	"errors"
	"fmt"

	badger "github.com/dgraph-io/badger/v4"
	"github.com/iamtanay/pulsesyn/core/claim"
)

// WriteClaim persists a Claim to the store.
// Write-once: returns ErrAlreadyExists if a record for c.ClaimID already exists.
// See PulseSyn Protocol Specification v0.1, Section 2.1.
func (s *Store) WriteClaim(c *claim.Claim) error {
	if c == nil {
		return fmt.Errorf("WriteClaim: claim is nil")
	}

	val, err := marshal(c)
	if err != nil {
		return fmt.Errorf("WriteClaim: %w", err)
	}

	return s.db.Update(func(txn *badger.Txn) error {
		pk := claimKey(c.ClaimID)
		if _, err := txn.Get(pk); err == nil {
			return fmt.Errorf("WriteClaim (%s): %w", c.ClaimID, ErrAlreadyExists)
		} else if !errors.Is(err, badger.ErrKeyNotFound) {
			return fmt.Errorf("WriteClaim: %w", err)
		}
		return txn.Set(pk, val)
	})
}

// ReadClaim returns the Claim for the given claimID.
// Returns ErrClaimNotFound if no claim exists with that ID.
func (s *Store) ReadClaim(claimID string) (*claim.Claim, error) {
	var c claim.Claim
	err := s.db.View(func(txn *badger.Txn) error {
		val, err := readValue(txn, claimKey(claimID), ErrClaimNotFound)
		if err != nil {
			return err
		}
		return unmarshal(val, &c)
	})
	if err != nil {
		return nil, fmt.Errorf("ReadClaim (%s): %w", claimID, err)
	}
	return &c, nil
}

// DeleteClaim removes the Claim with the given claimID from the store.
// Returns ErrClaimNotFound if no such claim exists.
// Note: Claim deletion is an administrative operation only — the on-chain
// record is permanent regardless.
func (s *Store) DeleteClaim(claimID string) error {
	return s.db.Update(func(txn *badger.Txn) error {
		pk := claimKey(claimID)
		if _, err := txn.Get(pk); errors.Is(err, badger.ErrKeyNotFound) {
			return fmt.Errorf("DeleteClaim (%s): %w", claimID, ErrClaimNotFound)
		} else if err != nil {
			return fmt.Errorf("DeleteClaim: %w", err)
		}
		return txn.Delete(pk)
	})
}
