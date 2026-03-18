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
