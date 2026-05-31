package store

import "fmt"

// Key prefix constants define the namespace for every record type and
// secondary index stored in BadgerDB. Prefixes are chosen to sort
// lexicographically into distinct, non-overlapping ranges.
const (
	prefixClaim      = "claim/"
	prefixValidator  = "validator/"
	prefixValidation = "validation/"
	prefixVote       = "vote/"

	// Secondary index prefixes.
	prefixIdxDomain          = "idx/domain/"
	prefixIdxEpoch           = "idx/epoch/"
	prefixIdxVoteByValidator = "idx/vote_by_validator/"
)

// claimKey returns the primary key for a Claim record.
func claimKey(claimID string) []byte {
	return []byte(prefixClaim + claimID)
}

// validatorKey returns the primary key for a ValidatorRecord.
func validatorKey(validatorID string) []byte {
	return []byte(prefixValidator + validatorID)
}

// validationKey returns the primary key for a ValidationRecord.
func validationKey(claimID string) []byte {
	return []byte(prefixValidation + claimID)
}

// voteKey returns the primary key for a VoteRecord (claim + validator pair).
func voteKey(claimID, validatorID string) []byte {
	return []byte(prefixVote + claimID + "/" + validatorID)
}

// idxDomainKey returns the secondary index key for looking up ValidationRecords
// by domain. Value is empty; the claimID is embedded in the key suffix.
func idxDomainKey(domain, claimID string) []byte {
	return []byte(prefixIdxDomain + domain + "/" + claimID)
}

// idxEpochKey returns the secondary index key for looking up ValidationRecords
// by epoch. Epoch is zero-padded to 16 hex digits for lexicographic ordering.
func idxEpochKey(epoch uint64, claimID string) []byte {
	return []byte(fmt.Sprintf("%s%016x/%s", prefixIdxEpoch, epoch, claimID))
}

// idxVoteByValidatorKey returns the secondary index key for looking up
// VoteRecords by validator. Value is empty; the claimID is in the key suffix.
func idxVoteByValidatorKey(validatorID, claimID string) []byte {
	return []byte(prefixIdxVoteByValidator + validatorID + "/" + claimID)
}

// claimIDFromDomainIndex extracts the claimID from a domain index key.
// key must begin with the full prefix including the domain segment.
func claimIDFromDomainIndex(key []byte, domain string) string {
	prefix := prefixIdxDomain + domain + "/"
	return string(key[len(prefix):])
}

// claimIDFromEpochIndex extracts the claimID from an epoch index key.
func claimIDFromEpochIndex(key []byte, epoch uint64) string {
	prefix := fmt.Sprintf("%s%016x/", prefixIdxEpoch, epoch)
	return string(key[len(prefix):])
}

// claimIDFromVoteByValidatorIndex extracts the claimID from a vote-by-validator
// index key.
func claimIDFromVoteByValidatorIndex(key []byte, validatorID string) string {
	prefix := prefixIdxVoteByValidator + validatorID + "/"
	return string(key[len(prefix):])
}
