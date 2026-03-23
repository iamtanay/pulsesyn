package simulation

import (
	"fmt"
	"math/rand"

	"github.com/iamtanay/pulsesyn/core/bias"
)

// Run executes a full simulation run for the given Config and returns a Report.
// It validates the config, builds a synthetic validator pool, runs all rounds,
// applies reputation updates, tracks bias observations, and finalises metrics.
//
// Run is the package's primary entry point. All other exported types exist to
// support configuring and interpreting Run's output.
//
// Returns an error if the config is invalid or if an internal protocol
// operation fails. Under normal conditions internal errors should not occur —
// if they do, they indicate a bug in the core packages, not in simulation input.
func Run(cfg Config) (Report, error) {
	if err := cfg.Validate(); err != nil {
		return Report{}, fmt.Errorf("simulation.Run: %w", err)
	}

	// Seed the RNG. A zero seed in Config requests non-deterministic behaviour,
	// but rand.New(rand.NewSource(0)) is still deterministic in Go's stdlib.
	// For simulation purposes, seed 0 and non-zero are both acceptable.
	rng := rand.New(rand.NewSource(cfg.Seed))

	// Build the validator pool.
	pool, err := buildValidatorPool(cfg, rng)
	if err != nil {
		return Report{}, fmt.Errorf("simulation.Run: %w", err)
	}

	// Initialise the bias tracker with the default window size.
	tracker, err := bias.NewTracker(0)
	if err != nil {
		return Report{}, fmt.Errorf("simulation.Run: %w", err)
	}

	report := newReport(cfg)

	// Run all rounds.
	for round := 0; round < cfg.Rounds; round++ {
		result, err := simulateRound(cfg, pool, tracker, rng, round)
		if err != nil {
			return Report{}, fmt.Errorf("simulation.Run: round %d: %w", round, err)
		}
		report.accumulate(result)
	}

	// Finalise derived metrics.
	trackerRef := &biasTrackerRef{
		countBiasedValidators: func(p []*simValidator, domain string) int {
			count := 0
			for _, sv := range p {
				result := tracker.BiasFor(sv.record.ValidatorID, domain)
				if result.Coefficient >= bias.BiasModerateThreshold {
					count++
				}
			}
			return count
		},
	}
	report.finalise(pool, trackerRef)

	return report, nil
}

// RunScenarios executes all four standard simulation scenarios in sequence
// and returns one Report per scenario. This is the convenience entry point
// for a complete Phase 1 validation run.
func RunScenarios(rounds int) ([]Report, error) {
	scenarios := []ScenarioName{
		ScenarioHonestNetwork,
		ScenarioCollusion15,
		ScenarioCollusion30,
		ScenarioHighBias,
	}

	reports := make([]Report, 0, len(scenarios))
	for _, scenario := range scenarios {
		cfg := DefaultScenarioConfig(scenario)
		cfg.Rounds = rounds

		report, err := Run(cfg)
		if err != nil {
			return nil, fmt.Errorf("RunScenarios: %s: %w", scenario, err)
		}
		reports = append(reports, report)
	}
	return reports, nil
}
