package consensus

import (
	"errors"
	"fmt"
)

// Protocol constants for the consensus algorithm.
// These are the Phase 1 defaults. Governable parameters will be configurable
// in later phases via the governance module.
// See PulseSyn Protocol Specification v0.1, Appendix A.
const (
	// QuorumThreshold is the minimum fraction of selected validators that
	// must submit votes for consensus to be valid. Default 2/3.
	QuorumThreshold = 2.0 / 3.0

	// VerdictMajorityThreshold is the minimum fraction of total adjusted
	// weight mass the winning verdict must hold to be declared.
	VerdictMajorityThreshold = 0.50

	// MinConfidence is the minimum allowed validator-stated confidence value.
	MinConfidence = 0.0

	// MaxConfidence is the maximum allowed validator-stated confidence value.
	MaxConfidence = 1.0

	// MinDomainReputation is the minimum allowed domain reputation value.
	MinDomainReputation = 0.0

	// MaxDomainReputation is the maximum allowed domain reputation value.
	MaxDomainReputation = 1.0

	// MinBiasCoefficient is the minimum allowed bias coefficient value.
	MinBiasCoefficient = 0.0

	// MaxBiasCoefficient is the maximum allowed bias coefficient value.
	MaxBiasCoefficient = 1.0
)

// Sentinel errors returned by ComputeConsensus and validateVote.
var (
	// ErrNoVotes is returned when the votes slice is empty.
	ErrNoVotes = errors.New("pulsesyn/consensus: no votes provided")

	// ErrQuorumNotMet is returned when fewer than 2/3 of validators voted.
	ErrQuorumNotMet = errors.New("pulsesyn/consensus: participation quorum not met")

	// ErrInvalidVote is returned when a Vote contains out-of-range field values.
	ErrInvalidVote = errors.New("pulsesyn/consensus: vote contains invalid field values")
)

// Vote represents a single validator's revealed vote in a validation session.
// All fields are set at reveal time and are immutable after construction.
// See PulseSyn Protocol Specification v0.1, Section 2.3.
type Vote struct {
	// ValidatorID is the public key hash of the voting validator.
	ValidatorID string

	// Verdict is the validator's verdict choice.
	Verdict VerdictState

	// Confidence is the validator-stated certainty in their verdict [0.0, 1.0].
	Confidence float64

	// DomainReputation is the validator's reputation score in the claim's
	// primary domain at the time of selection [0.0, 1.0].
	DomainReputation float64

	// BiasCoefficient is the validator's bias coefficient for this domain
	// at the time of selection [0.0, 1.0]. 0.0 = no detected bias.
	BiasCoefficient float64

	// ValidatorSetSize is the total number of validators selected for this
	// claim. Used to compute the participation rate.
	ValidatorSetSize int
}

// ComputeConsensus applies the PoV weighted vote aggregation algorithm to a
// set of validator votes and returns a ConsensusResult containing the verdict
// and aggregate confidence score.
//
// Returns ErrNoVotes if votes is empty.
// Returns a result with VerdictIndeterminate if quorum or majority thresholds
// are not met — this is not an error condition, it is a valid protocol outcome.
//
// See PulseSyn Protocol Specification v0.1, Section 4.2, 4.3, 4.4.
func ComputeConsensus(votes []Vote, validatorSetSize int) (*ConsensusResult, error) {
	if len(votes) == 0 {
		return nil, ErrNoVotes
	}
	if validatorSetSize <= 0 {
		validatorSetSize = len(votes)
	}

	for i, v := range votes {
		if err := validateVote(v); err != nil {
			return nil, fmt.Errorf("ComputeConsensus: vote[%d] (%s): %w", i, v.ValidatorID, err)
		}
	}

	participationRate := float64(len(votes)) / float64(validatorSetSize)

	// Quorum check — Section 4.4.
	// If fewer than 2/3 of validators submitted votes, return indeterminate.
	// This is a valid protocol outcome, not an error.
	if participationRate < QuorumThreshold {
		breakdown := aggregateWeights(votes)
		return &ConsensusResult{
			Verdict:           VerdictIndeterminate,
			ConfidenceScore:   0,
			Breakdown:         breakdown,
			ParticipationRate: participationRate,
			ValidatorCount:    len(votes),
		}, nil
	}

	breakdown := aggregateWeights(votes)

	if breakdown.TotalMass == 0 {
		return &ConsensusResult{
			Verdict:           VerdictIndeterminate,
			ConfidenceScore:   0,
			Breakdown:         breakdown,
			ParticipationRate: participationRate,
			ValidatorCount:    len(votes),
		}, nil
	}

	// Majority check — Section 4.4.
	// Winning verdict must hold > 50% of total adjusted weight mass.
	winner, winnerMass := findWinner(breakdown)
	majorityFraction := winnerMass / breakdown.TotalMass

	if majorityFraction <= VerdictMajorityThreshold {
		return &ConsensusResult{
			Verdict:           VerdictIndeterminate,
			ConfidenceScore:   0,
			Breakdown:         breakdown,
			ParticipationRate: participationRate,
			ValidatorCount:    len(votes),
		}, nil
	}

	confidence := computeConfidence(votes, winner)

	return &ConsensusResult{
		Verdict:           winner,
		ConfidenceScore:   confidence,
		Breakdown:         breakdown,
		ParticipationRate: participationRate,
		ValidatorCount:    len(votes),
	}, nil
}

// aggregateWeights computes the sum of bias-adjusted weights per verdict state.
// For each vote:
//
//	weighted_vote    = confidence x domain_reputation
//	adjusted_weight  = weighted_vote x (1 - bias_coefficient)
//
// See PulseSyn Protocol Specification v0.1, Section 4.2 and 4.3.
func aggregateWeights(votes []Vote) VerdictBreakdown {
	var b VerdictBreakdown
	for _, v := range votes {
		w := adjustedWeight(v)
		switch v.Verdict {
		case VerdictSupported:
			b.Supported += w
		case VerdictUnsupported:
			b.Unsupported += w
		case VerdictMisleading:
			b.Misleading += w
		case VerdictIndeterminate:
			b.Indeterminate += w
		}
		b.TotalMass += w
	}
	return b
}

// adjustedWeight computes the bias-corrected weight for a single vote.
//
//	weighted_vote   = confidence x domain_reputation
//	adjusted_weight = weighted_vote x (1 - bias_coefficient)
//
// See PulseSyn Protocol Specification v0.1, Section 4.2 and 4.3.
func adjustedWeight(v Vote) float64 {
	return v.Confidence * v.DomainReputation * (1.0 - v.BiasCoefficient)
}

// findWinner returns the verdict with the highest aggregated adjusted weight
// and that weight value.
func findWinner(b VerdictBreakdown) (VerdictState, float64) {
	candidates := []struct {
		verdict VerdictState
		mass    float64
	}{
		{VerdictSupported, b.Supported},
		{VerdictUnsupported, b.Unsupported},
		{VerdictMisleading, b.Misleading},
		{VerdictIndeterminate, b.Indeterminate},
	}

	winner := candidates[0]
	for _, c := range candidates[1:] {
		if c.mass > winner.mass {
			winner = c
		}
	}
	return winner.verdict, winner.mass
}

// computeConfidence calculates the aggregate confidence score for the winning
// verdict. It is the bias-adjusted weighted average of confidence values from
// validators who voted for the winning verdict, normalised to [0.0, 1.0].
func computeConfidence(votes []Vote, winner VerdictState) float64 {
	var weightedConfidenceSum, weightSum float64
	for _, v := range votes {
		if v.Verdict != winner {
			continue
		}
		w := adjustedWeight(v)
		weightedConfidenceSum += v.Confidence * w
		weightSum += w
	}
	if weightSum == 0 {
		return 0
	}
	return weightedConfidenceSum / weightSum
}

// validateVote checks that all numeric fields of a Vote are within their
// defined protocol ranges.
func validateVote(v Vote) error {
	if v.ValidatorID == "" {
		return fmt.Errorf("%w: validator_id is empty", ErrInvalidVote)
	}
	if v.Confidence < MinConfidence || v.Confidence > MaxConfidence {
		return fmt.Errorf("%w: confidence %f out of range [0.0, 1.0]", ErrInvalidVote, v.Confidence)
	}
	if v.DomainReputation < MinDomainReputation || v.DomainReputation > MaxDomainReputation {
		return fmt.Errorf("%w: domain_reputation %f out of range [0.0, 1.0]", ErrInvalidVote, v.DomainReputation)
	}
	if v.BiasCoefficient < MinBiasCoefficient || v.BiasCoefficient > MaxBiasCoefficient {
		return fmt.Errorf("%w: bias_coefficient %f out of range [0.0, 1.0]", ErrInvalidVote, v.BiasCoefficient)
	}
	if err := validateVerdictState(v.Verdict); err != nil {
		return err
	}
	return nil
}

// validateVerdictState returns ErrInvalidVote if the verdict is not one of
// the four recognised protocol verdict states.
func validateVerdictState(v VerdictState) error {
	switch v {
	case VerdictSupported, VerdictUnsupported, VerdictMisleading, VerdictIndeterminate:
		return nil
	default:
		return fmt.Errorf("%w: unrecognised verdict state %q", ErrInvalidVote, v)
	}
}
