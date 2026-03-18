# PulseSyn — Design Decision Log

Every significant design decision made during development is recorded here
with date, rationale, alternatives considered, and consequences.
This log is the primary evidence that PulseSyn's design decisions came
from a human designer over time.

---

## 2026-03-17 — Go module path set to github.com/baniloo/pulsesyn

**Decision:** Use github.com/iamtanay/pulsesyn as the canonical module path.

**Why:** Matches the intended public repository location and follows Go module
conventions. Consistent with the MIT open protocol release target.

**Alternatives considered:** baniloo.com/pulsesyn (custom domain path) — 
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
