# Review and approval

How a backlog item is accepted. The writer (agent or person) produces the item report, an independent reviewer checks the work, and the owner approves. An item becomes `done` only after the owner approves.

## Gates

| Gate | When | Who | Passes when |
| --- | --- | --- | --- |
| G1 Brief | Before any test or code, **every item** | Owner (5–10 min) | The item brief meets the Definition of Ready (docs/briefs/README.md): every `done_when` clause mapped to a named test (or a verify command); spec quoted; no `OPEN` decision; every path in Files; exact Contracts; Stop rules; an Executor that fits the risk; for critical items the lock order, a race table and the planned mutations; every skipped gate justified; nothing contradicts the spec. Committed as `docs(<ID>): add brief`. |
| G2 Tests | Before the implementation, unless the brief says `Skip: G2` | Owner (2–5 min) | Every `done_when` clause has a test; each test fails for the right reason (an assertion, not a compile error); assertions check stored facts (E31). The tests (and the stubs they need) are committed alone as `test(<ID>): …`; later test changes only in another such commit. |
| G3 Green | After the implementation | Machine | Every `verify` command passes; the baseline (`task check`: build, lint, archtest, unit tests, plan check) passes; the **full suite** (`task test`) passes, so nothing else regressed; concurrency tests pass under `-race` repeatedly (`-count=20`). |
| G4 Mutation | After G3 | Writer | For each guard, constraint and lock ordering the item adds: removed → a named test fails → restored (E32). |
| G5 Independent review | After G4, unless the brief says `Skip: G5` | `reviewer` subagent (fresh context) or another person | No blocker left; every major fixed or accepted by the owner with an open question or backlog item. |
| G6 Approval | At handoff | Owner | The item report is complete and shows no red flag. |

G1 applies to every item; critical items use the full brief. G4 applies to items marked `critical` (money, locks, state machines, concurrency); for the others the report says "G4: not applicable". An item that is not critical may skip G2 and G5 when its approved brief says so (AGENTS.md › Right-sizing); G1, G3 and G6 are never skipped. G1, G2, the test lock and the `Skip:` line are also checked by `go run scripts/brief/main.go -check` on the item branch (docs/briefs/README.md › Machine checks).

## Item report

The writer puts it in the item brief's Outcome section and shows it to the owner.

```markdown
## Item report — <item-id> <title>

Writer's assessment: ready for approval | not ready (why)

0. Executor: strong | standard; implementer runs; stops (each: question → answer, brief gap or owner decision); steps the strong model took over. Skipped gates: none, or each with the brief's reason.
1. Scope: `git diff --stat main...HEAD` summary; core files; every path is in the brief's Files (`-check` passes).
   Tests changed after the first `test(<ID>)` commit: none, or each `test(<ID>): …` commit and why ("G2 skipped" when the brief skips it).
2. Traceability, one row per `done_when` clause:
   | Clause | Test (file › name) | Result |
3. Guards: for each guard, constraint or lock ordering: invariant, location (file:line), test, mutation result.
4. Verify: each command and what it printed (counts, duration, `-race`, `-count`).
5. Spec: deviations (none, or section + proposed docs change + owner decision).
6. Decisions taken during the item, with reasons.
7. Review: reviewer verdict; each finding (rule ID, severity) and how it was resolved.
8. Follow-ups: open questions, debts, next item.
```

## Evidence

When the owner approves, the writer records the proof in the item's `evidence` in `docs/progress/features.json`. The item report is the full explanation; the evidence is the short, machine-checked proof.

```json
"evidence": {
  "date": "2026-11-02",
  "commit": "3f9c2ab",
  "results": [
    { "command": "go test ./internal/kit/ledger/...", "outcome": "pass", "output": "ok 31 tests, 8.2s" }
  ],
  "mutations": [
    "flush.go:61 sort by id removed → TestCC01_LedgerBothDirections deadlocked; restored"
  ],
  "review": "reviewer APPROVE after fixing 1 blocker (E18 batch.go:142)",
  "report": "docs/briefs/completed/P1-06-ledger-api.md",
  "approved_by": "Lukas"
}
```

- `commit`: the commit the `verify` commands ran on (HEAD before the handoff commit).
- `results`: every `verify` command of the item, character for character, with `pass` and the line it printed.
- `mutations`: required for `critical` items (G4), one line per guard, constraint or lock ordering.
- `report`: the item brief in `docs/briefs/completed/`, whose Outcome holds the item report.

`go run scripts/brief/main.go -check` rejects a done item with missing or incomplete evidence, a report that does not exist, or a commit outside HEAD's history. An item reopened later loses its evidence (moved to LOG.md) until it is approved again.

## Severity

| Severity | Meaning | Action |
| --- | --- | --- |
| Blocker | Breaks an invariant or the spec, or violates E8, E16–E24 or E34 | Must be fixed before approval |
| Major | Maintainability or test gap (E1–E7, E9–E15, E25–E33) | Fix, or the owner accepts it with an open question or backlog item |
| Minor | Style and naming | Fix when cheap |

## Red flags: reject immediately

- A `verify` result described instead of shown ("tests pass").
- A test deleted, skipped, loosened, or its expected values changed without an owner decision (E34), or, when G2 applies, changed outside a `test(<ID>): …` commit.
- Tests or code committed before the approved brief, or, when G2 applies, code before the approved tests.
- A gate skipped that the approved brief does not name, or any skip on a critical item.
- Files changed outside the brief's Files, or an unrelated refactor.
- A brief gap resolved by the executor's guess instead of the brief, the spec or the owner.
- A spec deviation without a docs/01–04 change and owner approval.
- A blocker-class rule violated (E8, E16–E24).
- A concurrency test not run under `-race`, or `time.Sleep` used for synchronization.
- A new dependency, `TODO`, commented-out code or `//nolint` without a reason.

## How the owner reviews (about 15 minutes)

1. Read the item report from top to bottom.
2. Check that `go run scripts/brief/main.go -check` passes on the item branch (order, test lock, `Skip:`, Files) and that `git diff --stat main...HEAD` matches "Scope".
3. Pick two traceability rows at random and open the tests: they assert what the clause says.
4. Run one `verify` command yourself.
5. Read the core diff (`git diff main...HEAD -- <core files>`) with E16–E24 in mind.
6. Answer `approve`, or `changes:` followed by the points and rule IDs.
