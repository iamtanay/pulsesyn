package bias

import (
	"errors"
	"fmt"
	"strings"
)

// Protocol constants for the bias detection module.
// These are Phase 1 defaults. Governable parameters will be configurable
// in later phases via the governance module.
// See PulseSyn Protocol Specification v0.1, Appendix A and Section 6.3.
const (
	// BiasExclusionThreshold is the bias coefficient above which a validator
	// is excluded from validator pools for that domain.
	BiasExclusionThreshold = 0.70

	// BiasHighThreshold is the coefficient above which the validator is
	// classified as High and excluded from domain pools.
	BiasHighThreshold = 0.50

	// BiasModerateThreshold is the coefficient above which the validator is
	// classified as Moderate and their domain vote weight is reduced.
	BiasModerateThreshold = 0.30

	// DefaultWindowSize is the default number of recent validations retained
	// in the sliding window per validator per domain.
	DefaultWindowSize = 50

	// MinObservationsForBias is the minimum number of observations required
	// before a non-zero bias coefficient is computed. With fewer observations
	// the sample is too small to be statistically meaningful.
	MinObservationsForBias = 5
)

// ResponseTier classifies the severity of detected bias and defines the
// protocol's response.
// See PulseSyn Protocol Specification v0.1, Section 6.3.
type ResponseTier string

const (
	// TierNegligible means bias coefficient is in [0.0, 0.30).
	// No action. Minor correction applied automatically in vote weighting.
	TierNegligible ResponseTier = "NEGLIGIBLE"

	// TierModerate means bias coefficient is in [0.30, 0.50).
	// Validator notified. Domain vote weight reduced proportionally.
	TierModerate ResponseTier = "MODERATE"

	// TierHigh means bias coefficient is in [0.50, 0.70).
	// Validator excluded from validator pools for this domain only.
	TierHigh ResponseTier = "HIGH"

	// TierSevere means bias coefficient is in [0.70, 1.00].
	// Validator suspended from all validation. Reputation audit and
	// collusion investigation initiated.
	TierSevere ResponseTier = "SEVERE"
)

// VerdictScore maps a verdict to a numeric position on the support axis
// used for deviation computation. SUPPORTED = 1.0, UNSUPPORTED = 0.0,
// MISLEADING = 0.5, INDETERMINATE = 0.5.
//
// This linear encoding captures the primary support dimension. MISLEADING and
// INDETERMINATE are both positioned at the midpoint — they represent neither
// strong support nor strong rejection — which is the conservative choice until
// a richer multi-axis encoding is justified by empirical data.
//
// See PulseSyn Protocol Specification v0.1, Section 6.2.
var verdictScore = map[string]float64{
	"SUPPORTED":     1.0,
	"UNSUPPORTED":   0.0,
	"MISLEADING":    0.5,
	"INDETERMINATE": 0.5,
}

// Sentinel errors returned by bias operations.
var (
	// ErrValidatorIDEmpty is returned when a validator_id field is blank.
	ErrValidatorIDEmpty = errors.New("pulsesyn/bias: validator_id is empty")

	// ErrDomainEmpty is returned when a domain field is blank.
	ErrDomainEmpty = errors.New("pulsesyn/bias: domain is empty")

	// ErrClaimIDEmpty is returned when a claim_id field is blank.
	ErrClaimIDEmpty = errors.New("pulsesyn/bias: claim_id is empty")

	// ErrInvalidVerdictString is returned when a verdict string is not one
	// of the four recognised protocol verdict states.
	ErrInvalidVerdictString = errors.New("pulsesyn/bias: verdict is not one of SUPPORTED, UNSUPPORTED, MISLEADING, INDETERMINATE")

	// ErrInvalidWindowSize is returned when a window size is less than
	// MinObservationsForBias.
	ErrInvalidWindowSize = errors.New("pulsesyn/bias: window_size must be >= 5")
)

// ValidationObservation is a single data point in the bias sliding window.
// It records the validator's verdict and the population-average verdict for
// the same claim, enabling deviation computation.
// See PulseSyn Protocol Specification v0.1, Section 6.2.
type ValidationObservation struct {
	// ClaimID is the claim this observation belongs to.
	ClaimID string

	// ValidatorVerdict is the verdict the validator submitted for this claim.
	// Must be one of: SUPPORTED, UNSUPPORTED, MISLEADING, INDETERMINATE.
	ValidatorVerdict string

	// PopulationAverageScore is the mean verdict score of all other validators
	// who evaluated the same claim in the same session, excluding this validator.
	// This is computed externally (by the session layer) and passed in.
	// Range: [0.0, 1.0].
	PopulationAverageScore float64
}

// BiasResult is the output of a bias coefficient computation for a single
// validator in a single domain. It is immutable once returned.
// See PulseSyn Protocol Specification v0.1, Section 6.2 and 6.3.
type BiasResult struct {
	// ValidatorID identifies the validator.
	ValidatorID string

	// Domain is the domain for which the bias was computed.
	Domain string

	// Coefficient is the computed bias coefficient in [0.0, 1.0].
	// 0.0 = no detectable systematic bias.
	// 1.0 = maximum systematic bias.
	Coefficient float64

	// Tier is the response tier classification based on the coefficient.
	Tier ResponseTier

	// CorrectionFactor is the multiplier to apply to the validator's vote
	// weight: adjusted_weight = raw_weight * CorrectionFactor.
	// Derived as (1 - Coefficient). Range: [0.0, 1.0].
	CorrectionFactor float64

	// ObservationCount is the number of observations used to compute the
	// coefficient. Coefficients based on fewer than MinObservationsForBias
	// observations are not computed and the result will have Coefficient=0.0.
	ObservationCount int
}

// Window holds the sliding window of ValidationObservations for a single
// validator in a single domain. It is the mutable state that accumulates
// over time.
type Window struct {
	validatorID string
	domain      string
	maxSize     int
	entries     []ValidationObservation
}

// WindowConfig carries the parameters needed to create a new Window.
type WindowConfig struct {
	// ValidatorID identifies the validator this window tracks.
	ValidatorID string

	// Domain is the domain this window tracks.
	Domain string

	// MaxSize is the maximum number of observations to retain. Older
	// observations are evicted when the window is full. Must be >= MinObservationsForBias.
	// Defaults to DefaultWindowSize if zero.
	MaxSize int
}

// NewWindow constructs a new empty Window for the given validator and domain.
// Returns an error if ValidatorID or Domain is empty, or if MaxSize is
// set to a value less than MinObservationsForBias.
func NewWindow(cfg WindowConfig) (*Window, error) {
	if strings.TrimSpace(cfg.ValidatorID) == "" {
		return nil, fmt.Errorf("NewWindow: %w", ErrValidatorIDEmpty)
	}
	if strings.TrimSpace(cfg.Domain) == "" {
		return nil, fmt.Errorf("NewWindow: %w", ErrDomainEmpty)
	}

	size := cfg.MaxSize
	if size == 0 {
		size = DefaultWindowSize
	}
	if size < MinObservationsForBias {
		return nil, fmt.Errorf("NewWindow: %w", ErrInvalidWindowSize)
	}

	return &Window{
		validatorID: strings.TrimSpace(cfg.ValidatorID),
		domain:      strings.ToLower(strings.TrimSpace(cfg.Domain)),
		maxSize:     size,
		entries:     make([]ValidationObservation, 0, size),
	}, nil
}

// ValidatorID returns the validator identifier this window belongs to.
func (w *Window) ValidatorID() string { return w.validatorID }

// Domain returns the domain this window tracks.
func (w *Window) Domain() string { return w.domain }

// Len returns the current number of observations in the window.
func (w *Window) Len() int { return len(w.entries) }

// Add appends a new ValidationObservation to the window. If the window is
// full, the oldest observation is evicted (FIFO). Returns an error if the
// observation fields are invalid.
func (w *Window) Add(obs ValidationObservation) error {
	if err := validateObservation(obs); err != nil {
		return fmt.Errorf("Window.Add (%s/%s): %w", w.validatorID, w.domain, err)
	}

	if len(w.entries) >= w.maxSize {
		// Evict oldest entry. Copy-forward is O(n) but windows are small (≤50)
		// and called at most once per finalized validation per validator.
		w.entries = w.entries[1:]
	}
	w.entries = append(w.entries, obs)
	return nil
}

// ComputeBias computes the bias coefficient for this window and returns a
// BiasResult. If the window contains fewer than MinObservationsForBias
// observations, the returned coefficient is 0.0 and the tier is TierNegligible
// — insufficient data is not the same as no bias, but the protocol does not
// penalise validators before there is statistical basis to do so.
//
// Bias coefficient formula:
//
//	mean_deviation    = mean( |score(v.verdict) - v.PopulationAverageScore| )
//	max_deviation     = 1.0   (maximum possible deviation on [0.0, 1.0] axis)
//	bias_coefficient  = mean_deviation / max_deviation
//	                  = mean_deviation  (since max_deviation = 1.0)
//
// See PulseSyn Protocol Specification v0.1, Section 6.2.
func (w *Window) ComputeBias() BiasResult {
	if len(w.entries) < MinObservationsForBias {
		return BiasResult{
			ValidatorID:      w.validatorID,
			Domain:           w.domain,
			Coefficient:      0.0,
			Tier:             TierNegligible,
			CorrectionFactor: 1.0,
			ObservationCount: len(w.entries),
		}
	}

	var totalDeviation float64
	for _, obs := range w.entries {
		score := verdictScore[obs.ValidatorVerdict]
		deviation := score - obs.PopulationAverageScore
		if deviation < 0 {
			deviation = -deviation
		}
		totalDeviation += deviation
	}

	coefficient := totalDeviation / float64(len(w.entries))
	// Clamp to [0.0, 1.0] as a safety invariant — the formula is bounded by
	// construction but floating-point arithmetic can produce tiny overflows.
	if coefficient > 1.0 {
		coefficient = 1.0
	}
	if coefficient < 0.0 {
		coefficient = 0.0
	}

	tier := classifyTier(coefficient)

	return BiasResult{
		ValidatorID:      w.validatorID,
		Domain:           w.domain,
		Coefficient:      coefficient,
		Tier:             tier,
		CorrectionFactor: 1.0 - coefficient,
		ObservationCount: len(w.entries),
	}
}

// classifyTier maps a bias coefficient to the appropriate ResponseTier.
// See PulseSyn Protocol Specification v0.1, Section 6.3.
func classifyTier(coefficient float64) ResponseTier {
	switch {
	case coefficient >= BiasExclusionThreshold:
		return TierSevere
	case coefficient >= BiasHighThreshold:
		return TierHigh
	case coefficient >= BiasModerateThreshold:
		return TierModerate
	default:
		return TierNegligible
	}
}

// IsEligibleForDomain returns true if the validator's computed bias coefficient
// does not exclude them from the given domain. A validator is excluded when
// their bias reaches TierHigh or TierSevere.
//
// This is a convenience function for the selector layer. It accepts a
// pre-computed BiasResult to avoid recomputing the window.
// See PulseSyn Protocol Specification v0.1, Section 3.3 and 6.3.
func IsEligibleForDomain(result BiasResult) bool {
	return result.Coefficient < BiasHighThreshold
}

// validateObservation checks that a ValidationObservation contains valid
// field values before it is admitted to a window.
func validateObservation(obs ValidationObservation) error {
	if strings.TrimSpace(obs.ClaimID) == "" {
		return ErrClaimIDEmpty
	}
	if _, ok := verdictScore[obs.ValidatorVerdict]; !ok {
		return fmt.Errorf("%w: got %q", ErrInvalidVerdictString, obs.ValidatorVerdict)
	}
	if obs.PopulationAverageScore < 0.0 || obs.PopulationAverageScore > 1.0 {
		return fmt.Errorf("pulsesyn/bias: population_average_score %f out of range [0.0, 1.0]", obs.PopulationAverageScore)
	}
	return nil
}

// Tracker manages a collection of Windows — one per (validatorID, domain)
// pair. It is the top-level entry point for the bias detection module.
// External packages that need to track bias across many validators use
// Tracker rather than managing individual Windows themselves.
type Tracker struct {
	windowSize int
	windows    map[string]*Window // key: validatorID + ":" + domain
}

// NewTracker constructs a new Tracker. windowSize sets the maximum number of
// observations per validator per domain window. If zero, DefaultWindowSize
// is used.
func NewTracker(windowSize int) (*Tracker, error) {
	if windowSize == 0 {
		windowSize = DefaultWindowSize
	}
	if windowSize < MinObservationsForBias {
		return nil, fmt.Errorf("NewTracker: %w", ErrInvalidWindowSize)
	}
	return &Tracker{
		windowSize: windowSize,
		windows:    make(map[string]*Window),
	}, nil
}

// Record adds a ValidationObservation for the given validator in the given
// domain. If no window exists for the pair, one is created automatically.
// Returns an error if validatorID, domain, or the observation are invalid.
func (t *Tracker) Record(validatorID, domain string, obs ValidationObservation) error {
	validatorID = strings.TrimSpace(validatorID)
	domain = strings.ToLower(strings.TrimSpace(domain))

	if validatorID == "" {
		return fmt.Errorf("Tracker.Record: %w", ErrValidatorIDEmpty)
	}
	if domain == "" {
		return fmt.Errorf("Tracker.Record: %w", ErrDomainEmpty)
	}

	key := validatorID + ":" + domain
	w, ok := t.windows[key]
	if !ok {
		var err error
		w, err = NewWindow(WindowConfig{
			ValidatorID: validatorID,
			Domain:      domain,
			MaxSize:     t.windowSize,
		})
		if err != nil {
			return fmt.Errorf("Tracker.Record: %w", err)
		}
		t.windows[key] = w
	}

	return w.Add(obs)
}

// BiasFor returns the BiasResult for the given validator in the given domain.
// If no observations have been recorded for this pair, returns a zero-bias
// result — the validator is unknown to this tracker, not biased.
func (t *Tracker) BiasFor(validatorID, domain string) BiasResult {
	validatorID = strings.TrimSpace(validatorID)
	domain = strings.ToLower(strings.TrimSpace(domain))

	key := validatorID + ":" + domain
	w, ok := t.windows[key]
	if !ok {
		return BiasResult{
			ValidatorID:      validatorID,
			Domain:           domain,
			Coefficient:      0.0,
			Tier:             TierNegligible,
			CorrectionFactor: 1.0,
			ObservationCount: 0,
		}
	}
	return w.ComputeBias()
}

// ObservationCount returns the number of observations recorded for the given
// validator and domain. Returns 0 if no observations exist for the pair.
func (t *Tracker) ObservationCount(validatorID, domain string) int {
	key := strings.TrimSpace(validatorID) + ":" + strings.ToLower(strings.TrimSpace(domain))
	w, ok := t.windows[key]
	if !ok {
		return 0
	}
	return w.Len()
}
