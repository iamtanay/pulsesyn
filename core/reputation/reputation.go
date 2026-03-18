package reputation

import (
	"errors"
	"fmt"
	"strings"
)

// ValidatorStatus represents the operational status of a validator.
// See PulseSyn Protocol Specification v0.1, Section 2.2.
type ValidatorStatus string

const (
	ValidatorStatusActive    ValidatorStatus = "ACTIVE"
	ValidatorStatusSuspended ValidatorStatus = "SUSPENDED"
	ValidatorStatusRetired   ValidatorStatus = "RETIRED"
)

// Protocol constants for the reputation system.
// See PulseSyn Protocol Specification v0.1, Appendix A.
const (
	// EligibilityThreshold is the minimum domain reputation a validator
	// must hold to be eligible for selection in that domain.
	EligibilityThreshold = 0.15

	// SubmissionMinReputation is the minimum global reputation required
	// to submit a claim.
	SubmissionMinReputation = 0.10

	// GenesisValidatorStartingReputation is the domain reputation assigned
	// to genesis validators in their registered domains at launch.
	GenesisValidatorStartingReputation = 0.75

	// ReputationFloor is the minimum value domain reputation can decay to.
	// Reputation never decays below the eligibility threshold.
	ReputationFloor = EligibilityThreshold

	// ReputationCeiling is the maximum value domain reputation can reach.
	ReputationCeiling = 1.0

	// DeltaHigh is the large reputation increment/decrement applied for
	// high-confidence correct/incorrect verdicts.
	DeltaHigh = 0.05

	// DeltaLow is the small reputation increment/decrement applied for
	// low-confidence correct/incorrect verdicts.
	DeltaLow = 0.01

	// DeltaAbsent is the participation penalty for validators who did not
	// submit a vote when selected.
	DeltaAbsent = 0.02

	// DeltaLate is the penalty for validators whose vote arrived after
	// the validation window closed.
	DeltaLate = 0.01

	// HighConfidenceThreshold is the confidence value above which a vote
	// is considered high-confidence for reputation update purposes.
	HighConfidenceThreshold = 0.7

	// LowConfidenceThreshold is the confidence value below which a vote
	// is considered low-confidence for reputation update purposes.
	LowConfidenceThreshold = 0.3
)

// Sentinel errors returned by reputation operations.
var (
	ErrValidatorIDEmpty  = errors.New("pulsesyn/reputation: validator_id is empty")
	ErrDomainEmpty       = errors.New("pulsesyn/reputation: domain is empty")
	ErrInvalidReputation = errors.New("pulsesyn/reputation: reputation value out of range [0.0, 1.0]")
	ErrValidatorNotFound = errors.New("pulsesyn/reputation: validator not found")
)

// DomainReputation maps domain names to reputation scores in [0.0, 1.0].
type DomainReputation map[string]float64

// ValidatorRecord holds the full reputation state for a single validator.
// It is immutable once constructed — update operations return new records.
// See PulseSyn Protocol Specification v0.1, Section 2.2.
type ValidatorRecord struct {
	// ValidatorID is the public key hash of the validator.
	ValidatorID string

	// GenesisValidator indicates whether this validator was part of the
	// initial genesis set. Genesis validators start with elevated reputation.
	GenesisValidator bool

	// RegistrationEpoch is the block number at which this validator registered.
	RegistrationEpoch uint64

	// DomainScores holds the validator's reputation score per domain.
	// Scores are in [0.0, 1.0]. Absence means no history in that domain.
	DomainScores DomainReputation

	// GlobalReputation is the weighted average of all domain scores.
	// Used for general participation eligibility only.
	GlobalReputation float64

	// TotalValidations is the cumulative count of validation rounds
	// this validator has participated in.
	TotalValidations uint64

	// Status is the current operational status of the validator.
	Status ValidatorStatus
}

// NewValidatorRecord constructs a new ValidatorRecord with zero reputation
// in all domains. For genesis validators, use NewGenesisValidatorRecord.
func NewValidatorRecord(validatorID string, registrationEpoch uint64) (*ValidatorRecord, error) {
	if strings.TrimSpace(validatorID) == "" {
		return nil, fmt.Errorf("NewValidatorRecord: %w", ErrValidatorIDEmpty)
	}
	return &ValidatorRecord{
		ValidatorID:       strings.TrimSpace(validatorID),
		GenesisValidator:  false,
		RegistrationEpoch: registrationEpoch,
		DomainScores:      make(DomainReputation),
		GlobalReputation:  0.0,
		TotalValidations:  0,
		Status:            ValidatorStatusActive,
	}, nil
}

// NewGenesisValidatorRecord constructs a ValidatorRecord for a genesis
// validator with GenesisValidatorStartingReputation in all registered domains.
// See PulseSyn Protocol Specification v0.1, Section 3.1.
func NewGenesisValidatorRecord(validatorID string, domains []string, registrationEpoch uint64) (*ValidatorRecord, error) {
	if strings.TrimSpace(validatorID) == "" {
		return nil, fmt.Errorf("NewGenesisValidatorRecord: %w", ErrValidatorIDEmpty)
	}
	if len(domains) == 0 {
		return nil, fmt.Errorf("NewGenesisValidatorRecord: %w", ErrDomainEmpty)
	}

	scores := make(DomainReputation)
	for _, d := range domains {
		d = strings.ToLower(strings.TrimSpace(d))
		if d != "" {
			scores[d] = GenesisValidatorStartingReputation
		}
	}

	global := computeGlobalReputation(scores)

	return &ValidatorRecord{
		ValidatorID:       strings.TrimSpace(validatorID),
		GenesisValidator:  true,
		RegistrationEpoch: registrationEpoch,
		DomainScores:      scores,
		GlobalReputation:  global,
		TotalValidations:  0,
		Status:            ValidatorStatusActive,
	}, nil
}

// DomainScore returns the validator's reputation score in the given domain.
// Returns 0.0 if the validator has no history in that domain.
func (r *ValidatorRecord) DomainScore(domain string) float64 {
	domain = strings.ToLower(strings.TrimSpace(domain))
	score, ok := r.DomainScores[domain]
	if !ok {
		return 0.0
	}
	return score
}

// IsEligibleForDomain returns true if the validator meets the minimum domain
// reputation threshold and is in ACTIVE status.
// See PulseSyn Protocol Specification v0.1, Section 3.3.
func (r *ValidatorRecord) IsEligibleForDomain(domain string) bool {
	if r.Status != ValidatorStatusActive {
		return false
	}
	return r.DomainScore(domain) >= EligibilityThreshold
}

// withUpdatedDomainScore returns a new ValidatorRecord with the given domain
// score updated and global reputation recomputed. The score is clamped to
// [ReputationFloor, ReputationCeiling].
func (r *ValidatorRecord) withUpdatedDomainScore(domain string, newScore float64) *ValidatorRecord {
	newScore = clamp(newScore, 0.0, ReputationCeiling)

	newScores := make(DomainReputation, len(r.DomainScores))
	for k, v := range r.DomainScores {
		newScores[k] = v
	}
	newScores[domain] = newScore

	updated := *r
	updated.DomainScores = newScores
	updated.GlobalReputation = computeGlobalReputation(newScores)
	return &updated
}

// computeGlobalReputation computes the simple average of all domain scores.
// In Phase 1 all domains are weighted equally. Domain-weighted averaging
// will be introduced when domain weight governance is implemented.
// See PulseSyn Protocol Specification v0.1, Section 2.2.
func computeGlobalReputation(scores DomainReputation) float64 {
	if len(scores) == 0 {
		return 0.0
	}
	var sum float64
	for _, v := range scores {
		sum += v
	}
	return sum / float64(len(scores))
}

// clamp returns v clamped to [min, max].
func clamp(v, min, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}
