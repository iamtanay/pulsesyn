# PulseSyn

An open, decentralized protocol for validating claims.

PulseSyn takes a structured, falsifiable claim submitted with a pointer to
supporting evidence, runs it through a reputation-weighted validator network,
and produces a permanent, tamper-proof verdict.

**Protocol specification:** docs/
**Development plan:** docs/
**License:** MIT Open Protocol License
**Author:** Baniloo — baniloo.com

## Status

Phase 1 — Foundation (in progress)

## Repository structure

    core/         Protocol heart — isolated, no external dependencies
    network/      Peer layer — libp2p integration
    selector/     Validator selection algorithm
    session/      Validation session orchestration
    store/        Local state — embedded DB
    chain/        Chain adapter — Solidity contracts + Go interface
    merkle/       Merkle proof generation and verification
    api/          JSON-RPC API server
    sdk/          Developer SDK
    simulation/   Simulation harness
    tests/        Integration and security tests
    docs/         Protocol documentation and session summaries

## Development

Every session begins with the coding standards document in context.
Every design decision is logged in DECISIONS.md before the session ends.
