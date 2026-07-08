# Proposal: Review ledger contract

## Intent

The 4R review framework (review-risk, review-resilience, review-readability, review-reliability) churns across rounds: each pass runs with fresh context and no memory, so every pass samples a different subset of real issues and re-reviews surface old issues as if new. Iterating never converges. Judgment-day has a fix loop but no persisted ledger and no scoped re-judge. Fix this with an exhaustive first pass, a persisted findings ledger, and a scoped re-review contract.

## Scope

### In Scope
- Exhaustive first pass: each lens loops until dry (N consecutive sweeps find nothing new) instead of a single read.
- Persisted findings ledger: first pass emits structured findings (id, file:line, severity, lens, status), persisted per the session artifact store.
- Scoped re-review contract: re-review input = ledger + fix diff; verify each ledger finding resolved and review only fix-touched lines. New findings on untouched lines are logged as a first-pass quality signal, not a trigger for another full round.
- Apply to all 13 adapter orchestrator variants and both execution modes (separate subagents: Claude/Kiro/Cursor/Kimi; inline lenses: the rest).
- Judgment-day gets the same ledger + scoped re-judge treatment.

### Out of Scope
- Engram protocol dedup/tiering (separate change).
- Persona canonical-channel injection (separate change).
- Changing which lenses fire when (trigger-rule policy), SDD phase order, or delegation mechanics.
- Implementing this change; this artifact defines the plan only.

## Capabilities

### New Capabilities
- `review-findings-ledger`: exhaustive first-pass, persisted findings ledger, and scoped re-review/re-judge contract for the 4R lenses and judgment-day, worded for both subagent and inline execution modes.

### Modified Capabilities
- None.

## Approach
- Codify the ledger contract in review-* and judgment-day agent assets, plus the inline-lens sections of orchestrator assets, replicated across all 13 adapter families.
- Ledger persistence follows the artifact store: OpenSpec file, Engram topic, or none (in-context only).
- Keep trigger-rule rendering behavior unchanged; update golden fixtures only if wording surfaces there.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `internal/assets/*/agents/review-{risk,resilience,readability,reliability}.md` | Modified | Add exhaustive-pass + ledger + scoped re-review contract (subagent variants). |
| `internal/assets/*/sdd-orchestrator*.md` | Modified | Inline-lens ledger contract for adapters without real subagents. |
| `internal/assets/claude/agents/jd-*.md`, `internal/assets/skills/judgment-day/` | Modified | Ledger + scoped re-judge. |
| `internal/components/sdd` | Modified | Injection/rendering + tests for ledger wording across variants. |
| `internal/testdata/golden` | Modified | Golden updates if rendered output changes. |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| One of 13 adapters left unfixed. | Med | Enumerate all asset families; add coverage per variant. |
| Inline vs subagent wording diverges. | Med | Single conceptual contract; assert both modes. |
| Exhaustive loop inflates review cost/time. | Med | Bounded "loop until N dry sweeps"; scoped re-review caps later rounds. |
| Ledger persistence mismatches artifact store. | Low | Contract branches on openspec/Engram/none explicitly. |

## Rollback Plan

1. Revert asset-normalization commit(s) for review-* , judgment-day, and orchestrator inline-lens sections.
2. Revert associated golden fixture and test updates from the same work unit.
3. File-level only; no data migration. Re-run targeted `internal/components/sdd` and golden tests to confirm baseline restored.

## Dependencies

- None blocking. Independent of the Engram and persona changes.

## Success Criteria

- [ ] Each 4R lens loops until dry on the first pass instead of a single read.
- [ ] First pass emits a persisted findings ledger honoring the session artifact store.
- [ ] Re-review verifies ledger findings and reviews only fix-touched lines; untouched-line findings are logged as a first-pass signal.
- [ ] Judgment-day uses the same ledger + scoped re-judge contract.
- [ ] Contract present across all 13 adapter variants and both execution modes.
- [ ] `go test ./...` and `go vet ./...` pass.
