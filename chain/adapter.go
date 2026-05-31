package chain

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// Sentinel errors returned by chain adapter operations.
var (
	ErrValidationNotFound = errors.New("pulsesyn/chain: validation record not found on-chain")
	ErrReputationNotFound = errors.New("pulsesyn/chain: reputation record not found on-chain")
	ErrInvalidRecord      = errors.New("pulsesyn/chain: record is invalid")
)

// Adapter is the interface for all on-chain read and write operations.
// Protocol code never interacts with the chain directly — all on-chain
// reads and writes go through this interface.
//
// The concrete EthereumAdapter is deferred to Phase 2.5. NullAdapter is
// provided for development use. See DECISIONS.md: 2026-05-31.
type Adapter interface {
	// WriteClaimRecord records a new claim on-chain. Returns the transaction
	// hash on success. Implementations must enforce write-once semantics — a
	// second write for the same ClaimID must be rejected.
	WriteClaimRecord(record ClaimRecord) (txHash string, err error)

	// WriteValidationRecord records a finalized validation result on-chain.
	// Write-once per ClaimID.
	WriteValidationRecord(record ValidationRecord) (txHash string, err error)

	// UpdateReputation records a reputation score change on-chain.
	// Not write-once — reputation evolves after each validation session.
	UpdateReputation(update ReputationUpdate) (txHash string, err error)

	// ReadValidationRecord returns the on-chain validation record for a claim.
	// Returns ErrValidationNotFound if no record exists for the claimID.
	ReadValidationRecord(claimID string) (*ValidationRecord, error)

	// ReadReputation returns the on-chain reputation state for a validator in
	// a specific domain. Returns ErrReputationNotFound if no record exists.
	ReadReputation(validatorID, domain string) (*ReputationState, error)
}

// ClaimRecord is the on-chain representation of a submitted claim. It carries
// the minimal set of fields required for on-chain verification and audit.
// See PulseSyn Protocol Specification v0.1, Section 7.1.
type ClaimRecord struct {
	// ClaimID is the protocol-derived canonical identifier for this claim.
	ClaimID string `json:"claim_id"`

	// ContentHash is the SHA-256 hash of the evidence content at ContentURL.
	ContentHash string `json:"content_hash"`

	// SubmitterID is the public key hash of the submitting participant.
	SubmitterID string `json:"submitter_id"`

	// ClaimType is the claim type string: FACTUAL, CONTEXTUAL, or PREDICTIVE.
	ClaimType string `json:"claim_type"`

	// Domain is the primary domain tag of the claim.
	Domain string `json:"domain"`

	// Epoch is the block number at the time of submission.
	Epoch uint64 `json:"epoch"`

	// SubmittedAt is the wall-clock time the claim was submitted.
	SubmittedAt time.Time `json:"submitted_at"`
}

// Validate checks that all required ClaimRecord fields are non-empty.
func (r ClaimRecord) Validate() error {
	if strings.TrimSpace(r.ClaimID) == "" {
		return fmt.Errorf("%w: claim_id is empty", ErrInvalidRecord)
	}
	if strings.TrimSpace(r.ContentHash) == "" {
		return fmt.Errorf("%w: content_hash is empty", ErrInvalidRecord)
	}
	if strings.TrimSpace(r.SubmitterID) == "" {
		return fmt.Errorf("%w: submitter_id is empty", ErrInvalidRecord)
	}
	if strings.TrimSpace(r.ClaimType) == "" {
		return fmt.Errorf("%w: claim_type is empty", ErrInvalidRecord)
	}
	if strings.TrimSpace(r.Domain) == "" {
		return fmt.Errorf("%w: domain is empty", ErrInvalidRecord)
	}
	return nil
}

// ValidationRecord is the on-chain record of a finalized validation result.
// It carries only the fields required for on-chain audit and cross-protocol
// verification. The full vote set is committed via MerkleRoot.
// See PulseSyn Protocol Specification v0.1, Section 7.2.
type ValidationRecord struct {
	// ClaimID is the claim this validation session evaluated.
	ClaimID string `json:"claim_id"`

	// Verdict is the consensus verdict string.
	Verdict string `json:"verdict"`

	// ConfidenceScore is the aggregate confidence score in [0.0, 1.0].
	ConfidenceScore float64 `json:"confidence_score"`

	// MerkleRoot is the hex-encoded root of the vote Merkle tree. Enables
	// cryptographic inclusion proofs for individual votes.
	MerkleRoot string `json:"merkle_root"`

	// Epoch is the block number at which the validation was finalized.
	Epoch uint64 `json:"epoch"`

	// FinalizedAt is the wall-clock time the validation was finalized.
	FinalizedAt time.Time `json:"finalized_at"`
}

// Validate checks that all required ValidationRecord fields are non-empty.
func (r ValidationRecord) Validate() error {
	if strings.TrimSpace(r.ClaimID) == "" {
		return fmt.Errorf("%w: claim_id is empty", ErrInvalidRecord)
	}
	if strings.TrimSpace(r.Verdict) == "" {
		return fmt.Errorf("%w: verdict is empty", ErrInvalidRecord)
	}
	if r.ConfidenceScore < 0.0 || r.ConfidenceScore > 1.0 {
		return fmt.Errorf("%w: confidence_score %f out of range [0.0, 1.0]", ErrInvalidRecord, r.ConfidenceScore)
	}
	if strings.TrimSpace(r.MerkleRoot) == "" {
		return fmt.Errorf("%w: merkle_root is empty", ErrInvalidRecord)
	}
	return nil
}

// ReputationUpdate represents a reputation score change to be recorded on-chain
// after a validation session is finalized.
// See PulseSyn Protocol Specification v0.1, Section 7.3.
type ReputationUpdate struct {
	// ValidatorID is the public key hash of the validator.
	ValidatorID string `json:"validator_id"`

	// Domain is the domain in which the reputation changed.
	Domain string `json:"domain"`

	// OldScore is the validator's score before the update.
	OldScore float64 `json:"old_score"`

	// NewScore is the validator's score after the update.
	NewScore float64 `json:"new_score"`

	// Reason describes the update case (e.g. CORRECT_HIGH_CONFIDENCE).
	Reason string `json:"reason"`

	// Epoch is the block number at which the update was applied.
	Epoch uint64 `json:"epoch"`
}

// Validate checks that all required ReputationUpdate fields are valid.
func (u ReputationUpdate) Validate() error {
	if strings.TrimSpace(u.ValidatorID) == "" {
		return fmt.Errorf("%w: validator_id is empty", ErrInvalidRecord)
	}
	if strings.TrimSpace(u.Domain) == "" {
		return fmt.Errorf("%w: domain is empty", ErrInvalidRecord)
	}
	if u.OldScore < 0.0 || u.OldScore > 1.0 {
		return fmt.Errorf("%w: old_score %f out of range [0.0, 1.0]", ErrInvalidRecord, u.OldScore)
	}
	if u.NewScore < 0.0 || u.NewScore > 1.0 {
		return fmt.Errorf("%w: new_score %f out of range [0.0, 1.0]", ErrInvalidRecord, u.NewScore)
	}
	return nil
}

// ReputationState is the on-chain reputation state for a validator in a domain,
// as returned by a ReadReputation call.
// See PulseSyn Protocol Specification v0.1, Section 7.3.
type ReputationState struct {
	// ValidatorID is the public key hash of the validator.
	ValidatorID string `json:"validator_id"`

	// Domain is the domain for which this score applies.
	Domain string `json:"domain"`

	// Score is the current reputation score in [0.0, 1.0].
	Score float64 `json:"score"`

	// Epoch is the block number of the last update.
	Epoch uint64 `json:"epoch"`

	// UpdatedAt is the wall-clock time of the last update.
	UpdatedAt time.Time `json:"updated_at"`
}

// NullAdapter is a no-op implementation of Adapter for development use.
// All write operations succeed and return a zero transaction hash.
// All read operations return ErrValidationNotFound / ErrReputationNotFound.
// Replace with EthereumAdapter when chain integration is ready (Phase 2.5).
type NullAdapter struct{}

// Compile-time assertion that NullAdapter implements Adapter.
var _ Adapter = (*NullAdapter)(nil)

const nullTxHash = "0x0000000000000000000000000000000000000000000000000000000000000000"

func (n *NullAdapter) WriteClaimRecord(record ClaimRecord) (string, error) {
	return nullTxHash, nil
}

func (n *NullAdapter) WriteValidationRecord(record ValidationRecord) (string, error) {
	return nullTxHash, nil
}

func (n *NullAdapter) UpdateReputation(update ReputationUpdate) (string, error) {
	return nullTxHash, nil
}

func (n *NullAdapter) ReadValidationRecord(claimID string) (*ValidationRecord, error) {
	return nil, fmt.Errorf("ReadValidationRecord (%s): %w", claimID, ErrValidationNotFound)
}

func (n *NullAdapter) ReadReputation(validatorID, domain string) (*ReputationState, error) {
	return nil, fmt.Errorf("ReadReputation (%s/%s): %w", validatorID, domain, ErrReputationNotFound)
}
