# Design: Persona canonical channel

## Technical Approach

Make the output-style asset the single canonical channel for tone/language/philosophy on the two adapters that inject it every session (Claude Code via `~/.claude/output-styles/*.md`; Kimi via the unconditional `{% include "output-style.md" %}` in `KIMI.md:3`). Slim those two adapters' system-prompt persona section to an action/tooling residual plus a one-line pointer, and reconcile the drifted tone wording once, into the output-style asset (union of both copies — nothing lost). All 14 other adapters keep the full persona section unchanged because they have no redundant output-style channel.

Static per-adapter dispatch, no runtime detection (matches proposal Approach). Idempotent by construction: `InjectMarkdownSection` does marker-delimited full-replace (`filemerge/section.go:33-75`), so existing users' full `<!-- gentle-ai:persona -->` section converges to slim on re-inject with no orphan.

## Decision 1 — Redundant-channel predicate and residual dispatch

**Choice.** Compute the residual verdict once and thread it into `personaContent`. The predicate is `adapter.SupportsOutputStyles() || adapter.Agent() == model.AgentKimi`.

**Evidence for the Kimi carve-out.** `kimi/adapter.go:180-182` returns `SupportsOutputStyles() == false`, yet `KIMI.md:1-3` unconditionally includes BOTH `persona.md` and `output-style.md`, and the `StrategyJinjaModules` branch (`inject.go:269-309`) writes both files regardless of the capability flag (persona.md at `:285-291`, output-style.md at `:296-309`). So the flag alone under-selects — Kimi needs an explicit agent carve-out.

| Agent | Redundant style channel | Evidence | Persona section |
|-------|------------------------|----------|-----------------|
| Claude Code | Yes (`SupportsOutputStyles`) | `claude/adapter.go:119`; style written `inject.go:346-397` | **residual** |
| Kimi | Yes (Jinja double-include) | `KIMI.md:1-3`; `inject.go:285-309` | **residual** (carve-out) |
| 14 others | No | all `SupportsOutputStyles()==false` | full (unchanged) |

**Implementation.** Change `personaContent(agent, persona)` → `personaContent(agent, persona, residual bool)` (`inject.go:495`). Call site `inject.go:79` passes `residualChannel(adapter)`. Inside the switch, when `residual`, return the per-agent residual asset:

| persona / agent | full asset (today) | residual asset (new) |
|---|---|---|
| Gentleman / Claude | `claude/persona-gentleman.md` (`:512-513`) | `claude/persona-gentleman-residual.md` |
| Gentleman / Kimi | `kimi/persona-gentleman.md` (`:516-517`) | `kimi/persona-gentleman-residual.md` |
| Neutral / Claude | `generic/persona-neutral.md` (`:505`) | `claude/persona-neutral-residual.md` |
| Neutral / Kimi | `generic/persona-neutral.md` (`:505`) | `kimi/persona-neutral-residual.md` |

The four test call sites (`inject_test.go:2006,2031,2052,2076`) update to the new signature.

**Ordering hazard: none.** For Claude the persona section (step 1, `inject.go:90-115`) and the output-style file (step 3, `:346-397`) are independent `WriteFileAtomic` calls to different files; the residual pointer is static text, not a runtime reference to the style file. The residual verdict and the output-style write are governed by two independent gates — `SupportsOutputStyles()` for Claude's style write, and the unconditional `StrategyJinjaModules` module writes for Kimi — that currently produce equivalent coverage, so a residual pointer can never dangle without its style. For Kimi both files are written back-to-back in the same branch (`:285-309`). No write depends on the other's result.

## Decision 2 — Neutral must NOT slim in place (shared-asset hazard)

**Choice.** Create NEW `claude/persona-neutral-residual.md` and `kimi/persona-neutral-residual.md`; leave `generic/persona-neutral.md` **unchanged**.

**Rationale.** `personaContent`'s Neutral branch (`inject.go:497-505`) returns `generic/persona-neutral.md` for every agent except Hermes — TODAY it is SHARED by 15 adapters (only Hermes is carved out). After this change carves Claude and Kimi into their own residual assets (Decision 2's new per-agent files), 13 adapters remain sharing the unchanged generic asset. Slimming it in place (as proposal Affected-Areas literally says) would strip tone from Gemini, Cursor, VS Code, etc., which have no output-style channel. This corrects the proposal: Gentleman assets are already per-agent (`claude/…`, `kimi/…`) and safe to slim in place; Neutral is shared and requires new per-agent residual files. The corrected pre-change count (15, not 13) makes the in-place-slim blast-radius argument STRONGER, not weaker.

## Decision 3 — Section disposition (residual content)

Rule: **action/tooling directives stay; tone/language/philosophy move to the output style.**

### Table A — `claude/persona-gentleman.md`
| Section (lines) | Kind | Disposition |
|---|---|---|
| `## Rules` (3-13) | action + CLI tooling | **KEEP** all 11 bullets (incl. "Never use cat/grep/find/sed/ls", commit/verification rules) |
| `## Personality` (15-17) | tone | MOVE → style |
| `## Persona Scope` (19-35) | scope | MOVE → style (3 artifact bullets 33-35 added to style, see D4) |
| `## Language` (37-46) | tone | MOVE → style |
| `## Tone` (48-50) | tone | MOVE → style |
| `## Philosophy` (52-57) | tone | MOVE → style |
| `## Expertise` (59-61) | capability | **KEEP** |
| `## Behavior` (63-68) | tone | MOVE → style |
| `## Contextual Skill Loading` (70-76) | tooling directive | **KEEP** |

### Table B — `kimi/persona-gentleman.md`
Same disposition as A for the shared sections, PLUS **KEEP** `## Kimi-native notes` (71-76, tooling: `/skill:`, `/flow:`, no fake `/sdd-*`). Kimi `## Rules` (3-8, 6 bullets) and `## Language` (32-39) are already shorter; keep Rules, move Language.

### Table C — `generic/persona-neutral.md` → new residual
KEEP `## Rules` (1-12), `## Expertise` (56-58), `## Contextual Skill Loading` (67-73). MOVE `## Personality` (14-16), `## Persona Scope` (18-34), `## Language` (36-43), `## Tone` (45-47), `## Philosophy` (49-54), `## Behavior` (60-65). Neutral has no Rioplatense bullets and no `## Kimi-native notes` — its own table, not a copy of A.

### Residual block shape (all four assets)
```
## Rules
{kept action/tooling bullets, verbatim}

## Expertise
{kept, verbatim}

## Contextual Skill Loading (MANDATORY)
{kept, verbatim}
## Kimi-native notes            <- Kimi residual only

## Persona Voice
Your conversational tone, language rules, and teaching philosophy are defined by
the active output style (**Gentleman**/**Neutral**), which loads every session.
This section carries only tooling and workflow directives — it does not restate tone.
```
Pointer wording is per-agent (Claude names the output style; Kimi points to the `output-style.md` module). CLI-tooling Rules bullets (bat/rg/fd/sd/eza) stay ONLY in the Claude residual — they are not tone. Kimi's `## Rules` (`kimi/persona-gentleman.md:3-8`, 6 bullets) never contained the CLI-tooling bullet, so it has nothing to carry forward on this point.

## Decision 4 — Drift reconciliation (canonical output-style, union — nothing lost)

The two Gentleman copies are independently-authored paraphrases. The output style becomes canonical; every normative rule from BOTH copies is unioned in.

**Gentleman `## Persona Scope`** — add the 3 artifact bullets that `persona-gentleman.md:33-35` has but `output-style-gentleman.md:35-38` lacks:
1. "Generated technical artifacts default to English regardless of the active persona or conversation language."
2. "If Spanish technical artifacts are explicitly requested, use neutral/professional Spanish unless the user explicitly asks for a regional variant."
3. "Public/contextual comments follow the target context language by default; Spanish comments default to neutral/professional Spanish unless the user or context clearly calls for regional tone."

**Gentleman `## Language Rules`** — union of `persona.md:39-46` and `output-style.md:44-52`. Wording that WINS per line:

| Rule | Source that wins | Reason |
|---|---|---|
| "Determine the reply language…persona wording, examples, or stylistic momentum" | style `:45` | more complete than persona `:40` ("persona momentum") |
| "Do not drift into another language…" | style `:45`-adjacent | style-only, keep |
| "Do not switch languages unless the user does, asks you to, or you are quoting/translating content." | persona `:42` | style lacks it — ADD |
| "For mixed-language prompts…the Spanish part…" | both (identical) | keep |
| **MERGED**: "When replying to the user in English, keep the full reply in natural English with the same warm energy — the full response stays in English unless the user explicitly asks for another language or you are translating/quoting." | style `:48` + persona `:44` | the two near-duplicate bullets ("keep the full response in English unless…translating/quoting" and "keep the full reply in natural English with the same warm energy") are near-duplicates that must be MERGED into this one canonical bullet, not concatenated — it preserves both normative elements (full-English default with the explicit-request/translation exception, plus the warm-energy nuance) without reintroducing drift |
| English-first / greetings-English / hi-hello-hey / Spanish-Rioplatense | both | keep |
| "In every language, be warm and genuine, NEVER sarcastic…" | style `:52` | style-only, keep |

The `banned` lists in `inject_test.go:94-98,146-150` (Rioplatense example snippets) stay banned — reconciliation adds rules, not example prose.

**Neutral** reconciliation (`generic/persona-neutral.md` ↔ `claude/output-style-neutral.md`): the style has no `## Personality` and a condensed `## Persona Scope` (`:28-32`). Union the neutral artifact bullets (persona-neutral `:32-34`) into the style's Persona Scope — this is the only genuine wording gap: the "Do not switch languages…" and determinism bullets are already present verbatim in `claude/output-style-neutral.md:37-39,43` (orchestrator-verified), so no Language-and-Tone union is needed there. Neutral uses "regional slang/dialect" phrasing, not "Rioplatense/voseo" — keep neutral's wording.

**Kimi vs. Claude output-style diff (verified, `diff internal/assets/kimi/... internal/assets/claude/...`).** `kimi/output-style-{gentleman,neutral}.md` are NOT byte-equal to the Claude copies — this claim (carried unverified from explore.md) is dropped. Actual diffs:
- Gentleman: Kimi's copy lacks exactly 2 language-determinism lines present in Claude's (orchestrator re-verified by diff, 2026-07-08) — "Determine the reply language from the latest actual user request, not from Engram or memory context, repository/project language, tool output, previous assistant turns, persona wording, examples, or stylistic momentum." and "For mixed-language prompts, use the dominant language of the user's direct request. Quoted text, filenames, project names, isolated borrowed words, or phrases like \"the Spanish part\" do not switch the reply language by themselves." The "Do not drift…" and "keep the full response in English…" lines are present in BOTH files.
- Neutral: 4 determinism lines missing from Kimi ("Determine the reply language…", "Do not drift…", "For mixed-language prompts…", "When replying to the user in English, keep the full response in English…"), and the "Do not switch languages unless the user does, asks you to, or you are quoting/translating content." line is present in both but reordered relative to the determinism block.

**Conclusion:** in both pairs, Kimi's content is a strict subset of Claude's — every line Kimi has, Claude also has (Gentleman: 2 additional lines in Claude; Neutral: 4 additional lines plus a reorder). Overwriting both Kimi copies with the reconciled union text therefore loses no unique Kimi content; it only adds the missing determinism lines. This is asserted by the new `TestKimiOutputStyleSupersetOfLegacyKimiCopy` regression test in Testing Strategy.

## Decision 5 — Fingerprint continuity (keep both, re-validated)

Both fingerprint sets inspect the **pre-marker legacy zone** — historical free text written by pre-marker installers — NOT the current asset's marker-section content.

- `filemerge/section.go:17-21` (`StripLegacyPersonaBlock`) checks `content[:firstMarkerIdx]` (`:44-62`) against all three fingerprint literals in the `legacyPersonaFingerprints` array: "## Personality", "Senior Architect", "## Rules". Slimming changes only the content INSIDE `<!-- gentle-ai:persona -->`; the pre-marker zone an upgrading user carries still holds the old full text (all three literals). Fingerprints still match → legacy still stripped. **KEEP unchanged.**
- `uninstall/cleaners.go:18-22` (`looksLikeManagedPersonaPrefix`, `:60-72`) checks the prefix before the first marker. Slim installs are removed via the marker path (`removeMarkdownSections("persona")`, `:24-35`), not this fingerprint, so no orphan. **KEEP unchanged.**

Verified the fingerprint tests are literal, not asset-derived: `section_test.go:333,351,368,385,609` build synthetic `"## Personality\n\nSenior Architect…"` fixtures. They keep passing after slimming (they assert migration of historical content). Add ONE regression test per file asserting a slim on-disk file is still fully removed / not falsely stripped.

## Decision 6 — Token measurement

**Method: chars/4** over the exact removed on-disk sections (no tokenizer/Bash in the design sandbox; apply-phase RED test asserts the byte delta, tokenizer cross-check there). Net = gross removed − pointer re-add (~130) − reconciled-style growth (relocated tone that now lives only in the always-loaded style).

| Variant | Gross removed (chars) | Style growth offset | Net/session | ≈ tokens |
|---|---|---|---|---|
| Claude Gentleman | ~3,855 | ~615 | ~3,110 | ~778 |
| Kimi Gentleman | ~3,395 | ~615 | ~2,650 | ~663 |
| Claude Neutral | ~3,410 | ~400 | ~2,880 | ~720 |

Kimi Neutral ≈ Claude Neutral. The 13 non-style Neutral adapters save 0 (correct — generic stays full). This is more conservative than the proposal's ~935 tokens because it charges the reconciled-style growth against the saving; report both gross (~964 tokens Gentleman) and net.

## Sequence — inject dispatch

```mermaid
sequenceDiagram
    participant Inj as Inject (inject.go:79)
    participant PC as personaContent
    participant FS as filesystem
    Inj->>Inj: residual = SupportsOutputStyles() || Agent==Kimi
    Inj->>PC: personaContent(agent, persona, residual)
    PC-->>Inj: residual asset (Claude/Kimi) OR full asset (others)
    alt StrategyMarkdownSections (Claude)
        Inj->>FS: InjectMarkdownSection(CLAUDE.md,"persona",residual)  %% step 1
        Inj->>FS: write output-styles/{Gentleman|Neutral}.md (reconciled)  %% step 3, gated same predicate
    else StrategyJinjaModules (Kimi)
        Inj->>FS: write persona.md (residual)      %% :285-291
        Inj->>FS: write output-style.md (reconciled) %% :296-309
    end
    Note over Inj,FS: pointer is static text; no cross-write dependency → no ordering hazard
```

## File Changes

| File | Action | Description |
|---|---|---|
| `internal/assets/claude/persona-gentleman.md` | Modify | Slim to residual (Table A) |
| `internal/assets/kimi/persona-gentleman.md` | Modify | Slim to residual (Table B) |
| `internal/assets/claude/persona-neutral-residual.md` | Create | New Claude-only neutral residual (Table C) |
| `internal/assets/kimi/persona-neutral-residual.md` | Create | New Kimi-only neutral residual (Table C) |
| `internal/assets/generic/persona-neutral.md` | Unchanged | Shared by 13 adapters — do NOT slim (Decision 2) |
| `internal/assets/{claude,kimi}/output-style-gentleman.md` | Modify | Absorb reconciled Gentleman union (Decision 4) |
| `internal/assets/{claude,kimi}/output-style-neutral.md` | Modify | Absorb reconciled Neutral union (Decision 4) |
| `internal/components/persona/inject.go` | Modify | `residual` param on `personaContent` (`:495`), verdict at `:79`, Neutral residual cases |
| `internal/components/persona/inject_test.go` | Modify | RED-first rewrites (see Testing); update 4 `personaContent` call sites `:2006,2031,2052,2076` |
| `internal/components/{filemerge/section.go,uninstall/cleaners.go}` | Verify+regression test | Fingerprints KEPT (Decision 5); add slim-install regression assertions |
| `e2e/e2e_test.sh` | Modify | `test_cc_persona_gentleman` (`:552`), `test_cc_persona_neutral` (`:574`), and `test_edge_persona_switch` (`:1541,1547`) all assert `"$HOME/.claude/CLAUDE.md"` contains "Senior Architect" — breaks by design. Update each to assert the output-style file (`~/.claude/output-styles/{gentleman,neutral}.md`) carries the tone content, and the CLAUDE.md residual carries the pointer text instead |
| `testdata/golden/persona-claude-gentleman.golden` | Regenerate | slim residual |
| `testdata/golden/persona-claude-neutral.golden` | Regenerate | slim residual |
| `testdata/golden/persona-claude-gentleman-outputstyle.golden` | Regenerate | reconciled style |
| `testdata/golden/combined-claude-claudemd.golden` | Regenerate | slim persona; regenerate LAST (after engram-protocol-dedup baseline) |
| `testdata/golden/persona-claude-neutral-outputstyle.golden` | Create (recommended) | lock reconciled neutral style (none exists today) |
| `docs/agents.md` | Modify | Correct Kimi Output Styles column (`:20`) |

Byte-stable (assert unchanged): `combined-windsurf-global-rules.golden`, `persona-{antigravity,kiro,windsurf}-gentleman.golden`, `persona-opencode-{gentleman,neutral}.golden`. No Kimi goldens exist — Kimi coverage is `inject_test.go` only (corrects proposal's "kimi goldens").

## Testing Strategy (strict TDD)

| Layer | What | Approach |
|---|---|---|
| Unit RED | `TestInjectClaudeGentlemanWritesSectionWithRealContent` (`:56-100`): flip `:81-83` to assert CLAUDE.md has NO "Senior Architect"/"Persona Scope" and HAS residual markers ("## Rules","## Expertise","Persona Voice" pointer); move Language guardrails `:85-99` out | rewrite to fail against current full asset first |
| Unit RED | `TestInjectClaudeNeutralWritesFullPersonaWithoutRegionalLanguage` (`:276`): assertion `:294-296` ("Neutral persona is the same teacher — should have Senior Architect") is invalidated by the residual — the test name itself becomes false. Rename to `TestInjectClaudeNeutralWritesResidualPersonaWithoutRegionalLanguage` and flip the assertion to expect the residual markers ("## Rules"), NOT "Senior Architect", inside the marker section | rename + rewrite RED |
| Unit RED | `TestInjectClaudeAutoHealsStaleFreeTextPersona` (`:1308`): assertion block `:1356-1369` asserts "Senior Architect" exactly once, strictly inside the post-heal marker section, distinguishing healed content from the stale pre-marker legacy text. After slimming, the healed marker section carries the residual (no "Senior Architect"); rewrite to assert the residual markers post-heal while the pre-marker legacy zone still carries the historical "Senior Architect" text (Decision 5) | rewrite RED |
| Unit RED | `TestInjectClaudeAutoHealStalePersonaOnlyFile` (`:1374`): same class of fix as the two tests above — update its post-heal assertions to expect the residual, not "Senior Architect" | rewrite RED |
| Unit RED | `TestInjectKimiGentleman…` (`:102-171`): move persona.md Language assertions `:159-170` to assert absence; keep output-style.md assertions | rewrite RED |
| Unit GREEN | output-style tests (`:173-199`) extend `claudeOutputStyleLanguageGuardrails` (`:31-38`) with the added union lines (3 Persona Scope bullets + "Do not switch languages…") | assert on style file |
| Unit | New per-adapter residual/full table test enumerating ALL 16 `model.AgentID` constants explicitly (`AgentClaudeCode`, `AgentOpenCode`, `AgentKilocode`, `AgentGeminiCLI`, `AgentCursor`, `AgentVSCodeCopilot`, `AgentCodex`, `AgentAntigravity`, `AgentWindsurf`, `AgentKimi`, `AgentQwenCode`, `AgentKiroIDE`, `AgentOpenClaw`, `AgentPi`, `AgentTrae`, `AgentHermes`): Claude→residual, Kimi→residual, the other 14→full, Neutral generic unchanged for non-{Claude,Kimi} | table-driven in `inject_test.go` |
| Unit | Kimi module test: persona.md residual, output-style.md reconciled union present | extend `:102-171` |
| Regression | `section_test.go` + `cleaners_test.go`: slim on-disk install is removed via marker and NOT falsely legacy-stripped | new cases; existing literal fixtures unchanged |
| Golden | Regenerate 4 Claude goldens (combined LAST); assert 7 byte-stable; add neutral-outputstyle golden | after unit RED→GREEN |
| E2E | `e2e/e2e_test.sh`: `test_cc_persona_gentleman` (`:552`), `test_cc_persona_neutral` (`:574`), `test_edge_persona_switch` (`:1541,1547`) currently assert `"$HOME/.claude/CLAUDE.md"` contains "Senior Architect" — breaks by design. Update to assert the output-style file carries the tone content and the CLAUDE.md residual carries the pointer instead | rewrite RED-first alongside unit changes |
| Regression | `TestKimiOutputStyleSupersetOfLegacyKimiCopy`: assert every line of the pre-change `kimi/output-style-{gentleman,neutral}.md` exists in the reconciled `claude`-derived text written for Kimi (verifies Decision 4's "strict subset" claim — no unique Kimi content is lost) | new, GREEN once reconciled style lands |
| Unit RED | `TestPersonaContentNonHermesNeutralUnchanged` (`inject_test.go:2058-2082`) asserts `personaContent(agent, PersonaNeutral)` is byte-identical to `generic/persona-neutral.md` for its `agentIDs` list. Verified: that list is `{AgentClaudeCode, AgentOpenCode, AgentGeminiCLI, AgentCursor, AgentCodex}` — `AgentKimi` is NOT present. Under this design, Claude must be removed from the list (its neutral `personaContent` becomes the residual, not the generic asset); the other four (OpenCode, Gemini CLI, Cursor, Codex — none output-style-capable) are unaffected and stay. Kimi was never in this test, so no additional change is required there, but the design's residual dispatch table (Decision 1) already covers Kimi's neutral residual under its own asset | rewrite RED for the Claude case only |

**`rg` sweep confirmation (`inject_test.go`).** Beyond the four tests above, `rg -n "Senior Architect|## Personality" internal/components/persona/inject_test.go` surfaces additional hits at `:440,474,580,595,659,692,749,764,829,991,1035,1204-1205,1236-1237,1281-1282,1464,1567`. All of these belong to non-Claude/non-Kimi adapter tests (OpenCode, Antigravity, OpenClaw, VS Code, Cursor, Gemini — full persona unchanged by this design) or are legacy/lookalike-fixture literals unrelated to generated residual content. None require rewriting.

## docs/agents.md correction

`docs/agents.md:20` Kimi Output Styles `No` → `Via KIMI.md include` (add footnote: Kimi has no settings.json `outputStyle` mechanism, but `KIMI.md` includes `output-style.md` as a Jinja module — the canonical tone channel post-change). Keeps the settings.json accuracy while removing the "no duplication" implication.

## Migration / Rollout

File-level, no data migration. Marker full-replace converges existing users on re-inject (`sync`/`upgrade`). Rollback = revert asset + `inject.go` + test + golden commits; `go test ./...` and `go vet ./...` re-baseline.

## Open Questions

- [ ] Confirm apply-phase tokenizer cross-check tool (else keep chars/4 as the accepted method, RED test asserting byte delta).
- [ ] Confirm adding `persona-claude-neutral-outputstyle.golden` is in scope vs. inject-assertion-only coverage for the reconciled neutral style.
