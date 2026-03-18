// Package simulation implements the PulseSyn simulation harness. It runs the
// protocol across thousands of synthetic validation rounds to validate that the
// consensus algorithm, reputation engine, and bias detection behave correctly
// under configurable network conditions including collusion and bias scenarios.
// It imports from core/* and selector only — never from session, network, or chain.
package simulation
