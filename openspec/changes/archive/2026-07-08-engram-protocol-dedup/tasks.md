# Tasks: Engram protocol dedup

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | 900-1400 (1 new canonical asset, 3 deleted assets, ~6 Go files modified, 4 new test files/blocks, 2 goldens regenerated, 2 goldens created, 1 doc) |
| 400-line budget risk | High |
| Chained PRs recommended | Yes (defined as internal checkpoints; single PR accepted per delivery strategy) |
| Suggested split | Unit 1: RED tests → Unit 2: canonical asset + inject.go slim/full → Unit 3: version-gate plumbing (run.go/verify.go) → Unit 4: ProbeProtocolFlag seam (setup.go/run.go) → Unit 5: asset deletion + uninstaller → Unit 6: golden regeneration |
| Delivery strategy | exception-ok |
| Chain strategy | size-exception |

Decision needed before apply: No
Chained PRs recommended: Yes
Chain strategy: size-exception
400-line budget risk: High

### Suggested Work Units

| Unit | Goal | Likely PR | Notes |
|------|------|-----------|-------|
| 1 | RED: canonical renderer + table + version-gate + ProbeProtocolFlag tests, all failing | Checkpoint 1 | `internal/components/engram/*_test.go`; must fail before Unit 2 |
| 2 | GREEN: `internal/assets/engram/protocol.md` + `inject.go` slim/full selection + Codex render helpers | Checkpoint 2 | Req 1 + Req 2 |
| 3 | GREEN: `InjectOptions.Version` threading (`run.go` ← `VerifyVersion()`) + test seam | Checkpoint 3 | Req 2 (Claude Code gate) |
| 4 | GREEN: `ProbeProtocolFlag` (`setup.go`) + per-slug forwarding wiring (`run.go`) | Checkpoint 4 | Req 3 |
| 5 | GREEN: delete superseded assets + uninstaller regression | Checkpoint 5 | Req 1 + Req 4; only after Units 2-4 are green |
| 6 | REFACTOR: golden regeneration + full verification | Checkpoint 6 | Req 5; last, per design's explicit ordering |

Accepted delivery: single `size:exception` PR per session config (`exception-ok`); work units above define internal checkpoints/rollback boundaries, not mandatory separate PRs.

## TDD Evidence Requirements

Strict TDD is active. Required full test runner: `go test ./...`; use targeted package runs (`go test ./internal/components/engram/...`) during development.

- RED evidence: record the failing test command, failing test names, and the missing-asset/missing-plumbing reason before any asset or code edit.
- GREEN evidence: record the targeted passing command proving each surface is now green.
- REFACTOR evidence: after green, run `gofmt` on changed Go files, regenerate only the goldens the design specifies, re-run targeted + full tests.

## 1. Infrastructure (RED — write before any asset/code exists)

### 1.1 RED: Canonical protocol renderer unit tests

- [x] In `internal/components/engram/` (new or existing `*_test.go`), assert `protocolFull()` byte-equals today's `claude/engram-protocol.md` content, `codexInstructions()` equals `protocolFull()` + extracted `passive-capture` section (concatenation, content growth vs. today's asset asserted explicitly, not `Contains`), `codexCompact()` matches today's compact prompt, and `protocolSlim()` matches the Decision 2 slim block. Assert all four marker pairs (`section:full`, `section:slim`, `section:passive-capture`, `section:compact`) are correctly bounded. (Spec: Single canonical Codex protocol asset; PASSIVE CAPTURE content survives consolidation)
- [x] RED evidence: run against the not-yet-created `internal/assets/engram/protocol.md` and capture the expected file-not-found / compile failures.

### 1.2 RED: Per-adapter slim/full table test (16 rows)

- [x] Add a table-driven test in `inject_test.go` covering all 16 adapters from Decision 1: Claude Code → slim (gated on engram ≥ v1.4.0), all others → full, Pi → no protocol text (existing precedent, unchanged). (Spec: Conditional per-adapter section slimming)
- [x] RED evidence: run and capture failures (selection logic does not exist yet).

### 1.3 RED: Version-gate boundary tests

- [x] New test in `inject_test.go` using the not-yet-created test seam: below-floor version → full; unknown/unparseable version → full; exact-floor `v1.4.0` (inclusive boundary) → slim, asserted explicitly as its own case; above-floor → slim. (Spec: Conditional per-adapter section slimming, Verified adapter gets the slim section)
- [x] RED evidence: run and capture failures (`InjectOptions.Version` field and seam do not exist yet).

### 1.4 RED: `ProbeProtocolFlag` canned-output tests

- [x] New test faking the `execCommand` seam (same pattern as `verify_test.go`'s `VerifyVersion` fakes) with four canned outcomes: stdout containing `--protocol` (supported), stdout without it (unsupported), context-deadline timeout, non-zero exit — assert the first case detects support and the other three degrade to "flag unsupported" (omit). (Spec: Version-gated protocol-verbosity forwarding to engram setup, all three scenarios)
- [x] RED evidence: run and capture failures (`ProbeProtocolFlag` does not exist yet).

## 2. Implementation (GREEN)

### 2.1 GREEN: Author canonical `protocol.md` asset

- [x] Create `internal/assets/engram/protocol.md` with the four paired markers from Decision 3 (`section:full` byte-identical to today's Claude asset, `section:slim` per Decision 2, `section:passive-capture` Codex-only, `section:compact`). (Spec: Single canonical Codex protocol asset)
- [x] GREEN evidence: run 1.1's target test — PASS.

### 2.2 GREEN: `extractProtocolSection` + `protocolFor(agent)` in `inject.go`

- [x] Add local `extractProtocolSection(content, name)` (marker logic mirrors `sdd.extractModelSection`, kept local per Decision 3) and a `protocolFor(agent)` selector implementing the Decision 1 matrix, gated by `InjectOptions.Version` for Claude Code. (Spec: Conditional per-adapter section slimming)
- [x] GREEN evidence: run 1.2's table test — PASS (16/16 rows).

### 2.3 GREEN: Codex render helpers in `inject.go`

- [x] Implement `codexInstructions()` (= `protocolFull()` + `passive-capture`, in that order) and `codexCompact()`; wire both into `writeCodexInstructionFiles`. (Spec: Single canonical Codex protocol asset; PASSIVE CAPTURE content survives consolidation)
- [x] GREEN evidence: run 1.1's Codex-specific assertions — PASS.

### 2.4 GREEN: `InjectOptions.Version` + threading from `VerifyVersion()`

- [x] Add a `Version` field (raw version string, not boolean — needed for the at-floor boundary comparison) to `InjectOptions` in `inject.go`. In `internal/cli/run.go`, thread the existing `VerifyVersion()` call (currently only used at line ~1587 for the post-install health check, result discarded) into `InjectOptions.Version` before injection runs. (Spec: Conditional per-adapter section slimming)
- [x] GREEN evidence: run 1.3's version-gate boundary tests — PASS, including the exact-floor `v1.4.0` case.

### 2.5 GREEN: Exported test seam for pinning engram version

- [x] Add an exported seam (e.g. `SetVersionForTest`, or fake via the existing `execCommand` variable) in `internal/components/engram/verify.go` or `inject.go` so tests can pin the `engram version` result feeding the Decision 1 gate. (Spec: Test and golden fixture coverage per adapter)
- [x] GREEN evidence: 1.3's tests use the seam directly — PASS.

### 2.6 GREEN: `ProbeProtocolFlag(ctx)` in `setup.go`

- [x] Add `ProbeProtocolFlag(ctx context.Context) (string, error)` in `internal/components/engram/setup.go`, built on the package-level `execCommand` seam, running `engram setup --help` via `exec.CommandContext(ctx, ...)` with a hard 5-second deadline, `cmd.Stdin` unset/detached, returning captured stdout via `.Output()`. (Spec: Version-gated protocol-verbosity forwarding to engram setup)
- [x] GREEN evidence: run 1.4's canned-output tests — PASS (all four cases).

### 2.7 GREEN: `run.go` wiring — probe once, cache, per-slug forwarding

- [x] In `internal/cli/run.go` (~836-850), call `ProbeProtocolFlag()` once before the adapter loop, cache `protocolFlagSupported := strings.Contains(stdout, "--protocol")`. Forward `--protocol=<mode>` once per unique slug via `attemptedSlugs`, computing the slug verdict as the safest (full-wins) verdict across every adapter sharing that slug (Per-slug forwarding semantics). Timeout/non-zero-exit/any error → `protocolFlagSupported = false` (omit flag). (Spec: Version-gated protocol-verbosity forwarding to engram setup, all three scenarios)
- [x] GREEN evidence: extend 1.4's tests to assert the forwarded argv per slug and the safe-degradation path — PASS.

### 2.8 GREEN: Delete superseded source assets

- [x] Delete `internal/assets/claude/engram-protocol.md`, `internal/assets/codex/engram-instructions.md`, `internal/assets/codex/engram-compact-prompt.md` — ONLY after 2.1-2.7 are green and all renderers source from `protocol.md`. Written on-disk output paths stay identical (uninstaller does not churn). (Spec: Single canonical Codex protocol asset; Idempotent injection and clean uninstall across upgrades)
- [x] GREEN evidence: `go test ./internal/components/engram/...` — PASS after deletion, no import/embed errors.

### 2.9 GREEN: Uninstaller regression

- [x] Verify `internal/components/uninstall/cleaners.go` and `service.go` still target the unchanged on-disk paths (`~/.codex/engram-instructions.md`, `engram-compact-prompt.md`, CLAUDE.md `engram-protocol` marker); add a regression assertion that no renamed/new asset is orphaned after upgrade or uninstall. (Spec: Uninstall removes renamed and new assets)
- [x] GREEN evidence: extend `cleaners_test.go` — PASS.

## 3. Testing/Verification (golden regeneration LAST, per design ordering)

### 3.1 Golden regeneration: Claude fixtures

- [x] Regenerate `testdata/golden/engram-claude-claudemd.golden` (slim content) and `testdata/golden/combined-claude-claudemd.golden` (embeds the full `engram-protocol` section elsewhere — verified 2 marker hits, regenerates alongside). (Spec: Golden fixtures reflect new output)
- [x] GREEN evidence: `go test ./internal/components/... -run <affected tests> -update`, then re-run without `-update` — PASS.

### 3.2 Golden creation: new Codex fixtures

- [x] Create `testdata/golden/engram-codex-instructions.golden` and `testdata/golden/engram-codex-compact-prompt.golden`, capturing the rendered `model_instructions_file` / `experimental_compact_prompt_file` content growth from consolidating onto the canonical `full` text (Decision 3). (Spec: Table-driven coverage per adapter; Golden fixtures reflect new output)
- [x] GREEN evidence: new golden test — PASS; diff reviewed to confirm expected content growth only.

### 3.3 Golden verification: unaffected fixtures stay byte-stable

- [x] Confirm `testdata/golden/combined-windsurf-global-rules.golden` (also embeds the full `engram-protocol` section, verified 2 marker hits) is byte-stable — no regeneration, since Windsurf remains `full`. Confirm the remaining 5 of 7 `engram-*.golden` fixtures (MCP/settings-config-only) are unaffected. (Spec: Golden fixtures reflect new output)
- [x] Evidence: `git diff --stat testdata/golden/combined-windsurf-global-rules.golden` — empty.

### 3.4 Full required verification

- [x] Run `go test ./...` — all packages `ok`, 0 `FAIL`.
- [x] Run `go vet ./...` — clean, no output.
- [x] Compare the final implementation against every scenario in `openspec/changes/engram-protocol-dedup/specs/engram-protocol-injection/spec.md` — confirm all 5 requirements and their scenarios are satisfied.

## 4. Documentation

### 4.1 Document upstream `gentle-engram` contract handoff

- [x] Add a short contract note (e.g. `docs/engram-protocol-flag-contract.md`, or extend design.md's Decision 5 with an explicit handoff pointer) stating the three guarantees the sibling change MUST provide: (a) `--protocol` MUST appear in `engram setup --help` stdout for the probe to detect support, (b) the probe MUST be able to run with stdin detached/non-TTY without the binary blocking on interactive input, (c) forwarding is per-slug with safest-wins semantics when a slug is shared by adapters with divergent verdicts. This documents the handoff only — the upstream `--protocol` flag implementation itself is out of scope for this change. (Spec: adjacent to Version-gated protocol-verbosity forwarding to engram setup — documents the contract this change depends on)
- [x] Evidence: file exists and cross-referenced from `proposal.md`'s Dependencies section.
