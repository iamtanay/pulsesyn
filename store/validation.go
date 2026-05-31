package store

import (
	"errors"
	"fmt"
	"strings"
	"time"

	badger "github.com/dgraph-io/badger/v4"
)

// ValidationRecord is a permanent record of a finalized validation session.
// It is immutable once written — the on-chain ledger treats it the same way.
// See PulseSyn Protocol Specification v0.1, Section 4.5.
type ValidationRecord struct {
	// ClaimID is the claim this session evaluated.
	ClaimID string `json:"claim_id"`

	// Domain is the primary domain of the claim.
	Domain string `json:"domain"`

	// Verdict is the consensus verdict. One of: SUPPORTED, UNSUPPORTED,
	// MISLEADING, INDETERMINATE.
	Verdict string `json:"verdict"`

	// ConfidenceScore is the aggregate confidence score in [0.0, 1.0].
	ConfidenceScore float64 `json:"confidence_score"`

	// ParticipationRate is the fraction of selected validators that voted.
	ParticipationRate float64 `json:"participation_rate"`

	// ValidatorCount is the number of validators who submitted votes.
	ValidatorCount int `json:"validator_count"`

	// Epoch is the block number at which the validation was finalized.
	Epoch uint64 `json:"epoch"`

	// MerkleRoot is the hex-encoded root of the vote Merkle tree. Empty in
	// Phase 2 until the session layer wires it up in Phase 3.
	MerkleRoot string `json:"merkle_root,omitempty"`

	// FinalizedAt is the wall-clock time the validation was finalized.
	FinalizedAt time.Time `json:"finalized_at"`
}

// validVerdicts is the set of acceptable verdict strings for storage.
var validVerdicts = map[string]bool{
	"SUPPORTED":     true,
	"UNSUPPORTED":   true,
	"MISLEADING":    true,
	"INDETERMINATE": true,
}

// validateValidationRecord checks that all required fields are present and valid.
func validateValidationRecord(r ValidationRecord) error {
	if strings.TrimSpace(r.ClaimID) == "" {
		return fmt.Errorf("%w: claim_id is empty", ErrInvalidRecord)
	}
	if strings.TrimSpace(r.Domain) == "" {
		return fmt.Errorf("%w: domain is empty", ErrInvalidRecord)
	}
	if !validVerdicts[r.Verdict] {
		return fmt.Errorf("%w: verdict %q is not one of SUPPORTED, UNSUPPORTED, MISLEADING, INDETERMINATE", ErrInvalidRecord, r.Verdict)
	}
	if r.ConfidenceScore < 0.0 || r.ConfidenceScore > 1.0 {
		return fmt.Errorf("%w: confidence_score %f out of range [0.0, 1.0]", ErrInvalidRecord, r.ConfidenceScore)
	}
	if r.ParticipationRate < 0.0 || r.ParticipationRate > 1.0 {
		return fmt.Errorf("%w: participation_rate %f out of range [0.0, 1.0]", ErrInvalidRecord, r.ParticipationRate)
	}
	if r.FinalizedAt.IsZero() {
		return fmt.Errorf("%w: finalized_at is zero", ErrInvalidRecord)
	}
	return nil
}

// WriteValidationRecord persists a finalized validation to the store.
// Write-once: returns ErrAlreadyExists if a record for r.ClaimID already exists.
// Also writes secondary indexes for domain and epoch lookups.
func (s *Store) WriteValidationRecord(r ValidationRecord) error {
	if err := validateValidationRecord(r); err != nil {
		return fmt.Errorf("WriteValidationRecord: %w", err)
	}

	domain := strings.ToLower(strings.TrimSpace(r.Domain))
	r.Domain = domain

	val, err := marshal(r)
	if err != nil {
		return fmt.Errorf("WriteValidationRecord: %w", err)
	}

	return s.db.Update(func(txn *badger.Txn) error {
		pk := validationKey(r.ClaimID)
		if _, err := txn.Get(pk); err == nil {
			return fmt.Errorf("WriteValidationRecord (%s): %w", r.ClaimID, ErrAlreadyExists)
		} else if !errors.Is(err, badger.ErrKeyNotFound) {
			return fmt.Errorf("WriteValidationRecord: %w", err)
		}

		if err := txn.Set(pk, val); err != nil {
			return fmt.Errorf("WriteValidationRecord: %w", err)
		}
		// Domain index
		if err := txn.Set(idxDomainKey(domain, r.ClaimID), []byte{}); err != nil {
			return fmt.Errorf("WriteValidationRecord (domain index): %w", err)
		}
		// Epoch index
		if err := txn.Set(idxEpochKey(r.Epoch, r.ClaimID), []byte{}); err != nil {
			return fmt.Errorf("WriteValidationRecord (epoch index): %w", err)
		}
		return nil
	})
}

// ReadValidationRecord returns the ValidationRecord for the given claimID.
// Returns ErrValidationNotFound if no record exists.
func (s *Store) ReadValidationRecord(claimID string) (*ValidationRecord, error) {
	var r ValidationRecord
	err := s.db.View(func(txn *badger.Txn) error {
		val, err := readValue(txn, validationKey(claimID), ErrValidationNotFound)
		if err != nil {
			return err
		}
		return unmarshal(val, &r)
	})
	if err != nil {
		return nil, fmt.Errorf("ReadValidationRecord (%s): %w", claimID, err)
	}
	return &r, nil
}

// ValidationsByDomain returns all ValidationRecords whose primary domain
// matches domain. Returns an empty slice (not an error) if none exist.
func (s *Store) ValidationsByDomain(domain string) ([]*ValidationRecord, error) {
	domain = strings.ToLower(strings.TrimSpace(domain))
	prefix := []byte(prefixIdxDomain + domain + "/")

	claimIDs, err := prefixScanKeys(s.db, prefix, func(key []byte) string {
		return claimIDFromDomainIndex(key, domain)
	})
	if err != nil {
		return nil, fmt.Errorf("ValidationsByDomain: %w", err)
	}

	return s.fetchValidationRecords(claimIDs)
}

// ValidationsByEpoch returns all ValidationRecords finalized at the given epoch.
// Returns an empty slice (not an error) if none exist.
func (s *Store) ValidationsByEpoch(epoch uint64) ([]*ValidationRecord, error) {
	prefix := []byte(fmt.Sprintf("%s%016x/", prefixIdxEpoch, epoch))

	claimIDs, err := prefixScanKeys(s.db, prefix, func(key []byte) string {
		return claimIDFromEpochIndex(key, epoch)
	})
	if err != nil {
		return nil, fmt.Errorf("ValidationsByEpoch: %w", err)
	}

	return s.fetchValidationRecords(claimIDs)
}

// fetchValidationRecords reads ValidationRecords for the given claimIDs in a
// single read-only transaction.
func (s *Store) fetchValidationRecords(claimIDs []string) ([]*ValidationRecord, error) {
	results := make([]*ValidationRecord, 0, len(claimIDs))
	err := s.db.View(func(txn *badger.Txn) error {
		for _, id := range claimIDs {
			val, err := readValue(txn, validationKey(id), ErrValidationNotFound)
			if err != nil {
				return err
			}
			var r ValidationRecord
			if err := unmarshal(val, &r); err != nil {
				return err
			}
			results = append(results, &r)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return results, nil
}

// prefixScanKeys performs a prefix scan on the BadgerDB instance and returns
// the extracted string identifiers via the extractID function.
func prefixScanKeys(db *badger.DB, prefix []byte, extractID func([]byte) string) ([]string, error) {
	var ids []string
	err := db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()

		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			keyCopy := it.Item().KeyCopy(nil)
			ids = append(ids, extractID(keyCopy))
		}
		return nil
	})
	return ids, err
}
