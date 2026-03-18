package claim

import (
	"errors"
	"fmt"
)

// LifecycleState represents the current state of a Claim in the PulseSyn
// validation pipeline. Transitions are strictly enforced — only valid
// progressions are permitted.
// See PulseSyn Protocol Specification v0.1, Section 4.5.
type LifecycleState string

const (
	// StateSubmitted is the initial state. Claim schema has been validated
	// and the submission stake has been locked.
	StateSubmitted LifecycleState = "SUBMITTED"

	// StateQueued means the claim is awaiting validator selection.
	StateQueued LifecycleState = "QUEUED"

	// StateActive means the validator set is locked and the validation
	// window is open for commit-phase votes.
	StateActive LifecycleState = "ACTIVE"

	// StateComputing means votes have been collected and consensus is running.
	StateComputing LifecycleState = "COMPUTING"

	// StateProvisional means a preliminary verdict has been published and
	// the 48-hour dispute window is open.
	StateProvisional LifecycleState = "PROVISIONAL"

	// StateDisputed means a dispute has been filed and arbitration is underway.
	StateDisputed LifecycleState = "DISPUTED"

	// StateFinalized means the verdict is permanent and the on-chain record
	// has been written. This is a terminal state.
	StateFinalized LifecycleState = "FINALIZED"
)

// ErrInvalidTransition is returned when a caller attempts a state transition
// that is not permitted by the lifecycle state machine.
var ErrInvalidTransition = errors.New("pulsesyn/claim: invalid lifecycle state transition")

// validTransitions defines the complete set of permitted state progressions.
// Any transition not present in this map is illegal.
// See PulseSyn Protocol Specification v0.1, Section 4.5.
var validTransitions = map[LifecycleState][]LifecycleState{
	StateSubmitted:   {StateQueued},
	StateQueued:      {StateActive},
	StateActive:      {StateComputing},
	StateComputing:   {StateProvisional},
	StateProvisional: {StateFinalized, StateDisputed},
	StateDisputed:    {StateFinalized},
	StateFinalized:   {},
}

// Transition attempts to advance the claim to the next lifecycle state.
// It returns ErrInvalidTransition if the progression is not permitted.
// Because Claim is immutable, this returns a new Claim with the updated state.
// See PulseSyn Protocol Specification v0.1, Section 4.5.
func (c *Claim) Transition(next LifecycleState) (*Claim, error) {
	allowed := validTransitions[c.State]
	for _, s := range allowed {
		if s == next {
			updated := *c
			updated.State = next
			return &updated, nil
		}
	}
	return nil, fmt.Errorf("%w: %s -> %s", ErrInvalidTransition, c.State, next)
}

// IsTerminal returns true if the claim has reached a state from which no
// further transitions are possible.
func (c *Claim) IsTerminal() bool {
	return c.State == StateFinalized
}
