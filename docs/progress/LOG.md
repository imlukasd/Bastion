# Session log

Newest first. Prepend an entry at every handoff and never edit past entries. Read only the top entries you need.

Template:

```markdown
## YYYY-MM-DD — <item-id or topic> — <person and/or agent>
- Done: …
- Verified: `<command>` → <result> (or "not verified: <why>")
- Not done / next: …
- Decisions: … (link the AD or item brief)
- Surprises: …
```

---

## 2026-10-28 — Workflow: right-sizing and commit style — Claude with Lukas

- Done: AGENTS.md › Right-sizing: the full loop stays the default, and a brief may skip G2 (tests come with the code) and G5 (independent review) on items that are not critical, through a `Skip:` line with the reason in Decisions, approved with the brief; never skipped: G1, G3, G6, test-first and mutations on critical items, tests on money, state or concurrency paths. Commits: short conventional subjects, no names, no trailers; the `(approved: <owner>)` suffix is gone, the commit made after the owner's answer is the approval (`docs(<ID>): add brief`, `docs(<ID>): update brief …`, `test(<ID>): …`). scripts/brief `-check`: an approved brief is a `docs(<ID>)` commit that names and changes the brief; the last approved brief's `Skip:` line lifts the test order and test lock for G2 and is rejected for any gate but G2 and G5 or on a critical item. docs/briefs/README.md (template `Skip:` line, Definition of Ready 11, machine checks), REVIEW.md, WORKFLOW.md, `/catchup`, `/handoff` and `reviewer` follow.
- Verified: `go run scripts/brief/main.go -check` → `plan OK`; gofmt and go vet clean. Lifecycle simulated in a scratch repository, 16 cases, all as expected: Skip: G2 with code and tests together → OK, stage "G2 skipped"; test edited in a feat commit → OK; path outside Files → error; no Skip, code before tests → error; Skip: none with brief → tests → code → OK, then a test edit in feat → error; critical item with Skip → error; Skip: G1 → error; unedited template line → error; remark in parentheses → ignored; draft brief in a chore commit then code → error; brief update adding Skip → OK; Skip only in the working copy → error; main after `--no-ff` merge → `plan OK`.
- Not done / next: rebase item/P1-01-skeleton onto main, add `Skip: G2` and the slimmer Test map to the P1-01 brief, then G1.
- Decisions: the owner asked for judgment over ceremony (skip what adds nothing to correctness, keep the standard full) and for name-free commits; both live in AGENTS.md, not only in agent memory.
- Surprises: none.

## 2026-10-28 — Workflow: item briefs and model routing — Claude with Lukas

- Done: every item now starts with an item brief that the owner approves (G1 for all items). docs/briefs/README.md has the lifecycle, the Definition of Ready, how the executor is chosen, the machine checks and the template; critical items add Locks, Race table and Mutations. `docs/exec-plans/` is renamed `docs/briefs/`. New `.claude/agents/implementer.md` (Sonnet) implements `Executor: standard` briefs and stops on any decision the brief does not make; `reviewer` is pinned to Opus; `.claude/settings.json` sets Opus as the main model. Approvals are commits ending `(approved: <owner>)`. On item branches, scripts/brief `-check` now fails when anything but the brief is changed before its approval, when code comes before the approved tests, when a test changes outside an approved test commit, when a changed path is outside the Files of the last approved brief, and when the approved brief lacks a section or an Executor line (strong for critical items) or keeps an OPEN decision. The session brief shows the item's stage. AGENTS.md, CLAUDE.md, REVIEW.md, WORKFLOW.md (rewritten around "What you type, in order"), `/catchup` and `/handoff` follow.
- Verified: `go run scripts/brief/main.go -check` → `plan OK`; gofmt and go vet clean. Lifecycle simulated in a scratch repository, 17 stages, each as expected. Start → OK. Spike file in the repo before approval → error. Uncommitted draft with OPEN → OK. Approved with OPEN → error. Approved update → OK. feat before tests → error. Approved tests → OK. Code inside Files (braces, `**`, trailing `/`) → OK. Test changed in a feat commit → error. Uncommitted paths outside Files → error, also after widening Files only in the working copy. Uncommitted test edit → error, then OK as an approved test commit. Brief moved to completed → OK. main after `--no-ff` merge → `plan OK`. Critical item with `Executor: standard` and no Locks, Race table or Mutations → 4 errors.
- Not done / next: P1-01, starting with its brief (four OPEN decisions, see NOW.md).
- Decisions: see NOW.md "Recent decisions". The owner named the remote, https://github.com/imlukasd/Bastion; the initial commit was made and pushed to `main` at the owner's request. Briefs are not written in plan mode, which cannot write the file; the `approve brief` gate gives the same control.
- Surprises: "brief" already named the orientation scripts/brief prints; the docs now say "item brief" and "session brief" to keep the two apart.

## 2026-10-27 — Workflow audit — Claude with Lukas

- Done: audited the loop from session start to merge and fixed the gaps: explicit "red first" triage order and a defined baseline; one branch per item merged with `--no-ff`; tests committed alone before the implementation, later test changes reported; G3 runs the full suite; reviewer base = `git merge-base main HEAD`; phase status rules; P1-01's `task check` includes the plan check. scripts/brief now shows the item branch, unmerged item branches (WARN on main), an unclaimed item branch (WARN), the last done item, and on work branches reports checkpoint commits as information instead of a warning; merge commits no longer count as missed handoffs. New docs/WORKFLOW.md: the loop step by step from the owner's side and the role of every file. `/catchup` and `/handoff` rewritten to match.
- Verified: `go run scripts/brief/main.go -check` → `plan OK`. Lifecycle simulated in a scratch repository: fresh main (no WARN); branch without claim (WARN); claimed start commit with NOW.md (no WARN); mid-item checkpoints (info line); main before merge (WARN: work in flight); `--no-ff` merge (`-check` OK, evidence commit reachable, "Last done" shown, no WARN); a commit on main without handoff (WARN). Project-level SessionStart hook tested with a headless session: the brief was in its context.
- Not done / next: initial commit (owner); then P1-01.
- Decisions: see NOW.md "Recent decisions".
- Surprises: the first lifecycle run showed that every `--no-ff` merge and every start commit would have raised a false "NOW.md is stale" warning; both fixed and re-tested.

## 2026-10-27 — Full plan and evidence — Claude with Lukas

- Done: features.json now holds the whole plan: 8 phases and 98 items (P1-01 … P8-06) from PRD §7 and §12, TD §8–§35 and INV §10, each with `critical`, `refs`, `depends_on`, `done_when` and `verify`. Added `evidence` (date, verified commit, verify results, mutations, review, report, approved_by). scripts/brief validates evidence (`-check`) and shows the plan (`-plan`) and one item (`-item`). AGENTS.md lists the files to update at each handoff; REVIEW.md shows the evidence format.
- Verified: `go run scripts/brief/main.go -check` → `plan OK` on 98 items. In a scratch repository, a done item with complete evidence passes, and these are each rejected: wrong date format, a verify command without a passing result, empty output, a critical item without mutations, missing review/report/approver, a report file that does not exist, a commit outside HEAD's history, a done item without evidence, evidence on a todo item.
- Not done / next: the owner reviews the plan, moves the hook into `.claude/settings.json`, makes the initial commit; then P1-01.
- Decisions: CC1 through P2P proposed in P2-12; chaos harness placed in Phase 2 (P2-04), before the first chaos scenario; kit/webhookin and the job runner placed in Phase 2 (P2-02, P2-03), because Phase 1's scope does not name them.
- Surprises: kit/webhookin, the job runner and the chaos harness appear in the spec but in no phase scope of PRD §12.

## 2026-10-27 — Engineering and review rules — Claude with Lukas

- Done: docs/ENGINEERING.md (rules E1–E40 with how each is enforced, patterns and anti-patterns), docs/REVIEW.md (gates G1–G6, item report template, severities, red flags, the owner's 15-minute review), `.claude/agents/reviewer.md` (read-only reviewer with a fresh context). AGENTS.md, CLAUDE.md, CONTEXT-MAP and `/handoff` now require the gates and the owner's approval before `done`.
- Verified: SessionStart hook tested with a headless session: its context began with `== Bastion session brief ==`. `go run scripts/brief/main.go -check` → `backlog OK`.
- Not done / next: the owner moves the hook from user settings to `.claude/settings.json`; initial commit; then P1-01.
- Decisions: `done` requires the owner's `approve` of the item report.
- Surprises: the hook was added to the owner's user settings, so it would run (and fail) in every other project.

## 2026-10-27 — Session harness — Claude with Lukas

- Done: `git init` (`main`, LF line endings via .gitattributes, .gitignore). AGENTS.md (map, session protocol, state-file rules, engineering rules), CLAUDE.md (imports AGENTS.md and NOW.md). docs/progress/{NOW.md, LOG.md, features.json}, docs/exec-plans/ (template), docs/CONTEXT-MAP.md, scripts/brief (session brief and backlog validation), `/catchup` and `/handoff` skills. Phase 1 broken into P1-01 … P1-15 from PRD §12, TD §1–§7, §30.2, §31 and INV §10.
- Verified: `go run scripts/brief/main.go -check` → `backlog OK`; broken fixtures (unknown field, invalid status, duplicate id, done before a dependency, missing owner, two items in progress for one owner) → each reported, exit 1; staleness and dirty-tree warnings checked in a scratch repository.
- Not done / next: `.claude/settings.json` with the SessionStart hook (the owner adds it); initial commit; then P1-01.
- Decisions: the project rules that lived only in one person's agent memory moved into AGENTS.md.
- Surprises: no later exit gate names CC1 through P2P (open question in NOW.md). On this Windows machine the first `go run` of a new build was once blocked by an Application Control policy; the retry passed.
