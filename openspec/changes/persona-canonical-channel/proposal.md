# Proposal: Persona canonical channel

## Intent

The Gentleman/Neutral persona tone is injected TWICE per session on Claude Code and Kimi: once as a system-prompt section (`persona-*.md`) and again as an output style (`output-style-*.md`, a Jinja module on Kimi). Beyond token waste, the two copies have DRIFTED — `claude/persona-gentleman.md` and `claude/output-style-gentleman.md` are independently-authored paraphrases, not copies. Make the output style the single canonical channel for tone/language/philosophy, reconcile the drift, and slim the system-prompt section to action/tooling residuals only.

## Scope

### In Scope
- Slim the Claude AND Kimi system-prompt persona section to a residual block — `## Rules`, `## Expertise`, `## Contextual Skill Loading (MANDATORY)`, plus a one-line pointer to the output style. Static dispatch via `SupportsOutputStyles()` for Claude; explicit carve-out for Kimi (its `KIMI.md` unconditionally double-includes both Jinja modules).
- Reconcile the drift into the output-style asset as the single source (Gentleman AND Neutral variants). Neutral pair (`generic/persona-neutral.md` vs `claude/output-style-neutral.md`) has a different section structure — separate diff pass.
- Correct the `docs/agents.md` Output Styles column for Kimi.
- Update inject tests + goldens in strict-TDD lockstep; re-validate legacy fingerprints still match on-disk.

### Out of Scope
- Persona content rewrites beyond drift reconciliation.
- The 14 other adapters' persona sections (unchanged; no output-style channel).
- The SDD orchestrator block embedded in persona assets (stays in place).

## Capabilities

### New Capabilities
- None.

### Modified Capabilities
- `persona-behavior-contract`: system-prompt persona section becomes a residual (tooling/skill directives only); the output style becomes the CANONICAL and only source of tone/language/philosophy for Claude and Kimi; residual block is REQUIRED (not optional) because `keep-coding-instructions: true` keeps CLAUDE.md loading alongside the style.

## Approach
- Static per-adapter channel decision (no runtime detection): Claude gated on `SupportsOutputStyles()`, Kimi on an explicit carve-out in the `StrategyJinjaModules` branch of `injectInternal`.
- Marker-based `InjectMarkdownSection` full-replace converges existing users' full sections to slim on re-inject — no orphan risk.
- Reconcile canonical tone wording once, into the output-style asset.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `internal/components/persona/inject.go` | Modified | Residual selection for Claude + Kimi carve-out. |
| `internal/assets/claude/persona-gentleman.md`, `generic/persona-neutral.md` | Modified | Slim to residual block. |
| `internal/assets/kimi/persona-*.md` | Modified | Slim to residual block. |
| `internal/assets/claude/output-style-*.md`, `kimi/output-style-*.md` | Modified | Absorb reconciled canonical tone (drift fix). |
| `internal/components/persona/inject_test.go` | Modified | Rewrite persona assertions RED-first. |
| `internal/components/uninstall/cleaners.go`, `internal/filemerge/section.go` | Verified | Re-validate `## Rules`/`Senior Architect` fingerprints survive on-disk. |
| `testdata/golden/combined-claude-claudemd.golden` + kimi goldens | Modified | Regenerate (3+ fixtures). |
| `docs/agents.md` | Modified | Correct Kimi Output Styles column. |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Coordinated test/golden breakage. | High | Strict-TDD RED→GREEN; lockstep golden regen. |
| Legacy fingerprints stop matching after slimming. | Med | Residual keeps `## Rules`; verify `Senior Architect` literal survives in an asset or re-validate fingerprint constants. |
| Neutral diff differs structurally from Gentleman. | Med | Separate section-level diff pass for the Neutral pair. |
| Kimi carve-out missed (only capability flag covers Claude). | Med | Explicit branch decision + docs correction. |

## Rollback Plan
1. Revert the asset slimming commits (restore full `persona-*.md` sections).
2. Revert `inject.go` residual/carve-out logic and `inject_test.go` assertions.
3. Revert regenerated goldens and `docs/agents.md`.
4. File-level only; no data migration. Re-run `go test ./...` and `go vet ./...` for baseline.

## Dependencies
- None external. Spec `persona-behavior-contract` already mandates a meaningful output-style channel — no conflict.

## Success Criteria
- [ ] Claude + Kimi system-prompt persona section reduced to residual + pointer; tone/language/philosophy served ONLY by the output style.
- [ ] Persona drift reconciled: one canonical tone source per variant (Gentleman + Neutral).
- [ ] **Measured per-session saving (Gentleman, chars/4 method over the exact removed sections):** ~3,980 removed chars ≈ ~995 tokens gross, ~935 net after the residual pointer re-add. Design/spec MUST tokenizer-verify and measure Neutral separately.
- [ ] `docs/agents.md` Kimi Output Styles column corrected.
- [ ] Legacy fingerprints re-validated as still matching on-disk content.
- [ ] `go test ./...` and `go vet ./...` pass with goldens regenerated.
