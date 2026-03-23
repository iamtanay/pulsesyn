package simulation

import (
	"fmt"
	"math/rand"

	"github.com/iamtanay/pulsesyn/core/bias"
	"github.com/iamtanay/pulsesyn/core/consensus"
	"github.com/iamtanay/pulsesyn/core/reputation"
)

// groundTruth represents the correct answer for a synthetic claim.
// In the simulation, ground truth is assigned at claim generation time
// and used to evaluate validator accuracy after consensus.
type groundTruth struct {
	// verdict is the "correct" verdict for this claim.
	verdict string

	// verdictScore is the numeric encoding of the correct verdict on
	// the [0.0, 1.0] support axis used by the bias module.
	verdictScore float64
}

// roundResult holds the outcome of a single simulated validation round.
type roundResult struct {
	// consensusVerdict is the verdict produced by ComputeConsensus.
	consensusVerdict string

	// correct is true if the consensus verdict matched the ground truth,
	// and the consensus verdict is not INDETERMINATE.
	correct bool

	// participationRate is the fraction of selected validators who voted.
	participationRate float64

	// validatorCount is the number of votes included in consensus.
	validatorCount int

	// biasDetectionEvents counts validators whose bias window exceeded
	// BiasModerateThreshold after this round's observations were recorded.
	biasDetectionEvents int
}

// simulateRound runs a single validation round: generates a claim, assigns
// votes based on validator behaviour type (honest, colluding, biased), calls
// ComputeConsensus, applies post-finalization reputation updates, records
// bias observations, and returns the round outcome.
func simulateRound(
	cfg Config,
	pool []*simValidator,
	tracker *bias.Tracker,
	rng *rand.Rand,
	roundNum int,
) (roundResult, error) {
	// 1. Generate synthetic ground truth for this claim.
	truth := generateGroundTruth(rng)
	claimID := generateClaimID(roundNum)

	// 2. Select validator subset.
	selected := selectValidators(pool, cfg.ValidatorSetSize, rng)

	// 3. Build consensus votes. Each validator submits a vote influenced
	//    by their type: honest validators vote the ground truth verdict,
	//    colluders vote the fixed collusion verdict, biased validators
	//    vote a systematically skewed verdict.
	votes := make([]consensus.Vote, 0, len(selected))
	for _, sv := range selected {
		v := buildVote(sv, truth, cfg, rng)
		votes = append(votes, v)
	}

	// 4. Compute the actual population average verdict score across all
	//    selected validators. This is recorded in the bias window as the
	//    reference baseline, enabling the bias module to measure how far
	//    each validator deviates from the group.
	populationAvg := computePopulationAverageFromVotes(votes)

	// 5. Run consensus.
	result, err := consensus.ComputeConsensus(votes, cfg.ValidatorSetSize)
	if err != nil {
		return roundResult{}, err
	}

	// 6. Apply post-finalization reputation updates.
	for i, sv := range selected {
		vote := votes[i]
		wasCorrect := vote.Verdict == consensus.VerdictState(truth.verdict)

		outcome := reputation.VoteOutcome{
			ValidatorID:  sv.record.ValidatorID,
			Domain:       cfg.Domain,
			WasCorrect:   wasCorrect,
			Confidence:   vote.Confidence,
			Participated: true,
			WasLate:      false,
		}

		updated, _, err := reputation.ApplyPostFinalizationUpdate(sv.record, outcome)
		if err != nil {
			return roundResult{}, err
		}
		selected[i].record = updated
	}

	// 7. Record bias observations for each validator.
	// Each validator's observation uses the population average computed
	// from all votes in this round — giving the bias module a real signal
	// to detect systematic deviators.
	biasEvents := 0
	for i, sv := range selected {
		obs := bias.ValidationObservation{
			ClaimID:                claimID,
			ValidatorVerdict:       string(votes[i].Verdict),
			PopulationAverageScore: populationAvg,
		}
		if err := tracker.Record(sv.record.ValidatorID, cfg.Domain, obs); err != nil {
			return roundResult{}, err
		}

		// Count validators whose bias has crossed the moderate threshold.
		biasResult := tracker.BiasFor(sv.record.ValidatorID, cfg.Domain)
		if biasResult.Coefficient >= bias.BiasModerateThreshold {
			biasEvents++
		}
	}

	return roundResult{
		consensusVerdict:    string(result.Verdict),
		correct:             result.Verdict == consensus.VerdictState(truth.verdict),
		participationRate:   result.ParticipationRate,
		validatorCount:      result.ValidatorCount,
		biasDetectionEvents: biasEvents,
	}, nil
}

// buildVote constructs a consensus.Vote for a single validator in a round.
//
//   - Colluding validators always vote the configured collusion verdict with
//     high confidence, regardless of ground truth.
//   - Biased validators vote a systematically skewed verdict. With bias
//     strength >= 0.5, they flip to the opposite verdict from ground truth
//     (SUPPORTED↔UNSUPPORTED), creating a large, measurable deviation in
//     the bias window. This matches the spec's intent: bias is a systematic
//     non-epistemic voting pattern, not just a confidence anomaly.
//   - Honest validators vote the ground truth verdict with confidence drawn
//     from a realistic uniform distribution.
func buildVote(sv *simValidator, truth groundTruth, cfg Config, rng *rand.Rand) consensus.Vote {
	var verdict string
	var confidence float64

	switch {
	case sv.isColluder:
		verdict = cfg.CollusionVerdict
		confidence = 0.8 + rng.Float64()*0.2 // 0.8–1.0 — colluders are overconfident

	case sv.biasDirection > 0:
		// Biased validators deviate on their verdict, not just their
		// confidence. biasDirection is the probability that this validator
		// votes the opposite of ground truth on this particular claim.
		// With BiasStrength=0.75, they flip 75% of the time, producing a
		// mean_deviation well above BiasModerateThreshold (0.30).
		if rng.Float64() < sv.biasDirection {
			verdict = oppositeVerdict(truth.verdict)
		} else {
			verdict = truth.verdict
		}
		confidence = 0.4 + rng.Float64()*0.4 // 0.4–0.8

	default:
		// Honest validator: correct verdict, realistic confidence spread.
		verdict = truth.verdict
		confidence = 0.4 + rng.Float64()*0.5 // 0.4–0.9
	}

	return consensus.Vote{
		ValidatorID:      sv.record.ValidatorID,
		Verdict:          consensus.VerdictState(verdict),
		Confidence:       confidence,
		DomainReputation: sv.record.DomainScore(cfg.Domain),
		BiasCoefficient:  sv.biasDirection,
		ValidatorSetSize: cfg.ValidatorSetSize,
	}
}

// oppositeVerdict returns the verdict most opposite to v on the support axis.
// SUPPORTED ↔ UNSUPPORTED. MISLEADING and INDETERMINATE both map to UNSUPPORTED
// as the strongest available counter-signal.
func oppositeVerdict(v string) string {
	switch v {
	case "SUPPORTED":
		return "UNSUPPORTED"
	case "UNSUPPORTED":
		return "SUPPORTED"
	default:
		// MISLEADING and INDETERMINATE: flip to UNSUPPORTED as the most
		// distinct verdict from the midpoint.
		return "UNSUPPORTED"
	}
}

// generateGroundTruth assigns a random ground truth verdict for a synthetic
// claim. The distribution is weighted toward SUPPORTED and UNSUPPORTED
// (40% each) with smaller fractions for MISLEADING (15%) and INDETERMINATE (5%).
func generateGroundTruth(rng *rand.Rand) groundTruth {
	r := rng.Float64()
	switch {
	case r < 0.40:
		return groundTruth{verdict: "SUPPORTED", verdictScore: 1.0}
	case r < 0.80:
		return groundTruth{verdict: "UNSUPPORTED", verdictScore: 0.0}
	case r < 0.95:
		return groundTruth{verdict: "MISLEADING", verdictScore: 0.5}
	default:
		return groundTruth{verdict: "INDETERMINATE", verdictScore: 0.5}
	}
}

// generateClaimID produces a deterministic synthetic claim identifier.
func generateClaimID(roundNum int) string {
	return fmt.Sprintf("sim-claim-%06d", roundNum)
}

// computePopulationAverageFromVotes computes the mean verdict score across
// all votes in the round. This is the reference baseline passed to each
// validator's bias observation — it gives the bias module a real signal
// by comparing each validator's vote to the group's actual verdict distribution.
//
// Using actual votes (rather than ground truth) is the correct approach:
// it mirrors what the bias module does in production, where the population
// average is computed from the observed validator votes, not from a hidden
// ground truth that validators don't have access to.
func computePopulationAverageFromVotes(votes []consensus.Vote) float64 {
	if len(votes) == 0 {
		return 0.5
	}
	verdictScores := map[consensus.VerdictState]float64{
		consensus.VerdictSupported:     1.0,
		consensus.VerdictUnsupported:   0.0,
		consensus.VerdictMisleading:    0.5,
		consensus.VerdictIndeterminate: 0.5,
	}
	var sum float64
	for _, v := range votes {
		sum += verdictScores[v.Verdict]
	}
	return sum / float64(len(votes))
}
