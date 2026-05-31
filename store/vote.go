package store

import (
	"errors"
	"fmt"
	"strings"
	"time"

	badger "github.com/dgraph-io/badger/v4"
)

// VoteRecord is the persistent form of a single validator's vote in a
// validation session. It is immutable once written.
// See PulseSyn Protocol Specification v0.1, Section 2.3.
type VoteRecord struct {
	// ClaimID is the claim this vote was cast for.
	ClaimID string `json:"claim_id"`

	// ValidatorID is the public key hash of the voting validator.
	ValidatorID string `json:"validator_id"`

	// Verdict is the validator's verdict choice. One of: SUPPORTED,
	// UNSUPPORTED, MISLEADING, INDETERMINATE.
	Verdict string `json:"verdict"`

	// Confidence is the validator's stated certainty in [0.0, 1.0].
	Confidence float64 `json:"confidence"`

	// DomainReputation is the validator's domain reputation at selection time.
	DomainReputation float64 `json:"domain_reputation"`

	// BiasCoefficient is the validator's bias coefficient at selection time.
	BiasCoefficient float64 `json:"bias_coefficient"`

	// ValidatorSetSize is the total number of validators selected for this claim.
	ValidatorSetSize int `json:"validator_set_size"`

	// Epoch is the block number at which this vote was submitted.
	Epoch uint64 `json:"epoch"`

	// RecordedAt is the wall-clock time this vote was recorded.
	RecordedAt time.Time `json:"recorded_at"`
}

// validateVoteRecord checks that all required VoteRecord fields are present
// and within their defined protocol ranges.
func validateVoteRecord(v VoteRecord) error {
	if strings.TrimSpace(v.ClaimID) == "" {
		return fmt.Errorf("%w: claim_id is empty", ErrInvalidRecord)
	}
	if strings.TrimSpace(v.ValidatorID) == "" {
		return fmt.Errorf("%w: validator_id is empty", ErrInvalidRecord)
	}
	if !validVerdicts[v.Verdict] {
		return fmt.Errorf("%w: verdict %q is not one of SUPPORTED, UNSUPPORTED, MISLEADING, INDETERMINATE", ErrInvalidRecord, v.Verdict)
	}
	if v.Confidence < 0.0 || v.Confidence > 1.0 {
		return fmt.Errorf("%w: confidence %f out of range [0.0, 1.0]", ErrInvalidRecord, v.Confidence)
	}
	if v.DomainReputation < 0.0 || v.DomainReputation > 1.0 {
		return fmt.Errorf("%w: domain_reputation %f out of range [0.0, 1.0]", ErrInvalidRecord, v.DomainReputation)
	}
	if v.BiasCoefficient < 0.0 || v.BiasCoefficient > 1.0 {
		return fmt.Errorf("%w: bias_coefficient %f out of range [0.0, 1.0]", ErrInvalidRecord, v.BiasCoefficient)
	}
	if v.ValidatorSetSize <= 0 {
		return fmt.Errorf("%w: validator_set_size must be > 0", ErrInvalidRecord)
	}
	return nil
}

// WriteVote persists a VoteRecord to the store.
// Write-once per (ClaimID, ValidatorID) pair: returns ErrAlreadyExists if a
// vote from this validator for this claim already exists.
// Also writes the vote-by-validator secondary index.
func (s *Store) WriteVote(v VoteRecord) error {
	if err := validateVoteRecord(v); err != nil {
		return fmt.Errorf("WriteVote: %w", err)
	}

	val, err := marshal(v)
	if err != nil {
		return fmt.Errorf("WriteVote: %w", err)
	}

	return s.db.Update(func(txn *badger.Txn) error {
		pk := voteKey(v.ClaimID, v.ValidatorID)
		if _, err := txn.Get(pk); err == nil {
			return fmt.Errorf("WriteVote (%s/%s): %w", v.ClaimID, v.ValidatorID, ErrAlreadyExists)
		} else if !errors.Is(err, badger.ErrKeyNotFound) {
			return fmt.Errorf("WriteVote: %w", err)
		}

		if err := txn.Set(pk, val); err != nil {
			return fmt.Errorf("WriteVote: %w", err)
		}
		// Vote-by-validator index
		if err := txn.Set(idxVoteByValidatorKey(v.ValidatorID, v.ClaimID), []byte{}); err != nil {
			return fmt.Errorf("WriteVote (validator index): %w", err)
		}
		return nil
	})
}

// ReadVote returns the VoteRecord for the given (claimID, validatorID) pair.
// Returns ErrVoteNotFound if no such vote exists.
func (s *Store) ReadVote(claimID, validatorID string) (*VoteRecord, error) {
	var v VoteRecord
	err := s.db.View(func(txn *badger.Txn) error {
		val, err := readValue(txn, voteKey(claimID, validatorID), ErrVoteNotFound)
		if err != nil {
			return err
		}
		return unmarshal(val, &v)
	})
	if err != nil {
		return nil, fmt.Errorf("ReadVote (%s/%s): %w", claimID, validatorID, err)
	}
	return &v, nil
}

// VotesForClaim returns all VoteRecords for the given claimID. The primary
// key prefix `vote/{claim_id}/` is scanned directly — no secondary index needed.
// Returns an empty slice (not an error) if no votes exist.
func (s *Store) VotesForClaim(claimID string) ([]*VoteRecord, error) {
	prefix := []byte(prefixVote + claimID + "/")

	var votes []*VoteRecord
	err := s.db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()

		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			var v VoteRecord
			if err := it.Item().Value(func(val []byte) error {
				return unmarshal(val, &v)
			}); err != nil {
				return fmt.Errorf("VotesForClaim: %w", err)
			}
			votes = append(votes, &v)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("VotesForClaim (%s): %w", claimID, err)
	}
	return votes, nil
}

// VotesByValidator returns all VoteRecords submitted by the given validatorID
// across all claims. Uses the vote-by-validator secondary index.
// Returns an empty slice (not an error) if no votes exist.
func (s *Store) VotesByValidator(validatorID string) ([]*VoteRecord, error) {
	prefix := []byte(prefixIdxVoteByValidator + validatorID + "/")

	claimIDs, err := prefixScanKeys(s.db, prefix, func(key []byte) string {
		return claimIDFromVoteByValidatorIndex(key, validatorID)
	})
	if err != nil {
		return nil, fmt.Errorf("VotesByValidator: %w", err)
	}

	votes := make([]*VoteRecord, 0, len(claimIDs))
	err = s.db.View(func(txn *badger.Txn) error {
		for _, claimID := range claimIDs {
			val, err := readValue(txn, voteKey(claimID, validatorID), ErrVoteNotFound)
			if err != nil {
				return err
			}
			var v VoteRecord
			if err := unmarshal(val, &v); err != nil {
				return err
			}
			votes = append(votes, &v)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("VotesByValidator (%s): %w", validatorID, err)
	}
	return votes, nil
}
