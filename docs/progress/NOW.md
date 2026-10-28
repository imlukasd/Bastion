# Now

Overwritten at every handoff (AGENTS.md › Session protocol › End). At most 40 lines; history lives in [LOG.md](LOG.md).

- **Updated:** 2026-10-28, Claude with Lukas (workflow: right-sizing, commit style)
- **Phase:** 1 Foundations, not started
- **Active item:** none
- **Branch:** main, pushed to https://github.com/imlukasd/Bastion (`origin`)

## State

- Docs v4.1 (docs/01–04) are complete and are the source of truth. No Go code yet: no go.mod, Taskfile or Compose.
- The whole plan is in docs/progress/features.json (8 phases, P1-01 … P8-06): `go run scripts/brief/main.go -plan`.
- Loop for every item: item brief approved (G1) → tests approved (G2) → implementation by the brief's executor → G3–G5 → owner's `approve` → merge `--no-ff`. Items that are not critical may skip G2 and G5 when their brief says so (AGENTS.md › Right-sizing). Owner's side: docs/WORKFLOW.md › "What you type, in order"; agents: AGENTS.md › Session protocol; briefs: docs/briefs/README.md.
- Models: main session Opus (`.claude/settings.json`); `implementer` subagent (Sonnet) for `Executor: standard`; `reviewer` pinned to Opus. All of Phase 1 is `Executor: strong`.
- Baseline until P1-01 exists: `go run scripts/brief/main.go -check` → `plan OK` (on an item branch: `plan OK, item <ID> OK`).
- Not installed on this machine yet: task, golangci-lint, govulncheck (Go 1.25.6 and Docker are).

## Next step

1. `/clear`, then `/catchup`: it should report nothing red and propose **P1-01**. After `start P1-01`, the first action is its brief `docs/briefs/active/P1-01-skeleton.md`. The draft shown on 2026-10-28 had four OPEN decisions: D1 module path; D2 where repo-level tests live (`test/repo/` is not in TD §2); D3 golangci-lint and govulncheck as `go tool` dependencies, `task` installed per machine; D4 `task check` runs `go test -short`. Spikes before G1: does forbidigo flag `float64` as a type, can golangci-lint lint testdata fixtures, which versions to pin, does govulncheck flag Go 1.25.6.

## Open questions (owner decides)

- CC1 through P2P transfers: proposed in P2-12 (Phase 1 only has CC1 at ledger level). Confirm?
- The remote is GitHub (https://github.com/imlukasd/Bastion), so CI (TD §34.4) can run on GitHub Actions: add a CI item to the plan (a plan change the owner approves)? For P1-01 D1, the module path should match the repository's name and case (`github.com/imlukasd/Bastion`), or the repository is renamed to lowercase first.
- P7-05 uses placeholder load task names (hot_accounts, serializable) until those scenarios exist.

## Recent decisions

- 2026-10-28: Remote repository https://github.com/imlukasd/Bastion; the initial commit is pushed to `main`.
- 2026-10-28: Every item gets an item brief the owner approves before tests or code (G1 for all items); critical items use the full brief (Locks, Race table, Mutations).
- 2026-10-28: Executor per brief: `strong` (Opus, main session) for critical items and all of Phase 1, `standard` (Sonnet `implementer`) otherwise; brief, tests, gates and review always on the strong model.
- 2026-10-28: Right-sizing: the full loop is the default; a brief may skip G2 and G5 on items that are not critical (`Skip:` line, reason in Decisions), approved with the brief. Commits are short conventional commits with no names or trailers; an approval is the commit itself (`docs(<ID>): add brief`, `test(<ID>): …`).
- 2026-10-28: `docs/exec-plans/` is now `docs/briefs/`.
- 2026-10-27: One branch per item, merged with `--no-ff` after approval; G3 runs the full suite; `done` only with evidence that `-check` accepts and the owner's `approve`.
