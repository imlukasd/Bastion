# Item briefs

Every backlog item gets an item brief before any test or code: one file that says how the item will be built, with every decision already made. The owner approves it (REVIEW G1); the executor then follows it and makes no decisions of its own. A brief is ready when a standard model with only the repository and this file could implement the item without asking anything.

The **session brief** is something else: the orientation that `go run scripts/brief/main.go` prints at startup.

## Lifecycle

| When | What happens | Commit |
| --- | --- | --- |
| After `chore(<ID>): start` | The writer (strong model) drafts `active/<ID>-<slug>.md` from the template, asks the owner the decisions that are theirs, and settles technical unknowns with spikes outside the repository (a scratch directory; never committed). | none (draft) |
| Owner answers `approve brief` (G1) | The Definition of Ready holds; the `Approved:` field is filled in. | `docs(<ID>): brief (approved: <owner>)` |
| Implementation | The executor updates nothing in the brief; the writer adds Progress and Surprises. Changing any other section needs the owner's approval again. | `docs(<ID>): brief update (approved: <owner>)` |
| Owner answers `approve` (G6) | Outcome = the item report; the file moves to `completed/`. | part of `chore(<ID>): done` |

The slug is the item branch's (`item/<ID>-<slug>`). An approval is always a commit whose subject ends with `(approved: <owner>)`, written right after the owner's answer.

## Definition of Ready (the owner checks it at G1)

1. Goal numbers every `done_when` clause, and Test map names a test for each.
2. Spec quotes every rule the item needs, with its section, so the executor never opens docs/01–04.
3. Decisions has no row left `OPEN`; each technical unknown was settled by a spike and the row says how.
4. Files lists every path that will be created or changed.
5. Contracts gives the exact signatures, types, schemas, error values, configuration and command behavior that other code and the tests rely on.
6. Steps are ordered and small, each with a verify command and its expected result.
7. Out of scope names the neighbouring work and the item that owns it.
8. Stop rules name the situations where the executor must stop instead of choosing.
9. Executor fits the risk (below).
10. Critical items: Locks in the product's lock order (TD §18.1, §28.1), a Race table and the planned Mutations.

## Choosing the executor

- `strong`: every `critical` item; every item that sets a convention later items copy (all of Phase 1, the first item of a new kind); any item whose brief the writer doubts is decision-free.
- `standard`: everything else.

The writer proposes, the owner confirms at G1. The strong model always writes the brief and the tests, runs the gates and the review, whichever the executor. Which model plays which role is tool-specific (Claude Code: CLAUDE.md).

## Machine checks

On an item branch, `go run scripts/brief/main.go -check` (part of `task check`) fails on:

- a `test`, `feat`, `fix` or `refactor` commit before `docs(<ID>): brief (approved: …)`;
- a `feat`, `fix` or `refactor` commit before the first `test(<ID>): … (approved: …)`;
- a change to a test (`*_test.go` or anything under `testdata/`) in a commit that is not `test(<ID>): … (approved: …)`;
- a changed path, committed or not, outside the Files of the last approved brief (always allowed: the item's brief, `docs/progress/NOW.md`, `LOG.md`, `features.json`);
- an approved brief without a required section or the `Executor:` line, with a decision still `OPEN`, or with `Executor: standard` on a critical item.

Files are read from the last approved brief commit, not from the working copy: widening the scope takes a new approval.

## Template

Items that are not critical leave out Locks, Race table and Mutations. Keep every other heading, write "None." under one that does not apply, and keep the headings exactly as written: the machine checks find sections by them.

```markdown
# <ID> — <title>

Executor: strong | standard · Owner: <name> · Started: YYYY-MM-DD · Approved: YYYY-MM-DD by <owner>

## Goal
The item's `done_when`, one clause per line:
1. …

## Spec
The rules this item needs, quoted with their section; the executor does not open docs/01–04.
- TD §x.y: "…"

## Decisions
Everything the spec leaves open, settled before G1. `OPEN` marks a row waiting for the owner; none may remain at approval.
| # | Question | Decision | By | Why |
| --- | --- | --- | --- | --- |
| D1 | … | … | owner / writer / spike | … |

## Files
Every path created or changed, in backticks, one per line; globs `*`, `**` and `{a,b}` are allowed. Nothing else may change.
Create:
- `path/to/file.go`
Modify:
- `path/to/other.go`: what changes

## Contracts
Exact signatures, types, schemas (DDL), error values, configuration keys and command behavior that other code or the tests rely on.

## Steps
1. … Verify: `command` → expected result.

## Test map
| Clause | Test (file › name) | Asserts |
| --- | --- | --- |
| 1 | … | … |

## Locks
Critical items: the row locks each transaction takes, in the product's lock order (TD §18.1 or §28.1).

## Race table
Critical items: each pair of concurrent operations and what decides it.
| Operation A | Operation B | Decided by (guard, constraint or lock) | Test |
| --- | --- | --- | --- |

## Mutations
Critical items: what G4 will remove and which test must then fail.
| Guard, constraint or lock ordering | Test that must fail |
| --- | --- |

## Out of scope
- … (the item that owns it)

## Stop rules
Stop and report instead of choosing when:
- a decision is needed that this brief does not make;
- a file outside Files must change, a dependency not named here is needed, or a test seems wrong;
- the code or the spec contradicts this brief;
- … (item-specific)

## Progress
- YYYY-MM-DD HH:MM — step N done (strong | implementer); `verify command` → result

## Surprises
Brief gaps (each question the executor had to ask, and the answer), spec gaps, unexpected behavior, with short evidence.

## Outcome
When done: the item report (docs/REVIEW.md › Item report).
```
