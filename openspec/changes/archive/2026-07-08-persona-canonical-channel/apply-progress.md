# Apply Progress: Persona canonical channel

**Mode**: Strict TDD
**Batch**: 1 (first and only batch — all 21 tasks / 52 sub-checkboxes complete)
**Status**: 52/52 sub-items complete. Ready for verify.

## Summary

Implemented the canonical-channel design end to end: Claude Code and Kimi
system-prompt persona sections are now residuals (tooling/action directives +
a pointer), the drifted Gentleman/Neutral output-style wording is reconciled
into one canonical union per variant, Kimi's output-style copies are
overwritten to be byte-identical to Claude's reconciled text, legacy
fingerprints are re-validated (kept unchanged, per Decision 5), e2e assertions
and docs/agents.md are corrected, and goldens are regenerated/created per the
design's File Changes table.

## Deviation from design (documented, not silent)

**Decision 1's Implementation paragraph vs. File Changes/tasks.md conflict for
Gentleman residual filenames.** design.md's Decision 1 dispatch table lists
`claude/persona-gentleman-residual.md` / `kimi/persona-gentleman-residual.md`
as new residual asset filenames. This directly conflicts with the same
document's **File Changes** table (`claude/persona-gentleman.md` /
`kimi/persona-gentleman.md` marked **Modify**, not Create) and with
tasks.md 2.3/2.4 ("Slim in place"). I followed the **File Changes table +
tasks.md** (the more specific, doubly-corroborated instruction) and did
**NOT** create `*-gentleman-residual.md` files — `claude/persona-gentleman.md`
and `kimi/persona-gentleman.md` are slimmed in place and serve both the
residual and (in practice, unreachable for Claude/Kimi) full-dispatch code
paths, since `residualChannel()` evaluates `true` unconditionally for both
adapters today. Neutral correctly uses new `*-neutral-residual.md` files per
both parts of the design (no conflict there).

**Unlisted test-suite scope**: `internal/assets/assets_test.go` and
`internal/assets/language_contract_test.go` contain additional
language/tone/voice assertions directly against `claude/persona-gentleman.md`
and `kimi/persona-gentleman.md` that were not enumerated in tasks.md or the
review ledger. These are a real consequence of correctly implementing the
design (the residual no longer carries tone content) and were fixed using the
same "combined persona-residual + output-style channel" principle the delta
spec's MODIFIED "Generic Neutral Asset Parity" requirement already establishes
for Neutral — extended here to the equivalent Gentleman assertions. 4 test
functions updated: `TestGentlemanLanguageInstructionsDoNotBiasEnglishSessions`,
`TestClaudeGentlemanPersonaPreventsEnglishGreetingCodeSwitching`,
`TestManagedDirectReplyAssetsEnforceEnglishNoCodeSwitching`,
`TestGentlemanPersonaKeepsDirectConversationVoice`.

**Merged-bullet ripple**: Decision 4/JD-013's merge of the two near-duplicate
"reply fully in English" bullets changed the exact wording of that one line in
`claude/output-style-gentleman.md` (and, via the byte-identical overwrite,
`kimi/output-style-gentleman.md`). This broke 2 additional pre-existing
literal-string assertions outside the enumerated test scope
(`TestInjectKimiGentlemanIncludesProjectInstructionsAndLoadedSkills` in
inject_test.go, and the output-style-guardrail check inside
`TestGentlemanLanguageInstructionsDoNotBiasEnglishSessions`). Both were updated
to assert the merged bullet's surviving normative substring instead of the old
verbatim line — no normative content was lost, confirmed by
`TestKimiOutputStyleSupersetOfLegacyKimiCopy`'s explicit merged-bullet check.

## TDD Cycle Evidence

| Task | Test File | Layer | Safety Net | RED | GREEN | TRIANGULATE | REFACTOR |
|------|-----------|-------|------------|-----|-------|-------------|----------|
| 1.1 | `internal/components/persona/inject_test.go` (`TestInjectClaudeGentlemanWritesSectionWithRealContent`) | Unit | N/A (rewrite) | ✅ Written — new residual-marker assertions against full asset | ✅ Passed after 2.3/2.7 | ➖ Single scenario (residual dispatch) | ✅ Clean |
| 1.2 | same file (`TestInjectClaudeNeutralWritesResidualPersonaWithoutRegionalLanguage`, renamed) | Unit | N/A (rewrite) | ✅ Written | ✅ Passed after 2.5/2.7 | ➖ Single | ✅ Clean |
| 1.3 | same file (2 auto-heal tests) | Unit | ✅ baseline `go test ./internal/components/filemerge/... ./internal/components/uninstall/...` = ok before any asset change | ✅ Written | ✅ Passed after 2.3/2.7 | ✅ 2 tests, 2 fixture shapes (SDD-section preserved vs. persona-only file) | ✅ Clean |
| 1.4 | same file (`TestInjectKimiGentlemanIncludesProjectInstructionsAndLoadedSkills`) | Unit | N/A (rewrite) | ✅ Written | ✅ Passed after 2.4/2.7 | ➖ Single | ✅ Clean |
| 1.5 | same file (`TestPersonaContentNonHermesNeutralUnchanged`) | Unit | N/A (rewrite) | ✅ Written | ✅ Passed after 1.6/2.7 | ✅ 4 remaining agent cases | ✅ Clean |
| 1.6 | same file (4 call sites) | Unit (compile gate) | N/A | ✅ `go vet ./internal/components/persona/...` failed: "too many arguments in call to personaContent" | ✅ Passed after 2.7 | N/A | N/A |
| 1.7 | same file (`TestPersonaContentResidualDispatchAllAgents`, new) | Unit (table, 16 agents × 2 personas = 32 subtests) | N/A (new) | ✅ Written (compile-blocked until 2.7) | ✅ All 32 subtests passed after 2.7 | ✅ 32 cases (16 agents × gentleman/neutral) | ✅ Clean |
| 1.8 | same file (`TestKimiOutputStyleSupersetOfLegacyKimiCopy`, new) | Unit (regression, frozen fixture) | N/A (new) | ✅ Failed against unreconciled kimi assets (claude≠kimi) | ✅ Passed after 2.1/2.2 (1 fixture edge case fixed — merged bullet, see Deviation) | ✅ 2 subtests (gentleman, neutral) | ✅ Clean |
| 2.1 | `internal/assets/claude/output-style-gentleman.md`, `internal/assets/kimi/output-style-gentleman.md` | Asset | N/A | (RED = 1.8/existing tests) | ✅ 1.8 gentleman subtest PASS | ➖ | ✅ |
| 2.2 | `internal/assets/claude/output-style-neutral.md`, `internal/assets/kimi/output-style-neutral.md` | Asset | N/A | (RED = 1.8/existing tests) | ✅ 1.8 neutral subtest PASS | ➖ | ✅ |
| 2.3 | `internal/assets/claude/persona-gentleman.md` (slim in place) | Asset | N/A | (RED = 1.1) | ✅ 1.1 PASS | ➖ | ✅ |
| 2.4 | `internal/assets/kimi/persona-gentleman.md` (slim in place) | Asset | N/A | (RED = 1.4) | ✅ 1.4 PASS | ➖ | ✅ |
| 2.5 | `internal/assets/claude/persona-neutral-residual.md` (new) | Asset | N/A | (RED = 1.2) | ✅ 1.2 PASS | ➖ | ✅ |
| 2.6 | `internal/assets/kimi/persona-neutral-residual.md` (new) | Asset | N/A | (RED = 1.7 table coverage) | ✅ 1.7 kimi/neutral subtest PASS | ➖ | ✅ |
| 2.7 | `internal/components/persona/inject.go` (`residualChannel`, `personaContent` signature) | Unit (compile + behavior) | ✅ `go vet` baseline captured RED at 1.6 | ✅ (see 1.6) | ✅ `go vet ./internal/components/persona/...` clean; full persona package `go test` green | ✅ exercised via 1.1–1.8 | ✅ Boy-Scout: extracted `residualChannel` helper, added doc comments |
| 2.8 | `internal/components/filemerge/section_test.go`, `internal/components/uninstall/cleaners_test.go` | Unit (approval-style regression) | ✅ `go test ./internal/components/filemerge/... ./internal/components/uninstall/...` = ok before edits | ✅ Written (asserts existing correct behavior — Decision 5 claims no change needed) | ✅ Both new cases PASS immediately (approval test — confirms unchanged fingerprint logic already handles slim installs correctly) | ➖ Single case each (design's exact claim) | ➖ None needed |
| 3.1 | `e2e/e2e_test.sh` | Shell (Docker-gated, not run) | N/A | N/A — `bash -n` only per instructions | N/A | N/A | ✅ `bash -n` clean on all `e2e/*.sh` |
| 3.2 | `docs/agents.md` | Doc | N/A | N/A | ✅ `rg -n "Output Styles" docs/agents.md` shows corrected row + footnote | N/A | N/A |
| 3.3 | `testdata/golden/*` | Golden | ✅ confirmed via `git log` that `combined-claude-claudemd.golden` base = commit `0fe7d51` (pre-apply HEAD) | N/A (regeneration, not TDD) | ✅ `-update` then re-run without `-update` = ok; byte-stable goldens confirmed empty diff | N/A | N/A |
| 3.4 | Token measurement | N/A | N/A | N/A | ✅ measured below | N/A | N/A |
| 3.5 | Full verification | N/A | N/A | N/A | ✅ `go test ./...` all ok, `go vet ./...` clean, `gofmt -l internal/` shows 0 files I touched | N/A | N/A |

### Test Summary
- **Total tests written/rewritten**: 8 rewritten/renamed (1.1–1.5 = 5 tests, one of which touches 2 test functions) + 2 new (`TestPersonaContentResidualDispatchAllAgents`, `TestKimiOutputStyleSupersetOfLegacyKimiCopy`) + 2 new fingerprint regression tests (section_test.go, cleaners_test.go) + 4 unlisted-scope fixes in `internal/assets` (see Deviation) + 1 golden test extended (`TestGoldenPersona_Claude_Neutral` now also asserts the new outputstyle golden).
- **Total tests passing**: full repo `go test ./...` — all packages `ok`, 0 `FAIL`.
- **Layers used**: Unit (all — Go standard `testing`, table-driven where applicable per go-testing skill).
- **Approval tests**: 2 (`TestStripLegacyPersonaBlock_SlimResidualInstallNotFalselyStripped`, `TestRemoveMarkdownSections_RemovesSlimResidualPersonaViaMarkerNotFingerprint`) — both confirm EXISTING fingerprint logic already correctly handles the new slim-install shape, no production change needed (Decision 5).
- **Pure functions**: `residualChannel(adapter) bool` — new pure predicate extracted in `inject.go`.

## RED evidence (captured verbatim)

```
$ go vet ./internal/components/persona/...
# github.com/gentleman-programming/gentle-ai/internal/components/persona
# [github.com/gentleman-programming/gentle-ai/internal/components/persona]
vet: internal/components/persona/inject_test.go:2031:61: too many arguments in call to personaContent
	have (model.AgentID, model.PersonaID, bool)
	want (model.AgentID, model.PersonaID)
```

```
$ go test ./internal/components/persona/... -run TestKimiOutputStyleSupersetOfLegacyKimiCopy -v
    inject_test.go:2308: reconciled kimi/output-style-gentleman.md lost legacy line
    "- When replying to the user in English, keep the full response in English
    unless the user explicitly asks for another language or you are translating/quoting."
--- FAIL: TestKimiOutputStyleSupersetOfLegacyKimiCopy (0.00s)
```
(Resolved by the merged-bullet fixture fix documented above — not a defect in
the reconciled asset; the design's own Decision 4/JD-013 intentionally merges
this line.)

## Token measurement (Decision 6, chars/4 method — measured against FINAL committed assets)

Method: `wc -c` on the removed persona-section content vs. the reconciled
output-style growth, per variant. `tokens ≈ chars / 4`.

| Variant | Old persona chars | New persona chars | Gross removed | Old style chars | New style chars | Style growth | Net/session (chars) | Net ≈ tokens |
|---|---|---|---|---|---|---|---|---|
| Claude Gentleman | 5,639 | 2,023 | 3,616 | 5,649 | 6,358 | 709 | 2,907 | ~727 |
| Kimi Gentleman | 4,999 | 1,889 | 3,110 | 5,195 | 6,358 | 1,163 | 1,947 | ~487 |
| Claude Neutral | 5,099 (shared `generic/persona-neutral.md`) | 1,953 | 3,146 | 3,423 | 4,509 | 1,086 | 2,060 | ~515 |
| Kimi Neutral | 5,099 (shared `generic/persona-neutral.md`) | 1,981 | 3,118 | 2,712 | 4,509 | 1,797 | 1,321 | ~330 |

**Post-apply correction (review round 2, JD-016/JD-017)**: the numbers above
are re-measured with `wc -c` (chars/4 method, unchanged) AFTER restoring the
union content the judges found missing (see "Post-apply correction note"
below) — `New style chars` grew further (+87 Gentleman, +634 Neutral) because
the Behavior/Tone/analogies content that should have shipped in the first
pass is now actually present. Net token savings per session dropped
accordingly (still net-positive in all four variants) but are now the
*true* measurement of the *complete* union, not an undercount produced by
missing content.

Design's projected table (pre-implementation estimate): Claude Gentleman
~3,110 net (~778 tok), Kimi Gentleman ~2,650 net (~663 tok), Claude Neutral
~2,880 net (~720 tok), Kimi Neutral ≈ Claude Neutral.

**Deviation from projection (not a defect — expected estimate variance)**:
measured net savings are lower than projected, especially for Kimi, because
Kimi's output styles started further behind Claude's (missing more
determinism lines pre-reconciliation — 2 lines for Gentleman, 4 lines +
reorder for Neutral, per Decision 4's verified diff) and therefore grew more
during reconciliation than the design's uniform ~615/~400-char growth-offset
assumption. The design explicitly asked the apply phase to record the
*measured* number, not to match the projection; gross savings (ignoring
style growth) remain close to the design's ~995 gross-token estimate for
Claude Gentleman (measured gross: 3,616/4 ≈ 904 tokens).

## Files Changed

| File | Action | Lines |
|------|--------|-------|
| `internal/components/persona/inject.go` | Modified | +43/-14 (residualChannel, personaContent signature+dispatch) |
| `internal/components/persona/inject_test.go` | Modified | +384/-~90 (RED rewrites, 2 new tests, fixture data) |
| `internal/components/filemerge/section_test.go` | Modified | +17 (new regression case) |
| `internal/components/uninstall/cleaners_test.go` | Modified | +35 (new regression case) |
| `internal/components/golden_test.go` | Modified | +5 (new neutral outputstyle golden assertion) |
| `internal/assets/assets_test.go` | Modified | +46/-17 (combined-channel fixes, unlisted scope) |
| `internal/assets/language_contract_test.go` | Modified | +47/-13 (combined-channel fixes, unlisted scope) |
| `internal/assets/claude/persona-gentleman.md` | Modified (slim in place) | 76→32 lines |
| `internal/assets/kimi/persona-gentleman.md` | Modified (slim in place) | 75→34 lines |
| `internal/assets/claude/persona-neutral-residual.md` | Created | 32 lines |
| `internal/assets/kimi/persona-neutral-residual.md` | Created | 33 lines |
| `internal/assets/generic/persona-neutral.md` | Unchanged (Decision 2) | — |
| `internal/assets/claude/output-style-gentleman.md` | Modified | +6/-3 |
| `internal/assets/kimi/output-style-gentleman.md` | Modified (overwritten, byte-identical to claude) | +8/-3 |
| `internal/assets/claude/output-style-neutral.md` | Modified | +4 |
| `internal/assets/kimi/output-style-neutral.md` | Modified (overwritten, byte-identical to claude) | +10/-3 |
| `e2e/e2e_test.sh` | Modified | +39/-8 (3 test functions rewritten) |
| `docs/agents.md` | Modified | +4/-1 (Kimi row + footnote) |
| `testdata/golden/persona-claude-gentleman.golden` | Regenerated | — |
| `testdata/golden/persona-claude-neutral.golden` | Regenerated | — |
| `testdata/golden/persona-claude-gentleman-outputstyle.golden` | Regenerated | — |
| `testdata/golden/combined-claude-claudemd.golden` | Regenerated (base verified via `git log` = `0fe7d51`) | — |
| `testdata/golden/persona-claude-neutral-outputstyle.golden` | Created | — |
| Byte-stable (confirmed via empty `git status --short`) | Unchanged | `combined-windsurf-global-rules.golden`, `persona-{antigravity,kiro,windsurf}-gentleman.golden`, `persona-opencode-{gentleman,neutral}.golden` |

Total tracked diff: 19 files changed, 571 insertions(+), 365 deletions(-)
(`git diff --stat`), plus 3 new untracked files (2 residual assets + 1 golden).
Within the design's 700–1100 forecasted line-change range; delivered as a
single `size:exception` PR per the accepted `exception-ok` delivery strategy.

## Final verification

```
$ go test ./...
ok for all packages (0 FAIL)

$ go vet ./...
(clean, no output)

$ gofmt -l internal/
(12 pre-existing files unrelated to this change — none touched by this apply)
```

## Spec scenario check (`specs/persona-behavior-contract/spec.md`)

All 7 requirements (5 ADDED, 1 MODIFIED, 1 additional ADDED for docs/testing)
verified against the implementation:
1. Canonical Tone Channel for Output-Style-Capable Adapters — ✅ residual
   sections contain only Rules/Expertise/Contextual Skill Loading/pointer
   (+ Kimi-native notes), no tone/language/philosophy.
2. Persona Section Parity for Non-Output-Style Adapters — ✅ 14/16 agents
   unaffected, verified by `TestPersonaContentResidualDispatchAllAgents`.
3. Persona Drift Reconciliation — ✅ union verified by
   `TestKimiOutputStyleSupersetOfLegacyKimiCopy` + extended
   `claudeOutputStyleLanguageGuardrails`.
4. Idempotent Convergence on Re-Injection — ✅ existing idempotency/auto-heal
   tests pass unchanged in structure (only content expectations updated).
5. Legacy Fingerprint Continuity — ✅ fingerprints kept unchanged (Decision 5);
   2 new regression tests confirm slim installs still uninstall/auto-heal
   correctly.
6. Kimi Non-Duplication and Documentation Accuracy — ✅ persona.md residual +
   output-style.md canonical; docs/agents.md corrected.
7. Per-Adapter Test and Golden Coverage — ✅ 4 Claude goldens regenerated + 1
   created; 7 byte-stable goldens confirmed unchanged; Kimi covered by unit
   assertions only (no Kimi goldens exist, per design).
MODIFIED — Generic Neutral Asset Parity (combined-channel evaluation) — ✅
   applied consistently to both the Neutral delta-spec scenario and the
   equivalent (unlisted-scope) Gentleman assertions in `internal/assets`.

## Post-apply correction note (round-2 judgment-day, JD-016..JD-022)

A second judgment-day pass over this uncommitted apply diff found that the
"union" step of Decision 4's reconciliation was incomplete, and that its own
regression net (`TestKimiOutputStyleSupersetOfLegacyKimiCopy`) was
structurally blind to the gap:

- **JD-016 (CRITICAL)**: `generic/persona-neutral.md`'s entire `## Behavior`
  section (4 rules) and the normative content of its `## Tone` section (the
  3-step correction structure + CAPS-for-emphasis rule) were marked MOVE in
  Table C but were never actually copied into `output-style-neutral.md`.
  Neutral-persona Claude/Kimi users lost all of this teaching-methodology
  guidance from both channels (persona residual AND output style). Fixed by
  copying the HEAD text verbatim into two new sections
  (`## Tone`, `## Behavior`) in `claude/output-style-neutral.md`, then
  overwriting `kimi/output-style-neutral.md` byte-identical.
- **JD-017 (CRITICAL)**: the Gentleman `## Behavior` rule "Use
  construction/architecture analogies when they clarify the point, not by
  default" (HEAD `claude/persona-gentleman.md:66`) was dropped from every
  current Claude/Kimi asset (`rg -i analog` = zero hits pre-fix). Fixed by
  appending it as item 6 of `claude/output-style-gentleman.md`'s `## Behavior`
  list, then overwriting `kimi/output-style-gentleman.md` byte-identical.
- **Root cause**: `TestKimiOutputStyleSupersetOfLegacyKimiCopy` (the only
  regression net for Decision 4) diffs style-vs-style (Kimi pre- vs
  post-reconciliation). It cannot detect content that should have MOVED from
  a *persona* section into the style but never arrived, because that content
  was never part of the Kimi style's "before" snapshot in the first place —
  there was nothing to diff against. Fixed by adding a companion test,
  `TestReconciledStylesCarryAllMovedPersonaRules` (JD-018), which freezes
  every normative rule from the HEAD MOVE-tagged persona sections
  (Personality/Persona Scope/Language/Tone/Philosophy/Behavior, for
  `claude/persona-gentleman.md`, `kimi/persona-gentleman.md`, and
  `generic/persona-neutral.md`) and asserts each rule is still discoverable —
  verbatim, or via a documented merged-form exception — in the corresponding
  reconciled style. This test was written FIRST and confirmed RED against the
  unfixed styles (failing exactly on the JD-016/017 gaps), then the JD-016/017
  content additions turned it GREEN.
- **JD-019 (WARNING)**: `assets_test.go`'s required-string check for the
  "match current language" guardrail had been weakened uniformly to
  `"the user's current language"` for all 5 Gentleman persona paths, hiding
  that Claude/Kimi's residual no longer carries the literal `"...REPLY ONLY"`
  phrase (it moved to the output style's own wording). Fixed with per-path
  required strings: `generic/kiro/opencode` still assert the exact
  `"Match the user's current language in your REPLY ONLY"` phrase;
  `claude/kimi` assert the actual combined-channel phrase, `"Always match the
  user's current language in your reply."`.
- **JD-020 (WARNING)**: `personaContent`'s doc comment implied Gentleman
  residual selection was flag-driven for all personas; it is not — for
  Claude/Kimi, Gentleman dispatch is agent-hardcoded (the assets were slimmed
  in place, Decision 3) and ignores the `residual` argument entirely. Fixed
  the doc comment and added
  `TestPersonaContentGentlemanResidualIgnoredForClaudeAndKimi`, which locks
  today's behavior and documents the latent content-loss trap if
  `SupportsOutputStyles()`/the Kimi carve-out ever become conditional.
- **JD-021 (SUGGESTION)**: added `TestResidualChannelAllAgents`, a direct
  table test over `residualChannel(adapter)` using real `agents.NewAdapter`
  instances for all 16 `model.AgentID` constants (the existing 16-agent test
  only hand-derives the predicate, never calling `residualChannel` itself).
- **JD-022 (SUGGESTION)**: `e2e_test.sh:585,1564` asserted only the `"Neutral
  Output Style"` H1 title while claiming to check "mentor tone content" — now
  asserts an actual tone phrase (`"Push back when user asks for code without
  context or understanding"`, added by the JD-016 fix). The stale comment
  at the second call site (claiming "Senior Architect" appears in the neutral
  style) was corrected. The delta spec's `kimi/persona-gentleman.md:71-75`
  line pin (stale post-apply; the section is now at `:22-26`) was dropped,
  keeping only the section-name reference (`## Kimi-native notes`).

Net effect: no scope change, no new assets, no design deviation beyond what
Round 1 already approved — this was a completeness gap in Decision 4's
implementation, now closed and covered by a dedicated regression test.
