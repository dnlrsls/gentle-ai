## Verification Report

**Change**: engram-protocol-dedup
**Version**: N/A (spec has no version marker)
**Mode**: Strict TDD

### Completeness
| Metric | Value |
|--------|-------|
| Tasks total | 37 |
| Tasks complete | 37 |
| Tasks incomplete | 0 |

Spot-checked (not just checkbox-trusted) tasks 1.1–4.1 against actual source: canonical asset markers, `protocolFor`/`IsVerifiedSlimAdapter` selection matrix, `InjectOptions.Version` threading, `ProbeProtocolFlag`/`runProtocolProbeCommand` seam, per-slug safest-wins forwarding in `run.go`, asset deletion + `assets_test.go` updates, uninstaller regression test, and all 4 golden fixtures. All match the checked-off task description; no checkbox was found to overstate actual work.

### Build & Tests Execution
**Build**: N/A (no separate build step configured; `go vet` used as static check)

**Tests**: `go test ./...` — ✅ 52 packages `ok`, 0 `FAIL` (fresh run, `-count=1`, no cache)
```text
$ go test ./... -count=1
ok  	.../internal/agentbuilder ... (52 packages, all ok)
```

**Vet**: `go vet ./...` — ✅ clean, no output, exit 0

**gofmt**: `gofmt -l` on all 14 changed/created Go files in this change — ✅ clean (no output)

**Coverage**: Not configured in `openspec/config.yaml` (`coverage_threshold: 0`) — not measured; informational only per strict-tdd-verify rules.

### Spec Compliance Matrix

| Requirement | Scenario | Test | Result |
|-------------|----------|------|--------|
| Single canonical Codex protocol asset | Codex outputs render from one source | `protocol_test.go::TestCodexInstructionsIsFullPlusPassiveCaptureInOrder`, `TestCodexCompactMatchesPreConsolidationCompactPrompt`, golden `TestGoldenEngram_Codex` | ✅ COMPLIANT |
| Single canonical Codex protocol asset | PASSIVE CAPTURE content survives consolidation | `protocol_test.go::TestCodexInstructionsIsFullPlusPassiveCaptureInOrder` (+ independently verified: golden `engram-codex-instructions.golden` PASSIVE CAPTURE block byte-identical to pre-change `codex/engram-instructions.md`'s PASSIVE CAPTURE block) | ✅ COMPLIANT |
| Single canonical Codex protocol asset | No normative rule regression | `protocol_test.go::TestProtocolFullByteEqualsPreConsolidationClaudeAsset` (byte-identical `full` vs. HEAD's `claude/engram-protocol.md`, independently re-verified via `diff`), content-growth assertion vs. old Codex asset | ✅ COMPLIANT |
| Conditional per-adapter section slimming | Verified adapter gets the slim section | `inject_test.go::TestProtocolForSelectsSlimOrFullPerDecision1Matrix` (Claude Code row), `protocol_test.go::TestProtocolSlimMatchesDecision2Block` (pointer assertion) | ✅ COMPLIANT |
| Conditional per-adapter section slimming | Unverified adapter keeps the full section | `inject_test.go::TestProtocolForSelectsSlimOrFullPerDecision1Matrix` (14 non-Claude rows + Pi handled separately, 16 total) | ✅ COMPLIANT |
| Version-gated protocol-verbosity forwarding to engram setup | Supported binary receives the verbosity flag | `internal/cli/protocol_forwarding_test.go::TestRunInstallForwardsProtocolSlimForClaudeCodeWhenSupported` | ✅ COMPLIANT |
| Version-gated protocol-verbosity forwarding to engram setup | Unsupported binary is invoked unchanged | `setup_test.go::TestProbeProtocolFlagDegradesWhenFlagAbsent`, `internal/cli/protocol_forwarding_test.go::TestRunInstallOmitsProtocolFlagWhenProbeFails` | ✅ COMPLIANT |
| Version-gated protocol-verbosity forwarding to engram setup | Setup never fails due to verbosity detection | `setup_test.go::TestProbeProtocolFlagDegradesOnContextDeadlineTimeout`, `TestProbeProtocolFlagDegradesOnNonZeroExit`; `run.go:861-864` swallows probe error into `protocolFlagSupported=false`, never returns it | ✅ COMPLIANT |
| Idempotent injection and clean uninstall across upgrades | Re-inject converges to target state | `inject_test.go::TestInjectWithOptionsReInjectConvergesFullToSlimAndBack` (full→slim→full, marker count asserted =1 each hop) | ✅ COMPLIANT |
| Idempotent injection and clean uninstall across upgrades | Uninstall removes renamed and new assets | `internal/components/uninstall/service_test.go::TestComponentOperationsEngram_CodexRemovesConsolidatedProtocolAssetsWithNoOrphans` | ✅ COMPLIANT |
| Test and golden fixture coverage per adapter | Table-driven coverage per adapter | `inject_test.go::TestProtocolForSelectsSlimOrFullPerDecision1Matrix` (16 rows), `TestProtocolForVersionGateBoundary` (5 rows) | ✅ COMPLIANT |
| Test and golden fixture coverage per adapter | Golden fixtures reflect new output | `TestGoldenEngram_Claude`, `TestGoldenCombined_Claude`, `TestGoldenEngram_Codex` (new); byte-diff independently verified vs. HEAD (see Correctness) | ✅ COMPLIANT |

**Compliance summary**: 12/12 scenarios compliant

### Correctness (Independent Static + Runtime Evidence)

| Requirement | Status | Notes |
|------------|--------|-------|
| `full` section byte-identical to deleted `claude/engram-protocol.md` | ✅ Verified | Independently diffed `git show HEAD:internal/assets/claude/engram-protocol.md` against the extracted `<!-- section:full -->` body — `diff` reports zero differences. |
| `compact` section byte-identical to deleted `codex/engram-compact-prompt.md` | ✅ Verified | Same independent diff method — zero differences. |
| Codex `model_instructions_file` render = full + passive-capture, PASSIVE CAPTURE preserved | ✅ Verified | `engram-codex-instructions.golden` (113 lines) contains full + reordered-but-byte-identical PASSIVE CAPTURE block (confirmed via independent `awk`/`diff` — only the trailing "### AFTER COMPACTION" heading differs due to intentional reordering, no content lost). |
| Slim only for Claude Code, gated on `InjectOptions.Version` ≥ v1.4.0 | ✅ Verified | `protocolFor`/`IsVerifiedSlimAdapter` in `protocol.go` correctly gate on `model.AgentClaudeCode` + `engramVersionMeetsFloor`; exact-floor `"1.4.0"` boundary test exists (`TestProtocolForVersionGateBoundary`) and asserts slim (inclusive `>=`, not `>`). |
| Probe never fails setup (timeout/non-zero/absent → omit flag) | ✅ Verified | `run.go:861-864`: `probeEngramProtocolFlag` error path silently sets `protocolFlagSupported=false`; no error is ever returned from probe failure. All 4 canned outcomes tested in `setup_test.go`. |
| Per-slug safest-wins forwarding | ✅ Verified | `run.go:865-879` computes `slugSlimVerdict` via boolean AND across every adapter sharing a slug. Test coverage: `TestRunInstallSafestWinsAcrossSharedSlug` (gemini-cli shared by Gemini CLI + Antigravity, both full → forwards `full`). **Note**: no test exercises a genuinely divergent per-adapter verdict within one slug (both current test cases are full/full) — see Issues, WARNING. |
| Idempotent re-inject full→slim (and back) | ✅ Verified | `TestInjectWithOptionsReInjectConvergesFullToSlimAndBack` asserts marker count = 1 on every hop, no duplication/corruption. |
| Uninstaller regression coverage | ✅ Verified | New regression test writes real files via `InjectWithOptions` then confirms `componentOperations` targets and removes them — not hand-crafted fixtures, so it fails if the renderer ever drifts from uninstaller-expected paths. |

### Coherence (Design)

| Decision | Followed? | Notes |
|----------|-----------|-------|
| Decision 1 — 16-adapter verification matrix, Claude Code only slim, v1.4.0 floor | ✅ Yes | `IsVerifiedSlimAdapter` matches exactly; 16-row table test matches design's row count. |
| Decision 2 — slim block content | ✅ Yes | `protocolSlim()` output matches design.md's exact slim block text verbatim. |
| Decision 3 — 4 paired markers, `codexInstructions = full + passive-capture` | ✅ Yes | `protocol.md` has exactly 4 paired marker sections; `codexInstructions()` implements concatenation in the specified order. |
| Decision 4 — `ProbeProtocolFlag`, 5s deadline, detached stdin, `execCommand` seam | ✅ Yes | Implemented as specified in `setup.go`; deviation (seam split into `runVersionCommand`/`runProtocolProbeCommand`) is a faithful, documented refinement — see Deviations below. |
| Decision 5 — upstream contract (specify only) | ✅ Yes | `upstream-protocol-flag-contract.md` documents exactly the 3 guarantees Decision 5/task 4.1 require. |
| Per-slug forwarding semantics (safest-wins) | ✅ Yes | `run.go` AND-reduction across adapters sharing a slug matches the documented rule. |

### Deviations from Design — Legitimacy Check

All 5 deviations documented in `apply-progress.md` were independently inspected against actual code:

1. **`execCommand` seam split into `runVersionCommand`/`runProtocolProbeCommand`.** ✅ Legitimate. Confirmed both wrapper vars are thin (`verify.go:29-31`, `setup.go:23-46`) and both still ultimately call the shared `execCommand` var underneath. This is a necessary Go-specific workaround (`*exec.Cmd` is a concrete struct, its `.Output()` result cannot be faked without a result-returning seam) and matches the codebase's existing function-var seam convention. Does not contradict design intent — design named `execCommand` as the shared seam and this deviation preserves that (both functions still route through it).
2. **`internal/cli` `TestMain` hermetic isolation.** ✅ Legitimate. `protocol_probe_test.go` confirmed to exist with defaults that reproduce pre-change behavior exactly (empty version → full; probe error → omit flag) — this is a test-infrastructure necessity (the dev machine has a real `engram` binary installed) with no behavioral risk, and it does not change production code paths.
3. **`IsVerifiedSlimAdapter` exported.** ✅ Legitimate and directly verified in use — `run.go:872` calls `engram.IsVerifiedSlimAdapter` to compute the per-slug forwarding verdict, achieving the "one source of truth" design intent (Decision 1 matrix feeds both section rendering and `--protocol` forwarding) more cleanly than a duplicated private matrix would.
4. **Golden tests updated to call `InjectWithOptions` with `Version: "1.18.0"`.** ✅ Legitimate and necessary — confirmed `git diff` on `golden_test.go`: without this change the golden tests would call bare `Inject()` (empty Version → safe-default full) and never actually exercise/lock in the slim path, contradicting the design's own Testing Strategy table ("regenerate to slim content").
5. **Doc placed at `openspec/changes/engram-protocol-dedup/upstream-protocol-flag-contract.md`** instead of `docs/`. ✅ Acceptable per stated orchestrator instruction; file exists and is cross-referenced from `proposal.md`.

No deviation breaks a spec requirement. All are additive engineering choices, consistent with the judgment-day-approved design intent.

### Golden Fixture Independent Verification

| Fixture | Expected | Verified |
|---------|----------|----------|
| `engram-claude-claudemd.golden` | Slim content (~11-13 lines) | ✅ Read directly — contains the exact Decision 2 slim block, `<!-- gentle-ai:engram-protocol -->` marker, no "needs_review"/full-only text. |
| `combined-claude-claudemd.golden` | Also carries slim (Claude embeds engram-protocol section) | ✅ `git diff --stat` shows regeneration (112 lines changed), consistent with slim swap. |
| `combined-windsurf-global-rules.golden` | Byte-stable vs. HEAD (Windsurf stays full) | ✅ `git diff HEAD -- testdata/golden/combined-windsurf-global-rules.golden` — zero output, confirmed byte-stable. |
| `engram-codex-instructions.golden` (new) | Full protocol + passive capture, content growth vs. old 104-line asset | ✅ 113 lines confirmed; PASSIVE CAPTURE block content independently diffed byte-identical to HEAD's `codex/engram-instructions.md` PASSIVE CAPTURE section (only reordered, per Decision 3's concatenation-not-replacement design). |
| `engram-codex-compact-prompt.golden` (new) | Byte-identical to old compact prompt | ✅ Independently diffed against `git show HEAD:internal/assets/codex/engram-compact-prompt.md` — zero differences. |

### Upstream Contract Doc Coverage

`upstream-protocol-flag-contract.md` was checked against the 4 items in the verification instructions:

| Item | Covered? | Notes |
|------|----------|-------|
| `--protocol` flag visible in `engram setup --help` stdout | ✅ Yes | Guarantee 1 |
| Stdin-detached probe guarantee | ✅ Yes | Guarantee 2 |
| Per-slug safest-wins semantics | ✅ Yes | Guarantee 3 |
| Version floor v1.4.0 semantics | ❌ Not covered | See Issues — SUGGESTION, not a defect against design/tasks. |

### Test Layer Distribution

| Layer | Tests | Files | Tools |
|-------|-------|-------|-------|
| Unit | ~120+ (canonical renderer, 16-row + 5-row tables, probe canned outcomes, version-floor math) | `protocol_test.go`, `inject_test.go`, `setup_test.go` | Go stdlib `testing` |
| Integration | ~11 (`internal/cli` RunInstall-level, uninstaller `componentOperations`) | `protocol_forwarding_test.go`, `protocol_probe_test.go`, `uninstall/service_test.go` | Go stdlib `testing`, in-process fakes |
| Golden | 4 fixtures touched (2 regenerated, 2 created, 1 byte-stable-verified) | `golden_test.go` | Custom `assertGolden` harness |
| **Total (spot-run)** | **153 passed / 0 failed** (filtered run: `-run "Protocol\|Codex\|Slim\|Engram"`) | | |

### TDD Compliance

| Check | Result | Details |
|-------|--------|---------|
| TDD Evidence reported | ✅ | Full table present in `apply-progress.md`, all 13 rows filled |
| All tasks have tests | ✅ | 37/37 tasks map to test files or explicit approval-test rationale (deletion tasks 2.8/2.9) |
| RED confirmed (tests exist) | ✅ | All listed test files verified to exist and contain the described test functions |
| GREEN confirmed (tests pass) | ✅ | `go test ./...` fresh run (`-count=1`) — 52/52 packages ok, 0 FAIL |
| Triangulation adequate | ✅ | 16-row adapter table, 5-row version-boundary table, 4-outcome probe table — no under-triangulated behavior found |
| Safety Net for modified files | ✅ | Reported "full suite green pre-change" for every modified-file row; consistent with a strict-TDD workflow |

**TDD Compliance**: 6/6 checks passed

### Assertion Quality

No tautologies, no ghost loops over possibly-empty collections, no assertion-without-production-code-call patterns found in `protocol_test.go`, `inject_test.go` (new sections), `setup_test.go`, `protocol_forwarding_test.go`, or `service_test.go`'s new test. Assertions consistently check real rendered byte content, explicit content-growth (length comparisons, not `Contains`-only), and exact marker counts. The table-driven loop in `TestExtractProtocolSectionBoundsAllFourMarkerPairs` iterates a fixed non-empty 4-element slice with per-iteration real assertions — not a ghost loop.

**Assertion quality**: ✅ All assertions verify real behavior

### Issues Found

**CRITICAL**: None

**WARNING**:
- Per-slug safest-wins forwarding (`run.go:865-879`) is only tested with same-verdict adapters sharing a slug (`gemini-cli` = Gemini CLI + Antigravity, both `full`). No test exercises a genuinely divergent verdict (one adapter `slim`, another `full`) sharing the same slug — the AND-reduction logic itself is simple and low-risk, and the design explicitly notes this scenario is a current no-op (Claude Code doesn't share its slug), so this is not a functional gap today but is a latent test gap if a future adapter shares Claude Code's slug.

**SUGGESTION**:
- `upstream-protocol-flag-contract.md` does not mention "version floor v1.4.0" semantics. This appears intentional: the version floor (Decision 1) gates the already-shipped MCP `instructions` channel and is unrelated to the not-yet-implemented `--protocol` flag (Decision 4/5) that this doc's 3 guarantees actually govern. Tasks.md 4.1 and design.md Decision 5 both scope the doc to exactly 3 guarantees. Flagging only because the verification instructions explicitly asked to check for this 4th item — recommend the orchestrator confirm this scoping is intentional, or add a short cross-reference sentence for upstream-team discoverability if not.

### Verdict
**PASS WITH WARNINGS**
All 37 tasks complete and verified; all 12 spec scenarios compliant with real passing tests; `go test ./...` and `go vet ./...` both clean; all 5 design deviations verified legitimate; goldens independently byte-diffed and confirmed correct. One WARNING (untested divergent-slug edge case, currently unreachable) and one SUGGESTION (upstream contract doc scope question) do not block archive.
