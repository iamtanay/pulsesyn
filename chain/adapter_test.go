package chain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func sampleClaimRecord() ClaimRecord {
	return ClaimRecord{
		ClaimID:     "claim-001",
		ContentHash: "abc123def456",
		SubmitterID: "submitter-pubkey-001",
		ClaimType:   "FACTUAL",
		Domain:      "politics",
		Epoch:       1000,
		SubmittedAt: time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC),
	}
}

func sampleValidationRecord() ValidationRecord {
	return ValidationRecord{
		ClaimID:         "claim-001",
		Verdict:         "SUPPORTED",
		ConfidenceScore: 0.85,
		MerkleRoot:      "deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
		Epoch:           2000,
		FinalizedAt:     time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC),
	}
}

func sampleReputationUpdate() ReputationUpdate {
	return ReputationUpdate{
		ValidatorID: "validator-001",
		Domain:      "politics",
		OldScore:    0.75,
		NewScore:    0.80,
		Reason:      "CORRECT_HIGH_CONFIDENCE",
		Epoch:       2000,
	}
}

// --- ClaimRecord.Validate ---

func TestClaimRecord_Validate_Valid(t *testing.T) {
	assert.NoError(t, sampleClaimRecord().Validate())
}

func TestClaimRecord_Validate_EmptyClaimID(t *testing.T) {
	r := sampleClaimRecord()
	r.ClaimID = ""
	require.ErrorIs(t, r.Validate(), ErrInvalidRecord)
}

func TestClaimRecord_Validate_EmptyContentHash(t *testing.T) {
	r := sampleClaimRecord()
	r.ContentHash = ""
	require.ErrorIs(t, r.Validate(), ErrInvalidRecord)
}

func TestClaimRecord_Validate_EmptySubmitterID(t *testing.T) {
	r := sampleClaimRecord()
	r.SubmitterID = ""
	require.ErrorIs(t, r.Validate(), ErrInvalidRecord)
}

func TestClaimRecord_Validate_EmptyDomain(t *testing.T) {
	r := sampleClaimRecord()
	r.Domain = ""
	require.ErrorIs(t, r.Validate(), ErrInvalidRecord)
}

// --- ValidationRecord.Validate ---

func TestValidationRecord_Validate_Valid(t *testing.T) {
	assert.NoError(t, sampleValidationRecord().Validate())
}

func TestValidationRecord_Validate_EmptyClaimID(t *testing.T) {
	r := sampleValidationRecord()
	r.ClaimID = ""
	require.ErrorIs(t, r.Validate(), ErrInvalidRecord)
}

func TestValidationRecord_Validate_EmptyVerdict(t *testing.T) {
	r := sampleValidationRecord()
	r.Verdict = ""
	require.ErrorIs(t, r.Validate(), ErrInvalidRecord)
}

func TestValidationRecord_Validate_ConfidenceOutOfRange(t *testing.T) {
	r := sampleValidationRecord()
	r.ConfidenceScore = 1.5
	require.ErrorIs(t, r.Validate(), ErrInvalidRecord)
}

func TestValidationRecord_Validate_EmptyMerkleRoot(t *testing.T) {
	r := sampleValidationRecord()
	r.MerkleRoot = ""
	require.ErrorIs(t, r.Validate(), ErrInvalidRecord)
}

// --- ReputationUpdate.Validate ---

func TestReputationUpdate_Validate_Valid(t *testing.T) {
	assert.NoError(t, sampleReputationUpdate().Validate())
}

func TestReputationUpdate_Validate_EmptyValidatorID(t *testing.T) {
	u := sampleReputationUpdate()
	u.ValidatorID = ""
	require.ErrorIs(t, u.Validate(), ErrInvalidRecord)
}

func TestReputationUpdate_Validate_EmptyDomain(t *testing.T) {
	u := sampleReputationUpdate()
	u.Domain = ""
	require.ErrorIs(t, u.Validate(), ErrInvalidRecord)
}

func TestReputationUpdate_Validate_NewScoreOutOfRange(t *testing.T) {
	u := sampleReputationUpdate()
	u.NewScore = -0.1
	require.ErrorIs(t, u.Validate(), ErrInvalidRecord)
}

// --- NullAdapter ---

func TestNullAdapter_WriteClaimRecord(t *testing.T) {
	a := &NullAdapter{}
	txHash, err := a.WriteClaimRecord(sampleClaimRecord())
	require.NoError(t, err)
	assert.NotEmpty(t, txHash)
}

func TestNullAdapter_WriteValidationRecord(t *testing.T) {
	a := &NullAdapter{}
	txHash, err := a.WriteValidationRecord(sampleValidationRecord())
	require.NoError(t, err)
	assert.NotEmpty(t, txHash)
}

func TestNullAdapter_UpdateReputation(t *testing.T) {
	a := &NullAdapter{}
	txHash, err := a.UpdateReputation(sampleReputationUpdate())
	require.NoError(t, err)
	assert.NotEmpty(t, txHash)
}

func TestNullAdapter_ReadValidationRecord_NotFound(t *testing.T) {
	a := &NullAdapter{}
	_, err := a.ReadValidationRecord("claim-001")
	require.ErrorIs(t, err, ErrValidationNotFound)
}

func TestNullAdapter_ReadReputation_NotFound(t *testing.T) {
	a := &NullAdapter{}
	_, err := a.ReadReputation("validator-001", "politics")
	require.ErrorIs(t, err, ErrReputationNotFound)
}
