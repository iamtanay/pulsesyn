// Package network implements the PulseSyn peer layer using libp2p. It handles
// peer discovery via DHT, connection multiplexing, protocol message serialisation,
// and message signing and verification. All nine protocol message types are
// defined here. Network imports from core/claim for message types only.
package network
