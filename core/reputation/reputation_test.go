package reputation

import (
	"errors"
	"math"
	"testing"
)

const floatTolerance = 1e-9

func floatEqual(a, b float64) bool {
	return math.Abs(a-b) < floatTolerance
}

func floatNear(a, b, tolerance float64) bool {
	return math.Abs(a-b) < tolerance
}

func TestNewValidatorRecord_ValidID_ReturnsRecord(t *testing.T) {
	r, err := NewValidatorRecord("validator-001", 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.ValidatorID != "validator-001" {
		t.Errorf("ValidatorID: got %q, want %q", r.ValidatorID, "validator-001")
	}
	if r.GlobalReputation != 0.0 {
		t.Errorf("GlobalReputation: got %f, want 0.0", r.GlobalReputation)
	}
	if r.Status != ValidatorStatusActive {
		t.Errorf("Status: got %q, want ACTIVE", r.Status)
	}
	if r.GenesisValidator {
		t.Error("GenesisValidator should be false for regular validators")
	}
}

func TestNewValidatorRecord_EmptyID_ReturnsError(t *testing.T) {
	_, err := NewValidatorRecord("", 0)
	if !errors.Is(err, ErrValidatorIDEmpty) {
		t.Errorf("expected ErrValidatorIDEmpty, got: %v", err)
	}
}

func TestNewGenesisValidatorRecord_SetsStartingReputation(t *testing.T) {
	domains := []string{"science", "health"}
	r, err := NewGenesisValidatorRecord("genesis-001", domains, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.GenesisValidator {
		t.Error("GenesisValidator should be true")
	}
	for _, d := range domains {
		score := r.DomainScore(d)
		if !floatEqual(score, GenesisValidatorStartingReputation) {
			t.Errorf("domain %q: got %f, want %f", d, score, GenesisValidatorStartingReputation)
		}
	}
}

func TestNewGenesisValidatorRecord_EmptyDomains_ReturnsError(t *testing.T) {
	_, err := NewGenesisValidatorRecord("genesis-001", []string{}, 0)
	if !errors.Is(err, ErrDomainEmpty) {
		t.Errorf("expected ErrDomainEmpty, got: %v", err)
	}
}

func TestIsEligibleForDomain(t *testing.T) {
	tests := []struct {
		name        string
		score       float64
		status      ValidatorStatus
		wantEligible bool
	}{
		{"above threshold active", 0.20, ValidatorStatusActive, true},
		{"at threshold active", EligibilityThreshold, ValidatorStatusActive, true},
		{"below threshold active", 0.10, ValidatorStatusActive, false},
		{"zero score active", 0.0, ValidatorStatusActive, false},
		{"above threshold suspended", 0.80, ValidatorStatusSuspended, false},
		{"above threshold retired", 0.80, ValidatorStatusRetired, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, _ := NewValidatorRecord("v1", 0)
			r = r.withUpdatedDomainScore("science", tt.score)
			r.Status = tt.status
			got := r.IsEligibleForDomain("science")
			if got != tt.wantEligible {
				t.Errorf("IsEligibleForDomain = %v, want %v", got, tt.wantEligible)
			}
		})
	}
}

func TestApplyPostFinalizationUpdate_SixCases(t *testing.T) {
	tests := []struct {
		name        string
		outcome     VoteOutcome
		startScore  float64
		wantDelta   float64
		wantReason  UpdateReason
	}{
		{
			name: "correct high confidence",
			outcome: VoteOutcome{
				ValidatorID: "v1", Domain: "science",
				WasCorrect: true, Confidence: 0.9, Participated: true,
			},
			startScore: 0.50,
			wantDelta:  +DeltaHigh,
			wantReason: ReasonCorrectHighConfidence,
		},
		{
			name: "correct low confidence",
			outcome: VoteOutcome{
				ValidatorID: "v1", Domain: "science",
				WasCorrect: true, Confidence: 0.2, Participated: true,
			},
			startScore: 0.50,
			wantDelta:  +DeltaLow,
			wantReason: ReasonCorrectLowConfidence,
		},
		{
			name: "correct mid confidence",
			outcome: VoteOutcome{
				ValidatorID: "v1", Domain: "science",
				WasCorrect: true, Confidence: 0.5, Participated: true,
			},
			startScore: 0.50,
			wantDelta:  +DeltaLow,
			wantReason: ReasonCorrectMidConfidence,
		},
		{
			name: "incorrect high confidence — overconfidence penalty",
			outcome: VoteOutcome{
				ValidatorID: "v1", Domain: "science",
				WasCorrect: false, Confidence: 0.9, Participated: true,
			},
			startScore: 0.50,
			wantDelta:  -DeltaHigh,
			wantReason: ReasonIncorrectHighConfidence,
		},
		{
			name: "incorrect low confidence",
			outcome: VoteOutcome{
				ValidatorID: "v1", Domain: "science",
				WasCorrect: false, Confidence: 0.2, Participated: true,
			},
			startScore: 0.50,
			wantDelta:  -DeltaLow,
			wantReason: ReasonIncorrectLowConfidence,
		},
		{
			name: "absent — participation penalty",
			outcome: VoteOutcome{
				ValidatorID: "v1", Domain: "science",
				Participated: false, Confidence: 0.0,
			},
			startScore: 0.50,
			wantDelta:  -DeltaAbsent,
			wantReason: ReasonAbsent,
		},
		{
			name: "late vote penalty",
			outcome: VoteOutcome{
				ValidatorID: "v1", Domain: "science",
				Participated: true, WasLate: true, Confidence: 0.8,
			},
			startScore: 0.50,
			wantDelta:  -DeltaLate,
			wantReason: ReasonLate,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, _ := NewValidatorRecord("v1", 0)
			r = r.withUpdatedDomainScore("science", tt.startScore)

			updated, result, err := ApplyPostFinalizationUpdate(r, tt.outcome)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !floatNear(result.Delta, tt.wantDelta, floatTolerance) {
				t.Errorf("Delta: got %f, want %f", result.Delta, tt.wantDelta)
			}
			if result.Reason != tt.wantReason {
				t.Errorf("Reason: got %q, want %q", result.Reason, tt.wantReason)
			}
			wantScore := clamp(tt.startScore+tt.wantDelta, ReputationFloor, ReputationCeiling)
			if !floatNear(updated.DomainScore("science"), wantScore, floatTolerance) {
				t.Errorf("NewScore: got %f, want %f", updated.DomainScore("science"), wantScore)
			}
			if r.DomainScore("science") != tt.startScore {
				t.Error("original record must not be mutated")
			}
		})
	}
}

func TestApplyPostFinalizationUpdate_ScoreClampedAtCeiling(t *testing.T) {
	r, _ := NewValidatorRecord("v1", 0)
	r = r.withUpdatedDomainScore("science", 0.99)
	outcome := VoteOutcome{
		ValidatorID: "v1", Domain: "science",
		WasCorrect: true, Confidence: 0.9, Participated: true,
	}
	updated, _, err := ApplyPostFinalizationUpdate(r, outcome)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updated.DomainScore("science") > ReputationCeiling {
		t.Errorf("score exceeded ceiling: %f", updated.DomainScore("science"))
	}
}

func TestApplyPostFinalizationUpdate_ScoreClampedAtFloor(t *testing.T) {
	r, _ := NewValidatorRecord("v1", 0)
	r = r.withUpdatedDomainScore("science", ReputationFloor)
	outcome := VoteOutcome{
		ValidatorID: "v1", Domain: "science",
		WasCorrect: false, Confidence: 0.9, Participated: true,
	}
	updated, _, err := ApplyPostFinalizationUpdate(r, outcome)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updated.DomainScore("science") < ReputationFloor {
		t.Errorf("score dropped below floor: %f", updated.DomainScore("science"))
	}
}

func TestApplyPostFinalizationUpdate_TotalValidationsIncrement(t *testing.T) {
	r, _ := NewValidatorRecord("v1", 0)
	r = r.withUpdatedDomainScore("science", 0.5)
	outcome := VoteOutcome{
		ValidatorID: "v1", Domain: "science",
		WasCorrect: true, Confidence: 0.8, Participated: true,
	}
	updated, _, _ := ApplyPostFinalizationUpdate(r, outcome)
	if updated.TotalValidations != r.TotalValidations+1 {
		t.Errorf("TotalValidations: got %d, want %d", updated.TotalValidations, r.TotalValidations+1)
	}
}

func TestApplyDecay_BelowInactivityThreshold_NoDecay(t *testing.T) {
	r, _ := NewValidatorRecord("v1", 0)
	r = r.withUpdatedDomainScore("science", 0.60)
	input := DecayInput{ValidatorID: "v1", Domain: "science", DaysInactive: 20}
	updated, result, err := ApplyDecay(r, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Applied {
		t.Error("decay should not apply before inactivity threshold")
	}
	if !floatEqual(updated.DomainScore("science"), 0.60) {
		t.Errorf("score changed unexpectedly: %f", updated.DomainScore("science"))
	}
}

func TestApplyDecay_AboveInactivityThreshold_ReducesScore(t *testing.T) {
	r, _ := NewValidatorRecord("v1", 0)
	startScore := 0.60
	r = r.withUpdatedDomainScore("science", startScore)

	daysInactive := 40
	effectiveDays := daysInactive - DecayInactivityDays
	expectedScore := clamp(
		startScore*math.Pow(1.0-DecayRate, float64(effectiveDays)),
		ReputationFloor, ReputationCeiling,
	)

	input := DecayInput{ValidatorID: "v1", Domain: "science", DaysInactive: daysInactive}
	updated, result, err := ApplyDecay(r, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Applied {
		t.Error("decay should have been applied")
	}
	if !floatNear(updated.DomainScore("science"), expectedScore, 1e-6) {
		t.Errorf("score: got %f, want %f", updated.DomainScore("science"), expectedScore)
	}
}

func TestApplyDecay_ScoreNeverDropsBelowFloor(t *testing.T) {
	r, _ := NewValidatorRecord("v1", 0)
	r = r.withUpdatedDomainScore("science", ReputationFloor+0.001)
	input := DecayInput{ValidatorID: "v1", Domain: "science", DaysInactive: 9999}
	updated, _, err := ApplyDecay(r, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updated.DomainScore("science") < ReputationFloor {
		t.Errorf("score dropped below floor: %f", updated.DomainScore("science"))
	}
}

func TestApplyDecay_AtExactThreshold_NoDecay(t *testing.T) {
	r, _ := NewValidatorRecord("v1", 0)
	r = r.withUpdatedDomainScore("science", 0.60)
	input := DecayInput{ValidatorID: "v1", Domain: "science", DaysInactive: DecayInactivityDays}
	_, result, err := ApplyDecay(r, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Applied {
		t.Error("decay should not apply at exactly the threshold day")
	}
}

func TestGlobalReputation_IsAverageOfDomainScores(t *testing.T) {
	r, _ := NewGenesisValidatorRecord("v1", []string{"science", "health"}, 0)
	want := GenesisValidatorStartingReputation
	if !floatNear(r.GlobalReputation, want, floatTolerance) {
		t.Errorf("GlobalReputation: got %f, want %f", r.GlobalReputation, want)
	}
}
