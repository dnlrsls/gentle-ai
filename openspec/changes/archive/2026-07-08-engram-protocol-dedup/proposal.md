# Proposal: Engram protocol dedup

## Intent

The Engram protocol reaches a single model session up to 3x. Codex is the worst offender: three gentle-ai-owned near-identical copies load together (AGENTS.md section + `model_instructions_file` + compact prompt). Claude Code duplicates the system-prompt section against the MCP handshake instructions and the SessionStart hook. This inflates every-session footprint (~13.6K tokens audited) with redundant text that costs tokens on every turn without adding signal. Pi already ships zero protocol text as a working slim-channel precedent.

## Scope

### In Scope
- Consolidate the Codex triplication into ONE canonical asset with section markers, reusing the existing `extractModelSection` mechanism. Must NOT drop Codex-only content (PASSIVE CAPTURE section).
- Slim the per-adapter `engram-protocol.md` system-prompt section ONLY where the design phase verifies a redundant channel actually reaches the model in that runtime (e.g. Claude Code = MCP instructions + SessionStart hook). Where no redundant channel is verified, keep the full section.
- Pass a protocol-verbosity flag from `internal/cli/run.go` engram setup invocation, guarded by a compatibility gate.
- Compatibility gate: detect whether the installed (latest, unpinned) engram binary supports the new flag (version check or `--help` probe); fall back to current behavior with older binaries. An unknown flag MUST never break setup.
- Update 7 golden fixtures, ~44 inject tests, and uninstaller cleaners in lockstep (strict TDD).

### Out of Scope
- Per-model capability tiering (no primary-session model signal exists — follow-up dependency).
- MCP instructions text (served by engram binary — upstream follow-up).
- The upstream `engram setup --protocol=slim|full` implementation (sibling change in gentle-engram repo).
- Persona canonical-channel (change 3).
- Antigravity ephemeral hook / `engram-convention.md` overlap: rationalize only if low-risk, else note as follow-up.

## Capabilities

### New Capabilities
- `engram-protocol-injection`: canonical single-source protocol asset, per-adapter slim/full section selection gated on verified redundant channels, and version-gated forwarding of the protocol-verbosity flag to engram setup.

### Modified Capabilities
- None.

## Approach
- Collapse the three Codex assets into one canonical source using `<!-- section:... -->` markers (same pattern as SDD `extractModelSection`); render Codex variants from it.
- Branch section verbosity per adapter on design-verified channel overlap; unverified adapters keep full text.
- Add a capability probe on the resolved engram binary before appending `--protocol=slim`; omit the flag when unsupported.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `internal/assets/codex/engram-*.md` | Removed/Merged | Collapse triplication into one canonical asset. |
| `internal/assets/claude/engram-protocol.md` | Modified | Slim section where a redundant channel is verified. |
| `internal/components/engram/inject.go` | Modified | Render canonical asset; per-adapter section selection. |
| `internal/components/engram/setup.go`, `internal/cli/run.go` | Modified | Version-gated `--protocol` forwarding. |
| `internal/components/engram/download.go` | Modified | Expose installed-version signal for the gate. |
| `internal/components/uninstall/cleaners.go` | Modified | Track renamed/removed Codex asset files. |
| `testdata/golden/engram-*.golden` | Modified | Regenerate 7 fixtures. |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Dropping Codex-only PASSIVE CAPTURE content. | Med | Section markers preserve it; assert in tests. |
| Unknown `--protocol` flag breaks setup on old binaries. | Med | Version/`--help` probe; fall back to no flag. |
| Slimming a section whose channel does not actually reach the model. | Med | Slim only design-verified overlaps; for Claude Code additionally gate on `engram --version` ≥ v1.4.0 (evidence: `WithInstructions`/`serverInstructions` shipped together in v1.4.0, see design.md Decision 1); below-floor or unknown version → keep full. |
| Uninstaller orphans renamed assets. | Med | Update cleaners in the same work unit. |
| Golden/test drift across ~44 tests + 7 fixtures. | High | Strict TDD red→green; lockstep fixture regen. |

## Rollback Plan
1. Revert the canonical-asset commit(s) and restore the three Codex asset files.
2. Revert `inject.go`/`setup.go`/`run.go`/`download.go` changes (flag forwarding + gate).
3. Revert golden fixtures, inject tests, and uninstaller cleaner updates from the same work unit.
4. File-level only; no data migration. Re-run `go test ./...` and `go vet ./...` to confirm baseline.

## Dependencies
- Upstream sibling change in gentle-engram (`/home/gentleman/work/engram`): `engram setup --protocol=slim|full`. gentle-ai forwards the flag only when the installed binary supports it, so this change ships independently and activates slimming once the upstream flag exists. The exact contract the upstream change MUST satisfy (probe discoverability, non-blocking `--help` on detached stdin, safest-wins per-slug forwarding) is documented in `upstream-protocol-flag-contract.md`.

## Success Criteria
- [ ] Codex renders its three surfaces from a single maintained source (down from 3 near-duplicate source files); PASSIVE CAPTURE content retained. This is source-of-truth consolidation, not a runtime-surface or token reduction for Codex — the rendered `model_instructions_file` grows slightly since Codex adopts the fuller canonical Claude text (see design.md Decision 3/4).
- [ ] Claude Code system-prompt section slimmed only where a redundant channel is design-verified and the installed engram binary is ≥ v1.4.0 (`VerifyVersion`-gated; see design.md Decision 1); est. **~1,045 net tokens/session saved** (~1,175 gross asset size minus ~130 for the slim replacement), plus ~2.5K tokens from the upstream hook once `--protocol` lands.
- [ ] Protocol-verbosity flag forwarded only when the installed engram binary supports it; older binaries keep full behavior with no breakage.
- [ ] 7 golden fixtures, ~44 inject tests, and uninstaller cleaners updated and green.
- [ ] `go test ./...` and `go vet ./...` pass.
