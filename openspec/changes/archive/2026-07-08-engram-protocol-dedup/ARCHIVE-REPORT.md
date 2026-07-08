# Archive Report: engram-protocol-dedup

**Change**: engram-protocol-dedup
**Archived**: 2026-07-08
**Archive Location**: `openspec/changes/archive/2026-07-08-engram-protocol-dedup/`
**Status**: COMPLETE AND VERIFIED

## Phase Summary

The SDD cycle for `engram-protocol-dedup` is COMPLETE. The change was:
- Proposed (proposal.md)
- Specified (specs/engram-protocol-injection/spec.md)
- Designed (design.md)
- Tasked (tasks.md — 37 tasks, all complete)
- Applied (implementation on branch feat/engram-protocol-dedup, commit d9c1c89)
- Verified (verify-report.md — PASS WITH WARNINGS)
- Judgment-day approved (review-ledger.md — 22 entries, terminal JUDGMENT APPROVED at both design and implementation rounds)
- Archived (this report)

## Spec Merge Results

**Delta Spec**: `openspec/changes/engram-protocol-dedup/specs/engram-protocol-injection/spec.md`

**Main Spec Created**: `openspec/specs/engram-protocol-injection/spec.md`

**Action**: Delta spec was a FULL spec (no prior main spec existed). Copied to main specs directory as-is. The spec defines 5 requirements, 12 scenarios, all verified COMPLIANT in verify-report.md.

## Artifacts Archived

All change folder contents preserved in archive:

| Artifact | Type | Status |
|----------|------|--------|
| explore.md | Exploration | ✅ Preserved |
| proposal.md | Proposal | ✅ Preserved |
| design.md | Design | ✅ Preserved |
| tasks.md | Tasks | ✅ Preserved (37/37 complete) |
| verify-report.md | Verification | ✅ Preserved (PASS WITH WARNINGS) |
| apply-progress.md | Apply Progress | ✅ Preserved |
| review-ledger.md | Review Ledger | ✅ Preserved (22 entries) |
| upstream-protocol-flag-contract.md | Documentation | ✅ Preserved |
| specs/engram-protocol-injection/spec.md | Specification | ✅ Preserved |

## Implementation Summary

**Scope Completed**:
- Codex protocol consolidation: 3 independent source files merged into 1 canonical marker-sectioned asset
- Per-adapter section slimming: Claude Code only (gated on engram ≥ v1.4.0); 15 adapters remain full; Pi unchanged (MCP-only precedent)
- Protocol-verbosity probing: side-effect-free `--help` probe with 5-second deadline, stdin detached
- Per-slug forwarding: safest-wins (full-wins) verdict across adapters sharing a setup slug
- Golden fixture regeneration: 4 fixtures touched (2 regenerated, 2 created, 1 byte-stable verified)
- Uninstaller regression: new test confirms no orphaned renamed/new assets after upgrade

**Change Metrics**:
- Lines changed: ~1,769 (1,544 insertions + 225 deletions)
- Go files modified/created: 14
- Test functions: ~20 new/modified
- Golden fixtures: 4 touched (2 regenerated, 2 created)
- Token savings (Claude Code system-prompt): ~1,045 tokens/session (verified by design)

**Test Evidence**:
- All 37 tasks complete and verified against actual source
- `go test ./...` — 52 packages ok, 0 FAIL
- `go vet ./...` — clean
- `gofmt -l` — clean on all changed files
- Judgment-day approved at design (3 rounds) and implementation (2 rounds)
- All 12 spec scenarios verified COMPLIANT

## Verification Details

**Completeness**: 37/37 tasks checked against actual codebase implementation

**Compliance**: 12/12 spec scenarios COMPLIANT (all requirements + scenarios tested)

**Test Layers**:
- Unit: ~120+ tests (canonical renderer, 16-row adapter table, 5-row version-boundary table, 4-outcome probe table)
- Integration: ~11 tests (internal/cli RunInstall-level, uninstaller componentOperations)
- Golden: 4 fixtures (2 regenerated to slim, 2 created new for Codex, 1 byte-stable verified)

**Design Coherence**: All 5 design decisions followed. 5 deviations documented in apply-progress.md, all verified legitimate (additive engineering choices consistent with codebase conventions, no spec contradictions).

## Key Design Decisions

1. **Single canonical asset** (`internal/assets/engram/protocol.md`) with 4 paired markers (full, slim, passive-capture, compact)
2. **Version floor for Claude Code**: engram ≥ v1.4.0 (researched in gentle-engram repo, verified commit 281bb1e)
3. **Per-adapter slim/full selection**: Claude Code → slim (gated on version floor); 15 adapters → full; Pi → no protocol text (precedent)
4. **Probe-and-forward strategy**: `ProbeProtocolFlag(ctx)` side-effect-free `--help` probe with 5s deadline, stdin detached; timeout/error → omit flag (safe default)
5. **Per-slug forwarding**: safest-wins (full-wins) reduction across adapters sharing a setup slug

## Upstream Dependencies

**Pending Sibling Change** (out of scope for this change):
- gentle-engram: `engram setup --protocol=<slim|full>` implementation
- Contract documented in `upstream-protocol-flag-contract.md` (3 guarantees: discoverability via `--help`, stdin-detached non-blocking probe, per-slug safest-wins semantics)
- This change ships independently; `--protocol` flag activates once upstream binary supports it

## Issues and Notes

**No CRITICAL issues** blocking archive.

**WARNING**: Per-slug safest-wins forwarding only tested with same-verdict adapters sharing `gemini-cli` slug (both full). No test exercises genuinely divergent verdict (one slim, one full) on shared slug — currently unreachable in adapter matrix but latent test gap for future adapters.

**SUGGESTION**: `upstream-protocol-flag-contract.md` does not mention version floor v1.4.0 semantics — intentional scoping (version floor gates MCP `instructions` channel, unrelated to `--protocol` flag). Recommend confirmation this scoping is intentional.

## Rollback Plan

If needed:
1. Revert canonical-asset + inject/run/setup/download commits
2. Restore 3 deleted source assets (`claude/engram-protocol.md`, `codex/engram-instructions.md`, `codex/engram-compact-prompt.md`)
3. Restore old golden fixtures from git history
4. Revert uninstaller + test updates
5. Run `go test ./...` and `go vet ./...` to confirm baseline

## Archive Integrity

- Source folder: `openspec/changes/engram-protocol-dedup/` (moved to archive)
- Archive folder: `openspec/changes/archive/2026-07-08-engram-protocol-dedup/` (contains all artifacts)
- Main spec: `openspec/specs/engram-protocol-injection/spec.md` (new, merged)
- Working tree: Left uncommitted per orchestrator instruction

## SDD Cycle Status

**CLOSED**. All phases complete:
- ✅ Exploration (explore.md)
- ✅ Proposal (proposal.md)
- ✅ Specification (specs/engram-protocol-injection/spec.md)
- ✅ Design (design.md)
- ✅ Tasks (tasks.md, 37/37 complete)
- ✅ Application (commit d9c1c89, feat/engram-protocol-dedup branch)
- ✅ Verification (verify-report.md, PASS WITH WARNINGS)
- ✅ Judgment-Day Review (review-ledger.md, 22 entries, JUDGMENT APPROVED)
- ✅ Archive (this report)

Ready for next change.
