# Now

Overwritten at every handoff (AGENTS.md › Session protocol › End). At most 40 lines; history lives in [LOG.md](LOG.md).

- **Updated:** 2026-11-09, Claude with Lukas (P1-03 started)
- **Phase:** 1 Foundations, in progress (2/15 done)
- **Active item:** P1-03 Platform basics (item report written; waiting for the owner's `approve`)
- **Branch:** item/P1-03-platform-basics

## State

- Docs v4.1 (docs/01–04) are the source of truth. P1-01: Go skeleton, Taskfile, lint. P1-02: local stack (`deploy/compose.yaml`: PostgreSQL 18.6 on `127.0.0.1:55432` with `bastion_pay`/`bastion_tickets`, roles `*_owner`/`*_app`; Redis 8.10.3 on 6379), forward-only migration runner `internal/platform/db/migrate` (`task migrate`), test helper `internal/platform/db/dbtest` (testcontainers).
- Baseline: `task check` → `check OK`. Full suite: `task test` (needs Docker; green under `-race`, archtest included).
- Environment: WSL2 (Linux), Go 1.27.3, task 3.54.0, Docker Desktop 4.97 (Engine 29.9.2). A native PostgreSQL on the Windows host holds port 5432, hence 55432. Images not yet local can be fetched with `go run github.com/google/go-containerregistry/cmd/crane@v0.20.6 pull <image> <file>.tar` then `docker load -i <file>.tar` while Docker Desktop's pulls fail.
- Loop: item brief (G1) → tests (G2, unless `Skip: G2`) → implementation → G3–G5 → owner's `approve` → merge `--no-ff` (AGENTS.md).

## Next step

1. Owner reads the item report in docs/briefs/active/P1-03-platform-basics.md › Outcome and answers `approve` (G6), or `changes: …`.
2. On approve: handoff (LOG.md with E39 reasons for `jackc/puddle/v2` and `golang.org/x/sync`, features.json evidence on b68cded, brief to completed/), `chore(P1-03): done`, merge `--no-ff` into main, baseline on main.

## Open questions (owner decides)

- Spec gap (P1-03 D15): TD §1.5 maps `55P03` (`lock_timeout`) but no section sets a `lock_timeout` value, so without one a lock wait never times out. Decide with the item that defines API and worker timeouts.
- Docker Desktop cannot pull images: its proxy `http.docker.internal:3128` ends every Docker Hub request in `EOF` (also after the update to 4.97), while WSL itself reaches Docker Hub. Check Settings › Resources › Proxies (and any Windows proxy/VPN). Workaround above.
- CI on GitHub Actions (TD §34.4): deferred by the owner on 2026-10-28; no CI item in the plan for now.

## Recent decisions

- 2026-11-09: govulncheck flagged GO-2026-6865 (klauspost/compress, via testcontainers); fixed on main by bumping to v1.20.2 (c57b7c5). Owner: small red-baseline fixes go straight to main, no fix/ branch.
- 2026-11-03: P1-02 D3: migrations use the in-house runner (checksums, forward-only, refuses COMMIT/ROLLBACK in a file); host port 55432 for PostgreSQL; baseline migration revokes PUBLIC on schema public and EXECUTE on functions by default (D8).
- 2026-11-03: Work moved to WSL2; the P1-01 Smart App Control block no longer occurs (`task test` fully green).
- 2026-10-28: P1-01 D13: noctx enabled; ENGINEERING.md E9, E13, E28 are "lint, review" and E11 says "at most 60 lines, cognitive complexity at most 15".
- 2026-10-28: Right-sizing: a brief may skip G2 and G5 on items that are not critical (`Skip:` line, reason in Decisions). Commits are short conventional commits with no names or trailers; an approval is the commit itself.
- 2026-10-28: Module path `github.com/imlukasd/Bastion`; Go 1.27.3.
- 2026-10-28: Executor per brief: `strong` (Opus, main session) for critical items and all of Phase 1, `standard` (Sonnet `implementer`) otherwise.
