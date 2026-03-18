// Package consensus implements the Proof of Verification (PoV) weighted vote
// aggregation algorithm. It takes a set of validator votes and produces a
// ConsensusResult containing the verdict and aggregate confidence score.
// It has no knowledge of networking, storage, chain, or session management.
package consensus
