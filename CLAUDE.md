# PulseSyn — Claude Code Context

PulseSyn is an open, decentralized protocol for validating claims. A structured,
falsifiable claim is submitted with a pointer to supporting evidence, run through
a reputation-weighted validator network, and produces a permanent, tamper-proof
verdict.

**Module:** `github.com/iamtanay/pulsesyn`  
**Language:** Go 1.26.1  
**License:** MIT Open Protocol License  
**Author:** Baniloo — baniloo.com

---

## Current Status

**Phase 1 — Complete.** 100 tests, 0 failures.

| Package | Status | Tests |
|---|---|---|
| `core/claim` | Done | 17 |
| `core/consensus` | Done | 10 |
| `core/reputation` | Done | 14 |
| `core/bias` | Done | 33 |
| `simulation` | Done | 26 |

**Phase 2 — Next (Session 3):** `store/` (embedded DB), `chain/` (Solidity + Go adapter), `merkle/`.

---

## Repository Structure

```
core/         Protocol heart — stdlib only, no external deps, no exceptions
network/      Peer layer — libp2p integration
selector/     Validator selection algorithm (Phase 3)
session/      Validation session orchestration
store/        Local state — embedded DB (Phase 2)
chain/        Chain adapter — Solidity contracts + Go interface (Phase 2)
merkle/       Merkle proof generation and verification (Phase 2)
api/          JSON-RPC API server
sdk/          Developer SDK
simulation/   Simulation harness — depends on core/* only
tests/        Integration and security tests
docs/         Protocol documentation and session summaries (gitignored)
```

---

## Package Dependency Rules

These rules are **non-negotiable**. Enforced from the first commit. Violations require
a new DECISIONS.md entry explaining why and how the rule was formally amended.

| Package | Allowed imports |
|---|---|
| `core/*` | stdlib only |
| `simulation/` | stdlib + `core/*` |
| `selector/` | stdlib + `core/*` |
| `store/` | stdlib + `core/*` + one approved embedded DB |
| `chain/` | stdlib + `core/*` + approved chain SDK |
| `merkle/` | stdlib + `core/*` |
| `session/` | stdlib + `core/*` + `store/` + `selector/` + `network/` |
| `network/` | stdlib + `core/*` + libp2p |
| `api/` | stdlib + `core/*` + `session/` |
| `sdk/` | stdlib + `core/*` + `api/` |
| `tests/` | all packages |

---

## Architecture Invariants

These are **protocol-level invariants**. Treat any code that violates them as a bug,
not an acceptable trade-off.

1. **Core isolation.** `core/*` never imports anything outside stdlib. If a feature
   seems to require an external import in `core/`, the design is wrong — fix it upstream.

2. **Immutability.** All protocol structs (`Claim`, `ValidatorRecord`, `ConsensusResult`,
   `BiasResult`) are immutable once constructed. Update operations return new structs.
   No setters. No mutation of passed-in slices.

3. **Sentinel errors.** Every package defines its own `var Err... = errors.New(...)` block.
   Error strings follow the `pulsesyn/<package>: message` format. Callers use `errors.Is`.
   Never return raw strings as errors.

4. **Validation at the boundary.** All external input is validated at the constructor
   (`NewXxx(input Input)` pattern). Downstream code trusts validated types.

5. **Test before commit.** `go test ./...` must pass 100% before any commit is made.
   No stubs. No skipped tests. No `t.Skip`. Every new function gets tests in the same session.

6. **Decisions logged.** Every design decision that deviates from the spec, picks between
   two valid approaches, or sets a protocol constant must be recorded in `DECISIONS.md`
   before the session ends.

---

## Known Phase 1 Deviations from Spec

| Item | Resolution Path |
|---|---|
| SHA3-256 → SHA-256 (claim ID) | Replace before Phase 2 chain integration when `golang.org/x/crypto` approved |
| `submitter_id` min reputation check is stubbed | Wire to `store/` in Phase 2 |
| `content_url` reachability bypassed in unit tests | Integration tests cover this in Phase 2 |
| Bias axis is single-dimensional | Multi-axis encoding if simulation data justifies it |

---

## Session Workflow

Every session follows this protocol:

1. **Read context.** Load the previous session summary from `docs/sessions/` and the
   spec before writing any code.
2. **One package at a time.** Do not start the next package until the current one is
   fully tested and committed.
3. **No commit without tests.** Every commit must have passing tests. Partial work does
   not get committed.
4. **Log decisions.** Any significant design choice goes into `DECISIONS.md` before the
   session ends.
5. **Write the session summary.** At the end of every session, write a summary to
   `docs/sessions/YYYY-MM-DD-session-N.md` and include the handoff instruction for
   the next session.

---

## Code Conventions

See `/coding-standards` for the full reference. Summary:

- **Every package has a `doc.go`** declaring its scope and dependency rules.
- **Constructors use an `Input` struct** for named parameters: `NewXxx(in Input) (*Xxx, error)`.
- **Constants are grouped** by category with a `const (...)` block and a leading comment
  pointing to the spec section.
- **Function comments** reference the spec section they implement:
  `// See PulseSyn Protocol Specification v0.1, Section 4.2.`
- **Normalisation** happens at the constructor — `strings.TrimSpace`, `strings.ToLower`.
  Downstream code works with clean values.
- **Error wrapping:** `return nil, fmt.Errorf("FunctionName: %w", ErrSentinel)`.
- **Test file naming:** `<package>_test.go`. One test file per package. All in the same
  package (white-box testing).
- **Testify assertions:** `require.NoError(t, err)` for fatal, `assert.Equal(t, ...)` for
  non-fatal. Never `t.Fatal` with string messages.
- **No magic numbers.** Every threshold, coefficient, or limit is a named constant
  with a comment pointing to the spec.

---

## Commit Convention

Format: `type(scope): message`

| Type | Use for |
|---|---|
| `feat` | New protocol feature |
| `fix` | Bug correction |
| `test` | Test additions or fixes |
| `refactor` | Code restructuring without behaviour change |
| `docs` | Documentation only |
| `chore` | Scaffolding, dependency updates, tooling |

Example: `feat(consensus): implement PoV weighted vote aggregation and bias correction`

---

## External Dependencies

| Package | Approved for | Phase |
|---|---|---|
| `github.com/stretchr/testify` | Test assertions only | 1 |
| embedded DB (TBD: BadgerDB or bbolt) | `store/` only | 2 |
| `golang.org/x/crypto` | SHA3-256 in `core/claim` | 2 (pending approval) |

Any new dependency requires a DECISIONS.md entry before being added.

---

## Running Tests

```sh
go test ./...
```

All tests must pass. No `-count`, no `-run` filters in CI. Run the full suite.

---

## DECISIONS.md Protocol

`DECISIONS.md` is the primary evidence that design decisions came from a human designer
over time. Every entry must have:

- **Date** in `YYYY-MM-DD` format
- **Decision** — what was chosen
- **Why** — the reasoning
- **Alternatives considered** — what was rejected and why
- **Consequences** — what downstream code must do as a result
