package consensus

import (
	"errors"
	"math"
	"testing"
)

const floatTolerance = 1e-9

func floatEqual(a, b float64) bool {
	return math.Abs(a-b) < floatTolerance
}

// makeVote returns a Vote with sensible defaults. Tests override only the
// fields relevant to the scenario under test.
func makeVote(validatorID string, verdict VerdictState, confidence, domainRep, bias float64) Vote {
	return Vote{
		ValidatorID:      validatorID,
		Verdict:          verdict,
		Confidence:       confidence,
		DomainReputation: domainRep,
		BiasCoefficient:  bias,
	}
}

func TestComputeConsensus_NoVotes_ReturnsError(t *testing.T) {
	_, err := ComputeConsensus([]Vote{}, 11)
	if !errors.Is(err, ErrNoVotes) {
		t.Errorf("expected ErrNoVotes, got: %v", err)
	}
}

func TestComputeConsensus_InvalidVote_ReturnsError(t *testing.T) {
	tests := []struct {
		name string
		vote Vote
	}{
		{
			name: "empty validator id",
			vote: makeVote("", VerdictSupported, 0.8, 0.7, 0.0),
		},
		{
			name: "confidence above 1.0",
			vote: makeVote("v1", VerdictSupported, 1.1, 0.7, 0.0),
		},
		{
			name: "confidence below 0.0",
			vote: makeVote("v1", VerdictSupported, -0.1, 0.7, 0.0),
		},
		{
			name: "domain reputation above 1.0",
			vote: makeVote("v1", VerdictSupported, 0.8, 1.1, 0.0),
		},
		{
			name: "bias coefficient below 0.0",
			vote: makeVote("v1", VerdictSupported, 0.8, 0.7, -0.1),
		},
		{
			name: "unrecognised verdict",
			vote: makeVote("v1", "FABRICATED", 0.8, 0.7, 0.0),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ComputeConsensus([]Vote{tt.vote}, 11)
			if !errors.Is(err, ErrInvalidVote) {
				t.Errorf("expected ErrInvalidVote, got: %v", err)
			}
		})
	}
}

func TestComputeConsensus_QuorumNotMet_ReturnsIndeterminate(t *testing.T) {
	tests := []struct {
		name             string
		votes            int
		validatorSetSize int
	}{
		{"5 of 11 voted", 5, 11},
		{"7 of 11 voted — just under 2/3", 7, 11},
		{"1 of 17 voted", 1, 17},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			votes := make([]Vote, tt.votes)
			for i := range votes {
				votes[i] = makeVote(
					"v"+string(rune('0'+i)),
					VerdictSupported, 0.9, 0.8, 0.0,
				)
			}
			result, err := ComputeConsensus(votes, tt.validatorSetSize)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.Verdict != VerdictIndeterminate {
				t.Errorf("verdict: got %q, want %q", result.Verdict, VerdictIndeterminate)
			}
		})
	}
}

func TestComputeConsensus_QuorumMet_HonestNetwork_ReturnsCorrectVerdict(t *testing.T) {
	tests := []struct {
		name           string
		votes          []Vote
		setSize        int
		wantVerdict    VerdictState
		wantMinConfidence float64
	}{
		{
			name: "strong supported majority",
			votes: []Vote{
				makeVote("v1", VerdictSupported, 0.9, 0.85, 0.0),
				makeVote("v2", VerdictSupported, 0.8, 0.80, 0.0),
				makeVote("v3", VerdictSupported, 0.85, 0.90, 0.0),
				makeVote("v4", VerdictSupported, 0.75, 0.70, 0.0),
				makeVote("v5", VerdictSupported, 0.9, 0.88, 0.0),
				makeVote("v6", VerdictSupported, 0.7, 0.75, 0.0),
				makeVote("v7", VerdictSupported, 0.8, 0.82, 0.0),
				makeVote("v8", VerdictUnsupported, 0.6, 0.60, 0.0),
				makeVote("v9", VerdictUnsupported, 0.5, 0.55, 0.0),
				makeVote("v10", VerdictIndeterminate, 0.4, 0.50, 0.0),
				makeVote("v11", VerdictIndeterminate, 0.3, 0.45, 0.0),
			},
			setSize:           11,
			wantVerdict:       VerdictSupported,
			wantMinConfidence: 0.5,
		},
		{
			name: "strong unsupported majority",
			votes: []Vote{
				makeVote("v1", VerdictUnsupported, 0.9, 0.85, 0.0),
				makeVote("v2", VerdictUnsupported, 0.85, 0.80, 0.0),
				makeVote("v3", VerdictUnsupported, 0.8, 0.90, 0.0),
				makeVote("v4", VerdictUnsupported, 0.75, 0.70, 0.0),
				makeVote("v5", VerdictUnsupported, 0.9, 0.88, 0.0),
				makeVote("v6", VerdictUnsupported, 0.7, 0.75, 0.0),
				makeVote("v7", VerdictUnsupported, 0.8, 0.82, 0.0),
				makeVote("v8", VerdictSupported, 0.5, 0.50, 0.0),
				makeVote("v9", VerdictSupported, 0.4, 0.45, 0.0),
				makeVote("v10", VerdictIndeterminate, 0.3, 0.40, 0.0),
				makeVote("v11", VerdictIndeterminate, 0.3, 0.40, 0.0),
			},
			setSize:           11,
			wantVerdict:       VerdictUnsupported,
			wantMinConfidence: 0.5,
		},
		{
			name: "misleading verdict wins",
			votes: []Vote{
				makeVote("v1", VerdictMisleading, 0.9, 0.9, 0.0),
				makeVote("v2", VerdictMisleading, 0.85, 0.85, 0.0),
				makeVote("v3", VerdictMisleading, 0.8, 0.88, 0.0),
				makeVote("v4", VerdictMisleading, 0.75, 0.80, 0.0),
				makeVote("v5", VerdictMisleading, 0.9, 0.92, 0.0),
				makeVote("v6", VerdictMisleading, 0.7, 0.75, 0.0),
				makeVote("v7", VerdictMisleading, 0.8, 0.78, 0.0),
				makeVote("v8", VerdictSupported, 0.5, 0.40, 0.0),
				makeVote("v9", VerdictSupported, 0.4, 0.35, 0.0),
				makeVote("v10", VerdictUnsupported, 0.3, 0.30, 0.0),
				makeVote("v11", VerdictUnsupported, 0.3, 0.30, 0.0),
			},
			setSize:           11,
			wantVerdict:       VerdictMisleading,
			wantMinConfidence: 0.5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ComputeConsensus(tt.votes, tt.setSize)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.Verdict != tt.wantVerdict {
				t.Errorf("verdict: got %q, want %q", result.Verdict, tt.wantVerdict)
			}
			if result.ConfidenceScore < tt.wantMinConfidence {
				t.Errorf("confidence %f below minimum %f", result.ConfidenceScore, tt.wantMinConfidence)
			}
			if result.ParticipationRate < QuorumThreshold {
				t.Errorf("participation rate %f below quorum", result.ParticipationRate)
			}
		})
	}
}

func TestComputeConsensus_BiasCorrection_ReducesInfluence(t *testing.T) {
	// Without bias: biased validators dominate and flip the verdict.
	// With bias coefficient applied: their weight is reduced enough that
	// the honest minority verdict wins.
	biasedVotes := []Vote{
		makeVote("v1", VerdictSupported, 0.9, 0.9, 0.8),
		makeVote("v2", VerdictSupported, 0.9, 0.9, 0.8),
		makeVote("v3", VerdictSupported, 0.9, 0.9, 0.8),
		makeVote("v4", VerdictSupported, 0.9, 0.9, 0.8),
		makeVote("v5", VerdictSupported, 0.9, 0.9, 0.8),
		makeVote("v6", VerdictSupported, 0.9, 0.9, 0.8),
		makeVote("v7", VerdictSupported, 0.9, 0.9, 0.8),
		makeVote("v8", VerdictUnsupported, 0.9, 0.9, 0.0),
		makeVote("v9", VerdictUnsupported, 0.9, 0.9, 0.0),
		makeVote("v10", VerdictUnsupported, 0.9, 0.9, 0.0),
		makeVote("v11", VerdictUnsupported, 0.9, 0.9, 0.0),
	}

	result, err := ComputeConsensus(biasedVotes, 11)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Biased validators (bias=0.8) have their weight reduced to 20%.
	// 7 biased * 0.9*0.9*(1-0.8) = 7 * 0.162 = 1.134
	// 4 honest * 0.9*0.9*(1-0.0) = 4 * 0.810 = 3.240
	// Unsupported wins despite being the minority by count.
	if result.Verdict != VerdictUnsupported {
		t.Errorf("bias correction failed: got %q, want %q — biased majority should not win",
			result.Verdict, VerdictUnsupported)
	}
}

func TestComputeConsensus_MajorityNotReached_ReturnsIndeterminate(t *testing.T) {
	// Evenly split vote — no verdict can reach 50% of total mass.
	votes := []Vote{
		makeVote("v1", VerdictSupported, 0.8, 0.8, 0.0),
		makeVote("v2", VerdictSupported, 0.8, 0.8, 0.0),
		makeVote("v3", VerdictSupported, 0.8, 0.8, 0.0),
		makeVote("v4", VerdictUnsupported, 0.8, 0.8, 0.0),
		makeVote("v5", VerdictUnsupported, 0.8, 0.8, 0.0),
		makeVote("v6", VerdictUnsupported, 0.8, 0.8, 0.0),
		makeVote("v7", VerdictMisleading, 0.8, 0.8, 0.0),
		makeVote("v8", VerdictMisleading, 0.8, 0.8, 0.0),
		makeVote("v9", VerdictMisleading, 0.8, 0.8, 0.0),
		makeVote("v10", VerdictIndeterminate, 0.8, 0.8, 0.0),
		makeVote("v11", VerdictIndeterminate, 0.8, 0.8, 0.0),
	}

	result, err := ComputeConsensus(votes, 11)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Verdict != VerdictIndeterminate {
		t.Errorf("verdict: got %q, want %q", result.Verdict, VerdictIndeterminate)
	}
}

func TestComputeConsensus_AdjustedWeight_Formula(t *testing.T) {
	// Verifies the weight formula directly:
	// adjusted_weight = confidence x domain_reputation x (1 - bias_coefficient)
	tests := []struct {
		name       string
		vote       Vote
		wantWeight float64
	}{
		{
			name:       "no bias full weight",
			vote:       makeVote("v1", VerdictSupported, 0.8, 0.75, 0.0),
			wantWeight: 0.8 * 0.75 * 1.0,
		},
		{
			name:       "half bias halves weight",
			vote:       makeVote("v1", VerdictSupported, 0.8, 0.75, 0.5),
			wantWeight: 0.8 * 0.75 * 0.5,
		},
		{
			name:       "maximum bias zeroes weight",
			vote:       makeVote("v1", VerdictSupported, 0.8, 0.75, 1.0),
			wantWeight: 0.0,
		},
		{
			name:       "zero confidence zeroes weight",
			vote:       makeVote("v1", VerdictSupported, 0.0, 0.75, 0.0),
			wantWeight: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := adjustedWeight(tt.vote)
			if !floatEqual(got, tt.wantWeight) {
				t.Errorf("adjustedWeight = %f, want %f", got, tt.wantWeight)
			}
		})
	}
}

func TestComputeConsensus_ParticipationRate_IsCorrect(t *testing.T) {
	votes := make([]Vote, 8)
	for i := range votes {
		votes[i] = makeVote("v"+string(rune('0'+i)), VerdictSupported, 0.8, 0.8, 0.0)
	}
	result, err := ComputeConsensus(votes, 11)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	wantRate := 8.0 / 11.0
	if !floatEqual(result.ParticipationRate, wantRate) {
		t.Errorf("participation rate: got %f, want %f", result.ParticipationRate, wantRate)
	}
}

func TestComputeConsensus_AllZeroWeight_ReturnsIndeterminate(t *testing.T) {
	// All votes have maximum bias — zero adjusted weight total.
	votes := make([]Vote, 11)
	for i := range votes {
		votes[i] = makeVote("v"+string(rune('0'+i)), VerdictSupported, 0.9, 0.9, 1.0)
	}
	result, err := ComputeConsensus(votes, 11)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Verdict != VerdictIndeterminate {
		t.Errorf("verdict: got %q, want %q", result.Verdict, VerdictIndeterminate)
	}
}
