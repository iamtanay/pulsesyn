package simulation

import "fmt"

// Report holds the aggregated results of a completed simulation run.
// It is the primary output of Run and is used for both automated assertion
// in tests and for human-readable reporting.
type Report struct {
	// Scenario is the name of the scenario that was run.
	Scenario ScenarioName

	// Config is the configuration used for this run.
	Config Config

	// TotalRounds is the number of rounds simulated.
	TotalRounds int

	// CorrectRounds is the number of decided (non-INDETERMINATE) rounds where
	// consensus matched ground truth. INDETERMINATE rounds are never counted
	// here regardless of ground truth.
	CorrectRounds int

	// IndeterminateRounds is the number of rounds that produced INDETERMINATE.
	IndeterminateRounds int

	// VerdictAccuracy is CorrectRounds / (TotalRounds - IndeterminateRounds).
	// Indeterminate verdicts are excluded because they represent genuine
	// protocol outcomes, not errors. Always in [0.0, 1.0].
	VerdictAccuracy float64

	// IndeterminateRate is the fraction of rounds that produced INDETERMINATE.
	IndeterminateRate float64

	// MeanParticipationRate is the average fraction of validators who voted
	// across all rounds.
	MeanParticipationRate float64

	// BiasDetectionRate is the fraction of rounds in which at least one
	// validator's bias crossed the moderate threshold.
	BiasDetectionRate float64

	// BiasedValidatorsDetected is the count of distinct validators who were
	// flagged for bias (>= BiasModerateThreshold) at least once during the run.
	BiasedValidatorsDetected int

	// VerdictDistribution records how many rounds produced each verdict.
	VerdictDistribution map[string]int

	// ReputationConvergence holds the mean domain reputation across all
	// validators at the end of the run. Higher values indicate the pool
	// has been calibrated by the update system.
	ReputationConvergence float64
}

// newReport constructs an empty Report for the given config.
func newReport(cfg Config) Report {
	return Report{
		Scenario:            cfg.Scenario,
		Config:              cfg,
		VerdictDistribution: make(map[string]int),
	}
}

// accumulate incorporates a single round result into the report totals.
func (r *Report) accumulate(result roundResult) {
	r.TotalRounds++
	r.VerdictDistribution[result.consensusVerdict]++

	if result.consensusVerdict == "INDETERMINATE" {
		r.IndeterminateRounds++
		// INDETERMINATE is never counted as correct. It is a protocol
		// non-decision — the consensus engine could not reach a verdict.
		// Counting an INDETERMINATE match as correct would inflate
		// CorrectRounds above decidedRounds, making VerdictAccuracy > 1.0.
	} else if result.correct {
		// Only increment for decided (non-INDETERMINATE) rounds.
		r.CorrectRounds++
	}

	r.MeanParticipationRate += result.participationRate
	if result.biasDetectionEvents > 0 {
		r.BiasDetectionRate++
	}
}

// finalise computes derived metrics after all rounds have been accumulated.
func (r *Report) finalise(pool []*simValidator, tracker *biasTrackerRef) {
	if r.TotalRounds == 0 {
		return
	}

	r.MeanParticipationRate /= float64(r.TotalRounds)
	r.BiasDetectionRate /= float64(r.TotalRounds)

	decidedRounds := r.TotalRounds - r.IndeterminateRounds
	if decidedRounds > 0 {
		r.VerdictAccuracy = float64(r.CorrectRounds) / float64(decidedRounds)
	}

	r.IndeterminateRate = float64(r.IndeterminateRounds) / float64(r.TotalRounds)

	// Reputation convergence: mean domain reputation across all validators.
	var repSum float64
	for _, sv := range pool {
		repSum += sv.record.DomainScore(r.Config.Domain)
	}
	if len(pool) > 0 {
		r.ReputationConvergence = repSum / float64(len(pool))
	}

	// Biased validator detection count.
	if tracker != nil {
		r.BiasedValidatorsDetected = tracker.countBiasedValidators(pool, r.Config.Domain)
	}
}

// Summary returns a human-readable multi-line string summarising the report.
func (r Report) Summary() string {
	return fmt.Sprintf(
		"Scenario: %s\n"+
			"Rounds: %d | Correct: %d | Indeterminate: %d\n"+
			"Verdict Accuracy (excl. INDETERMINATE): %.2f%%\n"+
			"Indeterminate Rate: %.2f%%\n"+
			"Mean Participation Rate: %.2f%%\n"+
			"Bias Detection Rate: %.2f%%\n"+
			"Biased Validators Detected: %d\n"+
			"Reputation Convergence (mean domain rep): %.4f\n"+
			"Verdict Distribution: SUPPORTED=%d UNSUPPORTED=%d MISLEADING=%d INDETERMINATE=%d",
		r.Scenario,
		r.TotalRounds, r.CorrectRounds, r.IndeterminateRounds,
		r.VerdictAccuracy*100,
		r.IndeterminateRate*100,
		r.MeanParticipationRate*100,
		r.BiasDetectionRate*100,
		r.BiasedValidatorsDetected,
		r.ReputationConvergence,
		r.VerdictDistribution["SUPPORTED"],
		r.VerdictDistribution["UNSUPPORTED"],
		r.VerdictDistribution["MISLEADING"],
		r.VerdictDistribution["INDETERMINATE"],
	)
}

// biasTrackerRef is a thin wrapper used to pass the Tracker to finalise
// without importing the bias package in report.go's top-level. It calls back
// into the Tracker via a closure, keeping report.go free of direct bias
// package references for cleaner layering within the simulation package.
type biasTrackerRef struct {
	countBiasedValidators func(pool []*simValidator, domain string) int
}
