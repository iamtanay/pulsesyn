// Package store manages local protocol node state using an embedded key-value
// database. It provides CRUD operations for Claim, ValidatorRecord, Vote, and
// ValidationRecord structs, indexed by claim_id, validator_id, domain, and
// epoch. It enables full state recovery on node restart. It imports from
// core/claim and core/reputation for struct definitions only.
package store
