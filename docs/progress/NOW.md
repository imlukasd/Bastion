# Now

Overwritten at every handoff (AGENTS.md › Session protocol › End). At most 40 lines; history lives in [LOG.md](LOG.md).

- **Updated:** 2026-11-03, Claude with Lukas (P1-02 started)
- **Phase:** 1 Foundations, in progress (1/15 done)
- **Active item:** P1-02 Local stack (brief being drafted)
- **Branch:** item/P1-02-local-stack

## State

- Docs v4.1 (docs/01–04) are the source of truth. P1-01 created the Go skeleton: go.mod (`github.com/imlukasd/Bastion`, Go 1.27), the TD §2 layout (54 packages with `doc.go` or stub `main.go`), Taskfile, `.golangci.yml`, lint fixtures in `internal/archtest`, and `scripts/check`.
- Baseline: `task check` → `check OK` (build, vet, golangci-lint, govulncheck, `go test -short`, plan check). Full suite: `task test`.
- Toolchain: Go 1.27.3 (MSI), task 3.54.0 (winget), gcc for `-race`; golangci-lint and govulncheck are pinned in `tools.mod` (`go tool -modfile=tools.mod …`; in PowerShell this argument gets split, so use bash or task). Since 2026-11-03 the repository is worked on in WSL2 (Linux); Docker 29.1.3 daemon is running there.
- Loop: item brief (G1) → tests (G2, unless `Skip: G2`) → implementation → G3–G5 → owner's `approve` → merge `--no-ff` (AGENTS.md).

## Next step

1. Draft docs/briefs/active/P1-02-local-stack.md (G1): refs TD §1.3, §1.4, §33, ARCH §12.1, AD-03, AD-15; ask the owner the OPEN decisions; wait for `approve brief`.

## Open questions (owner decides)

- Smart App Control (enforcement mode) blocks some unsigned test binaries: `task test` failed on the `-race` build of `internal/archtest`'s test. P1-01 accepted G3 with that exception. Options if it recurs: turn Smart App Control off, or run the suite in Linux (WSL2 or a container). Details: docs/briefs/completed/P1-01-skeleton.md › Surprises.
- CI on GitHub Actions (TD §34.4): deferred by the owner on 2026-10-28; no CI item in the plan for now.

## Recent decisions

- 2026-10-28: P1-01 D13: noctx enabled; ENGINEERING.md E9, E13, E28 are "lint, review" and E11 says "at most 60 lines, cognitive complexity at most 15".
- 2026-10-28: P1-01 G3 accepted although `task test` is blocked by Smart App Control on one test binary (substitute evidence in the item report).
- 2026-10-28: Right-sizing: a brief may skip G2 and G5 on items that are not critical (`Skip:` line, reason in Decisions). Commits are short conventional commits with no names or trailers; an approval is the commit itself.
- 2026-10-28: Module path `github.com/imlukasd/Bastion`; Go 1.27.3 (Go 1.25 is out of support and govulncheck flags its standard library).
- 2026-10-28: CC1 through P2P transfers is covered in P2-12.
- 2026-10-28: Executor per brief: `strong` (Opus, main session) for critical items and all of Phase 1, `standard` (Sonnet `implementer`) otherwise.
