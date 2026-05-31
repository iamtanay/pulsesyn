package store

import (
	"errors"
	"fmt"

	badger "github.com/dgraph-io/badger/v4"
	"github.com/iamtanay/pulsesyn/core/reputation"
)

// WriteValidatorRecord persists a ValidatorRecord to the store.
// Upsert semantics: if a record already exists for r.ValidatorID, it is
// replaced with the new record. Reputation evolves after every validation
// session — the store always reflects the latest state.
// See PulseSyn Protocol Specification v0.1, Section 2.2.
func (s *Store) WriteValidatorRecord(r *reputation.ValidatorRecord) error {
	if r == nil {
		return fmt.Errorf("WriteValidatorRecord: record is nil")
	}

	val, err := marshal(r)
	if err != nil {
		return fmt.Errorf("WriteValidatorRecord: %w", err)
	}

	return s.db.Update(func(txn *badger.Txn) error {
		if err := txn.Set(validatorKey(r.ValidatorID), val); err != nil {
			return fmt.Errorf("WriteValidatorRecord: %w", err)
		}
		return nil
	})
}

// ReadValidatorRecord returns the ValidatorRecord for the given validatorID.
// Returns ErrValidatorNotFound if no record exists.
func (s *Store) ReadValidatorRecord(validatorID string) (*reputation.ValidatorRecord, error) {
	var r reputation.ValidatorRecord
	err := s.db.View(func(txn *badger.Txn) error {
		val, err := readValue(txn, validatorKey(validatorID), ErrValidatorNotFound)
		if err != nil {
			return err
		}
		return unmarshal(val, &r)
	})
	if err != nil {
		return nil, fmt.Errorf("ReadValidatorRecord (%s): %w", validatorID, err)
	}
	return &r, nil
}

// DeleteValidatorRecord removes the ValidatorRecord with the given validatorID.
// Returns ErrValidatorNotFound if no such record exists.
// Note: This is an administrative operation. In a live network, validators
// are deactivated (status → RETIRED) rather than deleted from the store.
func (s *Store) DeleteValidatorRecord(validatorID string) error {
	return s.db.Update(func(txn *badger.Txn) error {
		pk := validatorKey(validatorID)
		if _, err := txn.Get(pk); errors.Is(err, badger.ErrKeyNotFound) {
			return fmt.Errorf("DeleteValidatorRecord (%s): %w", validatorID, ErrValidatorNotFound)
		} else if err != nil {
			return fmt.Errorf("DeleteValidatorRecord: %w", err)
		}
		return txn.Delete(pk)
	})
}
