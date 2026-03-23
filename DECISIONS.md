# PulseSyn — Design Decision Log

Every significant design decision made during development is recorded here.
This log is the primary evidence that PulseSyn's design decisions came from a human designer over time.

---

## 2026-03-17 — Go module path set to github.com/iamtanay/pulsesyn

**Decision:** Use `github.com/iamtanay/pulsesyn` as the canonical module path.

**Why:** Matches the intended public repository and follows Go module conventions.

**Alternatives considered:** `baniloo.com/pulsesyn` — deferred until custom domain module proxy is configured.

**Consequences:** All internal imports use this prefix. Changing it later requires a global find-and-replace.

---

## 2026-03-17 — Project scaffold established

**Decision:** Full directory tree created per coding standards v1.0. All packages have `doc.go` files.

**Why:** Enforces the core isolation principle from commit one. Boundaries are clear before any logic is written.

**Alternatives considered:** Flat structure — rejected. Protocol complexity requires strict separation of concerns.

**Consequences:** New top-level packages require a decision log entry explaining why they cannot fit an existing package.

---

## 2026-03-18 — SHA-256 used in Phase 1 instead of SHA3-256

**Decision:** `computeClaimID` uses SHA-256 (stdlib) in Phase 1.

**Why:** SHA3-256 requires `golang.org/x/crypto`, not yet approved as a dependency.

**Alternatives considered:** Adding `golang.org/x/crypto` immediately — deferred until dependency policy is decided.

**Consequences:** Phase 1 ClaimIDs will differ from production. Must migrate before Phase 2 chain integration.

---

## 2026-03-18 — ReputationFloor applies to update and decay operations only

**Decision:** `ReputationFloor` (0.15) enforced in `ApplyPostFinalizationUpdate` and `ApplyDecay` only. Internal writes clamp to `[0.0, ReputationCeiling]`.

**Why:** The floor protects established validators, not new ones. Clamping new validators to 0.15 on write would grant eligibility without earning it, contradicting spec §3.3.

**Alternatives considered:** Clamping to floor on all writes — rejected after the test suite caught it as a protocol correctness bug.

**Consequences:** Any new operation modifying domain scores must explicitly decide whether to enforce the floor.

---

## 2026-03-18 — Late normalization in consensus weight aggregation

**Decision:** Vote weights are not normalized before aggregation. Normalization happens at the decision boundary only (`winnerMass / TotalMass`).

**Why:** The majority check is a relative comparison. Pre-normalizing requires two passes. Single-pass produces identical results.

**Alternatives considered:** Per-vote normalization — rejected. Cross-session weight comparisons never happen inside `core/consensus`.

**Consequences:** Any future component comparing raw weight totals across sessions of different sizes must add normalization at that boundary.

---

## 2026-03-24 — Bias coefficient uses linear verdict score encoding

**Decision:** Verdicts map to a single axis: SUPPORTED=1.0, UNSUPPORTED=0.0, MISLEADING=0.5, INDETERMINATE=0.5. Deviation is the absolute difference from the population average on this axis.

**Why:** Minimal faithful encoding of the support dimension. MISLEADING and INDETERMINATE at the midpoint is epistemically conservative — neither strong support nor strong rejection.

**Alternatives considered:** Multi-axis encoding — deferred until simulation data justifies the complexity.

**Consequences:** Bias detection is blind to MISLEADING vs INDETERMINATE voting patterns. Documented as a known limitation.

---

## 2026-03-24 — Bias window uses FIFO eviction

**Decision:** The sliding window evicts the oldest observation when full. No recency weighting inside the window.

**Why:** Direct implementation of the spec's "last N validations" language.

**Alternatives considered:** Exponential recency weighting — deferred. If simulation shows FIFO is too slow to detect behaviour changes, can be added with a spec amendment.

**Consequences:** A validator takes up to `MaxWindowSize` (default 50) rounds to fully clear a bias flag after correcting their behaviour.

---

## 2026-03-24 — testify added as first external dependency

**Decision:** `github.com/stretchr/testify` v1.9.0 added to `go.mod`.

**Why:** Already used in all Session 1 test files. Formalising it is required for `go test ./...` to pass.

**Alternatives considered:** stdlib `testing` only — rejected. `require.NoError` and `assert.Equal` are significantly more readable for protocol-level tests.

**Consequences:** First external dependency in the module. Future test dependencies follow the same approval process.

---

## 2026-03-24 — Simulation uses uniform random validator selection in Phase 1

**Decision:** Simulation selects validators via `rand.Perm` rather than the composite score algorithm from spec §3.4.

**Why:** The composite score selector is Phase 3 work (`selector/`). Uniform selection is sufficient to validate the consensus math and reputation convergence properties.

**Alternatives considered:** Stub composite score inside simulation — rejected. Would duplicate logic and diverge from the real implementation.

**Consequences:** Simulation accuracy results may be slightly optimistic — uniform selection includes low-reputation validators more often than the real protocol would.

---

## 2026-03-24 — injectDomainReputation uses public API only

**Decision:** Simulation initialises validator reputation by calling `ApplyPostFinalizationUpdate` repeatedly rather than setting scores directly.

**Why:** Simulation is production code, not a test package. It must use only the public API of `core/reputation`. No test-only setters on immutable structs.

**Alternatives considered:** Export a `SetDomainScore` function — rejected. Violates the immutability invariant and creates a mutation path that could be misused.

**Consequences:** Validator initialisation is approximate (±0.05 of target). Acceptable — simulation measures statistical behaviour, not individual scores.
