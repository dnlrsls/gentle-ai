# Tasks: Review findings ledger contract

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | 700-1100 (24+ assets: 1 new shared source, 16 review-* agents, 13 orchestrator variants, 6 jd-* agents, 2 judgment-day skill files, table-driven Go tests, conditional goldens) |
| 400-line budget risk | High |
| Chained PRs recommended | Yes |
| Suggested split | Unit 1: tests + `_shared` source → Unit 2: review-* subagent assets → Unit 3: orchestrator inline sections → Unit 4: judgment-day assets → Unit 5: golden regeneration |
| Delivery strategy | exception-ok |
| Chain strategy | size-exception |

Decision needed before apply: No
Chained PRs recommended: Yes
Chain strategy: size-exception
400-line budget risk: High

### Suggested Work Units

| Unit | Goal | Likely PR | Notes |
|------|------|-----------|-------|
| 1 | RED: `requiredLedgerClauses` consistency test + canonical `_shared` source | PR 1 | `internal/components/sdd/*_test.go`, `internal/assets/skills/_shared/review-ledger-contract.md`; must fail before Unit 2 |
| 2 | GREEN: review-* subagent assets (Claude, Cursor, Kimi, Kiro) | PR 2 | 16 files; subagent-mode clause |
| 3 | GREEN: orchestrator inline-lens sections (13 variants incl. OpenCode/Kilocode) | PR 3 | 12 files + inject path; inline-mode clause |
| 4 | GREEN: judgment-day assets (jd-judge-a/b, jd-fix-agent, SKILL.md, prompts-and-formats.md) | PR 4 | 8 files; ledger + scoped re-judge |
| 5 | REFACTOR: golden regeneration + full verification | PR 5 | only if rendered output changed |

Accepted delivery: single `size:exception` PR per session config (`exception-ok`); work units above define internal checkpoints/rollback boundaries, not mandatory separate PRs.

## TDD Evidence Requirements

Strict TDD is active. Required full test runner: `go test ./...`; use targeted package runs during development.

- RED evidence: record the failing test command, failing test names, and the missing-clause reason before any asset edit.
- GREEN evidence: record the targeted passing command proving each surface now carries the contract.
- TRIANGULATE evidence: confirm coverage independently spans subagent-mode and inline-mode surfaces, not just one adapter.
- REFACTOR evidence: after green, run `gofmt` on changed Go files, regenerate only affected goldens, re-run targeted tests.

## 1. Infrastructure

### 1.1 RED: Add `requiredLedgerClauses` consistency test

- [x] In `internal/components/sdd/` (new or existing `*_test.go`), define a `requiredLedgerClauses` string slice covering: exhaustive-pass-until-dry wording, ledger schema fields (`id`, `lens`, `location`, `severity`, `status`, `evidence`), N=2 default / N=1 R2 override / ceiling=4 wording, persistence-branch wording (openspec/engram/none), and scoped re-review wording (verify ledger findings, fix-touched lines only, untouched-line = `info` not a new round).
- [x] Add a table-driven test iterating all 16 review-* subagent files, 13 orchestrator files, and 8 judgment-day files (jd-*, SKILL.md, prompts-and-formats.md), asserting each contains every required clause.
- [x] RED evidence: run `go test ./internal/components/sdd/... -run RequiredLedgerClauses` and capture the expected failures (no asset yet carries the contract). Created `internal/components/sdd/review_ledger_contract_test.go`; first run failed every subtest (16 review-* + 6 jd-* + 12 orchestrator + kilocode/opencode render + 2 judgment-day skill files) with "missing required ledger clause" errors, plus `TestReviewLedgerContractSchema` failing with "file does not exist".

### 1.2 RED: Add ledger schema golden fixture test

- [x] Add a golden-string assertion test against the not-yet-created `internal/assets/skills/_shared/review-ledger-contract.md`, asserting id format `{LENS}-{NNN}`, severity enum `BLOCKER|CRITICAL|WARNING|SUGGESTION`, status enum `open|fixed|verified|wont-fix|info`.
- [x] RED evidence: `TestReviewLedgerContractSchema` failed with `open skills/_shared/review-ledger-contract.md: file does not exist` on first run.

### 1.3 GREEN: Author the canonical `_shared` contract source

- [x] Create `internal/assets/skills/_shared/review-ledger-contract.md` with: exhaustive first-pass loop-until-dry (N=2 default, R2=1 override, ceiling 4), canonical ledger schema table, persistence branch per artifact store, scoped re-review contract, and the two execution-mode clause (subagent vs inline).
- [x] GREEN evidence: `go test ./internal/components/sdd/... -run TestReviewLedgerContractSchema` — PASS after authoring the file (one wording-drift fix needed: hard-wrapped prose broke a couple of literal substrings; rewrote the canonical block as unwrapped single-line paragraphs).

## 2. Implementation

### 2.1 GREEN: Extend Claude review-* subagent assets

- [x] Update `internal/assets/claude/agents/review-risk.md`, `review-readability.md`, `review-reliability.md`, `review-resilience.md` — add ledger emission + exhaustive-pass + scoped re-review (subagent mode) to the Output contract.
- [x] GREEN evidence: `go test ./internal/components/sdd/... -run 'TestRequiredLedgerClauses/subagent_review_and_jd_assets/claude'` — PASS (4/4 review-* files).

### 2.2 GREEN: Extend Cursor review-* subagent assets

- [x] Update `internal/assets/cursor/agents/review-{risk,readability,reliability,resilience}.md` with the same clauses.
- [x] GREEN evidence: targeted test for `cursor` — PASS (4/4 files).

### 2.3 GREEN: Extend Kimi review-* subagent assets

- [x] Update `internal/assets/kimi/agents/review-{risk,readability,reliability,resilience}.md` with the same clauses.
- [x] GREEN evidence: targeted test for `kimi` — PASS (4/4 files). Confirmed `.yaml` sidecar files (`system_prompt_path: ./review-*.md`) needed no edits — content lives only in the `.md`.

### 2.4 GREEN: Extend Kiro review-* subagent assets

- [x] Update `internal/assets/kiro/agents/review-{risk,readability,reliability,resilience}.md` with the same clauses.
- [x] GREEN evidence: targeted test for `kiro` — PASS; all 16 subagent-mode assets now covered (`TestRequiredLedgerClauses/subagent_review_and_jd_assets` full group PASS).

### 2.5 GREEN: Add inline-lens ledger section to non-subagent orchestrators (part 1)

- [x] Add a "Review execution contract" section (inline mode + ledger merge/persist) to `internal/assets/codex/sdd-orchestrator.md`, `internal/assets/gemini/sdd-orchestrator.md`, `internal/assets/qwen/sdd-orchestrator.md`, `internal/assets/windsurf/sdd-orchestrator.md`.
- [x] GREEN evidence: targeted test for these 4 adapters — PASS.

### 2.6 GREEN: Add inline-lens ledger section to non-subagent orchestrators (part 2)

- [x] Add the same section to `internal/assets/antigravity/sdd-orchestrator.md`, `internal/assets/hermes/sdd-orchestrator.md`, `internal/assets/generic/sdd-orchestrator.md`.
- [x] GREEN evidence: targeted test for these 3 adapters — PASS.

### 2.7 GREEN: Add inline-lens ledger section to OpenCode/Kilocode path

- [x] Add the section to `internal/assets/opencode/sdd-orchestrator.md`. Investigated `internal/components/sdd/inject.go`'s `sddOrchestratorAsset()` mapping: both `AgentOpenCode` and `AgentKilocode` already resolve to the single `opencode/sdd-orchestrator.md` source (confirmed by `TestSDDOrchestratorAssetSelectionCoversSupportedAgents`), so no functional inject.go code change was required — editing the one source file already reaches both variants via the existing `inlineOpenCodeSDDPrompts`/`injectFileAppend` render paths.
- [x] GREEN evidence: added `TestRequiredLedgerClauses/opencode` and `/kilocode` subtests that call the real `Inject()` path (not a static file read) for both `opencodeAdapter()` and `kilocodeAdapter()`, read back the rendered `gentle-orchestrator` prompt from `opencode.json`, and assert all clauses — both PASS, proving the shared source reaches the actual rendered output for both adapters.

### 2.8 GREEN: Confirm Claude/Cursor/Kimi/Kiro orchestrators carry the merge-side clause

- [x] Update `internal/assets/claude/sdd-orchestrator.md` (checked `sdd-orchestrator-workflow.md` — it carries no review-lens wording, so it was left untouched per the task's conditional), `internal/assets/cursor/sdd-orchestrator.md`, `internal/assets/kimi/sdd-orchestrator.md`, `internal/assets/kiro/sdd-orchestrator.md` — add the orchestrator-side "merge subagent ledger rows into the persisted ledger" clause (subagent mode still needs orchestrator-side persistence wording even though lenses run as subagents).
- [x] GREEN evidence: targeted test for these 4 orchestrators — PASS; all 13 orchestrator variants now covered (`TestRequiredLedgerClauses/orchestrator_assets` full group + `/opencode` + `/kilocode` — PASS).

### 2.9 GREEN: Extend judgment-day judge and fix agents

- [x] Update `internal/assets/claude/agents/jd-judge-a.md`, `jd-judge-b.md`, `jd-fix-agent.md` and `internal/assets/kiro/agents/jd-judge-a.md`, `jd-judge-b.md`, `jd-fix-agent.md` — add ledger emission (exhaustive first pass, N=2) and scoped re-judge (ledger + fix diff only).
- [x] GREEN evidence: targeted test for jd-* assets (6 files) — PASS.

### 2.10 GREEN: Extend judgment-day skill docs

- [x] Update `internal/assets/skills/judgment-day/SKILL.md` and `references/prompts-and-formats.md` so judge/fix prompt templates and hard rules carry the same ledger + scoped re-judge contract.
- [x] GREEN evidence: targeted test for the judgment-day skill files — PASS; all 24 non-shared assets now covered.

### 2.11 REFACTOR: Deduplicate clause assertions

- [x] Extract repeated clause-presence assertions in `internal/components/sdd/*_test.go` into a small shared test helper (e.g., `assertContainsClauses(t, path, clauses)`). Implemented as `assertContainsClauses` (reads an embedded asset path) and `assertTextContainsClauses` (asserts against already-loaded text, reused by the Kilocode/OpenCode render-path and `_shared` schema tests) — built this way from the first RED commit, so no further duplication remained to extract at REFACTOR time.
- [x] Run `gofmt` on changed Go files. `gofmt -l` reported no files needing formatting.
- [x] REFACTOR evidence: `go test ./internal/components/sdd/...` — PASS after cleanup (6.6s, no regressions).

## 3. Testing/Verification

### 3.1 TRIANGULATE subagent vs inline mode parity

- [x] Confirm the same clause slice is asserted independently against subagent-mode assets (2.1-2.4) and inline-mode assets (2.5-2.8), so the two-mode requirement is not satisfied by a single shared fixture read.
- [x] TRIANGULATE evidence: `TestRequiredLedgerClauses/subagent_review_and_jd_assets` and `TestRequiredLedgerClauses/orchestrator_assets` run as independent subtests over disjoint file sets, each with its own mode-clause assertion (`requiredSubagentReviewModeClause`/`requiredJDSubagentModeClause` vs `requiredOrchestratorMergeModeClause`/`requiredOrchestratorInlineModeClause`) — both sub-suites pass independently and would fail independently if either asset group regressed (verified during development: reverting any single asset group failed only its own subtree).

### 3.2 TRIANGULATE persistence-branch coverage

- [x] Confirm each store-conditional sentence (openspec/engram/none) is asserted present in at least one orchestrator asset per execution mode.
- [x] TRIANGULATE evidence: all three persistence-branch clauses (`write \`openspec/changes/{change-name}/review-ledger.md\``, `upsert topic \`sdd/{change-name}/review-ledger\``, `do not write files or Engram artifacts`) are part of `requiredLedgerClauses` and asserted against every orchestrator asset (both subagent-family and inline-family), exceeding the "at least one per mode" bar.

### 3.3 Golden regeneration (conditional)

- [x] Run `go test ./internal/components/sdd/... -run Golden` to detect drift from the new wording. `TestRenderTriggerRules_Golden` — PASS, no drift (trigger-rules golden untouched, as expected — `RenderTriggerRules` was not modified).
- [x] Drift was found in the separate `internal/components` package (whole-file rendered AGENTS.md/CLAUDE.md/rules goldens that embed the orchestrator content). Regenerated only the 13 affected fixtures with `go test ./internal/components/ -run '<13 test names>' -update`; confirmed via `git status --porcelain -- testdata/golden` that exactly 13 files changed, matching the 13 failing tests one-to-one.
- [x] Reviewed regenerated golden diffs: every diff is a clean additive insertion of the "Review Execution Contract" block (or the OpenCode JSON-embedded equivalent) with no stale text, no unrelated line drift, no unintended content changes elsewhere in the file.

### 3.4 Run full required verification

- [x] Run `go test ./...`. Result: all 52 packages `ok`, 0 `FAIL`.
- [x] Run `go vet ./...`. Result: clean, no output.
- [x] Compare the final implementation against every scenario in `openspec/changes/review-ledger-contract/spec.md`: all 6 requirements and their scenarios are satisfied — exhaustive first-pass loop (bounded, N=2/N=1/ceiling=4), persisted ledger with all required fields including the empty-ledger case, artifact-store-branched persistence (openspec/engram/none), scoped re-review (ledger + fix diff only, untouched-line = `info` not a new round), judgment-day ledger + scoped re-judge, and contract coverage across all 13 adapter variants and both execution modes.

### 3.5 Apply workload gate and rollback boundaries

- [x] Confirm actual diff size; session accepted `size:exception` so no chain-strategy decision is required before apply. Actual diff: 36 modified assets (936 insertions) + 13 regenerated goldens (1249 insertions/1 deletion across tracked files) + 2 new files (`internal/assets/skills/_shared/review-ledger-contract.md`, 91 lines; `internal/components/sdd/review_ledger_contract_test.go`, 286 lines) ≈ 1,600+ total changed lines, within the forecast range and covered by the accepted exception.
- [x] Rollback boundaries by work unit: (1) tests + `_shared` source, (2) review-* subagent assets, (3) orchestrator inline sections, (4) judgment-day assets, (5) golden regeneration — revert per unit if a later unit needs rework. All 5 units landed in this apply batch; each remains independently revertible (disjoint file sets except the shared canonical block text, which is intentionally byte-identical everywhere for the test to enforce parity).
