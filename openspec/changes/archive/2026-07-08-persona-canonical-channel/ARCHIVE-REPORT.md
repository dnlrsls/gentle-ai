# Archive Report: persona-canonical-channel

**Archived**: 2026-07-08
**Archive path**: `openspec/changes/archive/2026-07-08-persona-canonical-channel/`
**Commit**: `651b540` on branch `feat/persona-canonical-channel`

## What Shipped

Made the output-style asset the single canonical channel for persona tone, language, and philosophy on the two adapters with an active output-style mechanism — Claude Code (gated by `SupportsOutputStyles()`) and Kimi (explicit carve-out via `StrategyJinjaModules`). Previously, persona tone was injected twice per session on these adapters: once in the system-prompt persona section and once in the output-style module, and the two copies had drifted into independently authored paraphrases.

Key changes:

- **Residual persona sections**: The Claude and Kimi system-prompt persona sections were reduced to a residual block containing only `## Rules`, `## Expertise`, `## Contextual Skill Loading (MANDATORY)`, a one-line pointer to the output style, and any agent-native tooling section (Kimi: `## Kimi-native notes`). Tone, language, and philosophy prose was removed from these sections — it now lives exclusively in the output-style asset. `claude/persona-gentleman.md` and `kimi/persona-gentleman.md` were slimmed in place; `claude/persona-neutral-residual.md` and `kimi/persona-neutral-residual.md` were created new (the shared `generic/persona-neutral.md` used by 13 other adapters was left untouched).
- **Union reconciliation**: Where the persona and output-style copies had drifted (Gentleman and Neutral variants, Claude and Kimi), the output-style asset was reconciled to contain the union of normative tone rules found in either copy — no rule present in only one drifted source was dropped. This included merging artifact-language bullets, Language Rules tables, and determinism lines that existed in one copy but not the other.
- **Non-output-style adapters unaffected**: The other 14 `model.AgentID` adapters (OpenCode, Kilocode, GeminiCLI, Cursor, VSCodeCopilot, Codex, Antigravity, Windsurf, QwenCode, KiroIDE, OpenClaw, Pi, Trae, Hermes) continue to receive the full, unslimmed persona section — verified via a new 16-agent table-driven test.
- **Idempotent convergence**: Installations upgrading from a pre-slim full persona section converge to the residual block via the existing marker-delimited section-replace mechanism; re-running injection a second time leaves the on-disk content unchanged.
- **Fingerprint continuity**: Uninstall cleaner and auto-heal fingerprint checks (which rely on literal markers like `## Rules`, `Senior Architect`) were re-validated against the new residual on-disk content — new regression tests confirm slim installs are neither falsely stripped nor falsely flagged as needing repair.
- **Kimi non-duplication + docs accuracy**: Confirmed Kimi delivers tone content exactly once per session (sourced from the output-style module). `docs/agents.md`'s Kimi row was corrected from "Output Styles: No" to "Via KIMI.md include," with a footnote clarifying there is no `settings.json` `outputStyle` mechanism but `KIMI.md` includes `output-style.md` as the canonical tone channel.
- **Spec update**: `openspec/specs/persona-behavior-contract/spec.md` gained 7 new requirements (Canonical Tone Channel for Output-Style-Capable Adapters, Persona Section Parity for Non-Output-Style Adapters, Persona Drift Reconciliation, Idempotent Convergence on Re-Injection, Legacy Fingerprint Continuity, Kimi Non-Duplication and Documentation Accuracy, Per-Adapter Test and Golden Coverage) and the existing "Generic Neutral Asset Parity" requirement was modified to require parity evaluation over the COMBINED persona-residual + output-style channel for Claude/Kimi, rather than the persona file in isolation.

## Verification Outcome

**Verify phase**: PASS WITH WARNINGS (no CRITICAL issues).
- 52/52 tasks complete, 0 incomplete.
- `go test ./... -count=1` — all 49 test-bearing packages ok, 0 FAIL (independently re-run by verify, not cache-trusted).
- `go vet ./...` — clean.
- `gofmt -l` on all 7 touched Go files — clean.
- `bash -n e2e/e2e_test.sh` — clean.
- All 9 requirement/scenario groups from the delta spec mapped to passing, independently re-run tests.
- 3 non-blocking WARNINGs recorded (design.md self-contradiction on residual filenames — implementation correctly followed the 3-source-corroborated reading; `residual bool` param dead for the Gentleman branch in `personaContent` switch — harmless while `SupportsOutputStyles()` is a static `true`, latent smell if that ever becomes conditional; measured net token savings for Kimi came in below the design's projection, already explained as expected variance).

**Judgment Day**: APPROVED. 23-entry review ledger — 15 design-phase findings + 8 implementation-phase findings, all verified, including three real content-loss catches during design/apply that were corrected before merge (drift-reconciliation gaps that would have silently dropped normative tone rules from one of the two drifted copies).

## Measured Token Savings

Per-session net savings (chars/4 tokenizer approximation method), measured against final committed assets:

| Variant | Net tokens saved per session |
|---|---|
| Claude Gentleman | ~727 (verify independently reproduced: 749) |
| Claude Neutral | ~515 |
| Kimi Gentleman | ~487 |
| Kimi Neutral | ~330 |

Kimi's measured savings came in below the design's original projection (~663 projected vs. ~509/~330 actual for Gentleman/Neutral); apply-progress and verify both documented this as expected variance from the reconciliation adding determinism/artifact lines that were missing in the original Kimi copies, not a defect.

## Follow-Ups

1. **Residual-param hardcoding note**: `personaContent`'s `residual bool` parameter is currently dead for the Gentleman branch inside `inject.go`'s dispatch switch (unlike the Neutral case, which checks it). This is harmless today because Claude's `SupportsOutputStyles()` is a static `true`, but if that gate ever becomes conditional, the Gentleman branch will need the same conditional check the Neutral branch already has. Flagged for awareness, not blocking.
2. **Pre-existing gofmt drift in Kimi adapter files**: Noted during verification as pre-existing and untouched by this change — left as-is, out of scope for this archive.
3. **design.md doc hygiene**: The Decision-1/Decision-3 self-contradiction on residual persona filenames (Decision 1's table proposed new `*-gentleman-residual.md` files; Decision 3's tables and the actual File Changes table said "slim in place") was resolved correctly by implementation (3-source-corroborated reading), but design.md itself was not corrected post-hoc. Non-blocking, informational only.

## Traceability

| Artifact | Engram observation ID | Path |
|---|---|---|
| Proposal | #856 | `sdd/persona-canonical-channel/proposal` |
| Design | #858 | `sdd/persona-canonical-channel/design` |
| Tasks | #860 | `sdd/persona-canonical-channel/tasks` (filesystem: `tasks.md`, 52/52 checked) |
| Verify Report | #862 | `sdd/persona-canonical-channel/verify-report` |
| Delta spec (merged) | — | `specs/persona-behavior-contract/spec.md` (this archive folder) |
| Main spec (updated) | — | `openspec/specs/persona-behavior-contract/spec.md` |

## Spec Merge Self-Check

Delta spec: `openspec/changes/archive/2026-07-08-persona-canonical-channel/specs/persona-behavior-contract/spec.md`
Main spec: `openspec/specs/persona-behavior-contract/spec.md`

- Requirement headings in delta: 8 (7 ADDED + 1 MODIFIED) — all 8 present verbatim in updated main spec (7 appended at end of Requirements section, 1 replaced in place: "Generic Neutral Asset Parity").
- Given/When/Then scenario blocks in delta: 12 (9 across the 7 ADDED requirements + 3 in the MODIFIED "Generic Neutral Asset Parity" requirement) — all 12 present verbatim in updated main spec, no scenario dropped or paraphrased.
- Merge method: verbatim copy-paste, no summarization. ADDED requirements appended after the last pre-existing requirement ("Explicit Persona Selection Preservation"); MODIFIED requirement's full text (intro paragraph + all 3 scenarios) replaced the prior 2-scenario version in place.

## SDD Cycle Complete

The change has been fully planned, implemented, verified, and archived. `openspec/specs/persona-behavior-contract/spec.md` now reflects the new behavior as the source of truth. Ready for the next change.
