# Exploration — review-ledger-contract

## Problem

The 4R review framework (review-risk, review-resilience, review-readability, review-reliability) produces multi-iteration churn: a review pass finds issues, fixes are applied, a re-review then surfaces NEW issues that existed all along — repeating for several rounds. Each review pass runs with fresh context and no memory of prior passes, so every pass is an independent sample of a subset of the real issues. Iterating full reviews never converges deterministically.

Judgment-day (jd-judge-a/b → jd-fix-agent → re-judge) already has blind dual review and a fix loop, but no persisted findings ledger and no scoping of the re-judge pass.

## Root causes

1. **Single-pass sampling**: each review-* lens does one read of the diff. LLM review recall on one pass is a subset; different passes surface different subsets.
2. **No findings ledger**: findings live only in the conversation. Re-reviews cannot distinguish "verify this fix" from "review from scratch".
3. **Unscoped re-review**: the re-review re-reads the entire diff with fresh eyes instead of (a) verifying each prior finding is fixed and (b) reviewing only lines touched by the fixes.

## Agreed direction (from planning discussion)

1. **Exhaustive first pass**: within each lens, the reviewer loops until dry — keeps scanning until N consecutive sweeps find nothing new — instead of a single read.
2. **Persisted findings ledger**: first pass emits a structured ledger (finding id, file:line, severity, lens, status). Persisted as a change artifact (openspec) or Engram topic, depending on the session's artifact store.
3. **Scoped re-review contract**: re-review input = ledger + fix diff. Scope = verify each ledger finding resolved + review only fix-touched lines. A "new" finding on untouched lines is a first-pass quality signal, not a trigger for another full round.

## Where the contract lives (from codebase exploration)

- Orchestrator assets (13 adapter variants): `internal/assets/{claude,codex,opencode,windsurf,kiro,cursor,gemini,qwen,kimi,hermes,antigravity,generic}/sdd-orchestrator*.md`. Claude: `internal/assets/claude/sdd-orchestrator.md` (7.8KB) + `sdd-orchestrator-workflow.md` (13.8KB, lazy-loaded surface).
- Review agent definitions: `internal/assets/claude/agents/review-{risk,resilience,readability,reliability}.md` (~1.4–1.7KB each); variants under `internal/assets/{kiro,cursor,kimi}/agents/`.
- Judgment-day agents: `internal/assets/claude/agents/jd-judge-a.md`, `jd-judge-b.md`, `jd-fix-agent.md`; judgment-day skill `internal/assets/skills/judgment-day/`.
- Trigger rules (which lenses fire when): `internal/catalog/triggers.go` (`DefaultTriggerRuleSet`), rendered by `internal/components/sdd/triggerrules.go` (`RenderTriggerRules`), injected at `internal/components/sdd/inject.go:303-349`.
- Injection paths: marker sections via `filemerge.InjectMarkdownSection`; OpenCode inlines orchestrator prompt into `opencode.json` (`internal/components/sdd/inject.go:835-928`).
- Adapters WITH real subagents (Claude, Kiro, Cursor, Kimi) run review-* as separate agents; the rest run review lenses inline in the orchestrator — the ledger contract must be worded for both execution modes.

## Constraints

- Applies to all 13 adapter variants; single conceptual change replicated per asset.
- Ledger persistence must respect the session artifact store (openspec file vs Engram topic vs none).
- Strict TDD is active for this repo; trigger-rule rendering changes need table-driven tests (existing golden: `internal/testdata/golden/trigger-rules-default.golden`).

## Related workstreams (separate changes, not in scope here)

- Engram protocol dedup + per-model tiering (change 2).
- Persona canonical-channel injection (change 3).
