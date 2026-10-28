---
name: handoff
description: Close a working session on Bastion - verify; when the item is finished run the review gates, show the item report and wait for approval; then update features.json, NOW.md, LOG.md and the item brief, commit, and merge the item branch when approved. Use before /clear, before switching items, or when the user stops for the day.
---

Run **4. End (handoff)** of the session protocol in AGENTS.md. The user invoking `/handoff` is the request to commit and, after `approve`, to merge.

1. Run the item's `verify` commands. Report what they printed; never write "tests pass" without having run them in this step. Before the tests exist (brief or G2 stage), say so instead.
2. **Item not finished:** skip to step 5 and keep the item `in_progress` (or `blocked` with the reason in `notes`). A draft brief is committed with the handoff; it counts as approved only from a `docs(<ID>): add brief` (or `update brief …`) commit made after the owner's answer.
3. **Item finished**, gates of docs/REVIEW.md:
   - G3: the baseline (`task check`), the full suite (`task test`), every `verify` command, concurrency tests under `-race -count=20`.
   - G4 (critical items): for each row of the brief's Mutations, and anything the item added beyond them, remove it, run the named test, see it fail, restore it. Record each result.
   - `go run scripts/brief/main.go -check` passes: brief → tests → code order and no test change outside `test(<ID>): …` (unless the brief says `Skip: G2`), a valid `Skip:` line, every path inside the approved brief's Files.
   - G5, unless the brief says `Skip: G5`: run the `reviewer` subagent with the item ID (base: `git merge-base main HEAD`). Fix every blocker, then run it again.
   - Write the item report (docs/REVIEW.md template, including line 0 on the executor) into the brief's Outcome. Show it and **wait** for `approve` or `changes:`.
4. On `approve`: in `docs/progress/features.json`, only that item's fields: `status: done` and `evidence` as docs/REVIEW.md › Evidence shows, with `evidence.report` pointing at `docs/briefs/completed/<ID>-<slug>.md`. `evidence.commit` is the current HEAD (the code the gates ran on), recorded before the handoff commit. Set the phase `done` if this was its gate item. On `changes:`: keep `in_progress` and write the requested changes into NOW.md "Next step".
5. Overwrite `docs/progress/NOW.md` (at most 40 lines): header fields, State, an exact and actionable Next step (item ID, stage, the next brief step and its executor, what to read), Open questions, Recent decisions.
6. Prepend an entry to `docs/progress/LOG.md` using the template at its top.
7. Item brief: add Progress and Surprises (brief gaps, stops and their answers); when the item is done, move it to `docs/briefs/completed/`.
8. If the work changed the spec, the docs/01–04 change is part of this commit.
9. Run `go run scripts/brief/main.go -check` and fix any error. Check: could a new agent with only the repository continue from NOW.md and the brief without asking anything? If not, add what is missing.
10. Commit everything as `chore(<ID>): handoff` (or `chore(<ID>): done`) and confirm the working tree is clean.
11. Approved items only: `git switch main`, `git merge --no-ff item/<ID>-<slug>`, `git branch -d item/<ID>-<slug>`, run the baseline on `main`, and run `-check` again.
12. Tell the user in at most 6 lines: what is done and verified, what is not, the next step, and that `/clear` comes before the next item.
