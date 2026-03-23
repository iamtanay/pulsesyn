package simulation

import (
	"errors"
	"fmt"
)

// ScenarioName identifies one of the predefined simulation scenarios.
// See PulseSyn Development Plan v1.0, Phase 1, Section 1.5.
type ScenarioName string

const (
	// ScenarioHonestNetwork simulates a fully honest validator pool with no
	// collusion and no systematic bias. Establishes the accuracy baseline.
	ScenarioHonestNetwork ScenarioName = "HONEST_NETWORK"

	// ScenarioCollusion15 simulates a network where 15% of validators
	// coordinate their votes regardless of evidence.
	ScenarioCollusion15 ScenarioName = "COLLUSION_15"

	// ScenarioCollusion30 simulates a network where 30% of validators
	// collude. Tests the protocol's resistance near the theoretical limit.
	ScenarioCollusion30 ScenarioName = "COLLUSION_30"

	// ScenarioHighBias simulates a network with a concentrated bias
	// distribution — a large fraction of validators hold systematic
	// verdict preferences in the test domain.
	ScenarioHighBias ScenarioName = "HIGH_BIAS"
)

// Config carries all parameters needed to run a simulation. It is immutable
// after construction.
type Config struct {
	// Scenario identifies which predefined scenario this config represents.
	Scenario ScenarioName

	// ValidatorPoolSize is the total number of validators in the synthetic
	// pool. Must be >= MinValidatorPoolSize.
	ValidatorPoolSize int

	// ValidatorSetSize is the number of validators selected per claim.
	// Must be >= MinValidatorSetSize and <= ValidatorPoolSize.
	ValidatorSetSize int

	// Rounds is the number of synthetic validation rounds to simulate.
	// Must be >= MinRounds.
	Rounds int

	// CollusionRate is the fraction of validators that collude [0.0, 1.0].
	// Colluding validators vote for a fixed verdict regardless of the
	// ground truth. 0.0 means no collusion.
	CollusionRate float64

	// CollusionVerdict is the verdict that colluding validators always vote.
	// Only meaningful when CollusionRate > 0.
	CollusionVerdict string

	// BiasRate is the fraction of validators that hold a systematic bias
	// in the simulation domain [0.0, 1.0]. 0.0 means no biased validators.
	BiasRate float64

	// BiasStrength is the magnitude of bias for biased validators. It
	// represents the systematic deviation applied to their verdict scores
	// [0.0, 1.0]. 0.5 means biased validators skew their verdicts by 50%
	// of the maximum possible deviation toward a fixed direction.
	BiasStrength float64

	// Domain is the domain tag used for all claims in this simulation run.
	// Used to initialise validator domain reputations and track bias.
	Domain string

	// Seed is the random seed for reproducible simulation runs.
	// If zero, a non-deterministic seed is used.
	Seed int64
}

// Simulation size constraints.
const (
	MinValidatorPoolSize = 11
	MinValidatorSetSize  = 3
	MinRounds            = 1
)

// Validate checks that all Config fields are within acceptable ranges.
// Returns the first validation error encountered.
func (c Config) Validate() error {
	if c.ValidatorPoolSize < MinValidatorPoolSize {
		return fmt.Errorf("pulsesyn/simulation: validator_pool_size %d must be >= %d",
			c.ValidatorPoolSize, MinValidatorPoolSize)
	}
	if c.ValidatorSetSize < MinValidatorSetSize {
		return fmt.Errorf("pulsesyn/simulation: validator_set_size %d must be >= %d",
			c.ValidatorSetSize, MinValidatorSetSize)
	}
	if c.ValidatorSetSize > c.ValidatorPoolSize {
		return errors.New("pulsesyn/simulation: validator_set_size cannot exceed validator_pool_size")
	}
	if c.Rounds < MinRounds {
		return fmt.Errorf("pulsesyn/simulation: rounds %d must be >= %d", c.Rounds, MinRounds)
	}
	if c.CollusionRate < 0.0 || c.CollusionRate > 1.0 {
		return fmt.Errorf("pulsesyn/simulation: collusion_rate %f out of range [0.0, 1.0]", c.CollusionRate)
	}
	if c.BiasRate < 0.0 || c.BiasRate > 1.0 {
		return fmt.Errorf("pulsesyn/simulation: bias_rate %f out of range [0.0, 1.0]", c.BiasRate)
	}
	if c.BiasStrength < 0.0 || c.BiasStrength > 1.0 {
		return fmt.Errorf("pulsesyn/simulation: bias_strength %f out of range [0.0, 1.0]", c.BiasStrength)
	}
	if c.Domain == "" {
		return errors.New("pulsesyn/simulation: domain is required")
	}
	if c.CollusionRate > 0 {
		if err := validateVerdictString(c.CollusionVerdict); err != nil {
			return fmt.Errorf("pulsesyn/simulation: collusion_verdict: %w", err)
		}
	}
	return nil
}

// DefaultScenarioConfig returns a pre-populated Config for one of the four
// standard simulation scenarios. Callers may override individual fields
// before passing to Run.
func DefaultScenarioConfig(scenario ScenarioName) Config {
	base := Config{
		Scenario:         scenario,
		ValidatorPoolSize: 100,
		ValidatorSetSize:  11,
		Rounds:           1000,
		Domain:           "science",
		Seed:             42,
	}

	switch scenario {
	case ScenarioHonestNetwork:
		base.CollusionRate = 0.0
		base.BiasRate = 0.0
		base.BiasStrength = 0.0
	case ScenarioCollusion15:
		base.CollusionRate = 0.15
		base.CollusionVerdict = "SUPPORTED"
		base.BiasRate = 0.0
		base.BiasStrength = 0.0
	case ScenarioCollusion30:
		base.CollusionRate = 0.30
		base.CollusionVerdict = "SUPPORTED"
		base.BiasRate = 0.0
		base.BiasStrength = 0.0
	case ScenarioHighBias:
		base.CollusionRate = 0.0
		base.BiasRate = 0.40
		base.BiasStrength = 0.75
	}

	return base
}

// validateVerdictString checks that s is a recognised protocol verdict.
func validateVerdictString(s string) error {
	switch s {
	case "SUPPORTED", "UNSUPPORTED", "MISLEADING", "INDETERMINATE":
		return nil
	default:
		return fmt.Errorf("unrecognised verdict %q", s)
	}
}
