package reputation

import (
	"fmt"
	"strings"
)

// VoteOutcome describes a single validator's vote relative to the finalized
// verdict. It is the input to the post-finalization reputation update.
// See PulseSyn Protocol Specification v0.1, Section 5.2.
type VoteOutcome struct {
	// ValidatorID is the public key hash of the validator.
	ValidatorID string

	// Domain is the primary domain of the claim that was validated.
	Domain string

	// WasCorrect is true if the validator's verdict matched the finalized
	// consensus verdict.
	WasCorrect bool

	// Confidence is the validator-stated certainty at vote time [0.0, 1.0].
	Confidence float64

	// Participated indicates whether the validator submitted a vote.
	// False means the validator was selected but did not vote (abstention).
	Participated bool

	// WasLate indicates the validator's vote arrived after the window closed
	// and was discarded.
	WasLate bool
}

// UpdateResult holds the reputation delta and new score for a single validator
// after a post-finalization update.
type UpdateResult struct {
	ValidatorID string
	Domain      string
	OldScore    float64
	NewScore    float64
	Delta       float64
	Reason      UpdateReason
}

// UpdateReason describes why a reputation change was applied.
type UpdateReason string

const (
	ReasonCorrectHighConfidence   UpdateReason = "CORRECT_HIGH_CONFIDENCE"
	ReasonCorrectLowConfidence    UpdateReason = "CORRECT_LOW_CONFIDENCE"
	ReasonCorrectMidConfidence    UpdateReason = "CORRECT_MID_CONFIDENCE"
	ReasonIncorrectHighConfidence UpdateReason = "INCORRECT_HIGH_CONFIDENCE"
	ReasonIncorrectLowConfidence  UpdateReason = "INCORRECT_LOW_CONFIDENCE"
	ReasonIncorrectMidConfidence  UpdateReason = "INCORRECT_MID_CONFIDENCE"
	ReasonAbsent                  UpdateReason = "ABSENT"
	ReasonLate                    UpdateReason = "LATE"
)

// ApplyPostFinalizationUpdate applies the reputation update rules to a single
// validator based on their vote outcome relative to the finalized verdict.
//
// The six update cases from the spec are:
//  1. Correct verdict, high confidence (> 0.7)  → +DeltaHigh
//  2. Correct verdict, low confidence  (< 0.3)  → +DeltaLow
//  3. Correct verdict, mid confidence           → +DeltaLow
//  4. Incorrect verdict, high confidence        → -DeltaHigh (overconfidence penalty)
//  5. Incorrect verdict, low confidence         → -DeltaLow
//  6. Incorrect verdict, mid confidence         → -DeltaLow
//  7. No vote submitted (abstention)            → -DeltaAbsent
//  8. Late vote (discarded)                     → -DeltaLate
//
// See PulseSyn Protocol Specification v0.1, Section 5.2.
func ApplyPostFinalizationUpdate(record *ValidatorRecord, outcome VoteOutcome) (*ValidatorRecord, UpdateResult, error) {
	if err := validateOutcome(outcome); err != nil {
		return nil, UpdateResult{}, fmt.Errorf("ApplyPostFinalizationUpdate: %w", err)
	}

	domain := strings.ToLower(strings.TrimSpace(outcome.Domain))
	oldScore := record.DomainScore(domain)

	delta, reason := computeDelta(outcome)

	newScore := clamp(oldScore+delta, ReputationFloor, ReputationCeiling)

	updated := record.withUpdatedDomainScore(domain, newScore)
	updated.TotalValidations++

	result := UpdateResult{
		ValidatorID: record.ValidatorID,
		Domain:      domain,
		OldScore:    oldScore,
		NewScore:    newScore,
		Delta:       delta,
		Reason:      reason,
	}

	return updated, result, nil
}

// computeDelta returns the reputation delta and reason for a given vote outcome.
// See PulseSyn Protocol Specification v0.1, Section 5.2.
func computeDelta(outcome VoteOutcome) (float64, UpdateReason) {
	if !outcome.Participated {
		return -DeltaAbsent, ReasonAbsent
	}
	if outcome.WasLate {
		return -DeltaLate, ReasonLate
	}
	if outcome.WasCorrect {
		switch {
		case outcome.Confidence > HighConfidenceThreshold:
			return +DeltaHigh, ReasonCorrectHighConfidence
		case outcome.Confidence < LowConfidenceThreshold:
			return +DeltaLow, ReasonCorrectLowConfidence
		default:
			return +DeltaLow, ReasonCorrectMidConfidence
		}
	}
	// Incorrect verdict.
	switch {
	case outcome.Confidence > HighConfidenceThreshold:
		return -DeltaHigh, ReasonIncorrectHighConfidence
	case outcome.Confidence < LowConfidenceThreshold:
		return -DeltaLow, ReasonIncorrectLowConfidence
	default:
		return -DeltaLow, ReasonIncorrectMidConfidence
	}
}

// validateOutcome checks that a VoteOutcome contains valid field values.
func validateOutcome(o VoteOutcome) error {
	if strings.TrimSpace(o.ValidatorID) == "" {
		return ErrValidatorIDEmpty
	}
	if strings.TrimSpace(o.Domain) == "" {
		return ErrDomainEmpty
	}
	if o.Confidence < 0.0 || o.Confidence > 1.0 {
		return fmt.Errorf("%w: confidence %f out of range", ErrInvalidReputation, o.Confidence)
	}
	return nil
}
