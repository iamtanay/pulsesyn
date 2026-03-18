// Package session orchestrates complete PulseSyn validation sessions from claim
// submission through provisional verdict to finalization. It manages the
// commit-reveal voting window, triggers consensus, handles the dispute window,
// and writes finalized records to the chain. It imports from core/consensus,
// core/claim, and core/reputation only.
package session
