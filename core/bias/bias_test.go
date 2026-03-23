package bias

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Test fixtures
// ---------------------------------------------------------------------------

// makeObs builds a ValidationObservation for use in tests.
func makeObs(claimID, verdict string, populationAvg float64) ValidationObservation {
	return ValidationObservation{
		ClaimID:                claimID,
		ValidatorVerdict:       verdict,
		PopulationAverageScore: populationAvg,
	}
}

// fillWindow adds n identical observations to a window. Used to reach the
// minimum observation count quickly in tests that focus on tier classification.
func fillWindow(t *testing.T, w *Window, n int, verdict string, populationAvg float64) {
	t.Helper()
	for i := 0; i < n; i++ {
		obs := makeObs(fmt.Sprintf("claim-%d", i), verdict, populationAvg)
		require.NoError(t, w.Add(obs))
	}
}

// ---------------------------------------------------------------------------
// NewWindow
// ---------------------------------------------------------------------------

func TestNewWindow_ValidInput_ReturnsWindow(t *testing.T) {
	w, err := NewWindow(WindowConfig{ValidatorID: "v1", Domain: "science"})
	require.NoError(t, err)
	assert.Equal(t, "v1", w.ValidatorID())
	assert.Equal(t, "science", w.Domain())
	assert.Equal(t, DefaultWindowSize, w.maxSize)
	assert.Equal(t, 0, w.Len())
}

func TestNewWindow_CustomMaxSize_Accepted(t *testing.T) {
	w, err := NewWindow(WindowConfig{ValidatorID: "v1", Domain: "health", MaxSize: 20})
	require.NoError(t, err)
	assert.Equal(t, 20, w.maxSize)
}

func TestNewWindow_EmptyValidatorID_ReturnsError(t *testing.T) {
	_, err := NewWindow(WindowConfig{ValidatorID: "", Domain: "health"})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrValidatorIDEmpty)
}

func TestNewWindow_WhitespaceValidatorID_ReturnsError(t *testing.T) {
	_, err := NewWindow(WindowConfig{ValidatorID: "   ", Domain: "health"})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrValidatorIDEmpty)
}

func TestNewWindow_EmptyDomain_ReturnsError(t *testing.T) {
	_, err := NewWindow(WindowConfig{ValidatorID: "v1", Domain: ""})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrDomainEmpty)
}

func TestNewWindow_MaxSizeTooSmall_ReturnsError(t *testing.T) {
	_, err := NewWindow(WindowConfig{ValidatorID: "v1", Domain: "health", MaxSize: 4})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidWindowSize)
}

func TestNewWindow_DomainNormalised_Lowercase(t *testing.T) {
	w, err := NewWindow(WindowConfig{ValidatorID: "v1", Domain: "HEALTH"})
	require.NoError(t, err)
	assert.Equal(t, "health", w.Domain())
}

// ---------------------------------------------------------------------------
// Window.Add
// ---------------------------------------------------------------------------

func TestWindow_Add_ValidObservation_Accepted(t *testing.T) {
	w, err := NewWindow(WindowConfig{ValidatorID: "v1", Domain: "science"})
	require.NoError(t, err)
	obs := makeObs("claim-1", "SUPPORTED", 0.8)
	require.NoError(t, w.Add(obs))
	assert.Equal(t, 1, w.Len())
}

func TestWindow_Add_EmptyClaimID_ReturnsError(t *testing.T) {
	w, err := NewWindow(WindowConfig{ValidatorID: "v1", Domain: "science"})
	require.NoError(t, err)
	obs := makeObs("", "SUPPORTED", 0.8)
	err = w.Add(obs)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrClaimIDEmpty)
}

func TestWindow_Add_InvalidVerdict_ReturnsError(t *testing.T) {
	w, err := NewWindow(WindowConfig{ValidatorID: "v1", Domain: "science"})
	require.NoError(t, err)
	obs := makeObs("claim-1", "UNKNOWN", 0.5)
	err = w.Add(obs)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidVerdictString)
}

func TestWindow_Add_PopulationScoreOutOfRange_ReturnsError(t *testing.T) {
	w, err := NewWindow(WindowConfig{ValidatorID: "v1", Domain: "science"})
	require.NoError(t, err)

	tests := []struct {
		name  string
		score float64
	}{
		{"negative score", -0.1},
		{"score above 1", 1.1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obs := makeObs("claim-1", "SUPPORTED", tt.score)
			err := w.Add(obs)
			require.Error(t, err)
		})
	}
}

func TestWindow_Add_AllFourVerdicts_Accepted(t *testing.T) {
	w, err := NewWindow(WindowConfig{ValidatorID: "v1", Domain: "science"})
	require.NoError(t, err)

	verdicts := []string{"SUPPORTED", "UNSUPPORTED", "MISLEADING", "INDETERMINATE"}
	for i, v := range verdicts {
		obs := makeObs(fmt.Sprintf("claim-%d", i), v, 0.5)
		require.NoError(t, w.Add(obs))
	}
	assert.Equal(t, 4, w.Len())
}

func TestWindow_Add_EvictsOldestWhenFull(t *testing.T) {
	w, err := NewWindow(WindowConfig{ValidatorID: "v1", Domain: "science", MaxSize: 5})
	require.NoError(t, err)

	for i := 0; i < 5; i++ {
		require.NoError(t, w.Add(makeObs(fmt.Sprintf("old-%d", i), "SUPPORTED", 1.0)))
	}
	assert.Equal(t, 5, w.Len())

	// Adding a sixth evicts the first.
	require.NoError(t, w.Add(makeObs("new-0", "UNSUPPORTED", 0.0)))
	assert.Equal(t, 5, w.Len())
	// First entry should now be the second original observation.
	assert.Equal(t, "old-1", w.entries[0].ClaimID)
	assert.Equal(t, "new-0", w.entries[4].ClaimID)
}

// ---------------------------------------------------------------------------
// Window.ComputeBias — insufficient data
// ---------------------------------------------------------------------------

func TestComputeBias_InsufficientObservations_ReturnsZeroCoefficient(t *testing.T) {
	w, err := NewWindow(WindowConfig{ValidatorID: "v1", Domain: "science"})
	require.NoError(t, err)

	// Add fewer than MinObservationsForBias entries.
	for i := 0; i < MinObservationsForBias-1; i++ {
		require.NoError(t, w.Add(makeObs(fmt.Sprintf("claim-%d", i), "SUPPORTED", 1.0)))
	}

	result := w.ComputeBias()
	assert.Equal(t, 0.0, result.Coefficient)
	assert.Equal(t, TierNegligible, result.Tier)
	assert.Equal(t, 1.0, result.CorrectionFactor)
	assert.Equal(t, MinObservationsForBias-1, result.ObservationCount)
}

func TestComputeBias_ExactlyMinObservations_ComputesCoefficient(t *testing.T) {
	w, err := NewWindow(WindowConfig{ValidatorID: "v1", Domain: "science"})
	require.NoError(t, err)

	// Fill with exactly minimum observations, all perfectly aligned with
	// population average (no bias).
	for i := 0; i < MinObservationsForBias; i++ {
		// Validator always votes SUPPORTED (score=1.0) and population avg=1.0.
		require.NoError(t, w.Add(makeObs(fmt.Sprintf("claim-%d", i), "SUPPORTED", 1.0)))
	}

	result := w.ComputeBias()
	assert.Equal(t, MinObservationsForBias, result.ObservationCount)
	// No deviation → coefficient should be 0.
	assert.Equal(t, 0.0, result.Coefficient)
}

// ---------------------------------------------------------------------------
// Window.ComputeBias — coefficient and tier computation
// ---------------------------------------------------------------------------

func TestComputeBias_NoBias_ZeroCoefficient(t *testing.T) {
	w, err := NewWindow(WindowConfig{ValidatorID: "v1", Domain: "science"})
	require.NoError(t, err)

	// Validator always agrees exactly with the population average.
	// score("SUPPORTED") = 1.0, populationAvg = 1.0 → deviation = 0.
	for i := 0; i < 10; i++ {
		require.NoError(t, w.Add(makeObs(fmt.Sprintf("c-%d", i), "SUPPORTED", 1.0)))
	}

	result := w.ComputeBias()
	assert.Equal(t, 0.0, result.Coefficient)
	assert.Equal(t, TierNegligible, result.Tier)
	assert.Equal(t, 1.0, result.CorrectionFactor)
}

func TestComputeBias_MaxBias_CoefficientOne(t *testing.T) {
	w, err := NewWindow(WindowConfig{ValidatorID: "v1", Domain: "science"})
	require.NoError(t, err)

	// Validator always votes SUPPORTED (score=1.0) while population average
	// is 0.0 (everyone else says UNSUPPORTED). Deviation = 1.0 per observation.
	for i := 0; i < 10; i++ {
		require.NoError(t, w.Add(makeObs(fmt.Sprintf("c-%d", i), "SUPPORTED", 0.0)))
	}

	result := w.ComputeBias()
	assert.InDelta(t, 1.0, result.Coefficient, 1e-9)
	assert.Equal(t, TierSevere, result.Tier)
	assert.InDelta(t, 0.0, result.CorrectionFactor, 1e-9)
}

func TestComputeBias_ModerateDeviation_CorrectTier(t *testing.T) {
	w, err := NewWindow(WindowConfig{ValidatorID: "v1", Domain: "science"})
	require.NoError(t, err)

	// Validator votes SUPPORTED (score=1.0), population average is 0.6.
	// Deviation = 0.4 per observation. Mean deviation = 0.4.
	// Expected tier: MODERATE (0.30 <= 0.4 < 0.50).
	for i := 0; i < 10; i++ {
		require.NoError(t, w.Add(makeObs(fmt.Sprintf("c-%d", i), "SUPPORTED", 0.6)))
	}

	result := w.ComputeBias()
	assert.InDelta(t, 0.4, result.Coefficient, 1e-9)
	assert.Equal(t, TierModerate, result.Tier)
	assert.InDelta(t, 0.6, result.CorrectionFactor, 1e-9)
}

func TestComputeBias_HighDeviation_CorrectTier(t *testing.T) {
	w, err := NewWindow(WindowConfig{ValidatorID: "v1", Domain: "science"})
	require.NoError(t, err)

	// Validator votes SUPPORTED (1.0), population average 0.4.
	// Deviation = 0.6. Tier: HIGH (0.50 <= 0.6 < 0.70).
	for i := 0; i < 10; i++ {
		require.NoError(t, w.Add(makeObs(fmt.Sprintf("c-%d", i), "SUPPORTED", 0.4)))
	}

	result := w.ComputeBias()
	assert.InDelta(t, 0.6, result.Coefficient, 1e-9)
	assert.Equal(t, TierHigh, result.Tier)
}

func TestComputeBias_SevereDeviation_CorrectTier(t *testing.T) {
	w, err := NewWindow(WindowConfig{ValidatorID: "v1", Domain: "science"})
	require.NoError(t, err)

	// Validator votes SUPPORTED (1.0), population average 0.25.
	// Deviation = 0.75. Tier: SEVERE (>= 0.70).
	for i := 0; i < 10; i++ {
		require.NoError(t, w.Add(makeObs(fmt.Sprintf("c-%d", i), "SUPPORTED", 0.25)))
	}

	result := w.ComputeBias()
	assert.InDelta(t, 0.75, result.Coefficient, 1e-9)
	assert.Equal(t, TierSevere, result.Tier)
}

func TestComputeBias_NegligibleDeviation_CorrectTier(t *testing.T) {
	w, err := NewWindow(WindowConfig{ValidatorID: "v1", Domain: "science"})
	require.NoError(t, err)

	// Validator votes SUPPORTED (1.0), population average 0.8.
	// Deviation = 0.2. Tier: NEGLIGIBLE (< 0.30).
	for i := 0; i < 10; i++ {
		require.NoError(t, w.Add(makeObs(fmt.Sprintf("c-%d", i), "SUPPORTED", 0.8)))
	}

	result := w.ComputeBias()
	assert.InDelta(t, 0.2, result.Coefficient, 1e-9)
	assert.Equal(t, TierNegligible, result.Tier)
}

func TestComputeBias_MixedDeviations_CorrectMean(t *testing.T) {
	w, err := NewWindow(WindowConfig{ValidatorID: "v1", Domain: "science"})
	require.NoError(t, err)

	// 5 observations deviating by 0.4, 5 deviating by 0.0.
	// Expected mean deviation = (5*0.4 + 5*0.0) / 10 = 0.2.
	for i := 0; i < 5; i++ {
		require.NoError(t, w.Add(makeObs(fmt.Sprintf("c-a-%d", i), "SUPPORTED", 0.6)))
	}
	for i := 0; i < 5; i++ {
		require.NoError(t, w.Add(makeObs(fmt.Sprintf("c-b-%d", i), "SUPPORTED", 1.0)))
	}

	result := w.ComputeBias()
	assert.InDelta(t, 0.2, result.Coefficient, 1e-9)
	assert.Equal(t, TierNegligible, result.Tier)
}

func TestComputeBias_UnsupportedVerdict_ScoredAsZero(t *testing.T) {
	w, err := NewWindow(WindowConfig{ValidatorID: "v1", Domain: "science"})
	require.NoError(t, err)

	// UNSUPPORTED has score 0.0. Population average 0.0 → no deviation.
	for i := 0; i < 10; i++ {
		require.NoError(t, w.Add(makeObs(fmt.Sprintf("c-%d", i), "UNSUPPORTED", 0.0)))
	}

	result := w.ComputeBias()
	assert.Equal(t, 0.0, result.Coefficient)
}

func TestComputeBias_MisleadingVerdict_ScoredAsHalf(t *testing.T) {
	w, err := NewWindow(WindowConfig{ValidatorID: "v1", Domain: "science"})
	require.NoError(t, err)

	// MISLEADING has score 0.5. Population average 0.5 → no deviation.
	for i := 0; i < 10; i++ {
		require.NoError(t, w.Add(makeObs(fmt.Sprintf("c-%d", i), "MISLEADING", 0.5)))
	}

	result := w.ComputeBias()
	assert.Equal(t, 0.0, result.Coefficient)
}

func TestComputeBias_ResultFields_Populated(t *testing.T) {
	w, err := NewWindow(WindowConfig{ValidatorID: "validator-7", Domain: "health"})
	require.NoError(t, err)

	for i := 0; i < 8; i++ {
		require.NoError(t, w.Add(makeObs(fmt.Sprintf("c-%d", i), "SUPPORTED", 0.5)))
	}

	result := w.ComputeBias()
	assert.Equal(t, "validator-7", result.ValidatorID)
	assert.Equal(t, "health", result.Domain)
	assert.Equal(t, 8, result.ObservationCount)
	assert.InDelta(t, result.Coefficient+result.CorrectionFactor, 1.0, 1e-9)
}

// ---------------------------------------------------------------------------
// Tier boundary conditions
// ---------------------------------------------------------------------------

func TestClassifyTier_ExactBoundaries(t *testing.T) {
	tests := []struct {
		coefficient float64
		wantTier    ResponseTier
	}{
		{0.0, TierNegligible},
		{0.299, TierNegligible},
		{0.30, TierModerate},
		{0.499, TierModerate},
		{0.50, TierHigh},
		{0.699, TierHigh},
		{0.70, TierSevere},
		{1.0, TierSevere},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("coefficient_%.3f", tt.coefficient), func(t *testing.T) {
			assert.Equal(t, tt.wantTier, classifyTier(tt.coefficient))
		})
	}
}

// ---------------------------------------------------------------------------
// IsEligibleForDomain
// ---------------------------------------------------------------------------

func TestIsEligibleForDomain_NegligibleBias_Eligible(t *testing.T) {
	result := BiasResult{Coefficient: 0.2, Tier: TierNegligible}
	assert.True(t, IsEligibleForDomain(result))
}

func TestIsEligibleForDomain_ModerateBias_Eligible(t *testing.T) {
	result := BiasResult{Coefficient: 0.4, Tier: TierModerate}
	assert.True(t, IsEligibleForDomain(result))
}

func TestIsEligibleForDomain_HighBias_NotEligible(t *testing.T) {
	result := BiasResult{Coefficient: 0.6, Tier: TierHigh}
	assert.False(t, IsEligibleForDomain(result))
}

func TestIsEligibleForDomain_SevereBias_NotEligible(t *testing.T) {
	result := BiasResult{Coefficient: 0.8, Tier: TierSevere}
	assert.False(t, IsEligibleForDomain(result))
}

func TestIsEligibleForDomain_ExactHighThreshold_NotEligible(t *testing.T) {
	// At exactly BiasHighThreshold the validator should NOT be eligible.
	result := BiasResult{Coefficient: BiasHighThreshold}
	assert.False(t, IsEligibleForDomain(result))
}

func TestIsEligibleForDomain_JustBelowHighThreshold_Eligible(t *testing.T) {
	result := BiasResult{Coefficient: BiasHighThreshold - 0.001}
	assert.True(t, IsEligibleForDomain(result))
}

// ---------------------------------------------------------------------------
// Tracker
// ---------------------------------------------------------------------------

func TestNewTracker_DefaultWindowSize_Accepted(t *testing.T) {
	tracker, err := NewTracker(0)
	require.NoError(t, err)
	assert.Equal(t, DefaultWindowSize, tracker.windowSize)
}

func TestNewTracker_CustomWindowSize_Accepted(t *testing.T) {
	tracker, err := NewTracker(20)
	require.NoError(t, err)
	assert.Equal(t, 20, tracker.windowSize)
}

func TestNewTracker_WindowSizeTooSmall_ReturnsError(t *testing.T) {
	_, err := NewTracker(3)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidWindowSize)
}

func TestTracker_Record_CreatesWindowOnFirstCall(t *testing.T) {
	tracker, err := NewTracker(DefaultWindowSize)
	require.NoError(t, err)

	obs := makeObs("claim-1", "SUPPORTED", 0.8)
	require.NoError(t, tracker.Record("v1", "science", obs))
	assert.Equal(t, 1, tracker.ObservationCount("v1", "science"))
}

func TestTracker_Record_AccumulatesAcrossCalls(t *testing.T) {
	tracker, err := NewTracker(DefaultWindowSize)
	require.NoError(t, err)

	for i := 0; i < 7; i++ {
		obs := makeObs(fmt.Sprintf("c-%d", i), "SUPPORTED", 1.0)
		require.NoError(t, tracker.Record("v1", "science", obs))
	}
	assert.Equal(t, 7, tracker.ObservationCount("v1", "science"))
}

func TestTracker_Record_SeparateWindowsPerDomain(t *testing.T) {
	tracker, err := NewTracker(DefaultWindowSize)
	require.NoError(t, err)

	require.NoError(t, tracker.Record("v1", "science", makeObs("c-1", "SUPPORTED", 1.0)))
	require.NoError(t, tracker.Record("v1", "health", makeObs("c-2", "UNSUPPORTED", 0.0)))
	require.NoError(t, tracker.Record("v1", "health", makeObs("c-3", "UNSUPPORTED", 0.0)))

	assert.Equal(t, 1, tracker.ObservationCount("v1", "science"))
	assert.Equal(t, 2, tracker.ObservationCount("v1", "health"))
}

func TestTracker_Record_SeparateWindowsPerValidator(t *testing.T) {
	tracker, err := NewTracker(DefaultWindowSize)
	require.NoError(t, err)

	require.NoError(t, tracker.Record("v1", "science", makeObs("c-1", "SUPPORTED", 1.0)))
	require.NoError(t, tracker.Record("v2", "science", makeObs("c-1", "UNSUPPORTED", 1.0)))

	assert.Equal(t, 1, tracker.ObservationCount("v1", "science"))
	assert.Equal(t, 1, tracker.ObservationCount("v2", "science"))
}

func TestTracker_Record_EmptyValidatorID_ReturnsError(t *testing.T) {
	tracker, err := NewTracker(DefaultWindowSize)
	require.NoError(t, err)

	err = tracker.Record("", "science", makeObs("c-1", "SUPPORTED", 1.0))
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrValidatorIDEmpty)
}

func TestTracker_Record_EmptyDomain_ReturnsError(t *testing.T) {
	tracker, err := NewTracker(DefaultWindowSize)
	require.NoError(t, err)

	err = tracker.Record("v1", "", makeObs("c-1", "SUPPORTED", 1.0))
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrDomainEmpty)
}

func TestTracker_BiasFor_UnknownValidatorDomain_ReturnsZero(t *testing.T) {
	tracker, err := NewTracker(DefaultWindowSize)
	require.NoError(t, err)

	result := tracker.BiasFor("unknown-validator", "unknown-domain")
	assert.Equal(t, 0.0, result.Coefficient)
	assert.Equal(t, TierNegligible, result.Tier)
	assert.Equal(t, 1.0, result.CorrectionFactor)
	assert.Equal(t, 0, result.ObservationCount)
}

func TestTracker_BiasFor_InsufficientData_ReturnsZero(t *testing.T) {
	tracker, err := NewTracker(DefaultWindowSize)
	require.NoError(t, err)

	for i := 0; i < MinObservationsForBias-1; i++ {
		require.NoError(t, tracker.Record("v1", "science",
			makeObs(fmt.Sprintf("c-%d", i), "SUPPORTED", 0.0)))
	}

	result := tracker.BiasFor("v1", "science")
	assert.Equal(t, 0.0, result.Coefficient)
}

func TestTracker_BiasFor_SufficientData_ReturnsNonZero(t *testing.T) {
	tracker, err := NewTracker(DefaultWindowSize)
	require.NoError(t, err)

	// All votes SUPPORTED (1.0), population average 0.0 → maximum deviation.
	for i := 0; i < MinObservationsForBias; i++ {
		require.NoError(t, tracker.Record("v1", "science",
			makeObs(fmt.Sprintf("c-%d", i), "SUPPORTED", 0.0)))
	}

	result := tracker.BiasFor("v1", "science")
	assert.Equal(t, 1.0, result.Coefficient)
	assert.Equal(t, TierSevere, result.Tier)
}

func TestTracker_ObservationCount_UnknownPair_ReturnsZero(t *testing.T) {
	tracker, err := NewTracker(DefaultWindowSize)
	require.NoError(t, err)
	assert.Equal(t, 0, tracker.ObservationCount("nobody", "nowhere"))
}
