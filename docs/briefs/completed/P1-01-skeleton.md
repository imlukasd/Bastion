# P1-01 — Repository skeleton: go.mod, layout, Taskfile, lint

Executor: strong · Owner: Claude · Started: 2026-10-28 · Approved: 2026-10-28
Skip: G2 (reason: D12)

## Goal
The item's `done_when`, one clause per line:
1. `go.mod` exists with Go >= 1.25.
2. The TD §2 directory layout exists.
3. `Taskfile.yml` has `check`, which runs build, vet, golangci-lint, govulncheck, unit tests and `go run scripts/brief/main.go -check`.
4. `task check` prints failures only.
5. `Taskfile.yml` has `test` (the full suite), `up`, `down`, `migrate` and `chaos`.
6. golangci-lint enables the rules ENGINEERING.md marks *lint*.
7. `task check` passes on the skeleton.
8. AGENTS.md › Conventions lists the commands.

## Spec
- TD §2, the layout:
  ```
  bastion/
  ├── cmd/  pay-api/ pay-worker/ tickets-api/ tickets-worker/ relay/ auditor/ bastionctl/ pspsim/ banksim/
  ├── internal/
  │   ├── platform/  db, redisx, config, logx, metrics, tracing, ids, money, httpx, resilience, crypto, jobs
  │   ├── kit/       ledger, idempotency, fsm, outbox, webhookin
  │   ├── pay/       customers, limits, funding, transfers, intents, merchants, webhooks, safeguarding, httpapi
  │   ├── tickets/   catalog, waitingroom, inventory, orders, resale, payments (payclient), payouts, settlement, httpapi
  │   ├── audit/     pay, tickets and cross-company checks (imports platform only)
  │   └── sim/       psp, bank
  ├── db/{pay,tickets}/{migrations,queries}/
  ├── deploy/  scripts/  test/{integration,property,model,concurrency,chaos,load}/  docs/  Taskfile.yml
  ```
  and: "`internal/archtest` fails the build on forbidden imports (Architecture §7)."
- TD §34.1 (levels): Unit `go test`, every commit; Integration testcontainers, every commit; Property `rapid`, fixed seeds every commit, random nightly; Concurrency `go test -race`, every commit; Chaos (Toxiproxy, `docker kill`, failpoints) PR subset + nightly.
- TD §34.4: "lint (golangci-lint, govulncheck, archtest) → unit + integration + property (fixed seeds) + model + concurrency (-race) → chaos subset …"
- TD §35.1: "`task up` / `task down`, `task migrate`, `task test`, `task chaos [-- C4]`, `task load -- onsale_burst`, `task demo` …"
- AD-02: Go. AD-15: pgx + sqlc + SQL migrations (here only the `db/{pay,tickets}/{migrations,queries}/` folders).
- ENGINEERING "Enforced by": "*lint* means a golangci-lint rule configured in P1-01". Rules marked lint:
  - E5 "No package-level mutable state, no `init()` side effects" (lint, review)
  - E8 "No floating point on any money path, tests included (TD §1.1)" (lint, review)
  - E9 "Statuses, kinds and other closed sets are typed constants; every switch over them is exhaustive." (lint)
  - E11 "stays short (guide: under 60 lines, cognitive complexity under 15). No boolean parameter that switches behavior." (lint, review)
  - E13 "Every error is handled. Wrap with context using `%w`; compare with `errors.Is` / `errors.As`." (lint)
  - E21 "Business time comes from the database (`now()` in SQL). Go's clock is only for timeouts, backoff and metrics" (lint, review)
  - E28 "`context.Context` is the first parameter of every function that does I/O; request paths never use `context.Background()`." (lint)
  - E36 "PINs, OTPs, client secrets … are never logged. Secrets come from configuration, never from code." (lint, review)
  - E37 "Exported identifiers have doc comments. No commented-out code." (lint, review)
  - E39 "A new third-party dependency needs a reason in LOG.md"; E40 "`gofmt`/`goimports` and `golangci-lint` clean; `//nolint` only with a written reason." (`task check`)

## Decisions
| # | Question | Decision | By | Why |
| --- | --- | --- | --- | --- |
| D1 | Module path | `github.com/imlukasd/Bastion` | owner, 2026-10-28 | Matches the repository's name and case. |
| D2 | Which Go version | Go 1.27.3, installed from the go.dev MSI; `go.mod` says `go 1.27.0` and `toolchain go1.27.3`. | owner, 2026-10-28 | Spike: Go 1.25 is out of support; govulncheck on Go 1.25.6 reports 10+ reachable standard-library vulnerabilities for a program that only uses `net/http`, so `task check` cannot pass on 1.25. golangci-lint v2.15.0, govulncheck v1.9.0 and task v3.54.0 need Go >= 1.26. The toolchain the go command downloads by itself is blocked here by Windows Application Control (`asm.exe`), so the version must be installed; winget only offers 1.27.0. |
| D3 | Where the repository-level tests and lint fixtures live (TD §2 has no folder for them) | `internal/archtest` (tests + `testdata/lint/`); P1-04 adds the import rules to the same package. Rejected: `test/repo/` (a TD §2 change), the repository root. | owner, 2026-10-28 | archtest is already in TD §2 and is about the shape of the repository; one package for every repository-wide check. |
| D4 | How the tools are installed | golangci-lint v2.15.0 and govulncheck (golang.org/x/vuln v1.9.0) as `tool` directives in a separate modfile `tools.mod` (+ `tools.sum`), run as `go tool -modfile=tools.mod <tool>`. task v3.54.0 is installed per machine (`winget install Task.Task`). | writer (spike) | Pinned in the repository, identical on every machine. The separate modfile keeps about 220 lines of linter dependencies out of `go.mod` (spike), so they never change the versions our code builds with. `task` is the entry command, so it is on PATH; winget ships a prebuilt 3.54.0. |
| D5 | Unit tests vs the full suite | `task check` runs `go test -short ./...`. A test that needs an external tool, Docker or more than a few seconds calls `testing.Short()` and skips in short mode. `task test` runs `go test -race -count=1 ./...` (no `chaos` or `failpoints` tags). | writer | check stays fast; `-race` works here with gcc (spike); `-count=1` so tests against live stores are never served from the cache. |
| D6 | How `check` prints failures only | `task check` runs `go run ./scripts/check`, a stdlib-only Go runner with the step list. Steps run in order and stop at the first failure. A passing step prints nothing; a failing one prints `FAIL <step>: <command>` and its output. When all pass it prints one line, `check OK`. | writer | Same behavior on Windows and Linux without shell tricks; unit-testable with a fake executor; the final line gives evidence a non-empty result, like `plan OK`. |
| D7 | Which linter enforces each lint rule | E5 gochecknoglobals, gochecknoinits. E8 forbidigo `float32`, `float64`, `strconv.ParseFloat`, `strconv.FormatFloat`. E9 exhaustive (a `default` does not count). E11 funlen (body over 60 lines), gocognit (over 15), revive `flag-parameter`. E13 errcheck, errorlint. E21 forbidigo `time.Now`, `time.Since`, `time.Until`. E28 revive `context-as-argument`, forbidigo `context.Background`, `context.TODO`, noctx (HTTP, SQL and other I/O calls without a context). E36 gosec G101. E37 revive `exported`, gocritic `commentedOutCode`. E40 gofmt, goimports, nolintlint. | writer (spike) | Each linter was run on a violating fixture (golangci-lint v2.11.0 on Go 1.25.6) and reported it; the thresholds report a 62-line body and complexity 17, not a 60-line body. |
| D8 | What lint cannot catch | Untyped float literals (`x := 0.5`) pass forbidigo; G101 catches only high-entropy literals; logging a PIN or OTP is not detectable. G5 found more (golangci-lint v2.15.0 probes): an error discarded with `_ =` (errcheck `check-blank` is off); a closed set declared as untyped constants (E9); the Go clock through an aliased import (`t "time"`; forbidigo matches source text); I/O without a context outside noctx's list (E28). These stay with review, which D13 makes explicit. | writer (spike, G5) | Measured on fixtures. |
| D9 | Lint exemptions | `_test.go`: funlen (table tests). `scripts/`: forbidigo, funlen, gochecknoglobals, gocognit (tooling; `scripts/brief` has 14 such findings today and is not product code). `cmd/`: the E28 forbidigo pattern (`main` creates the root context). Exemptions for later packages (floats in `platform/metrics`, the Go clock in `platform/resilience`) are added by the item that creates the package, in its brief. | writer | Keeps the rules strict for product code without touching the harness script in this item. |
| D10 | `up`, `down`, `migrate`, `chaos` before their items | Each prints `task <name>: not implemented yet (<item>)` to stderr and exits 1: P1-02 for up, down and migrate; P2-04 for chaos. | writer | A placeholder that exits 0 could be mistaken for a running stack. |
| D11 | What fills the skeleton | Each Go package folder gets `doc.go` with its package comment; each `cmd/<name>` gets `main.go` that prints `<name>: not implemented yet` to stderr and exits 1; non-Go folders get `.gitkeep`. `internal/audit` is one package (its subpackages come with P1-14). `test/load` (k6 scripts) gets `.gitkeep`. | writer | git does not track empty folders; `doc.go` keeps every package buildable and documented (E37). |
| D12 | Process (AGENTS.md › Right-sizing) | `Skip: G2`: the tests come with the code in the `feat` commits. Only two test files are written: the lint fixtures (`internal/archtest/lint_test.go`) and the check runner (`scripts/check/main_test.go`); the other clauses are checked by verify commands. G5 stays. | owner, 2026-10-28 | A tooling item with no invariant. The lint test is the only proof that the *lint* rules fire, and later items rely on them; layout, go.mod, Taskfile and AGENTS.md are caught by the build, `task` and review. G5 stays because the lint configuration is a convention every later item follows. |
| D13 | G5 major: E9, E13 and E28 are marked lint only, so the D8 gaps would be checked by nobody (ENGINEERING.md line 7) | Enable noctx for E28 (fixture `e28/noctx.go`). In ENGINEERING.md, mark E9, E13 and E28 "lint, review", and word E11 as "at most 60 lines, cognitive complexity at most 15" to match funlen 60 and gocognit 15 (D7). | owner, 2026-10-28 | Lint covers the common cases; review covers what lint misses; the wording matches what the linters enforce. |

## Files
Create:
- `go.mod`
- `tools.mod`
- `tools.sum`
- `.golangci.yml`
- `Taskfile.yml`
- `scripts/check/main.go`
- `scripts/check/main_test.go`
- `cmd/{pay-api,pay-worker,tickets-api,tickets-worker,relay,auditor,bastionctl,pspsim,banksim}/main.go`
- `internal/platform/{db,redisx,config,logx,metrics,tracing,ids,money,httpx,resilience,crypto,jobs}/doc.go`
- `internal/kit/{ledger,idempotency,fsm,outbox,webhookin}/doc.go`
- `internal/pay/{customers,limits,funding,transfers,intents,merchants,webhooks,safeguarding,httpapi}/doc.go`
- `internal/tickets/{catalog,waitingroom,inventory,orders,resale,payments,payouts,settlement,httpapi}/doc.go`
- `internal/tickets/payments/payclient/doc.go`
- `internal/audit/doc.go`
- `internal/sim/{psp,bank}/doc.go`
- `internal/archtest/doc.go`
- `internal/archtest/lint_test.go`
- `internal/archtest/testdata/lint/**`
- `db/{pay,tickets}/{migrations,queries}/.gitkeep`
- `deploy/.gitkeep`
- `test/{integration,property,model,concurrency,chaos}/doc.go`
- `test/load/.gitkeep`
Modify:
- `AGENTS.md`: Conventions › "Commands" lists the tasks and the prerequisites (Contracts › AGENTS.md).
- `docs/ENGINEERING.md`: E9, E13, E28 "Enforced by" and E11 wording (D13).

## Contracts

**go.mod** (no requirements yet, so no `go.sum`):
```
module github.com/imlukasd/Bastion

go 1.27.0

toolchain go1.27.3
```

**tools.mod**: `module github.com/imlukasd/Bastion`, `go 1.27.0`, created with `go mod init -modfile=tools.mod github.com/imlukasd/Bastion`, then `go get -tool -modfile=tools.mod github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.15.0` and `go get -tool -modfile=tools.mod golang.org/x/vuln/cmd/govulncheck@v1.9.0`. Upgrades use the same `go get -tool -modfile=tools.mod`; never `go mod tidy -modfile=tools.mod` (it would pull the main module's imports into it).

**.golangci.yml** (validated with `golangci-lint config verify` in the spike):
```yaml
version: "2"

linters:
  default: standard
  enable:
    - errorlint        # E13
    - exhaustive       # E9
    - forbidigo        # E8, E21, E28
    - funlen           # E11
    - gochecknoglobals # E5
    - gochecknoinits   # E5
    - gocognit         # E11
    - gocritic         # E37
    - gosec            # E36
    - noctx            # E28
    - nolintlint       # E40
    - revive           # E11, E28, E37
  settings:
    exhaustive:
      default-signifies-exhaustive: false
    forbidigo:
      forbid:
        - pattern: ^float(32|64)$
          msg: "E8: money is int64 minor units, no floating point (TD §1.1)"
        - pattern: ^strconv\.(ParseFloat|FormatFloat)$
          msg: "E8: money is int64 minor units, no floating point (TD §1.1)"
        - pattern: ^time\.(Now|Since|Until)$
          msg: "E21: business time comes from now() in SQL (TD §1.3)"
        - pattern: ^context\.(Background|TODO)$
          msg: "E28: pass the caller's context; tests use t.Context()"
    funlen:
      lines: 60
      statements: -1
    gocognit:
      min-complexity: 15
    gocritic:
      disable-all: true
      enabled-checks:
        - commentedOutCode
    gosec:
      includes:
        - G101
    nolintlint:
      require-explanation: true
      require-specific: true
    revive:
      rules:
        - name: context-as-argument
        - name: exported
        - name: flag-parameter
  exclusions:
    generated: lax
    rules:
      - path: _test\.go$
        linters: [funlen]
      - path: ^scripts/
        linters: [forbidigo, funlen, gochecknoglobals, gocognit]
      - path: ^cmd/
        linters: [forbidigo]
        text: "E28"

issues:
  max-issues-per-linter: 0
  max-same-issues: 0

formatters:
  enable:
    - gofmt
    - goimports
  settings:
    goimports:
      local-prefixes:
        - github.com/imlukasd/Bastion
```

**Taskfile.yml**:
```yaml
version: '3'

silent: true

tasks:
  check:
    desc: "Baseline: build, vet, golangci-lint, govulncheck, unit tests (-short), plan check; prints failures only"
    cmds:
      - go run ./scripts/check
  test:
    desc: "Full suite with the race detector (TD §34.1)"
    cmds:
      - go test -race -count=1 ./...
  up:
    desc: "Start the local stack (P1-02)"
    cmds:
      - echo "task up: not implemented yet (P1-02)" >&2; exit 1
  down:
    desc: "Stop the local stack (P1-02)"
    cmds:
      - echo "task down: not implemented yet (P1-02)" >&2; exit 1
  migrate:
    desc: "Apply the Pay and Tickets migrations (P1-02)"
    cmds:
      - echo "task migrate: not implemented yet (P1-02)" >&2; exit 1
  chaos:
    desc: "Run chaos scenarios, all or one: task chaos -- C4 (P2-04)"
    cmds:
      - echo "task chaos: not implemented yet (P2-04)" >&2; exit 1
```

**scripts/check** (`package main`, stdlib only):
```go
// step is one baseline command; args[0] is the program.
type step struct {
	name string
	args []string
}

// execFunc runs one step and returns its combined stdout and stderr.
type execFunc func(ctx context.Context, s step) ([]byte, error)

// baseline returns the steps of task check, in order.
func baseline() []step

// run executes steps in order and stops at the first failure.
func run(ctx context.Context, w io.Writer, steps []step, exec execFunc) error

// execStep runs s with exec.CommandContext in the current directory.
func execStep(ctx context.Context, s step) ([]byte, error)
```
- `baseline()` returns exactly: `build` = `go build ./...`; `vet` = `go vet ./...`; `lint` = `go tool -modfile=tools.mod golangci-lint run ./...`; `vuln` = `go tool -modfile=tools.mod govulncheck ./...`; `unit` = `go test -short ./...`; `plan` = `go run scripts/brief/main.go -check`.
- `run`: a passing step writes nothing. A failing step writes `FAIL <name>: <args joined by one space>\n`, then its output (plus `\n` if the output does not end with one), and `run` returns `fmt.Errorf("%s: %w", name, err)` without running later steps. When every step passes, `run` writes `check OK\n` and returns nil.
- `main`: `ctx` from `signal.NotifyContext(context.Background(), os.Interrupt)`; `run(ctx, os.Stdout, baseline(), execStep)`; exit 1 on error, printing nothing more.

**Skeleton files**:
- `doc.go`: `// Package <name> <one-line role from TD §2 or ARCH §7>.` then `package <name>`; package name = folder name (`payclient`, `archtest`, …). Test folders: `package integration`, `property`, `model`, `concurrency`, `chaos`.
- `cmd/<name>/main.go`: `// Command <name> <role>. Not implemented yet.`, `package main`, and a `main` that writes `<name>: not implemented yet` to `os.Stderr` and calls `os.Exit(1)`.

**internal/archtest/lint_test.go** (`package archtest`; the repository root is `../..`):
- The lint-marked rules are the rows `| E<n> | … | <Enforced by> |` of `docs/ENGINEERING.md` whose last column contains the word `lint`; rule E<n> maps to the fixture folder `e<NN>` (two digits).
- golangci-lint runs from the root as `go tool -modfile=tools.mod golangci-lint run --output.json.path=stdout --show-stats=false ./internal/archtest/testdata/lint/...`; the test reads `Issues[].FromLinter`, `Issues[].Text`, `Issues[].Pos.Filename` and `Issues[].Pos.Line`. Exit status 1 with issues is expected.
- Lint fixtures, one package per folder under `testdata/lint/` (folder `eNN` = rule ENN): `e05` a package-level `var` and an `init`; `e08` a `float64` variable, a `float64(...)` conversion and `strconv.ParseFloat`; `e09` a switch missing a constant, with and without `default`; `e11` a 61-line body, a 60-line body (not reported), cognitive complexity 16, and a `bool` parameter that switches behavior; `e13` an unchecked error, `err == ErrX` and `fmt.Errorf("…: %v", err)`; `e21` `time.Now` and `time.Since`; `e28` `ctx` as second parameter, `context.Background()` and `context.TODO()`, and in `noctx.go` an `http.Get` without a context; `e36` a high-entropy string literal assigned to `password` (no real provider prefix such as `sk_live_`, so push protection does not fire); `e37` an exported function without a doc comment and a commented-out statement; `e40` a badly formatted file, a file with gofmt-sorted but ungrouped local imports, a bare `//nolint` and a `//nolint:errcheck` without a reason; `clean` compliant code (an `ErrX` sentinel, exhaustive switch, `%w`, `errors.Is`, `ctx` first, doc comments, `int64` money) with no issue.

**AGENTS.md** › Conventions, replacing "Commands: none yet. …":
```
- Commands (Taskfile.yml, TD §35.1). Prerequisites: Go 1.27.3 (go.dev MSI; the auto-downloaded toolchain is blocked by Windows Application Control), task 3.54.0 (`winget install Task.Task`), gcc for `-race`, Docker from P1-02.
  - `task check`: the baseline: build, vet, golangci-lint, govulncheck, unit tests (`-short`), plan check. Prints failures only; `check OK` when green.
  - `task test`: the full suite with `-race` (G3).
  - `task up`, `task down`, `task migrate`: local stack and migrations (P1-02).
  - `task chaos [-- C4]`: chaos scenarios (P2-04).
  - Lint and vulnerability tools are pinned in `tools.mod`: `go tool -modfile=tools.mod golangci-lint run ./...`; upgrade with `go get -tool -modfile=tools.mod <tool>@<version>`.
```

## Steps
0. Prerequisites (owner, once per machine): Go 1.27.3 and task 3.54.0 installed. Verify: `go version` → `go1.27.3`; `task --version` → `3.54.0`.
1. `go.mod` and `tools.mod` per Contracts. Verify: `go list -m` → `github.com/imlukasd/Bastion`; `go mod edit -json` → `"Go": "1.27.0"`, no `Require` or `Tool`; `go mod edit -json tools.mod` → `Tool` lists golangci-lint and govulncheck.
2. Skeleton folders and files per Contracts. Verify: `go build ./...` → no output; `go list ./...` → 56 packages; `git ls-files --others --exclude-standard | grep -c .gitkeep` → `6`.
3. `.golangci.yml`, the lint fixtures and `internal/archtest/lint_test.go` per Contracts. Verify: `go tool -modfile=tools.mod golangci-lint config verify` → exit 0; `go tool -modfile=tools.mod golangci-lint run ./...` → `0 issues.`; `go test ./internal/archtest/` → `ok`.
4. `scripts/check` and its tests per Contracts. Verify: `go test ./scripts/check/` → `ok`.
5. `Taskfile.yml` per Contracts. Verify: `task --list-all` → check, test, up, down, migrate, chaos, each with its description; `task up` → `task up: not implemented yet (P1-02)`, exit 1.
6. AGENTS.md per Contracts. Verify: AGENTS.md › Conventions names the six tasks of step 5.
7. Whole item. Verify: `task check` → `check OK`; `task test` → exit 0; `go run scripts/brief/main.go -check` → `plan OK, item P1-01 OK`. The handoff's LOG.md entry gives the reason for the two tool dependencies (E39).

## Test map
| Clause | Test (file › name) or verify command | Asserts |
| --- | --- | --- |
| 1 | Step 1 verify: `go list -m`, `go mod edit -json` | Module path `github.com/imlukasd/Bastion`; `go 1.27.0` (>= 1.25) |
| 2 | Step 2 verify: `go build ./...`, `go list ./...` (56 packages), 6 `.gitkeep` files | Every TD §2 folder of Files exists and builds |
| 3 | `scripts/check/main_test.go` › `TestBaseline_Steps` | The six steps, in order, with the exact commands of Contracts |
| 4 | `scripts/check/main_test.go` › `TestRun_PassingStepsPrintOnlyCheckOK` | All steps run; output is exactly `check OK\n`, though each step produced output |
| 4 | `scripts/check/main_test.go` › `TestRun_FailingStepPrintsItsOutputAndStops` | Output is exactly `FAIL vet: go vet ./...\n` + the step's output; later steps never run; the error names the step |
| 5 | Step 5 verify: `task --list-all`, `task up` | The six tasks exist with descriptions; a placeholder exits 1 with its message |
| 6 | `internal/archtest/lint_test.go` › `TestLint_RulesMarkedLintHaveFixtures` | Every ENGINEERING.md rule whose "Enforced by" column says `lint` has a `testdata/lint/eNN` fixture folder |
| 6 | `internal/archtest/lint_test.go` › `TestLint_FixturesReportExpectedIssues` (skipped with `-short`) | golangci-lint reports exactly the expected (file, line, linter) set on the fixtures, forbidigo texts carry the rule ID, and `clean` has no issue |
| 7 | Step 7 verify: `task check` → `check OK` | The baseline is green on the skeleton |
| 8 | Step 6 verify (review) | AGENTS.md › Conventions names `task check`, `task test`, `task up`, `task down`, `task migrate`, `task chaos` |

## Out of scope
- Compose, migrations, real `up`/`down`/`migrate` (P1-02); real `chaos` (P2-04); `task load` and `task demo` (Phase 7 and P8 items).
- Import-boundary rules in `internal/archtest` (P1-04).
- Package contents of platform, kit, products, audit, simulators (P1-03 onwards); lint exemptions those packages need (their items).
- CI on GitHub Actions (deferred by the owner, 2026-10-28).
- Refactoring `scripts/brief` to meet the product lint rules.

## Stop rules
Stop and report instead of choosing when:
- a decision is needed that this brief does not make;
- a file outside Files must change, a dependency not named here is needed, or a test seems wrong;
- the code or the spec contradicts this brief;
- golangci-lint v2.15.0 rejects a key of the Contracts config, or reports on a fixture anything other than the expected issues (never loosen the config or edit a fixture to make it pass);
- govulncheck reports a vulnerability on the skeleton;
- `go get -tool -modfile=tools.mod` writes to `go.mod`;
- Windows Application Control blocks a binary again after one retry.

## Progress
- 2026-10-28 Step 0: `go version` → `go version go1.27.3 windows/amd64`; `task --version` → `3.54.0` (installed with winget; a shell started before the install needs a restart to find it on PATH).
- 2026-10-28 Step 1: `go list -m` → `github.com/imlukasd/Bastion`; `go mod edit -json` → `"Go": "1.27.0"`, `"Toolchain": "go1.27.3"`, no `Require` or `Tool`; `go mod edit -json tools.mod` → `Tool`: golangci-lint, govulncheck; go.mod unchanged by `go get -tool`; `golangci-lint version` → 2.15.0, `govulncheck -version` → v1.9.0, both built with go1.27.3.
- 2026-10-28 Step 2: `go build ./...` and `go vet ./...` → no output; `go list ./...` → 55 packages (the 54 skeleton packages + `scripts/brief`; the brief's 56 counts `scripts/check`, which Step 4 adds); 6 untracked `.gitkeep` files; gofmt clean.
- 2026-10-28 Step 3: `golangci-lint config verify` → exit 0; `golangci-lint run ./...` → `0 issues.`; `go test -count=1 -v ./internal/archtest/` → both tests PASS (fixtures: exactly the 25 expected issues, nothing in `clean`, the 60-line body not reported); with `-short` the fixture test skips. Sanity check: `default-signifies-exhaustive: true` and `funlen lines: 61` → `TestLint_FixturesReportExpectedIssues` FAIL naming `e09/status.go:27 exhaustive` and `e11/long.go:69 funlen`; restored.
- 2026-10-28 Step 4: `go test -count=1 ./scripts/check/` → `ok` (3 tests); `go list ./...` → 56 packages. errcheck required the runner's writes to be checked (E13): a failed write is returned as `write <step> output: …` / `write result: …`. An empty step output adds no extra newline. `go run ./scripts/check` on the tree → `check OK` in 13 s; with a temporary broken file → `FAIL build: go build ./...` + the compiler output, exit 1, later steps not run (`go run` adds its own `exit status 1` line); file removed.
- 2026-10-28 Step 5: `task --list-all` → chaos, check, down, migrate, test, up, each with its description; `task up` → `task up: not implemented yet (P1-02)` on stderr, the command exits 1 (`task: Failed to run task "up": exit status 1`); same for down, migrate, chaos and `task chaos -- C4`.
- 2026-10-28 Step 6: AGENTS.md › Conventions › Commands holds the Contracts text and names `task check`, `task test`, `task up`, `task down`, `task migrate`, `task chaos` (AGENTS.md: 115 lines).
- 2026-10-28 Step 7 (partial): `task check` → `check OK` (8 s); `go run scripts/brief/main.go -check` → `plan OK, item P1-01 OK`. `task test` → FAIL twice: `fork/exec …\go-build…\archtest.test.exe: An Application Control policy has blocked this file.`; every other package passes (`scripts/check` ok under `-race`). Stopped by the stop rule; waiting for the owner (NOW.md › Open questions).
- 2026-10-28 Owner: G3 accepted with the Smart App Control exception (the archtest `-race` test binary, built with `go test -race -c`, passed both tests when run from its folder; every other package passes under `-race`).
- 2026-10-28 G5 round 1: CHANGES NEEDED, no blocker, 1 major (D13) and 6 minors. Fixed in `af91eb6`: lint_test handles the `strconv.Atoi` error, and a forbidigo issue in `clean/` now reaches the diff instead of stopping the test. D13 (owner chose to enable noctx and add review marks): noctx enabled, fixture `e28/noctx.go` (`e28/noctx.go:7 noctx` expected), ENGINEERING.md E9/E13/E28 marked "lint, review", E11 reworded to "at most". `golangci-lint run ./...` → `0 issues.`; `go test -count=1 -v ./internal/archtest/` → both PASS (26 expected issues).
- 2026-10-28 Owner: `approve`. Evidence at 4ddf130; brief moved to completed; merged into `main` with `--no-ff`.

## Surprises
- 2026-10-28: Go 1.25 is out of support (current: 1.27.3, 1.26.10) and govulncheck flags its standard library; the latest tools need Go >= 1.26 (D2).
- 2026-10-28: the go command's automatic toolchain download is unusable here: Windows Application Control blocks the downloaded `asm.exe` on every run (compile and link pass). The installed Go under Program Files is not blocked.
- 2026-10-28: switching toolchains during `go get -tool -modfile=…` fails with "-modfile cannot be used with commands that ignore the current module"; with a matching local Go it works.
- 2026-10-28 (brief gap, Step 5): the Contracts Taskfile is invalid YAML: the plain scalar `echo "task up: not …"` contains `: `, so task 3.54.0 reads it as a mapping ("invalid keys in command"). Fixed by single-quoting each placeholder command; the command text and its behavior are unchanged.
- 2026-10-28 (Step 5): task reports a failing command with its own exit code 201, not the command's 1; `task -x <name>` passes the command's code through. D10's purpose (a placeholder never exits 0) holds either way.
- 2026-10-28 (Step 7): Smart App Control is on in enforcement mode (`HKLM\…\CI\Policy` `VerifiedAndReputablePolicyState` = 1). The Code Integrity log (events 3033/3077, policy {0283ac0f-fff1-49ae-ada1-8a933130cad6}) shows it blocking the `-race` build of `archtest.test.exe`, which is unsigned and has no reputation, on both runs. The same package passes without `-race`. The same `-race` test binary, built with `go test -race -c` into `%TEMP%` and run from the package folder, passes both tests. The block is per binary, so any later test binary can hit it. Only the `docker-desktop` WSL distribution exists on this machine.
- 2026-10-28 (brief gap, D13): noctx flags two `exec.Command` calls in `scripts/brief` (outside Files and out of scope). Answered by D9: `scripts/` is tooling, exempt from the product rules, so noctx joins its exclusion list.
- 2026-10-28 (Step 1): in PowerShell, `-modfile=tools.mod` reaches the go command as `-modfile=tools` (PowerShell splits the argument at the dot); run such commands from bash, cmd or task, or quote the argument.

## Outcome

## Item report — P1-01 Repository skeleton: go.mod, layout, Taskfile, lint

Writer's assessment: ready for approval

0. Executor: strong (main session); no implementer runs. Stops: Step 7, Smart App Control blocked a test binary (stop rule) → owner accepted G3 with that exception; Step 5, invalid YAML in the Contracts Taskfile → answered by quoting (Surprises); D13, noctx in `scripts/brief` → answered by D9 (Surprises). Skipped gates: G2 (D12: a tooling item with no invariant; the tests came with the code). G4: not applicable (not critical).
1. Scope: `git diff --stat main...HEAD` → 88 files, +2545 −21. Core: `.golangci.yml`, `internal/archtest/lint_test.go` and `testdata/lint/**`, `scripts/check/main.go` and its test, `Taskfile.yml`, `go.mod`, `tools.mod` and `tools.sum` (tool pins), 54 skeleton `doc.go`/`main.go` files, AGENTS.md, ENGINEERING.md. Every path is in Files: `-check` → `plan OK, item P1-01 OK`.
   Tests changed after they were first written: `internal/archtest/lint_test.go` in `af91eb6` (G5 minors) and `4ddf130` (the noctx expected issue, D13); G2 skipped.
2. Traceability:
   | Clause | Test or verify | Result |
   | --- | --- | --- |
   | 1 go.mod, Go >= 1.25 | `go list -m`, `go mod edit -json` | `github.com/imlukasd/Bastion`; `"Go": "1.27.0"`, `"Toolchain": "go1.27.3"` |
   | 2 TD §2 layout | `go build ./...`, `go list ./...`, `.gitkeep` count | no output; 56 packages; 6 `.gitkeep` |
   | 3 check runs the six steps | `scripts/check/main_test.go` › `TestBaseline_Steps` | PASS |
   | 4 prints failures only | › `TestRun_PassingStepsPrintOnlyCheckOK`, `TestRun_FailingStepPrintsItsOutputAndStops`; live: broken file → `FAIL build: go build ./...` + output | PASS |
   | 5 test, up, down, migrate, chaos | `task --list-all`, `task up` | six tasks with descriptions; message on stderr, command exits 1 (task: 201) |
   | 6 lint rules enabled | `internal/archtest/lint_test.go` › `TestLint_RulesMarkedLintHaveFixtures`, `TestLint_FixturesReportExpectedIssues` | PASS (26 expected issues, `clean` none) |
   | 7 task check passes | `task check` | `check OK` |
   | 8 AGENTS.md lists the commands | review of AGENTS.md › Conventions | names the six tasks |
3. Guards: no invariant in this item. The lint rules guard later items: each lint-marked rule has a fixture, and loosening `exhaustive` or `funlen` made `TestLint_FixturesReportExpectedIssues` fail (Progress, Step 3).
4. Verify at `4ddf130`: `task check` → `check OK`; `go run scripts/brief/main.go -check` → `plan OK, item P1-01 OK`; `golangci-lint run ./...` → `0 issues.`; `go test -count=1 -v ./internal/archtest/` → 2 PASS; `go test ./scripts/check/` → `ok`. `task test` → blocked twice by Smart App Control on the `-race` build of `archtest.test.exe`, every other package `ok` under `-race`; the same binary built with `go test -race -c` → 2 PASS (owner accepted). No concurrency tests in this item.
5. Spec: no deviation from docs/01–04. ENGINEERING.md: E9, E13, E28 now "lint, review"; E11 says "at most 60 lines, cognitive complexity at most 15" (D13, owner).
6. Decisions and brief gaps: D13 (owner). Taskfile placeholders single-quoted (YAML). task exits 201 for a failing command, not 1; D10's purpose holds and `task -x` passes 1. Step 2 counted 55 packages, 56 after Step 4 adds `scripts/check`. `scripts/` exempt from noctx (D9). PowerShell splits `-modfile=tools.mod`; use bash, cmd or task.
7. Review: `reviewer` round 1 CHANGES NEEDED, 0 blockers. Major (E9/E13/E28 lint only, gaps unchecked) → D13, fixed in `4ddf130`. Minors: `strconv.Atoi` error discarded in lint_test (E13) → fixed `af91eb6`; forbidigo in `clean/` stopped the test → fixed `af91eb6`; E11 limits off by one → E11 reworded (D13); aliased `time` import passes forbidigo (E21) → D8, covered by review; LOG.md reason for the tool dependencies (E39) and NOW.md → this handoff; brief gaps → §6.
8. Follow-ups: Smart App Control (NOW.md › Open questions). Next item: P1-02 Local stack (Compose, two PostgreSQL databases, Redis, migrations); it needs the Docker daemon running.
