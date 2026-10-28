# Engineering standards

How code in Bastion is written. Read it before writing code (CONTEXT-MAP lists it under "Always" for code changes); reviewers cite the rule IDs (E1 …) in their findings. The spec (docs/01–04) wins over this document: if they disagree, raise an open question instead of choosing.

Priorities, in order: **correct** (invariants hold under concurrency and failure) → **clear** (a reviewer can verify it) → **changeable** (the next feature does not need a rewrite) → **fast** (only where the spec sets a target, and measured).

"Enforced by": *lint* means a golangci-lint rule configured in P1-01; until then, and for everything marked *review*, the reviewer checks it.

## 1. Structure and dependencies

| ID | Rule | Enforced by |
| --- | --- | --- |
| E1 | Layout per TD §2. Dependencies point one way: `cmd` → `httpapi` → module services → `kit` → `platform`. No cycles. | compiler, archtest |
| E2 | `pay/**` and `tickets/**` never import each other; `audit` imports only `platform`; simulators import no product code (ARCH §7.1). | archtest |
| E3 | A module owns its tables. Other modules call its Go API with the shared `pgx.Tx` and never query its tables (ARCH §7.1 rule 4). | review |
| E4 | A module exports a small API: a service type, its request and result types, and sentinel errors. Everything else is unexported. | review |
| E5 | Dependencies are passed explicitly through constructors. No package-level mutable state, no `init()` side effects, no service locator. | lint, review |
| E6 | Interfaces are declared by their consumer, small (1–3 methods), and only at real seams: partner clients (card network, bank, Pay client), cross-module hooks the spec defines (TD §2 `Lock` + `Apply`), and alternatives the spec names (wallet or card). Functions return concrete types. No interface only for mocking. | review |
| E7 | Extend through the seams the spec defines. No speculative abstraction, generic repository or framework; extract a helper after the third similar code path, not before. | review |

## 2. Types and functions

| ID | Rule | Enforced by |
| --- | --- | --- |
| E8 | Money is `int64` minor units with a currency; fees and splits only through `platform/money` (floor of basis points, remainder to the other party). No floating point on any money path, tests included (TD §1.1). | lint, review |
| E9 | Statuses, kinds and other closed sets are typed constants; every switch over them is exhaustive. | lint, review |
| E10 | Invalid states are unrepresentable where it is cheap: constructors validate, and zero values are either valid or unusable outside the package. | review |
| E11 | A function does one thing, returns early, and stays short (guide: at most 60 lines, cognitive complexity at most 15). No boolean parameter that switches behavior. | lint, review |
| E12 | Names come from the spec and the glossary (PRD §16); no unexplained abbreviations. | review |

## 3. Errors

| ID | Rule | Enforced by |
| --- | --- | --- |
| E13 | Every error is handled. Wrap with context using `%w`; compare with `errors.Is` / `errors.As`. | lint, review |
| E14 | Modules export sentinel errors. SQLSTATEs are mapped once, in `platform/db`, per TD §1.5; HTTP problem types only in `httpapi` (TD §29.6). | review |
| E15 | An error is logged once, where it is handled (usually the edge); never log and return. No `panic` on request or job paths. | review |

## 4. Database and transactions

| ID | Rule | Enforced by |
| --- | --- | --- |
| E16 | SQL lives in `db/<product>/queries/*.sql` and is generated with sqlc. No SQL built from strings, no ORM (AD-15). | review |
| E17 | **Lock, then mutate, then post**: row locks in the product's lock order (Pay TD §18.1, Tickets TD §28.1), then the changes, then exactly one ledger `Batch.Flush` per transaction. | CC tests, review |
| E18 | State changes are guarded: `UPDATE … SET status = $new WHERE id = $id AND status IN (…)`, and the code checks the affected rows (sqlc `:execrows` or `:one`). Zero rows is a stale transition: counted, not an error (TD §5). | review |
| E19 | No network call inside a database transaction; external calls follow the three-phase template (TD §6.1). | review |
| E20 | Transaction bodies are retry-safe: the helper reruns them on `40001`/`40P01` (TD §1.5), so they have no effect outside the transaction. | review |
| E21 | Business time comes from the database (`now()` in SQL). Go's clock is only for timeouts, backoff and metrics (TD §1.3). | lint, review |
| E22 | Every effect is protected by a unique key in the database (INV M6). A constraint that protects an invariant is named and has a test that fails without it. | tests, review |
| E23 | Migrations are forward-only; an applied migration is never edited. | review |

## 5. Boundaries, asynchronous work and jobs

| ID | Rule | Enforced by |
| --- | --- | --- |
| E24 | Outbound calls: persist the intent and its derived key, commit, call with timeout `min(per-call, remaining deadline)`, record exactly what was observed (TD §6). An unknown outcome is resolved by lookup with the same key, never by a new key. | chaos tests, review |
| E25 | Retries happen only in the orchestrator (driver, resolver, processor), bounded, with full jitter and a budget (TD §6.3). | review |
| E26 | Events: the outbox row is written in the producing transaction; consumers commit effect and inbox together (TD §7). Incoming webhooks: verify, deduplicate, apply guarded, answer fast (TD §6.5). | tests |
| E27 | Jobs claim work with leases and `FOR UPDATE SKIP LOCKED`. Every goroutine has an owner, a cancellation path and a bounded lifetime (`context`, `errgroup`); no fire-and-forget goroutines. | `-race`, review |
| E28 | `context.Context` is the first parameter of every function that does I/O; request paths never use `context.Background()`. | lint, review |

## 6. Tests

| ID | Rule | Enforced by |
| --- | --- | --- |
| E29 | Test levels follow TD §34.1. SQL behavior is tested against real PostgreSQL (testcontainers), never a mocked database. | review |
| E30 | Tests are named after what they prove: `TestM4_…`, `TestCC05_…`, `TestC20_…` for invariants and scenarios; behavior names otherwise. | review |
| E31 | Assertions check stored facts (rows, balances recomputed from postings), not only the absence of an error. | review |
| E32 | Every guard, constraint and lock ordering an item adds has a test that fails when it is removed (mutation check, recorded in the item report). | item report |
| E33 | Tests are deterministic: recorded `rapid` seeds, no `time.Sleep` for synchronization (poll with a deadline), concurrency tests under `-race`. A flaky test is a bug. | review |
| E34 | Tests are never deleted, skipped or weakened to make them pass. A test that seems wrong is an open question for the owner. | review |

## 7. Observability and security

| ID | Rule | Enforced by |
| --- | --- | --- |
| E35 | `slog` JSON with request, trace and object IDs; metric names as in TD §32. | review |
| E36 | PINs, OTPs, client secrets, API keys, card tokens and QR payloads are never logged (TD §32). Secrets come from configuration, never from code. | lint, review |

## 8. Hygiene

| ID | Rule | Enforced by |
| --- | --- | --- |
| E37 | Comments explain why and cite the spec (`// TD §3.3`, `// INV M4`, `// AD-13`). Exported identifiers have doc comments. No commented-out code. | lint, review |
| E38 | No `TODO` without a backlog item or an open question in NOW.md. | review |
| E39 | A new third-party dependency needs a reason in LOG.md, or an AD when it is architectural. | review |
| E40 | `gofmt`/`goimports` and `golangci-lint` clean; `//nolint` only with a written reason. | `task check` |

## 9. Patterns used in Bastion

Use these where the spec calls for them, and nothing heavier.

| Pattern | Where | Spec |
| --- | --- | --- |
| Unit of work: one ledger batch per transaction | `kit/ledger` `Batch` | TD §3.3 |
| Guarded update (compare-and-set on status) | every state change | TD §5, AD-09 |
| Ordered locking | every write transaction | TD §18.1, §28.1 |
| State machine enforced by the database | `kit/fsm` | TD §5 |
| Idempotency key with a lease | `kit/idempotency`, every mutating API | TD §4 |
| Transactional outbox and idempotent consumer (inbox) | `kit/outbox`, `kit/webhookin` | TD §6.5, §7 |
| Three-phase external call; resolver by lookup with the same key | every partner call | TD §6, AD-13 |
| Two-phase money: hold, then capture or release | wallet and card payments | AD-06 |
| Saga with compensation | Tickets payment saga | TD §24, AD-07 |
| Lease and `SKIP LOCKED` job claiming | resolvers, workers | TD §6.4, AD-17 |
| Circuit breaker, retry budget, token-bucket rate limit | partner clients | TD §6.3 |
| Ports only at real seams | `payments` hooks `Lock` + `Apply`; partner clients | TD §2 |

**Anti-patterns:** a transaction that spans both products; a generic repository over sqlc; one service that owns everything; reading idempotency state or balances from a replica; checking a bound in Go without holding the lock (INV M4 pitfalls); retries at several layers; abstractions with a single implementation and no seam in the spec.
