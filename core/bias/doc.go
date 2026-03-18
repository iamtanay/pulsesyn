// Package bias implements the PulseSyn bias detection module. It computes
// bias coefficients per validator per domain using a sliding window of recent
// validations, classifies validators into response tiers, and produces bias
// correction factors consumed by the consensus engine.
// It has no knowledge of networking, storage, chain, or session management.
package bias
