---
name: catchup
description: Re-orient on Bastion at the start of a session or after /clear - read the state, fix what is red first, verify the baseline and propose the next item. Use when starting work, or when someone new asks where the project stands and what to do next.
---

Run **1. Start: red first** of the session protocol in AGENTS.md.

1. If the session brief from the SessionStart hook is not in context, run `go run scripts/brief/main.go`.
2. NOW.md is loaded through CLAUDE.md. Read the active item brief if the session brief or NOW.md names one (its header, Decisions and Progress first), and the top entry of `docs/progress/LOG.md`.
3. Triage in this order, and stop at the first thing that is red:
   1. `WARN` lines of the session brief. Uncommitted changes: read `git diff` and say whether they match NOW.md; never discard them without the user's consent. Unmerged item branch while on `main`: propose `git switch` to it. NOW.md behind HEAD: read `git log <rev>..HEAD`.
   2. The baseline: `task check` if `Taskfile.yml` exists, else `go run scripts/brief/main.go -check`. Show what it printed.
   3. An item `in_progress`: propose to resume it at its stage: brief not approved yet (finish it, ask the `OPEN` decisions), tests not approved yet (G2, unless the brief says `Skip: G2`), or the next step in the brief's Progress (who executes it: the brief's `Executor:`).
   4. Owner-requested changes in NOW.md.
4. Report in at most 12 lines, in the user's language:
   - **Red**: what is red and the fix you propose, or "nothing red" with the baseline output line;
   - **State**: phase, last done item, current branch;
   - **Next**: the item (from NOW.md "Next step", else the first ready item; read it with `go run scripts/brief/main.go -item <ID>`), why it is next, whether it is `critical` (full brief), the executor you expect (`strong` for critical items and all of Phase 1), and the sections its `refs` name. Every item starts with its brief (G1);
   - **Questions**: open questions that need the owner.
5. Wait for the user's go before fixing, branching or writing anything.

If arguments name an item ID, propose that item instead, after checking its dependencies: $ARGUMENTS
