package reputation

import (
	"fmt"
	"math"
	"strings"
)

// DecayRate is the daily reputation decay rate for inactive validators.
// Applied per day of inactivity beyond DecayInactivityDays.
// See PulseSyn Protocol Specification v0.1, Section 5.4 and Appendix A.
const (
	DecayRate           = 0.001
	DecayInactivityDays = 30
)

// DecayInput carries the parameters needed to apply reputation decay to a
// single validator in a single domain.
type DecayInput struct {
	// ValidatorID is the public key hash of the validator.
	ValidatorID string

	// Domain is the domain for which decay is being applied.
	Domain string

	// DaysInactive is the number of consecutive days the validator has not
	// participated in any validation in this domain.
	DaysInactive int
}

// DecayResult holds the outcome of a decay operation.
type DecayResult struct {
	ValidatorID  string
	Domain       string
	OldScore     float64
	NewScore     float64
	DaysInactive int
	Applied      bool
}

// ApplyDecay applies the reputation decay formula to a single validator in a
// single domain. Decay activates only after DecayInactivityDays consecutive
// days of inactivity. Reputation does not decay below ReputationFloor.
//
// Decay formula:
//
//	new_score = old_score x (1 - DecayRate)^days_inactive
//
// See PulseSyn Protocol Specification v0.1, Section 5.4.
func ApplyDecay(record *ValidatorRecord, input DecayInput) (*ValidatorRecord, DecayResult, error) {
	if err := validateDecayInput(input); err != nil {
		return nil, DecayResult{}, fmt.Errorf("ApplyDecay: %w", err)
	}

	domain := strings.ToLower(strings.TrimSpace(input.Domain))
	oldScore := record.DomainScore(domain)

	result := DecayResult{
		ValidatorID:  record.ValidatorID,
		Domain:       domain,
		OldScore:     oldScore,
		DaysInactive: input.DaysInactive,
		Applied:      false,
	}

	// Decay only activates beyond the inactivity threshold.
	if input.DaysInactive <= DecayInactivityDays {
		result.NewScore = oldScore
		return record, result, nil
	}

	effectiveDays := input.DaysInactive - DecayInactivityDays
	newScore := oldScore * math.Pow(1.0-DecayRate, float64(effectiveDays))
	newScore = clamp(newScore, ReputationFloor, ReputationCeiling)

	updated := record.withUpdatedDomainScore(domain, newScore)

	result.NewScore = newScore
	result.Applied = newScore < oldScore

	return updated, result, nil
}

// validateDecayInput checks that a DecayInput contains valid field values.
func validateDecayInput(input DecayInput) error {
	if strings.TrimSpace(input.ValidatorID) == "" {
		return ErrValidatorIDEmpty
	}
	if strings.TrimSpace(input.Domain) == "" {
		return ErrDomainEmpty
	}
	if input.DaysInactive < 0 {
		return fmt.Errorf("pulsesyn/reputation: days_inactive cannot be negative")
	}
	return nil
}
