# P1-04 — Architecture test for module boundaries

Executor: strong · Owner: Claude · Started: 2026-11-18 · Approved: 2026-11-18
Skip: none

## Goal

1. `internal/archtest` fails when `pay/**` and `tickets/**` import each other.
2. It fails when `audit` imports anything but `platform`.
3. It fails when simulators import product code.
4. A test proves that each rule fails on a violating fixture.

## Spec

- ARCH §7.1 layout: "`internal/platform` shared technical code · `internal/kit` ledger, idempotency, fsm, outbox, webhookin · `internal/pay/...` Bastion Pay modules · `internal/tickets/...` Bastion Tickets modules · `internal/audit` independent checks · `internal/sim/...` simulators".
- ARCH §7.1 rules, "enforced by an architecture test":
  1. "`pay/**` and `tickets/**` never import each other. Tickets reaches Pay only through `tickets/payments/payclient` over HTTP; Pay reaches Tickets only through webhooks."
  2. "Both may import `platform` and `kit`; each kit instance owns its schema in its product's database."
  3. "`audit` imports only `platform`; simulators import no product code."
  4. "Inside a product, modules use each other's Go APIs with a shared `pgx.Tx`, never each other's tables." (Not an import rule; out of scope.)
- TD §2: "`internal/archtest` fails the build on forbidden imports (Architecture §7)." The layout puts `cmd/pay-api`, `pay-worker`, `tickets-api`, `tickets-worker`, `relay`, `auditor`, `bastionctl`, `pspsim`, `banksim` under `cmd/`; `audit` holds "pay, tickets and cross-company checks (imports platform only)"; `sim` holds `psp`, `bank`.
- ENGINEERING E1: "Layout per TD §2. Dependencies point one way: `cmd` → `httpapi` → module services → `kit` → `platform`. No cycles." Enforced by "compiler, archtest".
- ENGINEERING E2: "`pay/**` and `tickets/**` never import each other; `audit` imports only `platform`; simulators import no product code (ARCH §7.1)." Enforced by "archtest".
- REVIEW G3: the baseline `task check` includes archtest. It already runs `go test -short ./...`, so a test in `internal/archtest` that does not skip under `-short` is part of the baseline.

## Decisions

| # | Question | Decision | By | Why |
| --- | --- | --- | --- | --- |
| D1 | What does "audit imports only platform" cover? | Only imports inside this module: `internal/audit/**` may import `internal/platform/**` and `internal/audit/**`. The standard library and third-party modules (pgx …) are allowed. | owner, 2026-11-18 | The auditor needs a database driver; the rule is about Bastion code it could share mistakes with. |
| D2 | What may simulators import? | `internal/sim/**` may import only `internal/platform/**` and `internal/sim/**` (stricter than "no product code": no `kit`, no `audit`). | owner, 2026-11-18 | A simulator plays an outside party; it should not share any Bastion domain code. |
| D3 | Add the E1 layering rule? | Yes: `internal/platform/**` imports only `internal/platform/**`; `internal/kit/**` imports only `internal/platform/**` and `internal/kit/**`. | owner, 2026-11-18 | Without it `pay → kit → tickets` crosses the boundary through an allowed edge; E1 names archtest as its enforcer. |
| D4 | Are `cmd/*` and `test/**` constrained? | Rules apply by the importing package's folder. `cmd/pay-*` follow the pay rule, `cmd/tickets-*` the tickets rule. `test/**`, `scripts/**`, `cmd/bastionctl`, `cmd/relay` and `internal/archtest` are unconstrained. | owner, 2026-11-18 | Integration tests and the operator CLI legitimately span both products. |
| D5 | Check "Tickets reaches Pay only through `payclient`" here? | No. | owner, 2026-11-18 | The P3 item whose `done_when` names it owns that check. |
| D6 | Skip G2 or G5? | No; `Skip: none`. | owner, 2026-11-18 | Small item; the tests are most of the work and an independent review is cheap. |
| D7 | `cmd/auditor`, `cmd/pspsim`, `cmd/banksim`? | `cmd/auditor` follows the audit rule; `cmd/pspsim` and `cmd/banksim` follow the sim rule. | writer | Same parties as their `internal` packages (D4's "by folder" applied to the other `cmd` entries the spec ties to a party). |
| D8 | How are imports read? | `go/parser.ParseFile(…, parser.ImportsOnly)` on every `.go` file under the root, including `_test.go` files and files excluded by build tags. No new dependency. | spike | Spike 2026-11-18 (scratch dir, not committed): 78 files in 1.6 ms on this repo, every module-internal import found with its line. `go list` would miss files excluded by build tags and needs a buildable fixture; `x/tools/go/packages` is a new dependency (E39). |
| D9 | Which folders are skipped? | Folders below the root named `testdata`, or starting with `.` or `_` (the folders the go command ignores). | writer | The lint fixtures in `internal/archtest/testdata` violate rules on purpose; `.git` and `.claude` hold no Go code of ours. |
| D10 | Where does the module path come from? | The `module` line of `<root>/go.mod`; each fixture is a mini repository with its own `go.mod` naming `github.com/imlukasd/Bastion`. | writer | One code path for the repository and the fixtures; a renamed module cannot silently make every import "external". |
| D11 | Direct or transitive imports? | Direct imports only. Every rule is checked on every edge, so with D3 a forbidden transitive path always contains a forbidden direct edge. | writer | Simple, and the message points at the exact line to change. |
| D12 | How are fixtures organized? | One mini repository per rule group under `internal/archtest/testdata/arch/<case>/`, plus `clean` (allowed edges and skipped folders) and `broken` (a parse error). | writer | Each group proves its rule independently; `clean` proves there are no false positives. |
| D13 | Code in a `_test.go` file or a normal file? | The checker in `imports.go` (normal file, package `archtest`), the tests in `imports_test.go`. | writer | Tests first (G2) compile against stubs; the checker obeys lint like other code. |
| D14 | Update ENGINEERING E2? | Yes, its text names what archtest enforces: "`pay/**` and `tickets/**` (and `cmd/pay-*`, `cmd/tickets-*`) never import each other; `audit` and `cmd/auditor` import only `platform` among Bastion packages; simulators (`sim/**`, `cmd/pspsim`, `cmd/banksim`) import only `platform` (ARCH §7.1)." E1 is unchanged. | writer | The rule text and the test must agree; approving the brief approves the ENGINEERING change. |

## Files

Create:
- `docs/briefs/active/P1-04-archtest.md`
- `internal/archtest/imports.go`
- `internal/archtest/imports_test.go`
- `internal/archtest/testdata/arch/**`

Modify:
- `docs/ENGINEERING.md`: E2 text (D14).

## Contracts

```go
package archtest

// Rule names; Violation.Rule is one of them.
const (
	RuleProducts = "E2-products" // pay and tickets never import each other
	RuleAudit    = "E2-audit"    // audit imports only platform
	RuleSim      = "E2-sim"      // simulators import only platform
	RulePlatform = "E1-platform" // platform imports only platform
	RuleKit      = "E1-kit"      // kit imports only platform and kit
)

// Import is one import of a Bastion package by a Go file under the scanned root.
type Import struct {
	File string // file path relative to the root, slash-separated: "internal/pay/funding/funding.go"
	Line int    // line of the import spec
	From string // folder of File relative to the root: "internal/pay/funding"
	To   string // imported path relative to the module: "internal/tickets/orders" ("" for the module root)
}

// Violation is an Import that breaks Rule.
type Violation struct {
	Import
	Rule string
}

// String formats "<File>:<Line>: <From> imports <To> (<Rule>: <reason>)", where
// reason is the comment of the Rule constant, e.g.
// "internal/pay/funding/funding.go:5: internal/pay/funding imports internal/tickets/orders (E2-products: pay and tickets never import each other)".
func (v Violation) String() string

// Scan reads the module path from <root>/go.mod and returns every import of a
// path equal to the module or under "<module>/" in every .go file below root
// (including _test.go files and files excluded by build tags), in file-walk
// order (lexical) then source order. Folders below root named testdata or
// starting with "." or "_" are skipped.
// Errors: go.mod missing, unreadable or without a module line; a file that
// does not parse (the error names the file). Errors wrap the cause (%w).
func Scan(root string) ([]Import, error)

// Check returns the imports that break a rule, in input order.
func Check(imports []Import) []Violation
```

A path `p` is "under `X`" when `p == X` or `p` starts with `X + "/"`; `cmd/pay-*` means a path starting with `cmd/pay-`.

Rules (the importer is `From`, the target is `To`):

| Importer `From` | Allowed targets | On any other target |
| --- | --- | --- |
| under `internal/pay`, or `cmd/pay-*` | everything except under `internal/tickets` | `RuleProducts` |
| under `internal/tickets`, or `cmd/tickets-*` | everything except under `internal/pay` | `RuleProducts` |
| under `internal/audit`, or `cmd/auditor` | under `internal/platform`, under `internal/audit` | `RuleAudit` |
| under `internal/sim`, `cmd/pspsim`, `cmd/banksim` | under `internal/platform`, under `internal/sim` | `RuleSim` |
| under `internal/platform` | under `internal/platform` | `RulePlatform` |
| under `internal/kit` | under `internal/platform`, under `internal/kit` | `RuleKit` |
| anything else | everything | — |

Fixtures: `internal/archtest/testdata/arch/<case>/`, each with `go.mod` = `module github.com/imlukasd/Bastion` + `go 1.27.0`; files are minimal (package clause, the imports, `var _ = x.Y` uses are not needed since nothing compiles them).

| Case | File → imported path | Expected |
| --- | --- | --- |
| `products` | `internal/pay/funding/funding.go` → `internal/tickets/orders` | `RuleProducts` |
| | `internal/tickets/orders/orders_test.go` → `internal/pay/intents` | `RuleProducts` (test files count) |
| | `cmd/pay-api/main.go` → `internal/tickets/catalog` | `RuleProducts` |
| `audit` | `internal/audit/pay/check.go` → `internal/platform/db` (allowed), `internal/kit/ledger`, `internal/pay/merchants` | `RuleAudit` ×2 |
| | `cmd/auditor/main.go` → `internal/tickets/orders` | `RuleAudit` |
| `sim` | `internal/sim/psp/psp.go` → `internal/pay/intents` | `RuleSim` |
| | `internal/sim/bank/bank.go` → `internal/tickets/payouts`, `internal/kit/outbox` | `RuleSim` ×2 |
| | `cmd/pspsim/main.go` → `internal/pay/funding` | `RuleSim` |
| `layers` | `internal/platform/db/db.go` → `internal/kit/ledger` | `RulePlatform` |
| | `internal/kit/ledger/ledger.go` → `internal/pay/customers` | `RuleKit` |
| | `internal/kit/outbox/outbox.go` → `internal/audit` | `RuleKit` |
| `clean` | allowed edges: pay → platform, kit, pay; tickets → platform, kit, tickets; audit → platform, audit; sim/psp → platform, sim/bank; kit → platform, kit; platform → platform; `cmd/pay-api` → pay, platform; `cmd/tickets-api` → tickets; `cmd/bastionctl` → pay, tickets; `test/integration` → pay, tickets; stdlib and `github.com/jackc/pgx/v5` imports; a file with `//go:build ignore` that imports only allowed paths | none |
| | skipped folders, each with a file that would break `RuleProducts`: `internal/pay/testdata/`, `internal/pay/.hidden/`, `internal/pay/_old/` | none |
| `broken` | `internal/x/x.go` with an unterminated import string | `Scan` error naming `internal/x/x.go` |

## Steps

1. Tests first (G2): stubs of the Contracts in `imports.go` (`Scan` returns `nil, errors.New("not implemented")`, `Check` returns `nil`, `String` returns `""`), the fixtures, `imports_test.go`. Verify: `go test ./internal/archtest/ -run 'Imports|Scan|Violation'` → FAIL, every test failing on "not implemented" or a missing violation, none on a compile error; `go vet ./internal/archtest/` → clean. Commit `test(P1-04): …` after the owner's `ok tests`.
2. Implement `Scan`. Verify: `go test ./internal/archtest/ -run 'Scan|Repo'` → the Scan tests pass. Commit `feat(P1-04): scan module imports`.
3. Implement `Check` and `String`. Verify: `go test ./internal/archtest/...` → ok. Commit `feat(P1-04): check import rules`.
4. ENGINEERING E2 text (D14). Verify: `task check` → `check OK`. Commit `docs(P1-04): name archtest rules in e2`.
5. Gates: `task test` (G3), reviewer (G5), item report.

## Test map

All tests in `internal/archtest/imports_test.go`; none skips under `-short`.

| Clause | Test | Asserts |
| --- | --- | --- |
| 1–3 | `TestImports_RepoFollowsRules` | `Scan("../..")` has no error, contains `{From: "internal/platform/db/dbtest", To: "internal/platform/db/migrate"}` (the scan is not vacuous), and `Check` returns no violation (each printed with `String`). |
| 1, 4 | `TestImports_FixturesReportExpectedViolations/products` | Exactly the `products` rows: `file:line rule` set equality. |
| 2, 4 | `…/audit` | Exactly the `audit` rows; the allowed `internal/platform/db` import is not reported. |
| 3, 4 | `…/sim` | Exactly the `sim` rows. |
| 4 (D3) | `…/layers` | Exactly the `layers` rows. |
| 4 | `…/clean` | No violation; `Scan` found the `clean` imports (count > 0) and none from a skipped folder. |
| 4 | `TestScan_Errors` | Missing `go.mod` → error; `broken` → error whose text contains `internal/x/x.go`. |
| 4 | `TestViolation_String` | The exact `String` example of the Contracts. |

## Out of scope

- "Tickets reaches Pay only through `tickets/payments/payclient`": the P3 item whose `done_when` names it (D5).
- ARCH §7.1 rule 4 (no module reads another module's tables): review (E3), and later the per-module database roles if a phase item adds them.
- The module-level arrows inside a product (ARCH §7.2, §7.3, e.g. `httpapi` → services only): later items, if the owner wants them enforced.
- Cycles: the compiler (E1).

## Stop rules

Stop and report instead of choosing when:

- a decision is needed that this brief does not make;
- a file outside Files must change, a dependency not named here is needed, or a test seems wrong;
- the code or the spec contradicts this brief;
- the repository itself breaks a rule (`TestImports_RepoFollowsRules` fails after step 3): report the violation, do not change product code or the rules;
- golangci-lint or govulncheck reports anything under `internal/archtest/testdata/arch`.

## Progress

## Surprises

## Outcome
