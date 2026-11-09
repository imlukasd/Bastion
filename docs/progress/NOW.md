# Now

Overwritten at every handoff (AGENTS.md › Session protocol › End). At most 40 lines; history lives in [LOG.md](LOG.md).

- **Updated:** 2026-11-09, Claude with Lukas (P1-03 done)
- **Phase:** 1 Foundations, in progress (3/15 done)
- **Active item:** none
- **Branch:** main

## State

- Docs v4.1 (docs/01–04) are the source of truth.
  - P1-01: Go skeleton, Taskfile, lint.
  - P1-02: local stack (`deploy/compose.yaml`, PostgreSQL 18.6 on `127.0.0.1:55432`, Redis 8.10.3), migration runner, `dbtest`.
  - P1-03 platform basics in `internal/platform`:
    - `money.BPS`/`Split`;
    - `ids.New` (UUIDv7, standard `uuid`; lint bans `uuid.New`/`NewV4` elsewhere);
    - `config.LoadPay`/`LoadTickets`;
    - `db.Open`, `db.InTx` (retries 40001/40P01, at most 3), `db.MapError` + sentinels (`ErrDuplicate` …);
    - `logx.New`.
- Baseline: `task check` → `check OK`. Full suite: `task test` (needs Docker; green under `-race`).
- Environment: WSL2, Go 1.27.3, task 3.54.0, Docker Desktop 4.97 (Engine 29.9.2); enable WSL integration for the distro. A native PostgreSQL on the Windows host holds 5432, hence 55432. When Docker Desktop's pulls fail: `go run github.com/google/go-containerregistry/cmd/crane@v0.20.6 pull <image> <file>.tar`, then `docker load -i <file>.tar`.
- Loop: item brief (G1) → tests (G2, unless `Skip: G2`) → implementation → G3–G5 → owner's `approve` → merge `--no-ff` (AGENTS.md).

## Next step

1. Owner runs `/clear`, then `/catchup`.
2. Propose the next ready item: P1-04 Architecture test for module boundaries (not critical, `Executor: strong` as all of Phase 1), or P1-05 Ledger kit schema (critical). Read `go run scripts/brief/main.go -item <ID>` and its refs. Later items use `db.InTx` for transactions and `money.Split` for every fee (E8, E14, E20).

## Open questions (owner decides)

- Spec gap (P1-03 D15): TD §1.5 maps `55P03` (`lock_timeout`), but no section sets a `lock_timeout` value, so without one a lock wait never times out. Decide with the item that defines API and worker timeouts.
- Docker Desktop cannot pull images: its proxy `http.docker.internal:3128` ends Docker Hub requests in `EOF`. Check Settings › Resources › Proxies. The workaround is above.
- CI on GitHub Actions (TD §34.4): deferred by the owner on 2026-10-28.

## Recent decisions

- 2026-11-09: P1-03: standard library `uuid` for UUIDv7; `InTx` retries 40001/40P01 from the body or COMMIT, at most 3 times, full jitter ≤ 70 ms; config errors never echo values; no `lock_timeout` set (D15).
- 2026-11-09: Small red-baseline fixes go straight to main, with no fix/ branch (c57b7c5: klauspost/compress v1.20.2 for GO-2026-6865).
- 2026-10-28: Executor per brief: `strong` for critical items and all of Phase 1, `standard` (Sonnet `implementer`) otherwise.
