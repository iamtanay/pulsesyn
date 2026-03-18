package consensus

// VerdictState is the output of the PoV consensus algorithm.
// See PulseSyn Protocol Specification v0.1, Section 2.5.
type VerdictState string

const (
	// VerdictSupported means the submitted evidence sufficiently supports
	// the claim as stated. Not an absolute declaration of truth — a
	// declaration that this evidence supports this claim.
	VerdictSupported VerdictState = "SUPPORTED"

	// VerdictUnsupported means the evidence does not support the claim.
	// The claim may still be true — the evidence simply fails to establish it.
	VerdictUnsupported VerdictState = "UNSUPPORTED"

	// VerdictMisleading means the claim is technically supportable but uses
	// framing, omission, or selective presentation to create a false impression.
	VerdictMisleading VerdictState = "MISLEADING"

	// VerdictIndeterminate means evidence is insufficient to reach a verdict,
	// or quorum or majority thresholds were not met.
	VerdictIndeterminate VerdictState = "INDETERMINATE"
)

// VerdictBreakdown holds the aggregated adjusted weight for each verdict state
// after bias correction is applied. Used for transparency and audit.
type VerdictBreakdown struct {
	Supported    float64
	Unsupported  float64
	Misleading   float64
	Indeterminate float64
	TotalMass    float64
}

// ConsensusResult is the output of ComputeConsensus. It is immutable once
// returned — callers must not modify it.
// See PulseSyn Protocol Specification v0.1, Section 4.4.
type ConsensusResult struct {
	// Verdict is the winning verdict state, or VerdictIndeterminate if
	// quorum or majority thresholds were not met.
	Verdict VerdictState

	// ConfidenceScore is the aggregate weighted confidence of the winning
	// verdict, normalised to [0.0, 1.0].
	ConfidenceScore float64

	// Breakdown holds the full per-verdict weight distribution for audit.
	Breakdown VerdictBreakdown

	// ParticipationRate is the fraction of selected validators who submitted
	// votes. Must be >= QuorumThreshold for a non-indeterminate verdict.
	ParticipationRate float64

	// ValidatorCount is the number of votes that were included in the
	// consensus computation.
	ValidatorCount int
}
