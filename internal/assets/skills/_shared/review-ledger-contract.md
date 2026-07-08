# Review Ledger Contract (shared across the 4R review lenses and judgment-day)

Canonical source of truth for the exhaustive first-pass loop, the persisted
findings ledger, the artifact-store persistence branches, and the scoped
re-review/re-judge contract. Every review-* subagent asset, every jd-*
subagent asset, every orchestrator's inline-lens "Review Execution Contract"
section, and the judgment-day skill docs hand-copy the clauses below verbatim
so a single table-driven test (`internal/components/sdd/review_ledger_contract_test.go`)
can assert they stay in sync across all 13 adapter variants and both
execution modes.

Why this exists: the 4R lenses (review-risk / R1, review-readability / R2,
review-reliability / R3, review-resilience / R4) and judgment-day previously
ran a single-pass read with no memory across rounds — each pass sampled a
different subset of real issues, and re-review surfaced old issues as if new.
Iterating never converged. This contract replaces that with a bounded
exhaustive first pass, a persisted ledger, and a re-review scoped to the
ledger plus the fix diff.

## Canonical block (hand-copy verbatim into every adopting asset)

**Exhaustive first pass.** Loop until dry: sweep the diff repeatedly until N consecutive sweeps yield zero new findings, then stop; the loop MUST be finite. Default N = 2 consecutive dry sweeps. R2 Readability MAY use N = 1. Hard ceiling: 4 sweeps regardless of N.

**Findings ledger.** Emit a findings ledger with this schema for every entry:

| Field | Values |
|-------|--------|
| `id` | `{LENS}-{NNN}` (e.g. `R1-001`) |
| `lens` | risk \| readability \| reliability \| resilience \| judgment-day |
| `location` | `path/to/file.ext:line` or `:start-end` |
| `severity` | BLOCKER \| CRITICAL \| WARNING \| SUGGESTION |
| `status` | open \| fixed \| verified \| wont-fix \| info |
| `evidence` | why it matters |

If the first pass finds nothing, persist an empty ledger record rather than skip persistence.

**Ledger persistence honors the artifact store.**
- `openspec`: write `openspec/changes/{change-name}/review-ledger.md`.
- `engram`: upsert topic `sdd/{change-name}/review-ledger` (ad-hoc judgment-day without a change: `review/{target-slug}/ledger`, where `target-slug` = `pr-{number}` when reviewing a PR, else the current branch name kebab-cased, else a kebab-case slug of the user-stated review target).
- `none`: keep the ledger inline in the response; do not write files or Engram artifacts — the ledger lives only in this conversation; complete the review → fix → re-review loop within the session because it is not persisted across compaction.

**Scoped re-review.** A re-review pass takes the persisted ledger and the fix diff as input. It MUST verify each ledger finding's resolution and MUST review only fix-touched lines; it MUST NOT re-read the full original diff. A finding on an untouched line MUST be logged with status `info` as a first-pass quality signal and MUST NOT by itself trigger another full round.

## Notes on the schema (not part of the hand-copied block)

**N and the ceiling.** N = 2 catches the single-pass sampling gap; the ceiling caps runaway review cost. R2 Readability is suggestion-heavy and cheap to re-run, so it may relax to N = 1.

**Status lifecycle.** `open` (first-pass finding) → `fixed` (fix agent changed code) → `verified` (re-review confirmed resolved). `wont-fix` = accepted/deferred with reason. `info` = a new finding on an untouched line (first-pass quality signal, NOT a re-round trigger), and also covers judgment-day's `WARNING (theoretical)` items — JD's real/theoretical distinction collapses onto `severity=WARNING` plus `status` (`open` vs `info`), so JD and the 4R lenses write the same table.

**Judgment-day.** The re-judge pass (following jd-fix-agent) follows this same scoped re-review contract: it verifies ledger findings and reviews only fix-touched lines.

## Execution modes

The contract above is stated once; only ledger ownership differs by mode:

- **Subagent mode** (Claude, Cursor, Kimi, Kiro): each review-* / jd-* agent
  runs its lens exhaustively and returns its own ledger rows in its Output
  contract; the orchestrator merges those subagent ledger rows into the
  persisted ledger and persists per the branch above.
- **Inline mode** (Codex, Gemini, Qwen, OpenCode/Kilocode, Windsurf,
  Antigravity, Hermes, generic, and any adapter without dedicated review-*/
  jd-* subagents): the orchestrator runs each lens sequentially in its own
  context and maintains the merged ledger directly.

## Interfaces / Contracts

Canonical ledger row, rendered identically in every asset:

```
| id     | lens        | location            | severity | status | evidence            |
|--------|-------------|---------------------|----------|--------|---------------------|
| R1-001 | risk        | internal/x.go:42    | CRITICAL | open   | secret hardcoded    |
| JD-004 | judgment-day| internal/y.go:88    | WARNING  | info   | theoretical path    |
```

## Adopting assets

Hand-copy the sections above (Exhaustive first-pass, Findings ledger schema,
Ledger persistence, Scoped re-review) into:

- `internal/assets/{claude,cursor,kimi,kiro}/agents/review-{risk,readability,reliability,resilience}.md`
- `internal/assets/{claude,kiro}/agents/jd-{judge-a,judge-b}.md`
- Every `internal/assets/*/sdd-orchestrator.md` (Review Execution Contract section)
- `internal/assets/skills/judgment-day/SKILL.md` and `references/prompts-and-formats.md`

Exception: `internal/assets/{claude,kiro}/agents/jd-fix-agent.md` is NOT a
hand-copy target for this judge-oriented block. It carries the distinct
fix-agent clause set enforced by `requiredFixAgentClauses` in the test below —
the fix role applies confirmed fixes and does not run the exhaustive first
pass or emit a findings ledger. `references/prompts-and-formats.md` carries
both: judge clauses in the Judge Prompt template, fix clauses in the Fix
Agent Prompt template.

Each surface also states its own execution-mode sentence per the "Execution
modes" section above. `internal/components/sdd/review_ledger_contract_test.go`
enforces this parity with a table-driven `requiredLedgerClauses` consistency
check.
