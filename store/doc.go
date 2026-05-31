// Package store manages local protocol node state using BadgerDB v4, an
// embedded LSM-tree key-value database. It provides CRUD operations for
// Claim, ValidatorRecord, Vote, and ValidationRecord, indexed by claim_id,
// validator_id, domain, and epoch. It enables full state recovery on node restart.
// Dependency rules: stdlib + core/claim + core/reputation + BadgerDB v4.
package store
