# Workflow

The working loop on Bastion, seen from the owner's side: what you type, what the agent does, what you check. The agent's own instructions are in [AGENTS.md](../AGENTS.md); the acceptance rules are in [REVIEW.md](REVIEW.md); the item brief's template and checks are in [briefs/README.md](briefs/README.md).

## What you type, in order

| # | You type | When | Your time |
| --- | --- | --- | --- |
| 1 | `/catchup` | You open a session | read 12 lines |
| 2 | `start P1-01` (or `fix`, if something is red) | After the report | — |
| 3 | answers to the `OPEN` decisions (`D1: …; D2–D4: as proposed`) | The agent asks while writing the brief | 2–5 min |
| 4 | `approve brief` or `changes: …` | The agent shows the finished brief (G1) | 5–10 min |
| 5 | `ok tests` or `changes: …` | The agent shows the failing tests (G2) | 2–5 min |
| 6 | answers, only when asked | The agent stops on a decision the brief does not make | rare |
| 7 | `/handoff` | The agent says the implementation is done | — |
| 8 | `approve` or `changes: …` | The agent shows the item report (G6) | ~15 min |
| 9 | `/clear`, then `/catchup` | The item is merged | — |

Stopping before the item is finished: `/handoff` (the item stays in progress), then `/clear`; next time `/catchup` resumes where it stopped. You never run git yourself; every commit, branch and merge is the agent's job.

## The loop

```
/catchup ──► red? ──yes──► fix it first
               │ no
               ▼
start: item branch + claim
               ▼
① brief (strong model) ──► you answer OPEN decisions ──► you: approve brief (G1) ──► commit
               ▼
② tests + stubs (strong model) ──► you: ok tests (G2) ──► commit; tests are now locked
               ▼
③ implement ── Executor: strong   ──► the strong model, in the main session
             └ Executor: standard ──► implementer subagent (standard model)
                                        └ unclear? stops and asks ──► agent or you answer
               ▼
④ /handoff: G3 full suite ─► G4 mutations (critical) ─► G5 reviewer (strong model) ─► item report
               ▼
⑤ you: approve ──► evidence, NOW, LOG, merge --no-ff into main ──► /clear
   you: changes: … ──► back to ③
```

## Step by step

### 1. Open a session and triage

- **You:** open Claude Code in the repository, then `/catchup`.
- **Agent:** the session brief has already run (hook). The agent reads NOW.md, the active item brief and the top LOG entry, checks what is red (warnings, the baseline, an item in progress, changes you asked for) and reports **Red / State / Next / Questions** in at most 12 lines. It waits.
- **You check:** "Red" is empty and shows the baseline's output line. If something is red, answer `fix`: the fix comes before anything new.

### 2. Start the item

- **You:** `start P1-01` (or the item it proposed).
- **Agent:** creates the git branch `item/P1-01-<slug>` from `main` in your local repository (your working folder now shows that branch), sets the item `in_progress`, commits `chore(P1-01): start`.
- **You:** nothing.

### 3. The item brief (G1)

- **Agent (strong model):** reads the item, its spec sections and the code that exists, runs spikes in a scratch directory for anything it is unsure of, and writes `docs/briefs/active/P1-01-<slug>.md`: Goal, quoted Spec, Decisions, Files, Contracts, Steps, Test map, Out of scope, Stop rules, Executor (critical items also Locks, Race table, Mutations). Decisions that are yours are marked `OPEN` and asked in the chat.
- **You:** answer the `OPEN` decisions. The agent writes the answers into the brief and shows it.
- **You check (5–10 minutes):**
  1. every Goal clause has a test in the Test map;
  2. Files is complete and holds nothing extra;
  3. no `OPEN` row is left;
  4. Out of scope keeps neighbouring items out;
  5. Executor fits the risk (`strong` for critical items and all of Phase 1).
- **You:** `approve brief`, or `changes: …`.
- **Agent:** commits `docs(P1-01): brief (approved: <you>)`. That commit is the contract: if the brief changes later, `git diff` shows it, and a change beyond Progress and Surprises needs your approval again.

### 4. Tests first (G2)

- **Agent (strong model):** writes the tests of the Test map with stubs from the Contracts, runs them and shows that they fail on assertions, not on compile errors.
- **You check (2–5 minutes):** the test names match the Test map; the assertions check stored results, not only "no error".
- **You:** `ok tests`, or `changes: …`.
- **Agent:** commits `test(P1-01): … (approved: <you>)`. From now on a test changes only in another commit you approved; `-check` fails otherwise.

### 5. Implement

- **Agent:** follows the brief's Steps; every green step is a commit `feat(P1-01): …`.
  - `Executor: strong`: the strong model writes the code in the main session.
  - `Executor: standard`: the main session hands the steps to the `implementer` subagent (standard model) with only the item ID, the brief path and the step numbers. The implementer reads the brief, writes the code, runs the checks, commits, and stops when the brief does not answer something. After each return the main session checks the scope and records Progress.
- **You:** nothing, unless the agent asks. A question reaches you only when it needs a new decision or the spec is in conflict; questions the brief or the spec answers are settled by the agent and recorded as brief gaps.

### 6. Close: gates G3–G5

- **You:** `/handoff` when the agent says the implementation is done.
- **Agent:** runs the `verify` commands, the baseline, the full suite and the concurrency tests under `-race -count=20`; for critical items removes each guard, shows the test that fails and restores it; runs the `reviewer` (strong model, fresh context) and fixes every blocker; writes the item report into the brief's Outcome and shows it.

### 7. Approve (G6)

- **You check (about 15 minutes, REVIEW › How the owner reviews):** the report, including the executor line (stops, brief gaps, steps taken over); `-check` passes; `git diff --stat main...HEAD` against the brief's Files; one `verify` command run yourself; the core diff.
- **You:** `approve`, or `changes: …` with the points and rule IDs; on `changes:` the agent goes back to step 5.

### 8. Record and merge

- **Agent:** writes `evidence` into `features.json`, moves the brief to `docs/briefs/completed/`, overwrites NOW.md, prepends a LOG entry, runs `-check`, commits `chore(P1-01): done`, merges with `--no-ff` into `main`, deletes the branch and runs the baseline on `main`.
- **You:** `/clear`, then `/catchup` for the next item.

## When something interrupts the loop

| Situation | What happens |
| --- | --- |
| The context fills up, or you stop for the day | `/handoff` (unfinished) → `/clear` → `/catchup` resumes from NOW.md and the brief's Progress |
| The implementer stops with a question the brief or spec answers | The main session answers, records a brief gap under Surprises, hands the step back |
| The implementer stops on a new decision or a spec conflict | You decide; a change to the brief is committed as `docs(<ID>): brief update (approved: <you>)`, a spec change goes into docs/01–04 (and an AD) |
| The same step fails twice | The strong model takes the step over; the report says so |
| A test or a path outside Files was touched | `-check` fails; the agent reverts that work and redoes the step |
| You reject the report | `changes: …` → back to implementation; no evidence is written |
| Something unrelated breaks | It goes to NOW.md "Open questions" and gets its own item or `fix/` branch |
| An external decision blocks the item | `/handoff` with `blocked` and the reason in `notes`; pick another ready item |
| A session crashed | The session brief shows uncommitted changes, an unmerged branch or a stale NOW.md; `/catchup` reconciles that first |

## Models

| Role | Claude Code |
| --- | --- |
| Main session: triage, item brief, tests, gates, implementing `Executor: strong` | Opus (`"model": "opus"` in `.claude/settings.json`) |
| Implementing `Executor: standard` | `implementer` subagent, Sonnet (`.claude/agents/implementer.md`) |
| G5 review | `reviewer` subagent, pinned to Opus (`.claude/agents/reviewer.md`) |

Saving happens where the work is mechanical and fully specified; the decisions, the tests and the review always use the strong model. Phase 1 runs entirely on `Executor: strong`, because it sets the conventions later items copy.

## Files and their roles

| File | Role | Written by | Read |
| --- | --- | --- | --- |
| `AGENTS.md` | Rules and session protocol for every agent and person; map of the repository | Owner (the agent proposes changes) | Every session, automatically |
| `CLAUDE.md` | Claude Code's entry point: imports AGENTS.md and NOW.md; models and subagents | Owner | Every Claude session, automatically |
| `.claude/settings.json` | Main model, the SessionStart hook (session brief), permission allowlist | Owner | By Claude Code |
| `.claude/skills/catchup/SKILL.md` | `/catchup`: step 1 | Owner | When you type it |
| `.claude/skills/handoff/SKILL.md` | `/handoff`: steps 6–8 (or saving an unfinished item) | Owner | When you type it |
| `.claude/agents/implementer.md` | Standard-model executor of an approved brief | Owner | Step 5 |
| `.claude/agents/reviewer.md` | Independent, read-only reviewer with a fresh context (G5) | Owner | Step 6 |
| `scripts/brief/main.go` | The session brief; `-check` validates the plan, evidence and the item branch; `-plan` and `-item` show the plan | Owner | Every session and every handoff |
| `docs/progress/NOW.md` | Current state, the exact next step, open questions; overwritten | Agent, at each handoff | Every session |
| `docs/progress/LOG.md` | History of sessions, newest first; never edited | Agent, at each handoff | The top entry, each session |
| `docs/progress/features.json` | The whole plan: phases, items, `done_when`, `verify`, status and evidence | Agent (status, evidence); you approve plan changes | Through the session brief, `-plan`, `-item` |
| `docs/briefs/active/`, `completed/` | One item brief per item: how it is built, its progress and its report | Agent; you approve | Steps 3–8 |
| `docs/briefs/README.md` | Brief template, Definition of Ready, executor choice, machine checks | Owner | Step 3 |
| `docs/CONTEXT-MAP.md` | Which spec sections each area needs | Owner | Step 3 |
| `docs/ENGINEERING.md` | Code rules E1–E40 and the patterns of Bastion | Owner | Steps 3–6 |
| `docs/REVIEW.md` | Gates, item report, evidence format, red flags, your review checklist | Owner | Steps 3–7 |
| `docs/WORKFLOW.md` | This loop, from your side | Owner | Onboarding |
| `docs/01-prd.md` … `04-invariants.md` | The spec, source of truth | Owner; changes approved | By section, through the item's `refs` |
| `docs/README.md` | Index of the documentation | Owner | Onboarding |
| `.gitattributes`, `.gitignore` | LF line endings; personal and generated files kept out of git | Owner | By git |
| `CLAUDE.local.md` (optional, gitignored) | Your personal preferences for Claude | You | Your Claude sessions |

## Commands

| Command | Shows |
| --- | --- |
| `go run scripts/brief/main.go` | The session brief |
| `go run scripts/brief/main.go -plan` | Every item by phase, with status and evidence |
| `go run scripts/brief/main.go -item P1-06` | One item in full |
| `go run scripts/brief/main.go -check` | Plan and evidence validation; on an item branch also brief → tests → code order, the test lock and the brief's Files (exit 1 on errors) |
| `git log --oneline main..HEAD` | The commits of the current item |
| `git diff --stat main...HEAD` | The scope of the current item |
| `/catchup`, `/handoff`, `/clear`, `/context`, `/model` | Claude Code: triage, close, fresh context, context use, model choice |

## Spending tokens well without losing correctness

- One item per session, and `/clear` after every merge: a fresh context costs less per turn and keeps the agent focused.
- Use the session brief and `-item` instead of reading files whole; the spec is read by section, and the item brief quotes what the executor needs.
- A complete brief is what lets the standard model implement; a gap found during implementation costs more than a minute spent on the brief.
- Save tokens on reading and implementing, never on the brief, the tests or the gates: G1–G6 always run.
