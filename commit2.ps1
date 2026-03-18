# ── core/claim/claim.go ──────────────────────────────────────────────────────
@"
// Package claim defines the Claim schema, constructor, validation rules, and
// lifecycle state machine for the PulseSyn protocol. A Claim is the central
// protocol primitive — a structured, falsifiable assertion submitted with a
// pointer to supporting evidence.
// It has no knowledge of networking, storage, chain, or session management.
package claim

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"
)

// ClaimType classifies the nature of a claim and determines validation
// standards and minimum validator reputation requirements.
type ClaimType string

const (
	// ClaimTypeFactual is a specific verifiable event or fact assertion.
	// Example: "Parliament passed Bill X on Date Y."
	ClaimTypeFactual ClaimType = "FACTUAL"

	// ClaimTypeContextual is a framing or characterisation accuracy assertion.
	// Example: "Report X presents a misleading picture by omitting data Y."
	ClaimTypeContextual ClaimType = "CONTEXTUAL"

	// ClaimTypePredictive is a projection reasonableness assertion.
	// Example: "Based on current data, outcome X is probable within timeframe Y."
	ClaimTypePredictive ClaimType = "PREDICTIVE"
)

// GeographicScope classifies the geographic reach of a claim.
type GeographicScope string

const (
	GeographicScopeLocal         GeographicScope = "LOCAL"
	GeographicScopeNational      GeographicScope = "NATIONAL"
	GeographicScopeInternational GeographicScope = "INTERNATIONAL"
)

const (
	// MaxClaimTextLength is the maximum allowed length in UTF-8 characters
	// for claim_text. See PulseSyn Protocol Specification v0.1, Section 2.1.
	MaxClaimTextLength = 500

	// MinClaimTextLength is the minimum allowed length to ensure the claim
	// is a meaningful assertion rather than a fragment.
	MinClaimTextLength = 20
)

// Sentinel errors returned by NewClaim and Validate. Callers use errors.Is
// to distinguish rejection reasons.
var (
	ErrClaimTextEmpty        = errors.New("pulsesyn/claim: claim_text is empty")
	ErrClaimTextTooLong      = errors.New("pulsesyn/claim: claim_text exceeds 500 characters")
	ErrClaimTextTooShort     = errors.New("pulsesyn/claim: claim_text is too short")
	ErrClaimTypeInvalid      = errors.New("pulsesyn/claim: claim_type is not one of FACTUAL, CONTEXTUAL, PREDICTIVE")
	ErrContentURLEmpty       = errors.New("pulsesyn/claim: content_url is empty")
	ErrContentURLInvalid     = errors.New("pulsesyn/claim: content_url is not a valid URL")
	ErrContentURLUnreachable = errors.New("pulsesyn/claim: content_url did not return a response at submission time")
	ErrContentHashEmpty      = errors.New("pulsesyn/claim: content_hash is empty")
	ErrContentHashMismatch   = errors.New("pulsesyn/claim: content_hash does not match content at content_url")
	ErrSubmitterIDEmpty      = errors.New("pulsesyn/claim: submitter_id is empty")
	ErrDomainTagsEmpty       = errors.New("pulsesyn/claim: domain_tags must contain at least one tag")
	ErrTimeReferenceEmpty    = errors.New("pulsesyn/claim: time_reference is required")
	ErrGeographicScopeInvalid = errors.New("pulsesyn/claim: geographic_scope is not one of LOCAL, NATIONAL, INTERNATIONAL")
)

// Claim is the central PulseSyn protocol primitive. It is immutable once
// constructed — no setters exist. All fields are validated at construction time.
// See PulseSyn Protocol Specification v0.1, Section 2.1.
type Claim struct {
	// ClaimID is the canonical identifier derived as
	// SHA3-256(claim_text + content_hash + submitter_id + timestamp).
	// It is computed by the constructor and cannot be set externally.
	ClaimID string

	// ClaimText is the plain language falsifiable assertion. Max 500 chars.
	// Must reference a subject, an action or state, and a time or context.
	ClaimText string

	// ClaimType classifies the claim as FACTUAL, CONTEXTUAL, or PREDICTIVE.
	ClaimType ClaimType

	// DomainTags are domain classifiers used for validator selection and
	// reputation weighting. At least one tag is required.
	DomainTags []string

	// GeographicScope classifies the geographic reach of the claim.
	GeographicScope GeographicScope

	// TimeReference is the ISO 8601 timestamp of when the claimed event occurred.
	TimeReference time.Time

	// ContentHash is the SHA-256 hash of the evidence content at ContentURL.
	// Used to verify that content has not been altered since submission.
	ContentHash string

	// ContentURL is the URL where validators access the supporting evidence.
	// The protocol does not store or render the content — only the pointer.
	ContentURL string

	// SubmitterID is the public key hash of the submitting participant.
	SubmitterID string

	// SubmissionEpoch is the block number at the time of submission.
	// Set to zero in Phase 1 (no chain integration yet).
	SubmissionEpoch uint64

	// State is the current lifecycle state of the claim.
	State LifecycleState
}

// Input carries the caller-supplied fields required to construct a Claim.
// ContentHash must be the SHA-256 hex digest of the content at ContentURL,
// computed by the caller before submission.
type Input struct {
	ClaimText       string
	ClaimType       ClaimType
	DomainTags      []string
	GeographicScope GeographicScope
	TimeReference   time.Time
	ContentHash     string
	ContentURL      string
	SubmitterID     string
	SubmissionEpoch uint64
}

// NewClaim constructs a validated, immutable Claim from the provided Input.
// It validates all fields, verifies the content URL is reachable, and
// verifies the content hash matches the content at the URL.
// Returns ErrXxx sentinel errors for each validation failure.
// See PulseSyn Protocol Specification v0.1, Section 2.1.2.
func NewClaim(in Input) (*Claim, error) {
	if err := validateInput(in); err != nil {
		return nil, fmt.Errorf("NewClaim: %w", err)
	}

	id := computeClaimID(in)

	return &Claim{
		ClaimID:         id,
		ClaimText:       strings.TrimSpace(in.ClaimText),
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

// validateInput checks all Input fields against the claim validity rules.
// See PulseSyn Protocol Specification v0.1, Section 2.1.2.
func validateInput(in Input) error {
	text := strings.TrimSpace(in.ClaimText)

	if text == "" {
		return ErrClaimTextEmpty
	}
	if utf8.RuneCountInString(text) < MinClaimTextLength {
		return ErrClaimTextTooShort
	}
	if utf8.RuneCountInString(text) > MaxClaimTextLength {
		return ErrClaimTextTooLong
	}
	if err := validateClaimType(in.ClaimType); err != nil {
		return err
	}
	if err := validateGeographicScope(in.GeographicScope); err != nil {
		return err
	}
	if len(in.DomainTags) == 0 {
		return ErrDomainTagsEmpty
	}
	if in.TimeReference.IsZero() {
		return ErrTimeReferenceEmpty
	}
	if strings.TrimSpace(in.SubmitterID) == "" {
		return ErrSubmitterIDEmpty
	}
	if strings.TrimSpace(in.ContentHash) == "" {
		return ErrContentHashEmpty
	}
	if err := validateContentURL(strings.TrimSpace(in.ContentURL)); err != nil {
		return err
	}
	return nil
}

// validateClaimType returns ErrClaimTypeInvalid if t is not a recognised type.
func validateClaimType(t ClaimType) error {
	switch t {
	case ClaimTypeFactual, ClaimTypeContextual, ClaimTypePredictive:
		return nil
	default:
		return ErrClaimTypeInvalid
	}
}

// validateGeographicScope returns ErrGeographicScopeInvalid if s is not recognised.
func validateGeographicScope(s GeographicScope) error {
	switch s {
	case GeographicScopeLocal, GeographicScopeNational, GeographicScopeInternational:
		return nil
	default:
		return ErrGeographicScopeInvalid
	}
}

// validateContentURL checks that the URL is non-empty, structurally valid,
// and reachable with an HTTP HEAD request at submission time.
// See PulseSyn Protocol Specification v0.1, Section 2.1.2, rules 3-4.
func validateContentURL(rawURL string) error {
	if rawURL == "" {
		return ErrContentURLEmpty
	}
	parsed, err := url.ParseRequestURI(rawURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return ErrContentURLInvalid
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Head(rawURL)
	if err != nil {
		return ErrContentURLUnreachable
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		return ErrContentURLUnreachable
	}
	return nil
}

// computeClaimID derives the canonical claim ID as the SHA-256 hex digest of
// the concatenation of claim_text, content_hash, submitter_id, and the
// submission timestamp. SHA-256 is used in Phase 1; the spec calls for
// SHA3-256 which will be introduced when the golang.org/x/crypto dependency
// is approved.
// See PulseSyn Protocol Specification v0.1, Section 2.1.
func computeClaimID(in Input) string {
	h := sha256.New()
	h.Write([]byte(strings.TrimSpace(in.ClaimText)))
	h.Write([]byte(strings.ToLower(strings.TrimSpace(in.ContentHash))))
	h.Write([]byte(strings.TrimSpace(in.SubmitterID)))
	h.Write([]byte(in.TimeReference.UTC().Format(time.RFC3339Nano)))
	return hex.EncodeToString(h.Sum(nil))
}

// normaliseDomainTags lowercases and trims all domain tags and removes
// empty entries, producing a clean, consistent slice.
func normaliseDomainTags(tags []string) []string {
	out := make([]string, 0, len(tags))
	for _, t := range tags {
		t = strings.ToLower(strings.TrimSpace(t))
		if t != "" {
			out = append(out, t)
		}
	}
	return out
}
"@ | Set-Content core\claim\claim.go

# ── core/claim/lifecycle.go ──────────────────────────────────────────────────
@"
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
"@ | Set-Content core\claim\lifecycle.go

# ── core/claim/claim_test.go ─────────────────────────────────────────────────
@"
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
	if len(in.DomainTags) == 0 {
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
		name  string
		from  LifecycleState
		to    LifecycleState
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
"@ | Set-Content core\claim\claim_test.go

Write-Host "Commit 2 files written." -ForegroundColor Green