// Package chain provides the Go adapter interface between the PulseSyn protocol
// core and the Solidity smart contracts deployed on the Ethereum L2. Protocol
// code never interacts with the chain directly. This package defines the
// Adapter interface, all on-chain record types, and a NullAdapter for
// development use. The concrete EthereumAdapter is Phase 2.5 work — see
// DECISIONS.md: 2026-05-31.
// Dependency rules: stdlib only.
package chain
