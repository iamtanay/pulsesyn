// Package simulation provides a configurable harness for validating the
// PulseSyn protocol's mathematical model at scale.
//
// The simulation is NOT part of the production protocol. It exists to verify
// that the consensus algorithm, reputation engine, and bias detection module
// produce correct emergent behaviour across thousands of synthetic validation
// rounds before any networking, storage, or chain infrastructure is built
// around them.
//
// The simulation:
//   - Generates a synthetic pool of validators with configurable reputation
//     distributions and domain expertise.
//   - Runs configurable validation scenarios including honest networks,
//     colluding validator subgroups, and concentrated bias distributions.
//   - Collects accuracy, reputation convergence, and bias detection metrics
//     across all rounds and produces a structured Report.
//
// Dependency rules: this package imports core/consensus, core/reputation,
// and core/bias only. It never imports session, network, chain, or api.
// See PulseSyn Coding Standards v1.0, Dependency Direction section.
package simulation
