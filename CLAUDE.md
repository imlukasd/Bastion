@AGENTS.md
@docs/progress/NOW.md

## Claude Code

- The SessionStart hook in `.claude/settings.json` prints the session brief at startup, after `/clear` and after compaction. If it is missing from context, run `go run scripts/brief/main.go`.
- `/catchup` runs the Start protocol; `/handoff` runs the End protocol. Run `/handoff` before `/clear`, and never carry work across items with `/compact`.
- Models: the main session runs on Opus (`.claude/settings.json` sets it) and is the strong model: it writes every item brief and the tests (no code before `approve brief`; plan mode is not needed, since it cannot write the brief file), implements briefs marked `Executor: strong`, runs the gates. For `Executor: standard`, delegate the steps to the `implementer` subagent (Sonnet, `.claude/agents/implementer.md`) with the item ID, the brief path and the steps; never paste the brief into the prompt, the file is the contract. The `reviewer` is pinned to Opus.
- Gate G5: run the `reviewer` subagent (`.claude/agents/reviewer.md`) with the item ID; it starts with a fresh context, so it does not share the writer's assumptions.
- Read large spec sections or search broadly through a subagent and keep only its summary in the main context.
- Personal preferences go in `CLAUDE.local.md` (gitignored).
