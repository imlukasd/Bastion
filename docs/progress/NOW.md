# Now

Overwritten at every handoff (AGENTS.md › Session protocol › End). At most 40 lines; history lives in [LOG.md](LOG.md).

- **Updated:** 2026-11-18, Claude with Lukas (P1-04 done)
- **Phase:** 1 Foundations, in progress (4/15 done)
- **Active item:** none
- **Branch:** main

## State

- Docs v4.1 (docs/01–04) are the source of truth.
  - P1-01: Go skeleton, Taskfile, lint.
  - P1-02: local stack (`deploy/compose.yaml`, PostgreSQL 18.6 on `127.0.0.1:55432`, Redis 8.10.3), migration runner, `dbtest`.
  - P1-03: `internal/platform` basics: `money.BPS`/`Split`, `ids.New` (UUIDv7), `config.LoadPay`/`LoadTickets`, `db.Open`/`InTx` (retries 40001/40P01, at most 3)/`MapError`, `logx.New`.
  - P1-04: `internal/archtest` import rules (E1, E2): products import only their own product, `platform`, `kit`; audit only `platform`/`audit`; sim only `platform`/`sim`; no import of `testdata`, `.x`, `_x`.
- Baseline: `task check` → `check OK`. Full suite: `task test` (needs Docker; green under `-race`).
- Environment: WSL2, Go 1.27.4, task 3.54.0, Docker Desktop 4.97 (Engine 29.9.2); enable WSL integration for the distro. A native PostgreSQL on the Windows host holds 5432, hence 55432. When Docker Desktop's pulls fail: `go run github.com/google/go-containerregistry/cmd/crane@v0.20.6 pull <image> <file>.tar`, then `docker load -i <file>.tar`.
- Loop: item brief (G1) → tests (G2, unless `Skip: G2`) → implementation → G3–G5 → owner's `approve` → merge `--no-ff` (AGENTS.md).

## Next step

1. Owner runs `/clear`, then `/catchup`.
2. Propose the next ready item: P1-05 Ledger kit schema, triggers and privileges (critical, full brief with Locks/Race table/Mutations) or P1-10 Idempotency kit. New packages must fit the archtest allow-lists (E2).

## Open questions (owner decides)

- P1-04 consequence: archtest allows products to import only their own product, `platform` and `kit`, so generated Go code (sqlc) must live under `internal/<product>/**`, not `db/<product>/`. Decide with the first sqlc item.
- Spec gap (P1-03 D15): TD §1.5 maps `55P03` (`lock_timeout`), but no section sets a `lock_timeout` value, so without one a lock wait never times out. Decide with the item that defines API and worker timeouts.
- Docker Desktop cannot pull images (proxy `http.docker.internal:3128` → `EOF`); workaround above. If `docker ps` hangs or testcontainers stay `Created`: restart Docker Desktop, then remove containers labelled `org.testcontainers`.
- CI on GitHub Actions (TD §34.4): deferred by the owner on 2026-10-28.

## Recent decisions

- 2026-11-18: P1-04: archtest uses allow-lists (ARCH §7.1 rule 2), `RuleHidden`, products may not import `internal/sim` (D15); E2 text updated.
- 2026-11-18: govulncheck flagged GO-2026-6859 (net/http, standard library); fixed on main by moving to Go 1.27.4 (4d3103e).
- 2026-11-09: P1-03: standard library `uuid` for UUIDv7; `InTx` retries 40001/40P01 from the body or COMMIT, at most 3 times, full jitter ≤ 70 ms; config errors never echo values; no `lock_timeout` set (D15).
