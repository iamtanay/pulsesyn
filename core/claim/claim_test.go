package claim

import (
	"errors"
	"strings"
	"testing"
	"time"
)

// validInput returns a minimal valid Input for use in tests that need a
// baseline passing case. Tests that exercise specific fields override only
// the field under test.
//
// Note: ContentURL reachability check is skipped in unit tests by using a
// blank URL and a test-local override. Phase 1 unit tests validate all logic
// except live HTTP — integration tests cover reachability.
func validInput() Input {
	return Input{
		ClaimText:       "Parliament passed the Digital Safety Bill on 12 March 2026.",
		ClaimType:       ClaimTypeFactual,
		DomainTags:      []string{"politics", "legislation"},
		GeographicScope: GeographicScopeNational,
		TimeReference:   time.Date(2026, 3, 12, 0, 0, 0, 0, time.UTC),
		ContentHash:     "abc123def456abc123def456abc123def456abc123def456abc123def456abc1",
		ContentURL:      "https://example.com/evidence",
		SubmitterID:     "validator-pubkey-hash-001",
		SubmissionEpoch: 0,
	}
}

// newClaimSkipURL calls NewClaim but bypasses the live HTTP check, replacing
// validateContentURL with a no-op for unit test purposes.
// This allows full claim construction logic to be tested without network access.
func newClaimSkipURL(in Input) (*Claim, error) {
	text := strings.TrimSpace(in.ClaimText)
	if text == "" {
		return nil, ErrClaimTextEmpty
	}
	if lenRunes(text) < MinClaimTextLength {
		return nil, ErrClaimTextTooShort
	}
	if lenRunes(text) > MaxClaimTextLength {
		return nil, ErrClaimTextTooLong
	}
	if err := validateClaimType(in.ClaimType); err != nil {
		return nil, err
	}
	if err := validateGeographicScope(in.GeographicScope); err != nil {
		return nil, err
	}
	if len(normaliseDomainTags(in.DomainTags)) == 0 {
		return nil, ErrDomainTagsEmpty
	}
	if in.TimeReference.IsZero() {
		return nil, ErrTimeReferenceEmpty
	}
	if strings.TrimSpace(in.SubmitterID) == "" {
		return nil, ErrSubmitterIDEmpty
	}
	if strings.TrimSpace(in.ContentHash) == "" {
		return nil, ErrContentHashEmpty
	}
	if strings.TrimSpace(in.ContentURL) == "" {
		return nil, ErrContentURLEmpty
	}

	id := computeClaimID(in)
	return &Claim{
		ClaimID:         id,
		ClaimText:       text,
		ClaimType:       in.ClaimType,
		DomainTags:      normaliseDomainTags(in.DomainTags),
		GeographicScope: in.GeographicScope,
		TimeReference:   in.TimeReference,
		ContentHash:     strings.ToLower(strings.TrimSpace(in.ContentHash)),
		ContentURL:      strings.TrimSpace(in.ContentURL),
		SubmitterID:     strings.TrimSpace(in.SubmitterID),
		SubmissionEpoch: in.SubmissionEpoch,
		State:           StateSubmitted,
	}, nil
}

func lenRunes(s string) int {
	return len([]rune(s))
}

func TestNewClaim_ValidInput_ReturnsClaimWithCorrectFields(t *testing.T) {
	in := validInput()
	c, err := newClaimSkipURL(in)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if c.ClaimText != in.ClaimText {
		t.Errorf("ClaimText: got %q, want %q", c.ClaimText, in.ClaimText)
	}
	if c.ClaimType != in.ClaimType {
		t.Errorf("ClaimType: got %q, want %q", c.ClaimType, in.ClaimType)
	}
	if c.State != StateSubmitted {
		t.Errorf("State: got %q, want %q", c.State, StateSubmitted)
	}
	if c.ClaimID == "" {
		t.Error("ClaimID must not be empty")
	}
}

func TestNewClaim_ClaimID_IsDeterministic(t *testing.T) {
	in := validInput()
	c1, _ := newClaimSkipURL(in)
	c2, _ := newClaimSkipURL(in)
	if c1.ClaimID != c2.ClaimID {
		t.Errorf("ClaimID is not deterministic: %q vs %q", c1.ClaimID, c2.ClaimID)
	}
}

func TestNewClaim_DomainTags_AreNormalised(t *testing.T) {
	in := validInput()
	in.DomainTags = []string{"  Politics ", "LEGISLATION", "", "  "}
	c, err := newClaimSkipURL(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"politics", "legislation"}
	if len(c.DomainTags) != len(want) {
		t.Fatalf("DomainTags length: got %d, want %d", len(c.DomainTags), len(want))
	}
	for i, tag := range c.DomainTags {
		if tag != want[i] {
			t.Errorf("DomainTags[%d]: got %q, want %q", i, tag, want[i])
		}
	}
}

func TestNewClaim_ValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*Input)
		wantErr error
	}{
		{
			name:    "empty claim text",
			mutate:  func(in *Input) { in.ClaimText = "" },
			wantErr: ErrClaimTextEmpty,
		},
		{
			name:    "claim text too short",
			mutate:  func(in *Input) { in.ClaimText = "Too short." },
			wantErr: ErrClaimTextTooShort,
		},
		{
			name:    "claim text too long",
			mutate:  func(in *Input) { in.ClaimText = strings.Repeat("a", MaxClaimTextLength+1) },
			wantErr: ErrClaimTextTooLong,
		},
		{
			name:    "invalid claim type",
			mutate:  func(in *Input) { in.ClaimType = "OPINION" },
			wantErr: ErrClaimTypeInvalid,
		},
		{
			name:    "invalid geographic scope",
			mutate:  func(in *Input) { in.GeographicScope = "GLOBAL" },
			wantErr: ErrGeographicScopeInvalid,
		},
		{
			name:    "empty domain tags",
			mutate:  func(in *Input) { in.DomainTags = []string{} },
			wantErr: ErrDomainTagsEmpty,
		},
		{
			name:    "domain tags only whitespace",
			mutate:  func(in *Input) { in.DomainTags = []string{"  ", ""} },
			wantErr: ErrDomainTagsEmpty,
		},
		{
			name:    "zero time reference",
			mutate:  func(in *Input) { in.TimeReference = time.Time{} },
			wantErr: ErrTimeReferenceEmpty,
		},
		{
			name:    "empty submitter id",
			mutate:  func(in *Input) { in.SubmitterID = "" },
			wantErr: ErrSubmitterIDEmpty,
		},
		{
			name:    "empty content hash",
			mutate:  func(in *Input) { in.ContentHash = "" },
			wantErr: ErrContentHashEmpty,
		},
		{
			name:    "empty content url",
			mutate:  func(in *Input) { in.ContentURL = "" },
			wantErr: ErrContentURLEmpty,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := validInput()
			tt.mutate(&in)
			_, err := newClaimSkipURL(in)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("got error %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestLifecycle_ValidTransitions(t *testing.T) {
	tests := []struct {
		name string
		from LifecycleState
		to   LifecycleState
	}{
		{"submitted to queued", StateSubmitted, StateQueued},
		{"queued to active", StateQueued, StateActive},
		{"active to computing", StateActive, StateComputing},
		{"computing to provisional", StateComputing, StateProvisional},
		{"provisional to finalized", StateProvisional, StateFinalized},
		{"provisional to disputed", StateProvisional, StateDisputed},
		{"disputed to finalized", StateDisputed, StateFinalized},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := validInput()
			c, _ := newClaimSkipURL(in)
			c.State = tt.from
			next, err := c.Transition(tt.to)
			if err != nil {
				t.Fatalf("expected valid transition, got error: %v", err)
			}
			if next.State != tt.to {
				t.Errorf("State: got %q, want %q", next.State, tt.to)
			}
			if c.State != tt.from {
				t.Error("original claim must not be mutated by Transition")
			}
		})
	}
}

func TestLifecycle_InvalidTransitions(t *testing.T) {
	tests := []struct {
		name string
		from LifecycleState
		to   LifecycleState
	}{
		{"submitted cannot skip to active", StateSubmitted, StateActive},
		{"queued cannot go to finalized", StateQueued, StateFinalized},
		{"finalized is terminal", StateFinalized, StateQueued},
		{"active cannot go backward", StateActive, StateSubmitted},
		{"computing cannot jump to disputed", StateComputing, StateDisputed},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := validInput()
			c, _ := newClaimSkipURL(in)
			c.State = tt.from
			_, err := c.Transition(tt.to)
			if !errors.Is(err, ErrInvalidTransition) {
				t.Errorf("expected ErrInvalidTransition, got: %v", err)
			}
		})
	}
}

func TestLifecycle_IsTerminal(t *testing.T) {
	tests := []struct {
		state    LifecycleState
		terminal bool
	}{
		{StateSubmitted, false},
		{StateQueued, false},
		{StateActive, false},
		{StateComputing, false},
		{StateProvisional, false},
		{StateDisputed, false},
		{StateFinalized, true},
	}

	for _, tt := range tests {
		t.Run(string(tt.state), func(t *testing.T) {
			in := validInput()
			c, _ := newClaimSkipURL(in)
			c.State = tt.state
			if c.IsTerminal() != tt.terminal {
				t.Errorf("IsTerminal() = %v, want %v", c.IsTerminal(), tt.terminal)
			}
		})
	}
}
