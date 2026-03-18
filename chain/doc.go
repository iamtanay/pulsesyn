// Package chain provides the Go adapter between the PulseSyn protocol core and
// the Solidity smart contracts deployed on the Ethereum L2. Protocol code
// never interacts with the chain directly — all on-chain reads and writes go
// through this package. It imports from core/claim for record types only.
package chain
