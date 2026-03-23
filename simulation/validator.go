package simulation

import (
	"fmt"
	"math/rand"

	"github.com/iamtanay/pulsesyn/core/reputation"
)

// simValidator is the simulation's internal representation of a validator.
// It wraps a ValidatorRecord with simulation-specific state: whether the
// validator is a colluder and their bias profile.
type simValidator struct {
	// record is the protocol-level reputation record for this validator.
	record *reputation.ValidatorRecord

	// isColluder indicates this validator always votes the configured
	// collusion verdict regardless of the ground truth.
	isColluder bool

	// biasDirection is the systematic deviation applied to this validator's
	// confidence signals when they are a biased (but non-colluding) validator.
	// Range: [0.0, 1.0]. 0.0 means no bias.
	biasDirection float64
}

// buildValidatorPool constructs a synthetic pool of simValidators.
// Validators receive randomised domain reputations drawn from a realistic
// distribution. The colluder and biased fractions are applied in order.
//
// Reputation distribution: validators are assigned domain reputation scores
// from a triangular distribution between EligibilityThreshold and 1.0,
// weighted toward the centre (0.5). This produces a realistic bell-like
// spread rather than uniform randomness.
func buildValidatorPool(cfg Config, rng *rand.Rand) ([]*simValidator, error) {
	pool := make([]*simValidator, 0, cfg.ValidatorPoolSize)

	colluderCount := int(float64(cfg.ValidatorPoolSize) * cfg.CollusionRate)
	biasedCount := int(float64(cfg.ValidatorPoolSize) * cfg.BiasRate)

	for i := 0; i < cfg.ValidatorPoolSize; i++ {
		validatorID := fmt.Sprintf("validator-%04d", i)

		// Domain reputation: triangular distribution in [EligibilityThreshold, 1.0].
		// Using average of two uniform draws approximates a triangular distribution.
		repA := rng.Float64()*(1.0-reputation.EligibilityThreshold) + reputation.EligibilityThreshold
		repB := rng.Float64()*(1.0-reputation.EligibilityThreshold) + reputation.EligibilityThreshold
		domainRep := (repA + repB) / 2.0

		record, err := reputation.NewValidatorRecord(validatorID, 0)
		if err != nil {
			return nil, fmt.Errorf("buildValidatorPool: %w", err)
		}

		// Inject the domain reputation directly by applying a synthetic
		// post-finalization update using the internal update path. Since we
		// are in the simulation package, we construct a VoteOutcome that
		// brings the validator to approximately the desired score.
		// The exact value is less important than a realistic distribution.
		record = injectDomainReputation(record, cfg.Domain, domainRep)

		sv := &simValidator{
			record:        record,
			isColluder:    i < colluderCount,
			biasDirection: 0.0,
		}

		if i >= colluderCount && i < colluderCount+biasedCount {
			sv.biasDirection = cfg.BiasStrength
		}

		pool = append(pool, sv)
	}

	return pool, nil
}

// injectDomainReputation returns a new ValidatorRecord with the given domain
// reputation score set directly. This bypasses the normal update delta rules
// because the simulation needs to initialise validators at arbitrary points
// in the reputation space, not earn them through sequential updates.
//
// Implementation: applies ApplyPostFinalizationUpdate repeatedly to
// approximate the target score. This is deliberate — using only the public
// API of core/reputation ensures the simulation does not encode privileged
// knowledge of the reputation package's internals.
func injectDomainReputation(record *reputation.ValidatorRecord, domain string, targetScore float64) *reputation.ValidatorRecord {
	// A genesis-level starting score is the simplest approximation.
	// For simulation purposes, a score within 0.10 of the target is
	// sufficient. Exact precision is not required because the simulation
	// measures statistical emergent behaviour, not individual validator scores.
	current := record.DomainScore(domain)
	if current >= targetScore-0.10 && current <= targetScore+0.10 {
		return record
	}

	// Apply repeated high-confidence correct updates to bring the score up.
	// This converges because DeltaHigh = 0.05 and the starting score is 0.0.
	maxIterations := 40
	for i := 0; i < maxIterations && record.DomainScore(domain) < targetScore-0.05; i++ {
		outcome := reputation.VoteOutcome{
			ValidatorID:  record.ValidatorID,
			Domain:       domain,
			WasCorrect:   true,
			Confidence:   0.9,
			Participated: true,
			WasLate:      false,
		}
		updated, _, err := reputation.ApplyPostFinalizationUpdate(record, outcome)
		if err != nil {
			break
		}
		record = updated
	}
	return record
}

// selectValidators draws cfg.ValidatorSetSize validators from the pool
// without replacement. In Phase 1 simulation, selection is uniform random
// (the full composite score algorithm is implemented in the selector package
// in Phase 3). Uniform random is sufficient to validate the consensus math.
func selectValidators(pool []*simValidator, setSize int, rng *rand.Rand) []*simValidator {
	indices := rng.Perm(len(pool))
	selected := make([]*simValidator, 0, setSize)
	for _, idx := range indices[:setSize] {
		selected = append(selected, pool[idx])
	}
	return selected
}
