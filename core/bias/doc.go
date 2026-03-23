// Package bias implements the PulseSyn bias detection module.
//
// Bias detection tracks each validator's systematic deviation from population-
// average verdicts in a given domain over a sliding window of recent
// validations. When a validator's verdicts consistently diverge from the
// consensus of other validators on the same claims, a non-zero bias
// coefficient is computed and fed back into the consensus engine's vote
// weighting formula.
//
// This package:
//   - Maintains a sliding window of ValidationObservation records per
//     validator per domain.
//   - Computes a bias coefficient in [0.0, 1.0] for a validator in a domain.
//   - Classifies the coefficient into one of four response tiers.
//   - Returns a bias correction factor for use in core/consensus.
//
// This package does not import from outside core/. It has no knowledge of
// networking, storage, chain, API, or session management. It receives
// observations, computes coefficients, and returns results.
//
// See PulseSyn Protocol Specification v0.1, Section 6.
package bias
