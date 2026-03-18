// Package merkle implements Merkle proof generation and verification for
// PulseSyn validator vote sets. It proves that a specific set of votes produced
// a specific verdict without revealing individual votes, producing the
// merkle_root stored in every ValidationRecord.
package merkle
