# PulseSyn — Design Decision Log

Every significant design decision made during development is recorded here
with date, rationale, alternatives considered, and consequences.
This log is the primary evidence that PulseSyn's design decisions came
from a human designer over time.

---

## 2026-03-17 — Go module path set to github.com/iamtanay/pulsesyn

**Decision:** Use `github.com/iamtanay/pulsesyn` as the canonical module path.

**Why:** Matches the intended public repository location and follows Go module
conventions. Consistent with the MIT open protocol release target.

**Alternatives considered:** `baniloo.com/pulsesyn` (custom domain path) —
deferred until custom domain module proxy is configured.

**Consequences:** All internal imports use this path prefix. Changing it later
requires a global find-and-replace across the repository.

---

## 2026-03-17 — Project scaffold established

**Decision:** Full directory tree created per coding standards v1.0.
All packages have doc.go files declaring scope and dependency rules.

**Why:** Enforces the core isolation principle from commit one.
No package can accidentally import across architectural boundaries if the
boundaries are clear from the start.

**Alternatives considered:** Flat structure with single package — rejected.
Protocol complexity requires strict separation of concerns.

**Consequences:** All future packages must fit into this structure.
New top-level packages require a decision log entry explaining why they
cannot fit into an existing package.

---

## 2026-03-18 — SHA-256 used in Phase 1 instead of SHA3-256

**Decision:** `computeClaimID` uses SHA-256 (stdlib `crypto/sha256`) in Phase 1.

**Why:** The protocol specification calls for SHA3-256 but that requires
`golang.org/x/crypto`, an external dependency not yet approved for the module.
The substitution is documented in code comments at the call site.

**Alternatives considered:** Adding `golang.org/x/crypto` immediately — deferred
until the full external dependency policy is decided for the module.

**Consequences:** ClaimIDs computed in Phase 1 simulation will differ from those
computed in production. Migration path must be defined before Phase 2 chain
integration. This is tracked as a known deviation from the spec.

---

## 2026-03-18 — ReputationFloor applies to update and decay operations only

**Decision:** `ReputationFloor` (0.15) is enforced in `ApplyPostFinalizationUpdate`
and `ApplyDecay` only. The internal `withUpdatedDomainScore` helper clamps to
`[0.0, ReputationCeiling]`, not `[ReputationFloor, ReputationCeiling]`.

**Why:** `ReputationFloor` protects established validators from losing eligibility
through decay or a bad run. It is not a starting baseline. A validator with no
history in a domain has zero reputation there — clamping that to 0.15 on write
would make every new validator appear eligible without earning it, directly
contradicting Section 3.3 of the spec.

**Alternatives considered:** Clamping to floor on all writes — rejected. This was
caught by the test suite and confirmed as a protocol correctness issue, not a
test-passing convenience.

**Consequences:** Any future operation that modifies domain scores must explicitly
decide whether to enforce the floor. The default is no — floor enforcement is a
deliberate protocol policy decision applied only in named protocol operations.

---

## 2026-03-18 — Late normalization strategy in consensus weight aggregation

**Decision:** Individual vote weights are not normalized before aggregation.
Normalization happens at the decision boundary only — majority fraction is
computed as `winnerMass / TotalMass`, and confidence is a weighted average
normalized by `weightSum`.

**Why:** Late normalization is correct because the majority check is inherently
a relative comparison. Pre-normalizing individual weights would require knowing
the total weight in advance, forcing two passes over the votes. The current
single-pass approach is cleaner and produces identical results.

**Alternatives considered:** Per-vote normalization before aggregation — rejected.
Cross-session weight comparisons (e.g. 25-validator vs 11-validator claims) never
happen inside `core/consensus`. Reputation updates operate on verdict outcomes,
not raw weight values, so the boundary is clean.

**Consequences:** If any future component needs to compare raw weight totals across
validation sessions of different sizes, a normalization step must be added at that
boundary. This is not required in any currently planned phase.