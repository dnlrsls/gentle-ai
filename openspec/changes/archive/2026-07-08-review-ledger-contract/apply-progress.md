# Apply Progress: Review findings ledger contract

**Change**: review-ledger-contract
**Mode**: Strict TDD
**Batch**: 1 of 1 (first apply batch; no prior apply-progress existed)

## Summary

Implemented the full review ledger contract in one batch, covering all 5 work
units from `tasks.md`: RED test + canonical `_shared` source, 16 review-*
subagent assets, 13 orchestrator variants (12 source files + the
Kilocode-rendered variant), 8 judgment-day assets, and golden regeneration.
All tasks (1.1 through 3.5) are complete.

## TDD Cycle Evidence

| Task | Test File | Layer | Safety Net | RED | GREEN | TRIANGULATE | REFACTOR |
|------|-----------|-------|------------|-----|-------|-------------|----------|
| 1.1 | `internal/components/sdd/review_ledger_contract_test.go` | Unit (embedded-asset content) | N/A (new file) | ✅ Written — table-driven `TestRequiredLedgerClauses` over 16 review-* + 6 jd-* + 12 orchestrator + kilocode/opencode-render + 2 skill-doc groups | ✅ Passed after all asset edits | ✅ See 3.1/3.2 | ➖ See 2.11 |
| 1.2 | same file, `TestReviewLedgerContractSchema` | Unit (golden-string fixture) | N/A (new) | ✅ Written — asserted against not-yet-existing `_shared/review-ledger-contract.md` | ✅ Passed after authoring the file | ➖ Single fixture, no triangulation needed (structural/schema check) | ➖ None needed |
| 1.3 | `internal/assets/skills/_shared/review-ledger-contract.md` | N/A (canonical prose source) | N/A (new) | ✅ (driven by 1.2's RED) | ✅ `TestReviewLedgerContractSchema` PASS | N/A | ✅ Rewrote canonical block as unwrapped single-line paragraphs after first draft's hard-wrapped prose broke exact-substring matches |
| 2.1 | claude review-{risk,readability,reliability,resilience}.md | Unit | N/A (new section) | ✅ (from 1.1) | ✅ `TestRequiredLedgerClauses/subagent_review_and_jd_assets/claude` PASS (4/4) | ✅ 4 distinct lens files, same clause slice | ➖ None needed — identical block by design |
| 2.2 | cursor review-{risk,readability,reliability,resilience}.md | Unit | N/A | ✅ | ✅ `.../cursor` PASS (4/4) | ✅ | ➖ None needed |
| 2.3 | kimi review-{risk,readability,reliability,resilience}.md | Unit | N/A | ✅ | ✅ `.../kimi` PASS (4/4) | ✅ | ➖ None needed |
| 2.4 | kiro review-{risk,readability,reliability,resilience}.md | Unit | N/A | ✅ | ✅ `.../kiro` PASS (4/4); full subagent group PASS | ✅ 16/16 subagent-mode assets independently covered | ➖ None needed |
| 2.5 | codex, gemini, qwen, windsurf sdd-orchestrator.md | Unit | N/A | ✅ | ✅ `TestRequiredLedgerClauses/orchestrator_assets/{codex,gemini,qwen,windsurf}` PASS | ✅ Inline-mode clause distinct from subagent-mode clause | ➖ None needed |
| 2.6 | antigravity, hermes, generic sdd-orchestrator.md | Unit | N/A | ✅ | ✅ PASS (3/3) | ✅ | ➖ None needed |
| 2.7 | opencode/sdd-orchestrator.md + real `Inject()` render for OpenCode and Kilocode | Unit + Integration (real inject path) | N/A | ✅ `TestRequiredLedgerClauses/opencode` and `/kilocode` failed on first run with `open .../opencode.json: no such file or directory` (initial test used the wrong `SettingsPath`/mode; fixed to use `adapter.SettingsPath(home)` + `model.SDDModeMulti`, matching existing OpenCode/Kilocode test patterns) | ✅ Both PASS after editing the shared source; confirmed via `sddOrchestratorAsset()` that no `inject.go` logic change was needed — Kilocode already maps to the same source file | ✅ Rendered output for 2 distinct adapters from 1 source file | ➖ None needed |
| 2.8 | claude, cursor, kimi, kiro sdd-orchestrator.md (merge-side clause) | Unit | N/A | ✅ (same edits as orchestrator group) | ✅ `TestRequiredLedgerClauses/orchestrator_assets` full group PASS — 13/13 variants covered | ✅ Merge-mode clause distinct from inline-mode clause, asserted per family | ➖ Checked `sdd-orchestrator-workflow.md` for review-lens wording — none found, left untouched per task's conditional |
| 2.9 | claude/kiro jd-{judge-a,judge-b,fix-agent}.md | Unit | N/A | ✅ | ✅ 6/6 jd-* files PASS | ✅ | ➖ None needed |
| 2.10 | skills/judgment-day/SKILL.md, references/prompts-and-formats.md | Unit | N/A | ✅ | ✅ 2/2 PASS; all 24 non-shared assets now covered | ✅ | ➖ None needed |
| 2.11 | review_ledger_contract_test.go (helper extraction) | N/A (refactor) | ✅ `go test ./internal/components/sdd/...` full package PASS before refactor claim | N/A (refactor task) | ✅ `assertContainsClauses`/`assertTextContainsClauses` already factored from the first RED commit — no further duplication found | N/A | ✅ `gofmt -l` clean; `go test ./internal/components/sdd/...` PASS after |
| 3.1 | (cross-cutting) | Unit | N/A | N/A | N/A | ✅ Subagent group and orchestrator group are independent subtests over disjoint file sets with distinct mode-clause assertions | N/A |
| 3.2 | (cross-cutting) | Unit | N/A | N/A | N/A | ✅ All 3 persistence-branch clauses asserted in every orchestrator asset (exceeds "at least one per mode") | N/A |
| 3.3 | `internal/components/golden_test.go` (separate package) | Golden | ✅ `TestRenderTriggerRules_Golden` PASS, no drift (untouched code path) | ✅ 13 golden tests failed after asset edits (expected — rendered output changed) | ✅ Regenerated exactly the 13 affected fixtures with `-update`; re-ran without `-update` — all PASS | N/A | ✅ Reviewed every diff: clean additive insertion only, no stale text |
| 3.4 | full repo | Full suite | N/A | N/A | ✅ `go test ./...` — 52 packages `ok`, 0 `FAIL`; `go vet ./...` clean | N/A | N/A |
| 3.5 | (process) | N/A | N/A | N/A | ✅ Diff size confirmed (~1,600+ lines); `size:exception` honored; 5 work units remain independently revertible | N/A | N/A |

## Test Summary

- **Total tests written**: 2 new top-level test functions (`TestRequiredLedgerClauses` with 40+ subtests, `TestReviewLedgerContractSchema`), plus 2 helper functions (`assertContainsClauses`, `assertTextContainsClauses`) and 1 render-path helper (`readGentleOrchestratorPrompt`).
- **Total tests passing**: all subtests green on final run (`go test ./internal/components/sdd/... -run 'TestRequiredLedgerClauses|TestReviewLedgerContractSchema' -v` — full PASS); full repo `go test ./...` — 52/52 packages `ok`.
- **Layers used**: Unit (embedded-asset string matching) — bulk of coverage; Integration (real `Inject()` render path for OpenCode/Kilocode against `opencode.json`) — 2 subtests; Golden (whole-file rendered fixture regeneration in `internal/components`) — 13 fixtures.
- **Approval tests** (refactoring): None — no existing behavior was refactored; all changes are additive prose insertions into static assets plus one new test file.
- **Pure functions created**: `assertContainsClauses`, `assertTextContainsClauses`, `readGentleOrchestratorPrompt` — all pure/deterministic given embedded FS content or a rendered settings file.

## Files Changed

| File | Action | Lines | What Was Done |
|------|--------|-------|----------------|
| `internal/components/sdd/review_ledger_contract_test.go` | Created | 286 | RED test: `requiredLedgerClauses` slice (18 clauses), 4 mode-clause constants, table-driven `TestRequiredLedgerClauses` (subagent/orchestrator/opencode/kilocode/judgment-day groups), `TestReviewLedgerContractSchema` golden fixture test. |
| `internal/assets/skills/_shared/review-ledger-contract.md` | Created | 91 | Canonical authoring source: exhaustive first-pass, ledger schema, persistence branches, scoped re-review, two execution-mode clauses, adoption notes. |
| `internal/assets/{claude,cursor,kimi,kiro}/agents/review-{risk,readability,reliability,resilience}.md` | Modified (16) | +26 lines each | Added "## Review ledger contract" section after the existing Output contract, with subagent-mode execution clause. |
| `internal/assets/{claude,kiro}/agents/jd-{judge-a,judge-b,fix-agent}.md` | Modified (6) | +26 lines each | Added "## Review ledger contract" section with JD subagent-mode execution clause; fix-agent additionally notes setting `status=fixed` on addressed ids. |
| `internal/assets/{claude,cursor,kimi,kiro,codex,gemini,qwen,windsurf,antigravity,hermes,generic,opencode}/sdd-orchestrator.md` | Modified (12) | +26 lines each | Added "#### Review Execution Contract" section after "Review Lens Selection"; merge-mode clause for the 4 subagent-family adapters, inline-mode clause for the 8 others. Kilocode covered via the shared `opencode/sdd-orchestrator.md` source — no `inject.go` change needed. |
| `internal/assets/skills/judgment-day/SKILL.md` | Modified | +26 | Added "## Ledger and Re-Judge Contract" section before "## References". |
| `internal/assets/skills/judgment-day/references/prompts-and-formats.md` | Modified | +26 | Added "## Ledger and Re-Judge Contract" section before "## Language Snippets". |
| `openspec/changes/review-ledger-contract/tasks.md` | Modified | — | All `[ ]` marked `[x]` with GREEN/evidence notes inline. |
| `testdata/golden/{sdd-claude-claudemd,sdd-codex-agentsmd,sdd-codex-agentsmd-lowcost,sdd-codex-agentsmd-powerful,sdd-cursor-rules,sdd-gemini-geminimd,sdd-vscode-instructions,sdd-windsurf-global-rules,sdd-kiro-instructions,sdd-antigravity-rulesmd,sdd-opencode-multi-settings,combined-claude-claudemd,combined-windsurf-global-rules}.golden` | Regenerated (13) | +26 lines each (or JSON-embedded equivalent) | Regenerated via `-update` after confirming drift was purely additive (the new Review Execution Contract prose). `trigger-rules` goldens confirmed untouched. |

## Deviations from Design

1. **inject.go (task 2.7)**: Design anticipated an `inject.go` code change for the OpenCode/Kilocode path. Investigation of `sddOrchestratorAsset()` showed `AgentOpenCode` and `AgentKilocode` already resolve to the same `opencode/sdd-orchestrator.md` source (confirmed by the pre-existing `TestSDDOrchestratorAssetSelectionCoversSupportedAgents` test), and `inlineOpenCodeSDDPrompts` already reads that single source for both adapters. No functional `inject.go` change was required — editing the one source file was sufficient. Added integration-style test coverage (`TestRequiredLedgerClauses/opencode` and `/kilocode`) that exercises the real `Inject()` render path to prove this end-to-end rather than relying on a static file read.
2. **`sdd-orchestrator-workflow.md` (task 2.8)**: The task conditionally asked to update this file "if it carries phase-delegation wording" relevant to review. Checked — it contains no review-lens content, so it was left untouched.
3. **Wording drift during authoring**: The first draft of `_shared/review-ledger-contract.md` used conventional hard-wrapped markdown prose, which broke several exact-substring clause checks (a wrapped line has a newline where the test expects a space). Rewrote the canonical block as unwrapped single-line paragraphs — this is now the pattern hand-copied verbatim into all 24 adopting assets, so the same pitfall did not recur there.

## Issues Found

None blocking. One infrastructure note: the sandbox had no Go toolchain preinstalled; installed via `brew install go` (Homebrew, go1.26.5) before any test could run, per the project's `go.mod` requirement (`go 1.25.10`).

## Status

All tasks (1.1–3.5) complete. `go test ./...` — 52 packages `ok`, 0 `FAIL`. `go vet ./...` — clean. Ready for `sdd-verify`.
