# PulseSyn Coding Standards

Reference these standards at the start of every session. These are protocol-level
standards — they apply to all packages and cannot be overridden without a
DECISIONS.md entry.

---

## 1. Package Structure

### 1.1 Every package must have a `doc.go`

```go
// Package <name> <one-sentence scope statement>.
// Dependency rules: <list what this package may import>.
package <name>
```

The dependency comment is enforced manually. It documents the contract. If you
violate it, you must update both the comment and DECISIONS.md.

### 1.2 Directory layout within a package

```
<package>/
    doc.go              -- package declaration, scope, dependency rules
    <primary_type>.go   -- the main type and its constructor
    <secondary>.go      -- auxiliary types (lifecycle, updates, decay, etc.)
    <package>_test.go   -- all tests, white-box, same package
```

One test file per package. Never split tests across multiple files.

---

## 2. Type Design

### 2.1 All protocol structs are immutable

- No setters. No exported fields that are mutable slices or maps.
- Constructors return `*Type`. Update operations take `*Type` and return a new `*Type`.
- Internal helpers that update state are unexported and follow the `withUpdated<Field>`
  naming convention:

```go
func (r *ValidatorRecord) withUpdatedDomainScore(domain string, newScore float64) *ValidatorRecord {
    // deep-copy all fields, update the relevant one, return new pointer
}
```

### 2.2 Use an `Input` struct for constructors with more than two parameters

```go
type Input struct {
    ClaimText       string
    ClaimType       ClaimType
    DomainTags      []string
    GeographicScope GeographicScope
    TimeReference   time.Time
    ContentHash     string
    ContentURL      string
    SubmitterID     string
    SubmissionEpoch uint64
}

func NewClaim(in Input) (*Claim, error) { ... }
```

This avoids positional argument confusion and makes the call site self-documenting.

### 2.3 Config structs for configurable components

For components with optional parameters, use a `Config` struct with a `Validate()` method:

```go
type Config struct {
    PoolSize     int
    SetSize      int
    Rounds       int
    CollusionRate float64
    // ...
}

func (c Config) Validate() error { ... }
```

---

## 3. Error Handling

### 3.1 Sentinel errors

Every package defines a `var` block of sentinel errors at the top of the primary file:

```go
var (
    ErrClaimTextEmpty    = errors.New("pulsesyn/claim: claim_text is empty")
    ErrClaimTextTooLong  = errors.New("pulsesyn/claim: claim_text exceeds 500 characters")
    ErrClaimTypeInvalid  = errors.New("pulsesyn/claim: claim_type is not one of FACTUAL, CONTEXTUAL, PREDICTIVE")
    // ...
)
```

Rules:
- Format: `pulsesyn/<package>: <field_name> <reason>`
- Named `ErrXxx` — no exceptions
- Callers use `errors.Is(err, ErrXxx)` — never string matching

### 3.2 Wrapping errors from constructors and methods

```go
// Constructors: wrap with function name
return nil, fmt.Errorf("NewClaim: %w", ErrClaimTextEmpty)

// Methods: wrap with Receiver.Method
return fmt.Errorf("Window.Add (%s/%s): %w", w.validatorID, w.domain, ErrInvalidWindowSize)

// Do NOT double-wrap sentinel errors that already have package context
```

### 3.3 Validation is the constructor's job

- Validate all fields before constructing the struct.
- Call a private `validateXxx(in Input) error` function before allocating.
- Downstream code trusts that a non-nil `*Type` is valid.
- Do not validate in methods — if you're holding a `*Claim`, it's valid.

---

## 4. Constants and Protocol Parameters

### 4.1 Named constants for every threshold

No magic numbers anywhere. Every protocol constant must be named and documented:

```go
const (
    // QuorumThreshold is the minimum fraction of selected validators that
    // must submit votes for consensus to be valid. Default 2/3.
    // See PulseSyn Protocol Specification v0.1, Appendix A.
    QuorumThreshold = 2.0 / 3.0

    // VerdictMajorityThreshold is the minimum fraction of total adjusted
    // weight mass the winning verdict must hold.
    VerdictMajorityThreshold = 0.50
)
```

### 4.2 Group constants by category

Use multiple `const (...)` blocks if there are distinct categories. Do not mix
protocol parameters with implementation constants in a single block.

### 4.3 Phase 1 constants that will change

When a constant is a Phase 1 simplification of a spec value, document it:

```go
// computeClaimID uses SHA-256 in Phase 1. The spec calls for SHA3-256.
// See DECISIONS.md: 2026-03-18.
```

---

## 5. Comments

### 5.1 Package-level and type-level comments

Every exported type gets a comment. Format:

```go
// ValidatorRecord holds the full reputation state for a single validator.
// It is immutable once constructed — update operations return new records.
// See PulseSyn Protocol Specification v0.1, Section 2.2.
type ValidatorRecord struct { ... }
```

### 5.2 Function comments

Every exported function gets a comment. The comment must state:
- What the function does (not how)
- Error conditions (what errors can be returned)
- The spec section it implements

```go
// ComputeConsensus applies the PoV weighted vote aggregation algorithm to a
// set of validator votes and returns a ConsensusResult.
//
// Returns ErrNoVotes if votes is empty.
// Returns VerdictIndeterminate if quorum or majority thresholds are not met —
// this is not an error, it is a valid protocol outcome.
//
// See PulseSyn Protocol Specification v0.1, Section 4.2, 4.3, 4.4.
func ComputeConsensus(votes []Vote, validatorSetSize int) (*ConsensusResult, error) { ... }
```

### 5.3 Inline comments — only for non-obvious WHY

Do not comment what the code does (the code is readable). Comment why a constraint
exists, why a specific formula is used, or why a known workaround is in place:

```go
// Evict oldest entry. Copy-forward is O(n) but windows are small (≤50)
// and called at most once per finalized validation per validator.
w.entries = w.entries[1:]
```

```go
// Clamp to [0.0, 1.0] as a safety invariant — the formula is bounded by
// construction but floating-point arithmetic can produce tiny overflows.
if coefficient > 1.0 { coefficient = 1.0 }
```

---

## 6. Normalisation

Always normalise at the boundary (constructor or first point of entry). Downstream
code works with clean values and never needs to normalise again.

```go
// In constructors
ClaimText:  strings.TrimSpace(in.ClaimText),
DomainTags: normaliseDomainTags(in.DomainTags),  // lowercase + trim + remove empty
ContentHash: strings.ToLower(strings.TrimSpace(in.ContentHash)),

// In methods that accept string identifiers
domain = strings.ToLower(strings.TrimSpace(domain))
validatorID = strings.TrimSpace(validatorID)
```

---

## 7. Testing

### 7.1 Full coverage before commit

Every function written in a session must have tests in that session. No commit
with untested code. The only exception is `doc.go` files.

### 7.2 Test structure

```go
func TestNewClaim_Valid(t *testing.T) {
    in := validClaimInput()
    c, err := NewClaim(in)
    require.NoError(t, err)
    assert.Equal(t, strings.TrimSpace(in.ClaimText), c.ClaimText)
    assert.Equal(t, StateSubmitted, c.State)
}

func TestNewClaim_TextEmpty(t *testing.T) {
    in := validClaimInput()
    in.ClaimText = ""
    _, err := NewClaim(in)
    require.ErrorIs(t, err, ErrClaimTextEmpty)
}
```

Rules:
- Test function names: `TestXxx_<scenario>` — no underscores in the scenario word
  unless they separate two meaningful concepts (`_ValidInput`, `_EmptyText`, `_QuorumNotMet`)
- Use `require.NoError` (fatal) for setup; `assert.Equal` (non-fatal) for assertions
- Use `require.ErrorIs` (not `assert.Error`) when verifying specific error types
- One assertion per test when possible
- Use a `validXxxInput()` helper that returns a known-good input — tests then mutate one field

### 7.3 Test coverage areas (mandatory for every package)

For each new package, tests must cover:

1. **Constructor — valid input:** all fields set correctly, derived fields computed correctly
2. **Constructor — each error path:** one test per sentinel error
3. **Methods — normal operation:** the golden path for each public method
4. **Methods — edge cases:** zero values, maximum values, boundary conditions
5. **Immutability:** update operations must not modify the original struct
6. **Protocol invariants:** test that the spec guarantees hold at the boundary values

### 7.4 White-box testing

Tests are in the same package (not `<package>_test`). This allows testing
unexported helpers directly — which is important for protocol-level correctness
where internal functions implement specific spec formulas.

---

## 8. Floating-Point Arithmetic

Protocol scores and coefficients are in `[0.0, 1.0]`. Rules:

- **Always clamp output.** Any computation that produces a `float64` score
  must be clamped to its valid range before storage or return:

  ```go
  func clamp(v, min, max float64) float64 {
      if v < min { return min }
      if v > max { return max }
      return v
  }
  ```

- **Never compare floats with ==.** Use range checks (`>= threshold`, `< threshold`).
- **Document why late normalisation is correct** when you aggregate weights without
  normalising per-item (see DECISIONS.md: 2026-03-18 — Late normalisation).

---

## 9. String Type Aliases for Protocol Enumerations

Use typed string constants, not `iota` integers, for protocol enumerations. The
values are human-readable and must survive serialisation:

```go
type ClaimType string

const (
    ClaimTypeFactual     ClaimType = "FACTUAL"
    ClaimTypeContextual  ClaimType = "CONTEXTUAL"
    ClaimTypePredictive  ClaimType = "PREDICTIVE"
)
```

Validate with a `switch` statement. Unknown values return the sentinel error:

```go
func validateClaimType(t ClaimType) error {
    switch t {
    case ClaimTypeFactual, ClaimTypeContextual, ClaimTypePredictive:
        return nil
    default:
        return ErrClaimTypeInvalid
    }
}
```

---

## 10. Commit Protocol

Every commit must:

1. Pass `go test ./...` with zero failures
2. Include only one package's changes (one logical unit per commit)
3. Follow the commit message format: `type(scope): message`
4. Have its design decisions logged in `DECISIONS.md` (if any were made)

**Commit types:**

| Type | Use for |
|---|---|
| `feat` | New protocol feature |
| `fix` | Bug correction |
| `test` | Test additions or corrections |
| `refactor` | Code restructuring, no behaviour change |
| `docs` | Documentation only |
| `chore` | Scaffold, tooling, dependency updates |

**Never commit:**

- Code that does not compile
- Tests with `t.Skip()`
- Functions with `// TODO` placeholders that affect protocol correctness
- Commented-out code
- Files in `docs/raw-docs/` (gitignored)

---

## 11. DECISIONS.md Protocol

Log every:

- Design choice between two valid approaches
- Deviation from the spec (even temporary)
- New external dependency added
- Protocol constant whose value was not obvious
- Trade-off accepted for Phase 1 that must be revisited later

Entry format:

```markdown
## YYYY-MM-DD — <short title>

**Decision:** What was chosen.

**Why:** The reasoning.

**Alternatives considered:** What was rejected and why.

**Consequences:** What downstream code must account for as a result.
```

---

## 12. What NOT to Do

- Do not import anything outside stdlib in `core/*` — not even `golang.org/x/...`
- Do not export test-only helpers (no `SetXxx` or `NewXxxForTest` on immutable structs)
- Do not use `interface{}` or `any` in protocol types
- Do not return `error` from functions that cannot fail — use a boolean or the type itself
- Do not add `init()` functions
- Do not use `panic` for recoverable errors — return `error`
- Do not write multi-paragraph doc comments — one sentence per concept
- Do not leave `fmt.Println` or `log.Println` debug statements in committed code
