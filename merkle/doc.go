// Package merkle implements Merkle proof generation and verification for
// PulseSyn validator vote sets. A vote set is committed as a Merkle tree root
// stored in every ValidationRecord, enabling cryptographic inclusion proofs
// for individual votes without exposing the full set.
// Dependency rules: stdlib only.
package merkle
