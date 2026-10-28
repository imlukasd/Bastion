# Now

Overwritten at every handoff (AGENTS.md › Session protocol › End). At most 40 lines; history lives in [LOG.md](LOG.md).

- **Updated:** 2026-10-28, Claude with Lukas (P1-01 started)
- **Phase:** 1 Foundations, in progress
- **Active item:** P1-01 Repository skeleton, owner Claude, stage: Steps 1–6 done; Step 7 stopped (Smart App Control blocks a test binary)
- **Branch:** item/P1-01-skeleton

## State

- Docs v4.1 (docs/01–04) are complete and are the source of truth. No Go code yet: no go.mod, Taskfile or Compose.
- The whole plan is in docs/progress/features.json (8 phases, P1-01 … P8-06): `go run scripts/brief/main.go -plan`.
- Loop for every item: item brief approved (G1) → tests approved (G2) → implementation by the brief's executor → G3–G5 → owner's `approve` → merge `--no-ff`. Items that are not critical may skip G2 and G5 when their brief says so (AGENTS.md › Right-sizing).
- Models: main session Opus (`.claude/settings.json`); `implementer` subagent (Sonnet) for `Executor: standard`; `reviewer` pinned to Opus. All of Phase 1 is `Executor: strong`.
- Baseline until P1-01 exists: `go run scripts/brief/main.go -check` → `plan OK` (on an item branch: `plan OK, item <ID> OK`).
- Toolchain: Go 1.27.3 (MSI) and task 3.54.0 (winget) installed; golangci-lint and govulncheck are pinned in `tools.mod`. Docker is installed; its daemon was not running on 2026-10-28.

## Next step

1. Owner: answer the Smart App Control question below (brief Stop rule hit in Step 7).
2. Then finish Step 7 of `docs/briefs/active/P1-01-skeleton.md`: `task test` → exit 0 (`task check` is already `check OK`); then `/handoff` with G3, G5 and the item report.

## Open questions (owner decides)

- Smart App Control (enforcement mode) blocked the `-race` build of `internal/archtest`'s test binary twice, so `task test` fails on this machine. It can hit any unsigned test binary. Options: turn Smart App Control off, or run the suite in Linux (WSL2 or a container). Details: P1-01 brief › Surprises.

- CI on GitHub Actions (TD §34.4): deferred by the owner on 2026-10-28; no CI item in the plan for now.

## Recent decisions

- 2026-10-28: Right-sizing: the full loop is the default; a brief may skip G2 and G5 on items that are not critical (`Skip:` line, reason in Decisions), approved with the brief. Commits are short conventional commits with no names or trailers; an approval is the commit itself (`docs(<ID>): add brief`, `test(<ID>): …`).
- 2026-10-28: Module path `github.com/imlukasd/Bastion`, matching the repository's name and case (P1-01 D1).
- 2026-10-28: CC1 through P2P transfers is covered in P2-12 (confirmed by the owner).
- 2026-10-28: Remote repository https://github.com/imlukasd/Bastion; the initial commit is pushed to `main`.
- 2026-10-28: Every item gets an item brief the owner approves before tests or code (G1 for all items); critical items use the full brief (Locks, Race table, Mutations).
- 2026-10-28: Executor per brief: `strong` (Opus, main session) for critical items and all of Phase 1, `standard` (Sonnet `implementer`) otherwise; brief, tests, gates and review always on the strong model.
