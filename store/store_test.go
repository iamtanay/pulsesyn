package store

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/iamtanay/pulsesyn/core/claim"
	"github.com/iamtanay/pulsesyn/core/reputation"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// openTestStore opens a fresh Store in a temp directory. The store is
// automatically closed when the test ends.
func openTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	return s
}

// sampleClaim builds a valid claim.Claim struct directly without calling
// NewClaim (which performs a live HTTP check). Used only in store tests.
func sampleClaim(claimID string) *claim.Claim {
	return &claim.Claim{
		ClaimID:         claimID,
		ClaimText:       "Parliament passed the Infrastructure Reform Act on 2026-01-15.",
		ClaimType:       claim.ClaimTypeFactual,
		DomainTags:      []string{"politics", "legislation"},
		GeographicScope: claim.GeographicScopeNational,
		TimeReference:   time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC),
		ContentHash:     "abc123def456abc123def456abc123def456abc123def456abc123def456abc1",
		ContentURL:      "https://example.com/evidence/infra-act",
		SubmitterID:     "submitter-pubkey-hash-001",
		SubmissionEpoch: 1000,
		State:           claim.StateSubmitted,
	}
}

// sampleValidatorRecord builds a valid reputation.ValidatorRecord for tests.
func sampleValidatorRecord(validatorID string) *reputation.ValidatorRecord {
	return &reputation.ValidatorRecord{
		ValidatorID:       validatorID,
		GenesisValidator:  false,
		RegistrationEpoch: 100,
		DomainScores: reputation.DomainReputation{
			"politics":    0.75,
			"legislation": 0.80,
		},
		GlobalReputation: 0.775,
		TotalValidations: 50,
		Status:           reputation.ValidatorStatusActive,
	}
}

// sampleValidationRecord builds a valid ValidationRecord for tests.
func sampleValidationRecord(claimID, domain string) ValidationRecord {
	return ValidationRecord{
		ClaimID:           claimID,
		Domain:            domain,
		Verdict:           "SUPPORTED",
		ConfidenceScore:   0.82,
		ParticipationRate: 0.90,
		ValidatorCount:    9,
		Epoch:             2000,
		MerkleRoot:        "",
		FinalizedAt:       time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC),
	}
}

// sampleVoteRecord builds a valid VoteRecord for tests.
func sampleVoteRecord(claimID, validatorID string) VoteRecord {
	return VoteRecord{
		ClaimID:          claimID,
		ValidatorID:      validatorID,
		Verdict:          "SUPPORTED",
		Confidence:       0.85,
		DomainReputation: 0.75,
		BiasCoefficient:  0.05,
		ValidatorSetSize: 10,
		Epoch:            2000,
		RecordedAt:       time.Date(2026, 1, 15, 11, 30, 0, 0, time.UTC),
	}
}

// --- Open / Close ---

func TestOpen_ValidDir(t *testing.T) {
	s := openTestStore(t)
	assert.NotNil(t, s)
}

func TestOpen_InvalidDir(t *testing.T) {
	// Point BadgerDB at a plain file instead of a directory — must fail.
	tmp := t.TempDir()
	filePath := filepath.Join(tmp, "not_a_dir.txt")
	require.NoError(t, os.WriteFile(filePath, []byte("not a directory"), 0644))
	_, err := Open(filePath)
	assert.Error(t, err)
}

// --- Claim CRUD ---

func TestWriteClaim_OK(t *testing.T) {
	s := openTestStore(t)
	c := sampleClaim("claim-001")
	require.NoError(t, s.WriteClaim(c))
	got, err := s.ReadClaim("claim-001")
	require.NoError(t, err)
	assert.Equal(t, c.ClaimID, got.ClaimID)
	assert.Equal(t, c.ClaimText, got.ClaimText)
	assert.Equal(t, c.ClaimType, got.ClaimType)
}

func TestWriteClaim_AlreadyExists(t *testing.T) {
	s := openTestStore(t)
	c := sampleClaim("claim-dup")
	require.NoError(t, s.WriteClaim(c))
	err := s.WriteClaim(c)
	require.ErrorIs(t, err, ErrAlreadyExists)
}

func TestReadClaim_NotFound(t *testing.T) {
	s := openTestStore(t)
	_, err := s.ReadClaim("nonexistent")
	require.ErrorIs(t, err, ErrClaimNotFound)
}

func TestDeleteClaim_OK(t *testing.T) {
	s := openTestStore(t)
	c := sampleClaim("claim-del")
	require.NoError(t, s.WriteClaim(c))
	require.NoError(t, s.DeleteClaim("claim-del"))
	_, err := s.ReadClaim("claim-del")
	require.ErrorIs(t, err, ErrClaimNotFound)
}

func TestDeleteClaim_NotFound(t *testing.T) {
	s := openTestStore(t)
	err := s.DeleteClaim("nonexistent")
	require.ErrorIs(t, err, ErrClaimNotFound)
}

// --- ValidatorRecord CRUD ---

func TestWriteValidatorRecord_OK(t *testing.T) {
	s := openTestStore(t)
	r := sampleValidatorRecord("validator-001")
	require.NoError(t, s.WriteValidatorRecord(r))
	got, err := s.ReadValidatorRecord("validator-001")
	require.NoError(t, err)
	assert.Equal(t, r.ValidatorID, got.ValidatorID)
	assert.Equal(t, r.GlobalReputation, got.GlobalReputation)
	assert.Equal(t, r.DomainScores["politics"], got.DomainScores["politics"])
}

func TestWriteValidatorRecord_Upsert(t *testing.T) {
	s := openTestStore(t)
	r := sampleValidatorRecord("validator-upsert")
	require.NoError(t, s.WriteValidatorRecord(r))

	// Update the record with new reputation scores.
	updated := *r
	updated.DomainScores = reputation.DomainReputation{"politics": 0.90}
	updated.GlobalReputation = 0.90
	require.NoError(t, s.WriteValidatorRecord(&updated))

	got, err := s.ReadValidatorRecord("validator-upsert")
	require.NoError(t, err)
	assert.Equal(t, 0.90, got.DomainScores["politics"])
}

func TestReadValidatorRecord_NotFound(t *testing.T) {
	s := openTestStore(t)
	_, err := s.ReadValidatorRecord("nonexistent")
	require.ErrorIs(t, err, ErrValidatorNotFound)
}

func TestDeleteValidatorRecord_OK(t *testing.T) {
	s := openTestStore(t)
	r := sampleValidatorRecord("validator-del")
	require.NoError(t, s.WriteValidatorRecord(r))
	require.NoError(t, s.DeleteValidatorRecord("validator-del"))
	_, err := s.ReadValidatorRecord("validator-del")
	require.ErrorIs(t, err, ErrValidatorNotFound)
}

// --- ValidationRecord CRUD ---

func TestWriteValidationRecord_OK(t *testing.T) {
	s := openTestStore(t)
	r := sampleValidationRecord("claim-val-001", "politics")
	require.NoError(t, s.WriteValidationRecord(r))
	got, err := s.ReadValidationRecord("claim-val-001")
	require.NoError(t, err)
	assert.Equal(t, r.ClaimID, got.ClaimID)
	assert.Equal(t, r.Verdict, got.Verdict)
	assert.Equal(t, r.ConfidenceScore, got.ConfidenceScore)
}

func TestWriteValidationRecord_AlreadyExists(t *testing.T) {
	s := openTestStore(t)
	r := sampleValidationRecord("claim-val-dup", "politics")
	require.NoError(t, s.WriteValidationRecord(r))
	err := s.WriteValidationRecord(r)
	require.ErrorIs(t, err, ErrAlreadyExists)
}

func TestReadValidationRecord_NotFound(t *testing.T) {
	s := openTestStore(t)
	_, err := s.ReadValidationRecord("nonexistent")
	require.ErrorIs(t, err, ErrValidationNotFound)
}

func TestValidationsByDomain_OK(t *testing.T) {
	s := openTestStore(t)

	r1 := sampleValidationRecord("claim-domain-a", "politics")
	r2 := sampleValidationRecord("claim-domain-b", "politics")
	r3 := sampleValidationRecord("claim-domain-c", "science")
	require.NoError(t, s.WriteValidationRecord(r1))
	require.NoError(t, s.WriteValidationRecord(r2))
	require.NoError(t, s.WriteValidationRecord(r3))

	results, err := s.ValidationsByDomain("politics")
	require.NoError(t, err)
	assert.Len(t, results, 2)
}

func TestValidationsByDomain_Empty(t *testing.T) {
	s := openTestStore(t)
	results, err := s.ValidationsByDomain("nonexistent-domain")
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestValidationsByEpoch_OK(t *testing.T) {
	s := openTestStore(t)

	r1 := sampleValidationRecord("claim-epoch-a", "politics")
	r1.Epoch = 5000
	r2 := sampleValidationRecord("claim-epoch-b", "politics")
	r2.Epoch = 5000
	r3 := sampleValidationRecord("claim-epoch-c", "science")
	r3.Epoch = 6000
	require.NoError(t, s.WriteValidationRecord(r1))
	require.NoError(t, s.WriteValidationRecord(r2))
	require.NoError(t, s.WriteValidationRecord(r3))

	results, err := s.ValidationsByEpoch(5000)
	require.NoError(t, err)
	assert.Len(t, results, 2)

	results6, err := s.ValidationsByEpoch(6000)
	require.NoError(t, err)
	assert.Len(t, results6, 1)
}

// --- VoteRecord CRUD ---

func TestWriteVote_OK(t *testing.T) {
	s := openTestStore(t)
	v := sampleVoteRecord("claim-v-001", "validator-001")
	require.NoError(t, s.WriteVote(v))
	got, err := s.ReadVote("claim-v-001", "validator-001")
	require.NoError(t, err)
	assert.Equal(t, v.ClaimID, got.ClaimID)
	assert.Equal(t, v.ValidatorID, got.ValidatorID)
	assert.Equal(t, v.Verdict, got.Verdict)
	assert.Equal(t, v.Confidence, got.Confidence)
}

func TestWriteVote_AlreadyExists(t *testing.T) {
	s := openTestStore(t)
	v := sampleVoteRecord("claim-v-dup", "validator-dup")
	require.NoError(t, s.WriteVote(v))
	err := s.WriteVote(v)
	require.ErrorIs(t, err, ErrAlreadyExists)
}

func TestReadVote_NotFound(t *testing.T) {
	s := openTestStore(t)
	_, err := s.ReadVote("nonexistent-claim", "nonexistent-validator")
	require.ErrorIs(t, err, ErrVoteNotFound)
}

func TestVotesForClaim_OK(t *testing.T) {
	s := openTestStore(t)
	claimID := "claim-multi-vote"
	for i, vid := range []string{"v-001", "v-002", "v-003"} {
		v := sampleVoteRecord(claimID, vid)
		v.Confidence = 0.7 + float64(i)*0.05
		require.NoError(t, s.WriteVote(v))
	}
	// Different claim — must not appear in results.
	require.NoError(t, s.WriteVote(sampleVoteRecord("other-claim", "v-001")))

	votes, err := s.VotesForClaim(claimID)
	require.NoError(t, err)
	assert.Len(t, votes, 3)
}

func TestVotesByValidator_OK(t *testing.T) {
	s := openTestStore(t)
	validatorID := "validator-multi-claim"
	for _, cid := range []string{"claim-x", "claim-y", "claim-z"} {
		require.NoError(t, s.WriteVote(sampleVoteRecord(cid, validatorID)))
	}
	// Different validator — must not appear in results.
	require.NoError(t, s.WriteVote(sampleVoteRecord("claim-x", "other-validator")))

	votes, err := s.VotesByValidator(validatorID)
	require.NoError(t, err)
	assert.Len(t, votes, 3)
}

// --- State recovery ---

func TestStateRecovery(t *testing.T) {
	dir := t.TempDir()

	// Write data to the store.
	s1, err := Open(dir)
	require.NoError(t, err)
	c := sampleClaim("claim-persist")
	require.NoError(t, s1.WriteClaim(c))
	require.NoError(t, s1.WriteValidatorRecord(sampleValidatorRecord("validator-persist")))
	require.NoError(t, s1.WriteValidationRecord(sampleValidationRecord("claim-persist", "politics")))
	require.NoError(t, s1.WriteVote(sampleVoteRecord("claim-persist", "validator-persist")))
	require.NoError(t, s1.Close())

	// Reopen and verify all records are intact.
	s2, err := Open(dir)
	require.NoError(t, err)
	defer s2.Close()

	gotClaim, err := s2.ReadClaim("claim-persist")
	require.NoError(t, err)
	assert.Equal(t, c.ClaimID, gotClaim.ClaimID)

	gotValidator, err := s2.ReadValidatorRecord("validator-persist")
	require.NoError(t, err)
	assert.Equal(t, "validator-persist", gotValidator.ValidatorID)

	gotValidation, err := s2.ReadValidationRecord("claim-persist")
	require.NoError(t, err)
	assert.Equal(t, "SUPPORTED", gotValidation.Verdict)

	gotVote, err := s2.ReadVote("claim-persist", "validator-persist")
	require.NoError(t, err)
	assert.Equal(t, "SUPPORTED", gotVote.Verdict)
}

// --- Validation input errors ---

func TestWriteValidationRecord_InvalidVerdict(t *testing.T) {
	s := openTestStore(t)
	r := sampleValidationRecord("claim-bad", "politics")
	r.Verdict = "WRONG"
	err := s.WriteValidationRecord(r)
	require.ErrorIs(t, err, ErrInvalidRecord)
}

func TestWriteValidationRecord_EmptyClaimID(t *testing.T) {
	s := openTestStore(t)
	r := sampleValidationRecord("", "politics")
	err := s.WriteValidationRecord(r)
	require.ErrorIs(t, err, ErrInvalidRecord)
}

func TestWriteVote_InvalidConfidence(t *testing.T) {
	s := openTestStore(t)
	v := sampleVoteRecord("claim-bad-v", "validator-001")
	v.Confidence = 1.5
	err := s.WriteVote(v)
	require.ErrorIs(t, err, ErrInvalidRecord)
}
