## 2026-03-24 — Bias coefficient uses linear verdict score encoding

**Decision:** The bias module maps verdicts to a single numeric axis:
SUPPORTED=1.0, UNSUPPORTED=0.0, MISLEADING=0.5, INDETERMINATE=0.5.
Deviation is the absolute difference between the validator's verdict
score and the population average score on the same axis.

**Why:** The spec defines bias_coefficient as
`|mean_vote_deviation| / max_possible_deviation`. A single linear axis
is the minimal faithful encoding of the support dimension. MISLEADING and
INDETERMINATE are placed at the midpoint (0.5) because they represent
neither strong support nor strong rejection — this is epistemically
conservative and avoids encoding arbitrary ordering between the two
non-binary verdicts.

**Alternatives considered:** Multi-axis encoding (e.g. support × framing
quality) — deferred. The spec does not specify the axis structure and the
additional complexity is not justified in Phase 1. Can be revisited with
empirical data from simulation runs.

**Consequences:** Bias detection is currently blind to patterns in
MISLEADING vs INDETERMINATE voting — a validator who systematically
flips between these two verdicts will not be detected as biased. This is
a known limitation documented in the protocol's known limitations section.

---

## 2026-03-24 — Bias Window uses FIFO eviction, not weighted decay

**Decision:** The sliding window evicts the oldest observation when full
(FIFO). No recency weighting is applied to observations within the window.

**Why:** The spec says "sliding window of last N validations". FIFO is
the direct implementation of this definition. Recency-weighted decay
inside the window adds complexity without a spec basis.

**Alternatives considered:** Exponential recency weighting — rejected
in Phase 1. If the simulation shows that FIFO windows are too slow to
detect behaviour changes, weighted decay can be introduced with a spec
amendment.

**Consequences:** A validator who corrects their behaviour will take up
to MaxWindowSize rounds to fully clear their bias record. The default
window of 50 means approximately 50 validations to clear a bias flag.

---

## 2026-03-24 — testify added as the first external dependency

**Decision:** `github.com/stretchr/testify` v1.9.0 added to go.mod.

**Why:** All Session 1 packages use testify/require and testify/assert.
The dependency was already present in the test files from Session 1.
Formalising it in go.mod is required for `go test ./...` to pass.

**Alternatives considered:** stdlib `testing` package only — rejected.
Table-driven tests with require.NoError and assert.Equal are significantly
more readable than manual if/t.Fatal patterns for protocol-level tests.

**Consequences:** First external dependency in the module. All future
test dependencies must follow the same approval process.

---

## 2026-03-24 — Simulation uses uniform random validator selection in Phase 1

**Decision:** The simulation selects validators via uniform random draw
(rand.Perm) rather than the full composite score algorithm defined in
spec Section 3.4.

**Why:** The composite score selector is Phase 3 work (pulsesyn/selector).
The simulation's purpose in Phase 1 is to validate the consensus math and
reputation convergence properties — uniform selection is sufficient for
this. The selector package will be exercised in Phase 3 integration tests
where end-to-end selection behaviour matters.

**Alternatives considered:** Stub composite score selection inside
simulation — rejected. Stubbing the selection formula inside simulation
would duplicate logic and diverge from the real implementation. Clean
separation is preferable.

**Consequences:** Simulation accuracy results may be slightly optimistic
compared to a reputation-weighted selection because uniform selection
includes low-reputation validators more often than the real protocol
would. This is documented in the simulation package's doc.go.

---

## 2026-03-24 — injectDomainReputation uses public API only

**Decision:** The simulation initialises validator reputation scores by
calling `ApplyPostFinalizationUpdate` repeatedly rather than setting
scores directly via unexported fields or a testing-only setter.

**Why:** core/ packages are not allowed to export test helpers that
bypass their own invariants. The simulation is not a test package — it
is production code and must use only the public API of core/reputation.
The convergence achieved is sufficient for the simulation's statistical
purposes.

**Alternatives considered:** Export a `SetDomainScore` function on
ValidatorRecord — rejected. This would violate the immutability invariant
and add a mutation path that could be misused outside tests.

**Consequences:** Validator initialisation is approximate (±0.05 of the
target score). This is acceptable because the simulation measures
statistical emergent behaviour, not individual validator scores.
