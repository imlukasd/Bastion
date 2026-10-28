# AGENTS.md — Bastion

Instructions for every coding agent (Claude Code, Codex, Cursor, …) and every person working in this repository. It is loaded at the start of every session, so it stays short (under ~150 lines); details live in the files it points to. People: [docs/WORKFLOW.md](docs/WORKFLOW.md) shows the same loop from the owner's side.

## What Bastion is

Two products of equal depth in one Go monorepo. **Bastion Pay** is an e-wallet and payment gateway under Vietnamese rules; **Bastion Tickets** is a concert on-sale with official resale and Pay's first merchant. Each is a modular monolith with its own PostgreSQL database; they share Redis and talk only through Pay's merchant API and signed webhooks. Correctness is defined by 17 invariants. Overview: [docs/README.md](docs/README.md).

## Where things are

| Need | Go to |
| --- | --- |
| Where the project stands and the exact next step | [docs/progress/NOW.md](docs/progress/NOW.md) |
| The whole plan: phases and items, each with `done_when`, `verify` and, once done, `evidence` | [docs/progress/features.json](docs/progress/features.json). Do not read it whole: `go run scripts/brief/main.go -plan` or `-item <ID>` |
| What earlier sessions did | [docs/progress/LOG.md](docs/progress/LOG.md) (newest first; read the top entries only) |
| How the current item is built: its approved item brief | `docs/briefs/active/<ID>-<slug>.md` (template, Definition of Ready, machine checks: [docs/briefs/README.md](docs/briefs/README.md)) |
| Which spec sections a task needs | the item's `refs` and [docs/CONTEXT-MAP.md](docs/CONTEXT-MAP.md) |
| How code is written (rules E1–E40, patterns) | [docs/ENGINEERING.md](docs/ENGINEERING.md) |
| How work is accepted (gates G1–G6, item report, evidence) | [docs/REVIEW.md](docs/REVIEW.md) |
| What to build and the rules it obeys (source of truth) | `docs/01-prd.md`, `02-architecture.md`, `03-technical-design.md`, `04-invariants.md` |
| Session brief: git state, branches, plan, warnings | `go run scripts/brief/main.go` |

Never read the four spec documents in full (about 80k tokens). Read the sections the item's `refs` and CONTEXT-MAP name, including its "Always" list.

**Baseline** means the fast always-green check: `task check` once P1-01 has created it, `go run scripts/brief/main.go -check` before that.

## Session protocol

### 1. Start: red first

1. Get the session brief: `go run scripts/brief/main.go`. Claude Code runs it automatically at startup, after `/clear` and after compaction.
2. Read `docs/progress/NOW.md`, then the active item brief if there is one, and the top LOG.md entry.
3. Fix what is red, in this order; each comes before any new work:
   1. Every `WARN` of the session brief: uncommitted changes (read `git diff`; never discard someone's work without the owner's consent), work in flight on an unmerged item branch (switch to it), NOW.md behind HEAD (read the commits, bring NOW.md up to date), an invalid plan.
   2. The baseline. If it is red, fixing it is the task.
   3. An item `in_progress`: resume it from NOW.md "Next step" and the item brief's Progress.
   4. Changes the owner requested (NOW.md "Next step" after `changes:`).
4. Report the state to the owner and propose the next item: NOW.md "Next step", else the first ready item of the session brief.

### 2. Start an item

1. Read it: `go run scripts/brief/main.go -item <ID>`; then its `refs` and the CONTEXT-MAP "Always" list (for code, docs/ENGINEERING.md).
2. On the owner's go: from an up-to-date `main`, `git switch -c item/<ID>-<slug>`. Set the item `in_progress` with your name as `owner` (and its phase `in_progress` if it is the first item of the phase); in NOW.md set "Active item" and "Next step". Commit `chore(<ID>): start`.
3. **Item brief (G1), every item.** Write `docs/briefs/active/<ID>-<slug>.md` from the template in docs/briefs/README.md (critical items add Locks, Race table, Mutations). Ask the owner every decision that is theirs (rows marked `OPEN`); settle technical unknowns with spikes outside the repository. Propose any skipped gate on the brief's `Skip:` line (see Right-sizing). When the Definition of Ready holds, show the brief and wait. On `approve brief`: commit `docs(<ID>): add brief`. No test or code before this commit.
4. **Tests first (G2)**, unless the brief says `Skip: G2`. Write the tests of the brief's Test map (with stubs from its Contracts so they compile), run them, show that they fail for the right reason. On the owner's `ok tests`: commit `test(<ID>): …`. After it, tests change only in another such commit, after the owner approved the change. With `Skip: G2`, the tests come with the code in the `feat` commits.

### 3. Work

- Follow the brief's Steps; one item at a time; anything outside the brief goes to NOW.md "Open questions".
- **Executor** (the brief's `Executor:` line): `strong` → the strong model implements in the main session. `standard` → delegate the steps to the standard model with the item ID, the brief path and the steps (Claude Code: the `implementer` subagent). After each return: run `-check`, record Progress in the brief; answer a stop from the brief or the spec and record it under Surprises as a brief gap, or ask the owner when it needs a new decision; when the same step fails twice, the strong model takes it over.
- Small steps; after each green step commit `feat|fix|refactor(<ID>): …` and keep NOW.md "Next step" true, so a crash or `/clear` loses nothing.
- Spec conflict or gap: stop, write it under "Open questions" and in the brief's Surprises, ask the owner. Never silently diverge from docs/01–04. A change to the brief beyond Progress and Surprises needs the owner's approval, then `docs(<ID>): update brief …`.

### 4. End (handoff): before `/clear`, before switching items, before stopping

1. Run the item's `verify` commands and note what they printed.
2. If the item is finished: run the gates of docs/REVIEW.md (G3 full suite, G4 mutations for critical items, G5 `reviewer` unless the brief skips it), write the item report into the brief's Outcome, and wait for the owner's `approve`.
3. Update the files in the table below. An unfinished item stays `in_progress`, or `blocked` with the reason in `notes`.
4. `go run scripts/brief/main.go -check` passes. Commit `chore(<ID>): handoff` (or `done`), tree clean.
5. If approved: `git switch main`, `git merge --no-ff item/<ID>-<slug>`, `git branch -d item/<ID>-<slug>`, run the baseline on `main`. Then the owner runs `/clear` before the next item.

A handoff is good when a new agent or person, with only the repository and no chat history, can continue without asking.

### Files to update

| File | When | What |
| --- | --- | --- |
| `docs/progress/features.json` | Every handoff | The item's `status` and `owner`; when approved, `status: done` and `evidence` (REVIEW › Evidence); the phase's `status` at its first item and at its gate. Nothing else. |
| Item brief | Every handoff | Progress and Surprises; when done, Outcome = the item report, then move it to `docs/briefs/completed/` |
| `docs/progress/NOW.md` | Every handoff | Overwrite: state, the exact next step, open questions (at most 40 lines) |
| `docs/progress/LOG.md` | Every handoff | Prepend one entry (template at its top); never edit past entries |
| `AGENTS.md` | Commands or conventions changed | For example P1-01 adds the Taskfile commands |
| `docs/CONTEXT-MAP.md` | A new area or package needs mapping | One row |
| docs/01–04 (+ an AD) | The spec changed, with the owner's approval | In the same commit as the code |
| `docs/ENGINEERING.md` | A repeated mistake becomes a rule, with the owner's approval | One rule with an ID |

`-check` fails when a done item has no complete evidence, when evidence points at a missing report or a commit outside HEAD's history, when the plan is inconsistent, and, on an item branch, when the brief → tests → code order, the test lock, the brief's `Skip:` line or its Files are broken (docs/briefs/README.md › Machine checks).

## Right-sizing

The full loop is the default. A step that adds nothing to correctness for an item may be skipped:

- The writer proposes it in the item brief (`Skip:` line, reason in Decisions); the owner approves it with the brief. Never skip silently mid-item.
- Skippable, on items that are not critical only: G2 (tests come with the code, no separate approval) and G5 (independent review). A simple item's brief may write "None." under Spec and Contracts. A test that only re-checks what the build or `task check` already catches becomes a verify command.
- Never skipped: the brief (G1); a green baseline and full suite (G3); test-first and mutations for critical items (G2, G4); tests on money, state or concurrency paths; the evidence and the owner's `approve` (G6).

## Plan rules

- Work changes only an item's `status`, `owner`, `notes` and `evidence`, and a phase's `status`. Adding, removing or reordering items and changing `done_when` need the owner's approval (record it in LOG.md). A `verify` path may be made concrete once the package exists (record it in LOG.md).
- At the start of each phase, the owner reviews that phase's items against the code that exists by then.

## Engineering rules (non-negotiable)

- Money is `int64` minor units; no floating point on any money path, tests included (TD §1.1).
- The database clock decides: expiry, leases, limits and windows use `now()` in SQL (TD §1.3).
- State changes are guarded `UPDATE … WHERE status IN (…)` with the affected rows checked; stale transitions are counted no-ops (TD §5).
- Lock, then mutate, then post: row locks in the product's lock order (Pay TD §18.1, Tickets TD §28.1), then changes, then exactly one ledger `Batch` flush per transaction (TD §2, §3.3).
- No transaction spans both products; `pay/**` and `tickets/**` never import each other (ARCH §7.1).
- Calls across boundaries follow the three-phase template; an unknown outcome is resolved by lookup with the same derived key, never by a new call with a new key (TD §6, AD-13).
- Every effect is guarded by a unique key (M6).
- Never remove or weaken a test to make it pass. A test you believe is wrong is a question for the owner.
- Never mark an item done without its evidence and the owner's approval of the item report.
- Code follows docs/ENGINEERING.md; findings cite its rule IDs.

## Conventions

- Docs, code and comments in English.
- Branches: `item/<ID>-<slug>`, merged into `main` with `--no-ff` after approval, so verified commits stay in `main`'s history. A red baseline that belongs to no item is fixed on `fix/<slug>` and merged the same way after the owner approves the diff.
- Commits: `<type>(<ID>): <summary>`, type one of `feat`, `fix`, `test`, `docs`, `chore`, `refactor`; lowercase, imperative, under ~60 characters; no names and no trailers (no `Co-Authored-By`). An owner approval is the commit itself, made only after the owner's answer: the brief (`docs(<ID>): add brief`, later `docs(<ID>): update brief …`) and, unless the brief skips G2, the tests (`test(<ID>): …`).
- Commands: none yet. P1-01 adds the Taskfile (TD §35.1) and lists its commands here.
