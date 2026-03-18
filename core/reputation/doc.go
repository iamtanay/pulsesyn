// Package reputation implements the PulseSyn reputation engine. It computes
// domain-specific reputation scores for validators based on finalized verdicts,
// applies post-finalization update rules, manages reputation decay for inactive
// validators, and supports retrospective updates when ground truth emerges.
// It has no knowledge of networking, storage, chain, or session management.
package reputation
