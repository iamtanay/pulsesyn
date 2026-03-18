// Package selector implements the PulseSyn validator selection algorithm.
// It derives selection seeds from claim ID and previous block hash, constructs
// the eligible validator pool, applies domain and bias stratification, and
// performs a composite-score weighted random draw. It imports from
// core/reputation and core/bias only.
package selector
