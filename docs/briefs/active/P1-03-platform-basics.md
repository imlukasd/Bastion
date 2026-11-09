# P1-03 — Platform basics: money, ids, config, transactions, logging

Executor: strong · Owner: Claude · Started: 2026-11-09 · Approved: 2026-11-09
Skip: none

## Goal
The item's `done_when`, one clause per line:
1. money: `int64` minor units, basis points as `floor(amount × bps / 10 000)` with the 9.2 × 10¹⁴ bound, no floating point.
2. ids: UUIDv7.
3. config loads the TD §33 variables needed so far.
4. db: pgx pool.
5. db: a transaction helper that retries `40001`/`40P01` up to 3 times with jitter.
6. db: maps SQLSTATEs per TD §1.5.
7. logx: slog JSON.
8. Unit tests cover the money bounds and the error mapping.

## Spec
- TD §1.1 Money: "Amounts are `int64` minor units stored as `bigint`; currency VND (exponent 0); every amount has a `currency char(3)`." "Basis points are integer-only: `floor(amount × bps / 10 000)`, with `amount ≤ 9.2 × 10¹⁴`." "No floating point on any money path, including tests."
- TD §19 (Tickets DDL): `fee_bps int NOT NULL CHECK (fee_bps BETWEEN 0 AND 10000)`; Pay stores `mdr_wallet_bps int`, `mdr_card_bps int` (TD §8.2). So basis points lie in 0 … 10 000.
- INV M1 Prevent: "every split is exact (MDR, commission and resale fee use `floor`, the remainder goes to the other party)." ENGINEERING E8: "fees and splits only through `platform/money` (floor of basis points, remainder to the other party)."
- TD §9.3 / §26 worked examples: 13,118,000 at 150 bps → 196,770; at 250 bps → 327,950; resale 1,400,000 at 500 bps → 70,000; 1,430,000 at 250 bps → 35,750.
- TD §1.2 Identifiers: "Internal IDs are UUIDv7 stored as `uuid`; partner references are `text` with `UNIQUE` constraints; public IDs carry a prefix in the API (`pi_`, `re_`, `dp_`, `tk_`, `ls_`)." AD-14: "`int64` money; UUIDv7 IDs".
- TD §33 Configuration (row needed so far): "`PAY_DB_URL`, `TIX_DB_URL`, `BASTION_REDIS_URL` | — | Required". Every other §33 row belongs to the item that first uses it.
- TD §1.5 Errors:

  | SQLSTATE | Meaning | Handling |
  | --- | --- | --- |
  | `23505` | Duplicate effect | "Already done"; never retried |
  | `23514` | A guard or bound was hit | Domain error; `guard_bypass_total` if a guarded `UPDATE` should have prevented it |
  | `40001` / `40P01` | Serialization failure / deadlock | Whole-transaction retry ≤ 3, jittered; `deadlocks_total` alerts |
  | `55P03` | `lock_timeout` | API `503`; worker next tick |

  "Modules export sentinel errors; each API maps them to problem types (§29.6)." ENGINEERING E14: "SQLSTATEs are mapped once, in `platform/db`, per TD §1.5."
- TD §2: "Module APIs take `pgx.Tx` so callers compose transactions." ENGINEERING E20: "Transaction bodies are retry-safe: the helper reruns them on `40001`/`40P01` (TD §1.5), so they have no effect outside the transaction." E21: "Go's clock is only for timeouts, backoff and metrics." E25: retries "bounded, with full jitter".
- TD §31: the auditor uses "short `REPEATABLE READ READ ONLY` snapshots", so the helper takes `pgx.TxOptions`.
- TD §32 / AD-21: "Logs: `slog` JSON with request, trace and object IDs; PINs, OTPs, client secrets, API keys, card tokens and QR payloads are never logged." ENGINEERING E35, E36.

## Decisions
| # | Question | Decision | By | Why |
| --- | --- | --- | --- | --- |
| D1 | Basis-point domain and the bound | `amount` in `0 … MaxBPSAmount` (`920_000_000_000_000`, inclusive), `bps` in `0 … 10_000`; anything else → `ErrOutOfRange`. Then `amount × bps ≤ 9.2 × 10¹⁸ < 2⁶³ − 1`, so plain `int64` arithmetic never overflows and integer division of non-negatives is the floor. | writer | TD §1.1 bound; §19 `CHECK (fee_bps BETWEEN 0 AND 10000)`; fees are never computed on negative amounts. |
| D2 | Money API shape | Functions on `int64` (`BPS`, `Split`) plus a `Currency` type with the constant `VND`; no `Money` struct. | writer | sqlc maps `bigint` to `int64`; a struct would add conversions at every query without preventing an error the DB `CHECK`s do not already catch. `Currency` fixes the name later items use (E9). |
| D3 | UUID library | Standard library `uuid` (Go 1.27): `uuid.NewV7`. No new module. | spike | Spike 2026-11-09: `uuid.UUID` round-trips through pgx v5.12 as `$1::uuid` and scans from a `uuid` column; `NewV7` sorts increasingly. |
| D4 | Keeping IDs UUIDv7 | `ids.New()` is the only constructor; forbidigo bans `uuid.New` and `uuid.NewV4` outside `internal/platform/ids`. | writer | AD-14 enforced by lint, like E8/E21. |
| D5 | `ids.Parse` strictness | Exactly the 36-character hyphenated form, version 7, RFC 9562 variant; else `ErrInvalid`. | writer | API input is checked once; braces/URN forms are never valid in Bastion. |
| D6 | Public ID prefixes (`pi_` …) | Not in this item. | writer | Their encoding is undefined in the spec; the first API that exposes one decides (P2-15 / P3). |
| D7 | Config variables loaded now | `PAY_DB_URL`, `TIX_DB_URL`, `BASTION_REDIS_URL` only, in per-product structs `config.Pay` and `config.Tickets`; later items add their §33 variables as fields with the §33 default. | writer | "needed so far" (done_when); a Pay process must not require `TIX_DB_URL`. |
| D8 | Config validation and secrets | Required and non-empty; DB URLs parse with scheme `postgres`/`postgresql` and a host; Redis URL with scheme `redis`/`rediss` and a host. All problems are reported together (`errors.Join`); messages name the variable, never its value (URLs carry passwords, E36). Values come through a `Lookup` function (`os.LookupEnv` in `cmd`). | writer | E5 (no global state; tests pass a map), E36. |
| D9 | Retry count | "Up to 3 times" = up to 3 retries after the first attempt: at most 4 attempts. | writer | Literal reading of done_when and TD §1.5 "retry ≤ 3". |
| D10 | Backoff | Full jitter: before retry `n` (1 … 3) sleep a uniform random duration in `[0, min(100 ms, 10 ms × 2ⁿ⁻¹))` from `math/rand/v2`; the sleep stops when `ctx` is done. | writer | E25 full jitter; total wait ≤ 70 ms keeps request latency bounded. Not crypto-relevant. |
| D11 | What is retried | A `*pgconn.PgError` with code `40001` or `40P01` anywhere in the error chain, returned by the body or by `COMMIT`; the transaction is rolled back and the whole body rerun. Errors from `BEGIN` and every other code are not retried. | writer | TD §1.5; E20. |
| D12 | Error mapping form | `MapError` adds the sentinel with `fmt.Errorf("%w: %w", sentinel, err)`, so both `errors.Is(err, db.ErrDuplicate)` and `errors.As(err, &*pgconn.PgError)` (constraint name) work; already-mapped errors are returned unchanged; unknown codes and non-PostgreSQL errors pass through unchanged. `InTx` returns mapped errors. | writer | E14: mapped once, in `platform/db`; callers need the constraint name to tell which effect was a duplicate. |
| D13 | Metrics `deadlocks_total`, `guard_bypass_total` | Not emitted here. | writer | Prometheus arrives with P8-01 (TD §32 metrics); no metrics dependency in Phase 1. |
| D14 | Pool settings | `db.Open` uses pgx defaults (`pgxpool.ParseConfig`), then pings; it sets no session parameters. | writer | §33 names no pool variables. |
| D15 | `lock_timeout` value (spec gap: §1.5 maps `55P03`, no section sets `lock_timeout`) | P1-03 sets no `lock_timeout`; it maps `55P03` whatever sets it. The gap goes to NOW.md "Open questions" for the item that defines API and worker timeouts. | owner (2026-11-09) | No spec change now; the mapping does not depend on the value. |
| D16 | Log level | `logx.New` takes the level as a parameter; no environment variable. | writer | §33 has no log-level variable; adding one is a spec change. |
| D17 | Property check for money without a new dependency | Deterministic loop over `math/rand/v2` with a fixed PCG seed, checked with `math/big` (no floats). | writer | `rapid` arrives with P1-08; E33 determinism. |

## Files
Create:
- `internal/platform/money/money.go`
- `internal/platform/money/money_test.go`
- `internal/platform/ids/ids.go`
- `internal/platform/ids/ids_test.go`
- `internal/platform/config/config.go`
- `internal/platform/config/config_test.go`
- `internal/platform/db/db.go`
- `internal/platform/db/errors.go`
- `internal/platform/db/tx.go`
- `internal/platform/db/errors_test.go`
- `internal/platform/db/backoff_test.go`
- `internal/platform/db/db_integration_test.go`
- `internal/platform/logx/logx.go`
- `internal/platform/logx/logx_test.go`
Modify:
- `internal/platform/{money,ids,config,db,logx}/doc.go`: package comments only, if the API description needs it
- `.golangci.yml`: forbidigo rule for `uuid.New`/`uuid.NewV4` (D4), excluded for `internal/platform/ids`
- `go.mod`, `go.sum`: `pgxpool` brings `github.com/jackc/puddle/v2` (indirect); reason in LOG.md (E39)

## Contracts

```go
// package money — TD §1.1
type Currency string
const VND Currency = "VND" // exponent 0

const (
	MaxBPS       int64 = 10_000
	MaxBPSAmount int64 = 920_000_000_000_000 // 9.2 × 10^14
)
var ErrOutOfRange = errors.New("money: out of range")

// BPS returns floor(amount × bps / 10 000).
// ErrOutOfRange (wrapped, naming the bad argument) unless 0 ≤ amount ≤ MaxBPSAmount and 0 ≤ bps ≤ MaxBPS.
func BPS(amount, bps int64) (int64, error)

// Split returns share = BPS(amount, bps) and rest = amount − share (the remainder to the other party, INV M1).
func Split(amount, bps int64) (share, rest int64, err error)
```

```go
// package ids — TD §1.2, AD-14
var ErrInvalid = errors.New("ids: invalid id")
func New() uuid.UUID                     // uuid.NewV7()
func Parse(s string) (uuid.UUID, error) // D5; ErrInvalid wrapped
```

```go
// package config — TD §33
type Lookup func(key string) (string, bool) // os.LookupEnv
var (
	ErrMissing = errors.New("config: required variable missing")
	ErrInvalid = errors.New("config: invalid value")
)
type Pay struct {
	DBURL    string // PAY_DB_URL
	RedisURL string // BASTION_REDIS_URL
}
type Tickets struct {
	DBURL    string // TIX_DB_URL
	RedisURL string // BASTION_REDIS_URL
}
func LoadPay(lookup Lookup) (Pay, error)
func LoadTickets(lookup Lookup) (Tickets, error)
```
Each error is `fmt.Errorf("%w: %s", ErrMissing|ErrInvalid, NAME)` (plus a reason for `ErrInvalid` that never contains the value); several are joined with `errors.Join`. An unset variable and one set to `""` are both `ErrMissing`.

```go
// package db — TD §1.5, §2
var (
	ErrDuplicate      = errors.New("db: duplicate")              // 23505
	ErrCheckViolation = errors.New("db: check violation")        // 23514
	ErrSerialization  = errors.New("db: serialization failure")  // 40001
	ErrDeadlock       = errors.New("db: deadlock")               // 40P01
	ErrLockTimeout    = errors.New("db: lock timeout")           // 55P03
)
const MaxRetries = 3

// Open parses url, creates a pgx pool and pings it; the error never contains the password.
func Open(ctx context.Context, url string) (*pgxpool.Pool, error)

// MapError: D12. nil → nil.
func MapError(err error) error

// InTx runs fn in one transaction with opts (pgx.BeginTxFunc semantics: commit when fn returns nil,
// rollback on error or panic). D9–D11: on 40001/40P01 it rolls back, waits backoff(n) and reruns fn,
// at most MaxRetries times. Returns MapError(last error). If ctx ends during a backoff it returns
// errors.Join(ctx.Err(), MapError(last error)). fn must be retry-safe (E20). Not reentrant.
func InTx(ctx context.Context, pool *pgxpool.Pool, opts pgx.TxOptions, fn func(pgx.Tx) error) error

// unexported, tested in backoff_test.go (package db):
func backoff(retry int) time.Duration // D10; retry is 1-based
```

```go
// package logx — AD-21, E35
// New returns a slog logger with a JSON handler on w at level; every record carries service=<service>.
func New(w io.Writer, level slog.Leveler, service string) *slog.Logger
```
Output: one JSON object per line with keys `time`, `level`, `msg`, `service`, then the call's attributes.

`.golangci.yml` forbidigo adds: pattern `^uuid\.(New|NewV4)$`, msg `"AD-14: internal IDs are UUIDv7; use ids.New (TD §1.2)"`; an exclusion for path `^internal/platform/ids/` on that text.

## Steps
1. `money` and its tests. Verify: `go test ./internal/platform/money/` → ok.
2. `ids`, its tests and the forbidigo rule. Verify: `go test ./internal/platform/ids/` → ok; `task check` → `check OK` (the rule loads and nothing in the repo violates it).
3. `config` and its tests. Verify: `go test ./internal/platform/config/` → ok.
4. `db`: `errors.go` + `errors_test.go`, then `db.go`, `tx.go`, `backoff_test.go`, `db_integration_test.go`; go.mod/go.sum. Verify: `go test -short ./internal/platform/db/` → ok (integration skipped); `go test -race -count=1 ./internal/platform/db/...` → ok (Docker running).
5. `logx` and its tests. Verify: `go test ./internal/platform/logx/` → ok.
6. Gates. Verify: `task check` → `check OK`; `task test` → all ok; `go test ./internal/platform/...` (item verify) → ok; G4 mutations below each make their test fail.

## Test map
| Clause | Test (file › name) | Asserts |
| --- | --- | --- |
| 1, 8 | `money/money_test.go › TestBPS_WorkedExamples` | 13,118,000 @150 → 196,770; @250 → 327,950; 1,400,000 @500 → 70,000; 1,430,000 @250 → 35,750 |
| 1, 8 | `… › TestBPS_Floors` | 9,999 @1 → 0; 19,999 @5,000 → 9,999; 1 @9,999 → 0; 10,000 @1 → 1 (never rounds up) |
| 1, 8 | `… › TestBPS_Bounds` | `MaxBPSAmount` @10,000 → `MaxBPSAmount` (no overflow); `MaxBPSAmount` @1 → 92,000,000,000; 0 @any → 0; any @0 → 0 |
| 1, 8 | `… › TestBPS_RejectsOutOfRange` | amount −1, `MaxBPSAmount+1`, `math.MaxInt64`; bps −1, 10,001 → `ErrOutOfRange`, result 0 |
| 1 | `… › TestSplit_IsExact` | 100,000 deterministic samples (PCG seed recorded in the test) over the full domain: `share + rest == amount`, `0 ≤ share ≤ amount`, and with `math/big`: `share × 10,000 ≤ amount × bps < (share + 1) × 10,000` |
| 1 | `… › TestSplit_RejectsOutOfRange` | same bad inputs → `ErrOutOfRange` |
| 1 | `task check` (forbidigo `float32/64`, `strconv.ParseFloat/FormatFloat`) | No floating point in `money`, tests included |
| 2 | `ids/ids_test.go › TestNew_IsVersion7` | version nibble 7, variant `10`, 1,000 IDs strictly increasing |
| 2 | `… › TestParse` | round-trips `New().String()`; rejects version 4, wrong variant, braces, URN, the 32-hex form, 35 and 37 characters, `""` → `ErrInvalid` |
| 2 | `db/db_integration_test.go › TestOpen_StoresUUIDv7` | `ids.New()` inserted into a `uuid` column and scanned back equal |
| 3 | `config/config_test.go › TestLoadPay` / `TestLoadTickets` | valid values load; each required variable unset or `""` → `ErrMissing` naming it; all missing → all names in one error |
| 3 | `… › TestLoad_RejectsInvalidURLs` | wrong scheme, no host, unparsable → `ErrInvalid` naming the variable; the error text does not contain the value's password |
| 4 | `db/db_integration_test.go › TestOpen_ConnectsAsApp` | `Open(PayAppURL)` → `SELECT current_user` = `pay_app`; `Open` with a wrong password → error whose text lacks the password |
| 5 | `… › TestInTx_RetriesUpToThreeTimes` | subtests 40001 and 40P01: body fails with that code k times; k = 3 → nil, 4 calls; k = 4 → `ErrSerialization`/`ErrDeadlock`, exactly 4 calls; effects of failed attempts absent (rolled back) |
| 5 | `… › TestInTx_RetriesSerializationFailure` | real: `REPEATABLE READ` body reads a row, a second connection updates and commits it (first attempt only), body updates → 40001 → rerun; 2 calls; row value reflects both updates |
| 5 | `… › TestInTx_RetriesDeadlockVictim` | real: two concurrent `InTx` lock rows 1→2 and 2→1 behind a barrier on their first attempt; both return nil; 3 calls in total; both updates stored |
| 5, 6 | `… › TestInTx_DoesNotRetryOtherErrors` | subtests 23505, 23514, 55P03 (real `SET LOCAL lock_timeout` against a held row lock), a plain Go error: 1 call each; mapped sentinel (or the Go error unchanged) |
| 5, 6 | `… › TestInTx_MapsCommitErrors` | `DEFERRABLE INITIALLY DEFERRED` unique constraint violated → error at `COMMIT` → `ErrDuplicate`, 1 call, nothing stored |
| 5 | `… › TestInTx_StopsWhenContextEnds` | body cancels ctx and returns 40001 → 1 call; `errors.Is(err, context.Canceled)` and `errors.Is(err, ErrSerialization)` |
| 5 | `db/backoff_test.go › TestBackoff_FullJitter` | for retry 1, 2, 3: 1,000 samples each in `[0, 10/20/40 ms)`; each range has a sample above half its bound (jitter is not constant zero) |
| 6, 8 | `db/errors_test.go › TestMapError` | table: 23505 → `ErrDuplicate`, 23514 → `ErrCheckViolation`, 40001 → `ErrSerialization`, 40P01 → `ErrDeadlock`, 55P03 → `ErrLockTimeout`, each also `errors.As` the original `*pgconn.PgError` (constraint name kept), also when wrapped; 23503 and a plain error unchanged; nil → nil; mapping twice equals mapping once |
| 7 | `logx/logx_test.go › TestNew_WritesJSON` | one line per record that unmarshals into a map with `time`, `level`, `msg`, `service` and the call's attributes; records below the level are dropped |

Integration tests use one `dbtest.StartPostgres` per test function (they skip under `-short`), connect as `pay_owner` to `bastion_pay` and create their own tables.

## Locks
The helper takes no row locks of its own; callers take theirs in the product's lock order (Pay TD §18.1, Tickets TD §28.1) inside `fn`. Only `TestInTx_RetriesDeadlockVictim` locks two test rows in opposite orders on purpose, to produce `40P01`.

## Race table
| Operation A | Operation B | Decided by (guard, constraint or lock) | Test |
| --- | --- | --- | --- |
| `InTx` (REPEATABLE READ) updates a row it read | another transaction updates and commits that row | PostgreSQL `40001`; A's whole transaction reruns on a new snapshot | `TestInTx_RetriesSerializationFailure` |
| `InTx` locks row 1 then row 2 | `InTx` locks row 2 then row 1 | PostgreSQL deadlock detector aborts one with `40P01`; it reruns after backoff | `TestInTx_RetriesDeadlockVictim` |
| `InTx` inserts a key | the same key already committed (or at deferred `COMMIT`) | unique constraint `23505` → `ErrDuplicate`, never retried | `TestInTx_DoesNotRetryOtherErrors`, `TestInTx_MapsCommitErrors` |
| `InTx` waits for a row lock | holder keeps it beyond `lock_timeout` | `55P03` → `ErrLockTimeout`, not retried | `TestInTx_DoesNotRetryOtherErrors` |

## Mutations
| Guard, constraint or lock ordering | Test that must fail |
| --- | --- |
| Remove `40001` from the retryable codes | `TestInTx_RetriesUpToThreeTimes/40001`, `TestInTx_RetriesSerializationFailure` |
| Remove `40P01` from the retryable codes | `TestInTx_RetriesUpToThreeTimes/40P01`, `TestInTx_RetriesDeadlockVictim` |
| `MaxRetries` 3 → 2, and 3 → 4 | `TestInTx_RetriesUpToThreeTimes` |
| Add `23505` to the retryable codes | `TestInTx_DoesNotRetryOtherErrors/23505` |
| Backoff ignores `ctx` | `TestInTx_StopsWhenContextEnds` |
| Drop the amount bound check in `BPS` | `TestBPS_RejectsOutOfRange` |
| Drop the bps range check in `BPS` | `TestBPS_RejectsOutOfRange` |
| Round instead of floor (`(amount×bps + 5000) / 10000`) | `TestBPS_Floors`, `TestSplit_IsExact` |
| `Split` rest computed as `amount − share − 1` or share for the other party | `TestSplit_IsExact` |
| Swap two codes in `MapError`'s table | `TestMapError` |
| `ids.New` uses `uuid.NewV4` | `TestNew_IsVersion7` (and forbidigo) |

## Out of scope
- `deadlocks_total`, `guard_bypass_total` and all TD §32 metrics; log redaction checks (P8-01).
- Public ID prefixes `pi_`, `re_`, `dp_`, `tk_`, `ls_` (the first API exposing one: P2-15 / P3).
- The other TD §33 variables (the item that first uses each one).
- Wiring `config`, `db`, `logx` into `cmd/*` (the first item that runs a service).
- `redisx` client and key namespaces (P1-12 / waiting room); sqlc and `db/*/queries` (P1-05/P1-06).
- Currency conversion, money formatting, checked addition of balances (not in the spec; ledger sums are SQL `bigint`, P1-05/P1-06).

## Stop rules
Stop and report instead of choosing when:
- a decision is needed that this brief does not make;
- a file outside Files must change, a dependency not named here is needed, or a test seems wrong;
- the code or the spec contradicts this brief;
- the real 40001 or 40P01 tests are flaky (pass and fail on the same code): report with the run log instead of adding sleeps;
- pgx or the standard `uuid` package behaves differently from the D3 spike.

## Progress
- 2026-11-09 — G2 tests committed (86724c7, strong); all fail on the stubs as shown to the owner.
- 2026-11-09 — step 1 done (strong); `go test ./internal/platform/money/` → ok
- 2026-11-09 — step 2 done (strong); `go test ./internal/platform/ids/` → ok; lint probe outside `ids` (deleted): `uuid.New` and `uuid.NewV4` each reported as AD-14 by forbidigo; `golangci-lint run ./...` → 0 issues
- 2026-11-09 — step 3 done (strong); `go test ./internal/platform/config/` → ok

## Surprises

## Outcome
