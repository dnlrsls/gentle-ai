# Tasks: Persona canonical channel

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | 700-1100 (2 assets slimmed in place, 2 new residual assets, 4 output-style assets reconciled, ~8 test rewrites + 1 new 16-row table test + 1 new regression test, 5 e2e assertion blocks, 2 fingerprint regression tests, 5 goldens regenerated + 1 created, 1 doc correction) |
| 400-line budget risk | High |
| Chained PRs recommended | Yes (internal checkpoints; single PR accepted per delivery strategy) |
| Suggested split | Unit 1: RED test rewrites → Unit 2: reconciled output-style assets → Unit 3: residual persona assets → Unit 4: inject.go dispatch → Unit 5: fingerprint regression → Unit 6: e2e + docs → Unit 7: golden regen + token measurement |
| Delivery strategy | exception-ok |
| Chain strategy | size-exception |

Decision needed before apply: No
Chained PRs recommended: Yes
Chain strategy: size-exception
400-line budget risk: High

### Suggested Work Units

| Unit | Goal | Likely PR | Notes |
|------|------|-----------|-------|
| 1 | RED: all inject_test.go rewrites/renames + 4 call-site signature updates + new table test + Kimi superset test, all failing (compile failure is valid RED evidence) | Checkpoint 1 | Must fail/not-compile before Unit 2 touches any asset or inject.go |
| 2 | GREEN: reconciled output-style assets (Gentleman + Neutral, claude + kimi) | Checkpoint 2 | Decision 4; superset test passes |
| 3 | GREEN: 4 residual persona assets (2 slim-in-place, 2 new) | Checkpoint 3 | Decision 2, 3; Tables A/B/C |
| 4 | GREEN: inject.go residual predicate + personaContent signature + dispatch | Checkpoint 4 | Decision 1; Unit 1 tests now pass |
| 5 | GREEN: fingerprint regression tests | Checkpoint 5 | Decision 5 |
| 6 | GREEN: e2e_test.sh + docs/agents.md | Checkpoint 6 | JD-002 lowercase paths |
| 7 | REFACTOR: golden regeneration + token measurement + full verification | Checkpoint 7 | last, per design ordering |

Accepted delivery: single `size:exception` PR per session config (`exception-ok`); work units are internal checkpoints/rollback boundaries, not mandatory separate PRs.

## TDD Evidence Requirements

Strict TDD is active. Runner: `go test ./...`; targeted: `go test ./internal/components/persona/...`.
- RED evidence: failing test command + names + reason (missing signature/asset), captured BEFORE any asset or `inject.go` edit.
- GREEN evidence: targeted passing command per surface.
- REFACTOR evidence: `gofmt` on changed Go files, regenerate only design-specified goldens, re-run targeted + full tests.

## 1. Infrastructure (RED — write before any asset/inject.go change)

### 1.1 RED: rewrite `TestInjectClaudeGentlemanWritesSectionWithRealContent`
- [x] In `internal/components/persona/inject_test.go` (`:56-100`), flip assertions to expect CLAUDE.md has NO "Senior Architect"/"Persona Scope" and HAS residual markers (`## Rules`, `## Expertise`, "Persona Voice" pointer); move Language guardrails (`:85-99`) out of this test.
- [x] RED evidence: run, capture failure against current full asset.

### 1.2 RED: rename + rewrite Neutral residual test
- [x] Rename `TestInjectClaudeNeutralWritesFullPersonaWithoutRegionalLanguage` (`:276`) → `TestInjectClaudeNeutralWritesResidualPersonaWithoutRegionalLanguage`; flip assertion (`:294-296`) to expect residual markers (`## Rules`), not "Senior Architect".
- [x] RED evidence: run, capture failure.

### 1.3 RED: rewrite both auto-heal tests
- [x] `TestInjectClaudeAutoHealsStaleFreeTextPersona` (`:1308`, assertions `:1356-1369`): rewrite to assert residual markers post-heal inside the marker section, while the pre-marker legacy zone still carries historical "Senior Architect" text (Decision 5).
- [x] `TestInjectClaudeAutoHealStalePersonaOnlyFile` (`:1374`): same class of fix — post-heal assertions expect residual, not "Senior Architect".
- [x] RED evidence: run both, capture failures.

### 1.4 RED: Kimi Gentleman test — move Language assertions
- [x] `TestInjectKimiGentleman…` (`:102-171`): move `persona.md` Language assertions (`:159-170`) to assert absence; keep `output-style.md` assertions.
- [x] RED evidence: run, capture failure.

### 1.5 RED: rewrite `TestPersonaContentNonHermesNeutralUnchanged`
- [x] `inject_test.go:2058-2082`: remove `AgentClaudeCode` from the `agentIDs` list (verified list: `{AgentClaudeCode, AgentOpenCode, AgentGeminiCLI, AgentCursor, AgentCodex}` → drop Claude only; Kimi was never present, no change needed there).
- [x] RED evidence: run, capture failure.

### 1.6 RED: update 4 `personaContent` call sites to new signature
- [x] `inject_test.go:2006,2031,2052,2076`: update calls to `personaContent(agent, persona, residual bool)`. This breaks compilation until `inject.go` implements the new signature (Task 2.7) — compile failure is valid RED evidence per TDD Evidence Requirements.
- [x] RED evidence: `go build ./...` fails on these call sites; capture output.

### 1.7 RED: new 16-`AgentID` residual/full table test
- [x] Add table-driven test in `inject_test.go` enumerating all 16 `model.AgentID` constants (`AgentClaudeCode, AgentOpenCode, AgentKilocode, AgentGeminiCLI, AgentCursor, AgentVSCodeCopilot, AgentCodex, AgentAntigravity, AgentWindsurf, AgentKimi, AgentQwenCode, AgentKiroIDE, AgentOpenClaw, AgentPi, AgentTrae, AgentHermes`): Claude→residual, Kimi→residual, other 14→full, Neutral generic unchanged for non-{Claude,Kimi}.
- [x] RED evidence: run, capture failure (selection logic does not exist yet).

### 1.8 RED: `TestKimiOutputStyleSupersetOfLegacyKimiCopy`
- [x] New regression test asserting every line of pre-change `kimi/output-style-{gentleman,neutral}.md` exists in the reconciled Claude-derived text written for Kimi (verifies Decision 4 strict-subset claim).
- [x] RED evidence: run against unreconciled assets, capture failure.

## 2. Implementation (GREEN)

### 2.1 GREEN: reconciled output-style assets — Gentleman
- [x] `claude/output-style-gentleman.md`: add 3 artifact bullets to Persona Scope (Decision 4); merge Language Rules union table (persona `:39-46` ∪ style `:44-52`), including the single MERGED English-reply bullet (not concatenated).
- [x] `kimi/output-style-gentleman.md`: overwrite with the same reconciled text (adds the 2 missing determinism lines; Kimi content is a strict subset, nothing lost).
- [x] GREEN evidence: run 1.8's superset test — PASS.

### 2.2 GREEN: reconciled output-style assets — Neutral
- [x] `claude/output-style-neutral.md`: union neutral artifact bullets (`persona-neutral.md:32-34`) into Persona Scope; determinism/"Do not switch languages" lines already present verbatim (`:37-39,43`) — no duplication.
- [x] `kimi/output-style-neutral.md`: overwrite with reconciled text (adds 4 missing determinism lines + fixes reorder).
- [x] GREEN evidence: run 1.8's superset test (Neutral case) — PASS.

### 2.3 GREEN: `claude/persona-gentleman.md` → residual (Table A)
- [x] Slim in place: KEEP `## Rules` (all 11 bullets), `## Expertise`, `## Contextual Skill Loading (MANDATORY)`; MOVE Personality/Persona Scope/Language/Tone/Philosophy/Behavior (now covered by 2.1); ADD `## Persona Voice` pointer.
- [x] GREEN evidence: run 1.1 — PASS.

### 2.4 GREEN: `kimi/persona-gentleman.md` → residual (Table B)
- [x] Same disposition as A; KEEP `## Rules` (6 bullets, no CLI-tooling bullet) and `## Kimi-native notes` (`:71-75`); MOVE Language; ADD Kimi-worded pointer.
- [x] GREEN evidence: run 1.4 — PASS.

### 2.5 GREEN: create `claude/persona-neutral-residual.md` (Table C)
- [x] New file, NOT a slim of `generic/persona-neutral.md` (Decision 2 — shared by 13 other adapters, stays unchanged). KEEP `## Rules`, `## Expertise`, `## Contextual Skill Loading`; ADD Claude-worded pointer.
- [x] GREEN evidence: run 1.2 — PASS.

### 2.6 GREEN: create `kimi/persona-neutral-residual.md` (Table C)
- [x] Same content as 2.5, Kimi-worded pointer; `generic/persona-neutral.md` remains untouched.
- [x] GREEN evidence: extend Kimi table coverage (1.7) — PASS.

### 2.7 GREEN: `inject.go` residual predicate + `personaContent` signature
- [x] Add `residualChannel(adapter) bool = adapter.SupportsOutputStyles() || adapter.Agent() == model.AgentKimi` at call site `inject.go:79`.
- [x] Change `personaContent(agent, persona)` → `personaContent(agent, persona, residual bool)` (`:495`); inside switch, when `residual`, dispatch to the 4 new/slimmed assets per Decision 1's table (Gentleman: `claude/persona-gentleman.md` residual, `kimi/persona-gentleman.md` residual; Neutral: `claude/persona-neutral-residual.md`, `kimi/persona-neutral-residual.md`).
- [x] GREEN evidence: run 1.1, 1.2, 1.3, 1.4, 1.5, 1.6, 1.7 — all PASS.

### 2.8 GREEN: fingerprint regression tests
- [x] `filemerge/section_test.go`: add case asserting a slim on-disk install is NOT falsely legacy-stripped (pre-marker zone fingerprints unchanged, per Decision 5).
- [x] `uninstall/cleaners_test.go`: add case asserting a slim on-disk install is fully removed via the marker path, not the prefix fingerprint.
- [x] GREEN evidence: both new cases PASS; existing literal fixtures (`section_test.go:333,351,368,385,609`) still PASS unchanged.

## 3. Testing/Verification

### 3.1 GREEN: `e2e/e2e_test.sh` assertion updates
- [x] `test_cc_persona_gentleman` (`:552`) and `test_cc_persona_neutral` (`:574`): replace "CLAUDE.md contains Senior Architect" assertion with (a) CLAUDE.md residual contains the pointer text, (b) `~/.claude/output-styles/{gentleman,neutral}.md` (lowercase paths, per JD-002) carries the tone content.
- [x] `test_edge_persona_switch` (`:1541,1547`): same rewrite for both persona-switch assertions.
- [x] Evidence: e2e run passes locally (or documented as deferred to CI per project e2e gating).

### 3.2 GREEN: `docs/agents.md` Kimi correction
- [x] `docs/agents.md:20`: Kimi Output Styles `No` → `Via KIMI.md include`, with footnote noting no `settings.json` `outputStyle` mechanism but `KIMI.md` includes `output-style.md` as the canonical tone channel.
- [x] Evidence: `rg -n "Output Styles" docs/agents.md` shows corrected row.

### 3.3 REFACTOR: golden regeneration (LAST)
- [x] Verify base state of `testdata/golden/combined-claude-claudemd.golden` first (regenerated by `engram-protocol-dedup` earlier today — confirm `git log` shows that commit before diffing).
- [x] Regenerate `persona-claude-gentleman.golden`, `persona-claude-neutral.golden`, `persona-claude-gentleman-outputstyle.golden`, `combined-claude-claudemd.golden` (slim residual + reconciled style).
- [x] Create `persona-claude-neutral-outputstyle.golden` (locks reconciled neutral style; none exists today — accepted per design File Changes table).
- [x] Assert byte-stable (no regen): `combined-windsurf-global-rules.golden`, `persona-{antigravity,kiro,windsurf}-gentleman.golden`, `persona-opencode-{gentleman,neutral}.golden`.
- [x] Evidence: `go test ./... -run <persona goldens> -update`, then re-run without `-update` — PASS; `git diff --stat` on byte-stable goldens is empty.

### 3.4 Token measurement (chars/4 method, Decision 6)
- [x] Record actual removed-chars / style-growth-offset / net / ≈tokens for Claude Gentleman, Kimi Gentleman, Claude Neutral (and Kimi Neutral ≈ Claude Neutral) against the FINAL committed assets; log in apply-progress alongside the design's projected table (~3,110/~2,650/~2,880 net chars).
- [x] Evidence: measurement method and numbers recorded in apply-progress notes.

### 3.5 Full required verification
- [x] Run `go test ./...` — all packages `ok`, 0 `FAIL`.
- [x] Run `go vet ./...` — clean, no output.
- [x] Compare final implementation against every scenario in `openspec/changes/persona-canonical-channel/specs/persona-behavior-contract/spec.md` — confirm all 7 requirements (5 ADDED, 1 MODIFIED, plus scenarios) are satisfied.
