package simulation

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Config.Validate
// ---------------------------------------------------------------------------

func TestConfigValidate_ValidConfig_NoError(t *testing.T) {
	cfg := DefaultScenarioConfig(ScenarioHonestNetwork)
	require.NoError(t, cfg.Validate())
}

func TestConfigValidate_PoolSizeTooSmall_ReturnsError(t *testing.T) {
	cfg := DefaultScenarioConfig(ScenarioHonestNetwork)
	cfg.ValidatorPoolSize = MinValidatorPoolSize - 1
	assert.Error(t, cfg.Validate())
}

func TestConfigValidate_SetSizeTooSmall_ReturnsError(t *testing.T) {
	cfg := DefaultScenarioConfig(ScenarioHonestNetwork)
	cfg.ValidatorSetSize = MinValidatorSetSize - 1
	assert.Error(t, cfg.Validate())
}

func TestConfigValidate_SetSizeExceedsPool_ReturnsError(t *testing.T) {
	cfg := DefaultScenarioConfig(ScenarioHonestNetwork)
	cfg.ValidatorSetSize = cfg.ValidatorPoolSize + 1
	assert.Error(t, cfg.Validate())
}

func TestConfigValidate_RoundsTooSmall_ReturnsError(t *testing.T) {
	cfg := DefaultScenarioConfig(ScenarioHonestNetwork)
	cfg.Rounds = 0
	assert.Error(t, cfg.Validate())
}

func TestConfigValidate_CollusionRateOutOfRange_ReturnsError(t *testing.T) {
	tests := []struct {
		name string
		rate float64
	}{
		{"negative", -0.01},
		{"above one", 1.01},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultScenarioConfig(ScenarioHonestNetwork)
			cfg.CollusionRate = tt.rate
			assert.Error(t, cfg.Validate())
		})
	}
}

func TestConfigValidate_BiasRateOutOfRange_ReturnsError(t *testing.T) {
	cfg := DefaultScenarioConfig(ScenarioHonestNetwork)
	cfg.BiasRate = 1.5
	assert.Error(t, cfg.Validate())
}

func TestConfigValidate_BiasStrengthOutOfRange_ReturnsError(t *testing.T) {
	cfg := DefaultScenarioConfig(ScenarioHonestNetwork)
	cfg.BiasStrength = -0.1
	assert.Error(t, cfg.Validate())
}

func TestConfigValidate_EmptyDomain_ReturnsError(t *testing.T) {
	cfg := DefaultScenarioConfig(ScenarioHonestNetwork)
	cfg.Domain = ""
	assert.Error(t, cfg.Validate())
}

func TestConfigValidate_CollusionRateSetWithoutVerdict_ReturnsError(t *testing.T) {
	cfg := DefaultScenarioConfig(ScenarioHonestNetwork)
	cfg.CollusionRate = 0.15
	cfg.CollusionVerdict = "" // missing
	assert.Error(t, cfg.Validate())
}

func TestConfigValidate_InvalidCollusionVerdict_ReturnsError(t *testing.T) {
	cfg := DefaultScenarioConfig(ScenarioHonestNetwork)
	cfg.CollusionRate = 0.10
	cfg.CollusionVerdict = "MAYBE"
	assert.Error(t, cfg.Validate())
}

// ---------------------------------------------------------------------------
// DefaultScenarioConfig
// ---------------------------------------------------------------------------

func TestDefaultScenarioConfig_AllScenariosValidate(t *testing.T) {
	scenarios := []ScenarioName{
		ScenarioHonestNetwork,
		ScenarioCollusion15,
		ScenarioCollusion30,
		ScenarioHighBias,
	}
	for _, s := range scenarios {
		t.Run(string(s), func(t *testing.T) {
			cfg := DefaultScenarioConfig(s)
			require.NoError(t, cfg.Validate())
		})
	}
}

func TestDefaultScenarioConfig_Collusion15_CollusionRateSet(t *testing.T) {
	cfg := DefaultScenarioConfig(ScenarioCollusion15)
	assert.InDelta(t, 0.15, cfg.CollusionRate, 1e-9)
	assert.Equal(t, "SUPPORTED", cfg.CollusionVerdict)
}

func TestDefaultScenarioConfig_Collusion30_CollusionRateSet(t *testing.T) {
	cfg := DefaultScenarioConfig(ScenarioCollusion30)
	assert.InDelta(t, 0.30, cfg.CollusionRate, 1e-9)
}

func TestDefaultScenarioConfig_HighBias_BiasRateSet(t *testing.T) {
	cfg := DefaultScenarioConfig(ScenarioHighBias)
	assert.Greater(t, cfg.BiasRate, 0.0)
	assert.Greater(t, cfg.BiasStrength, 0.0)
}

// ---------------------------------------------------------------------------
// Run — structural correctness
// ---------------------------------------------------------------------------

func TestRun_InvalidConfig_ReturnsError(t *testing.T) {
	cfg := Config{} // zero value — missing required fields
	_, err := Run(cfg)
	require.Error(t, err)
}

func TestRun_SingleRound_ReturnsReport(t *testing.T) {
	cfg := DefaultScenarioConfig(ScenarioHonestNetwork)
	cfg.Rounds = 1
	report, err := Run(cfg)
	require.NoError(t, err)
	assert.Equal(t, 1, report.TotalRounds)
}

func TestRun_ReportTotalRoundsMatchesConfig(t *testing.T) {
	cfg := DefaultScenarioConfig(ScenarioHonestNetwork)
	cfg.Rounds = 50
	report, err := Run(cfg)
	require.NoError(t, err)
	assert.Equal(t, 50, report.TotalRounds)
}

func TestRun_VerdictDistributionSumsToTotalRounds(t *testing.T) {
	cfg := DefaultScenarioConfig(ScenarioHonestNetwork)
	cfg.Rounds = 100
	report, err := Run(cfg)
	require.NoError(t, err)

	var total int
	for _, count := range report.VerdictDistribution {
		total += count
	}
	assert.Equal(t, report.TotalRounds, total)
}

func TestRun_CorrectRoundsNeverExceedsTotal(t *testing.T) {
	cfg := DefaultScenarioConfig(ScenarioHonestNetwork)
	cfg.Rounds = 100
	report, err := Run(cfg)
	require.NoError(t, err)
	assert.LessOrEqual(t, report.CorrectRounds, report.TotalRounds)
}

func TestRun_IndeterminateRoundsNeverExceedsTotal(t *testing.T) {
	cfg := DefaultScenarioConfig(ScenarioHonestNetwork)
	cfg.Rounds = 100
	report, err := Run(cfg)
	require.NoError(t, err)
	assert.LessOrEqual(t, report.IndeterminateRounds, report.TotalRounds)
}

func TestRun_MeanParticipationRateInRange(t *testing.T) {
	cfg := DefaultScenarioConfig(ScenarioHonestNetwork)
	cfg.Rounds = 100
	report, err := Run(cfg)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, report.MeanParticipationRate, 0.0)
	assert.LessOrEqual(t, report.MeanParticipationRate, 1.0)
}

func TestRun_VerdictAccuracyInRange(t *testing.T) {
	cfg := DefaultScenarioConfig(ScenarioHonestNetwork)
	cfg.Rounds = 100
	report, err := Run(cfg)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, report.VerdictAccuracy, 0.0)
	assert.LessOrEqual(t, report.VerdictAccuracy, 1.0)
}

func TestRun_ReputationConvergencePositive(t *testing.T) {
	cfg := DefaultScenarioConfig(ScenarioHonestNetwork)
	cfg.Rounds = 100
	report, err := Run(cfg)
	require.NoError(t, err)
	// Validators start above EligibilityThreshold and update through rounds.
	assert.Greater(t, report.ReputationConvergence, 0.0)
}

func TestRun_Deterministic_SameSeedProducesSameReport(t *testing.T) {
	cfg := DefaultScenarioConfig(ScenarioHonestNetwork)
	cfg.Rounds = 200
	cfg.Seed = 99

	report1, err := Run(cfg)
	require.NoError(t, err)

	report2, err := Run(cfg)
	require.NoError(t, err)

	assert.Equal(t, report1.CorrectRounds, report2.CorrectRounds)
	assert.Equal(t, report1.IndeterminateRounds, report2.IndeterminateRounds)
	assert.InDelta(t, report1.VerdictAccuracy, report2.VerdictAccuracy, 1e-9)
}

// ---------------------------------------------------------------------------
// Scenario: honest network accuracy
// ---------------------------------------------------------------------------

func TestRun_HonestNetwork_HighAccuracy(t *testing.T) {
	// An honest network with no collusion and no bias should converge to
	// high accuracy. The threshold of 0.70 is deliberately conservative —
	// the protocol produces INDETERMINATE for some rounds, and excluding
	// those, honest consensus should be well above 70%.
	cfg := DefaultScenarioConfig(ScenarioHonestNetwork)
	cfg.Rounds = 500

	report, err := Run(cfg)
	require.NoError(t, err)

	assert.GreaterOrEqual(t, report.VerdictAccuracy, 0.70,
		"honest network accuracy below threshold: %s", report.Summary())
}

// ---------------------------------------------------------------------------
// Scenario: collusion resistance
// ---------------------------------------------------------------------------

func TestRun_Collusion15_AccuracyAboveBaseline(t *testing.T) {
	// With 15% collusion the protocol should still produce correct verdicts
	// more often than not. Accuracy >= 0.60 is the minimum acceptable bar.
	cfg := DefaultScenarioConfig(ScenarioCollusion15)
	cfg.Rounds = 500

	report, err := Run(cfg)
	require.NoError(t, err)

	assert.GreaterOrEqual(t, report.VerdictAccuracy, 0.60,
		"15%% collusion scenario accuracy too low: %s", report.Summary())
}

func TestRun_Collusion30_StillProducesVerdicts(t *testing.T) {
	// 30% collusion is near the theoretical limit. The protocol may produce
	// more INDETERMINATE verdicts but should not collapse entirely.
	// We only assert that the run completes and produces some correct verdicts.
	cfg := DefaultScenarioConfig(ScenarioCollusion30)
	cfg.Rounds = 500

	report, err := Run(cfg)
	require.NoError(t, err)

	// At 30% collusion the simulation should still produce at least some
	// non-indeterminate correct verdicts.
	assert.Greater(t, report.CorrectRounds, 0,
		"30%% collusion scenario produced zero correct rounds: %s", report.Summary())
}

// ---------------------------------------------------------------------------
// Scenario: bias detection
// ---------------------------------------------------------------------------

func TestRun_HighBias_DetectsBiasedValidators(t *testing.T) {
	// A network with 40% biased validators at 75% bias strength should
	// cause the bias tracker to flag a non-trivial number of validators.
	cfg := DefaultScenarioConfig(ScenarioHighBias)
	cfg.Rounds = 500

	report, err := Run(cfg)
	require.NoError(t, err)

	// Expect that some biased validators have been detected.
	assert.Greater(t, report.BiasedValidatorsDetected, 0,
		"high bias scenario detected no biased validators: %s", report.Summary())
}

func TestRun_HonestNetwork_LowBiasDetection(t *testing.T) {
	// An honest network with no systematic bias should have a low bias
	// detection rate. Some false positives are expected due to statistical
	// variance — the threshold of 0.30 is deliberately generous.
	cfg := DefaultScenarioConfig(ScenarioHonestNetwork)
	cfg.Rounds = 500

	report, err := Run(cfg)
	require.NoError(t, err)

	assert.LessOrEqual(t, report.BiasDetectionRate, 0.30,
		"honest network has unexpectedly high bias detection rate: %s", report.Summary())
}

// ---------------------------------------------------------------------------
// RunScenarios
// ---------------------------------------------------------------------------

func TestRunScenarios_ReturnsAllFourReports(t *testing.T) {
	reports, err := RunScenarios(50) // low round count for speed
	require.NoError(t, err)
	assert.Len(t, reports, 4)
}

func TestRunScenarios_AllReportsHaveExpectedRounds(t *testing.T) {
	const rounds = 30
	reports, err := RunScenarios(rounds)
	require.NoError(t, err)

	for _, r := range reports {
		assert.Equal(t, rounds, r.TotalRounds,
			"scenario %s has wrong round count", r.Scenario)
	}
}

func TestRunScenarios_ScenariosInOrder(t *testing.T) {
	reports, err := RunScenarios(10)
	require.NoError(t, err)

	expected := []ScenarioName{
		ScenarioHonestNetwork,
		ScenarioCollusion15,
		ScenarioCollusion30,
		ScenarioHighBias,
	}
	for i, r := range reports {
		assert.Equal(t, expected[i], r.Scenario)
	}
}

// ---------------------------------------------------------------------------
// Report.Summary
// ---------------------------------------------------------------------------

func TestReport_Summary_NonEmptyString(t *testing.T) {
	cfg := DefaultScenarioConfig(ScenarioHonestNetwork)
	cfg.Rounds = 10
	report, err := Run(cfg)
	require.NoError(t, err)
	assert.NotEmpty(t, report.Summary())
}
