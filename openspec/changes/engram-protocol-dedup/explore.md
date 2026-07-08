# Exploration — engram-protocol-dedup

## Current State

gentle-ai wires the external `engram` binary into 16 AgentID targets. `internal/components/engram/inject.go:285` (`injectWithOptions`) is the single entry point handling both MCP config and system-prompt protocol injection.

## Channel Inventory

1. **System-prompt section injection** (`internal/components/engram/inject.go:470-528`) — `StrategyMarkdownSections` (:472) and the `default` branch (:510) both call `filemerge.InjectMarkdownSection(existing, "engram-protocol", assets.MustRead("claude/engram-protocol.md"))`. Every adapter except Kimi and Pi lands here (Claude→CLAUDE.md, OpenCode/Codex/OpenClaw→AGENTS.md, Gemini/Antigravity→GEMINI.md, Qwen→QWEN.md, Cursor→.cursor/rules/gentle-ai.mdc, VSCode→instructions.md, Windsurf→global_rules.md, Kiro→gentle-ai.md, Trae→user_rules.md, Hermes→SOUL.md). Asset: `internal/assets/claude/engram-protocol.md` (102 lines). Kimi gets the same content via `StrategyJinjaModules` (:490) as an included module. **Pi is the sole exception** — `injectWithOptions` (:286-292) returns immediately after MCP provisioning for `piEngramProvisioner` adapters, so Pi gets zero protocol text (an existing "slim channel" precedent).

2. **Codex `model_instructions_file`** (`inject.go:388-467`, files written at :574-596) — writes `~/.codex/engram-instructions.md` from `internal/assets/codex/engram-instructions.md` (105 lines, near-verbatim duplicate of the Claude asset plus an extra PASSIVE CAPTURE section) plus `experimental_compact_prompt_file` (`codex/engram-compact-prompt.md`, 15 lines). Codex's `model_instructions_file` replaces Codex's built-in base instructions but NOT AGENTS.md — both load in the same session. **Codex is the worst offender: 3 gentle-ai-owned copies of near-identical content reach one session.**

3. **MCP server instructions** — NOT gentle-ai-owned. `engramServerJSON`/`engramOverlayJSON` (:79-150) only emit command/args (`["mcp", "--tools=agent"]`; `["mcp"]` for Antigravity). The instructions text at MCP handshake is served by the engram binary. No flag exists to request a slim/full variant.

4. **SessionStart hook** — external, installed by `engram setup <slug>` (`internal/components/engram/setup.go`). Invocation at `internal/cli/run.go:836-850` passes only the slug — no flags/env forwarded. `docs/codebase/memory-core.md:13,52` documents this as engram-CLI-owned.

5. **Antigravity ephemeral hook message** (`inject.go:215-234`) — short, distinct tool-checklist text, not a protocol copy, but additive on top of channel 1 (Antigravity is not a `piEngramProvisioner`).

6. **`skills/_shared/engram-convention.md`** (145 lines) — SDD artifact-naming reference, not the save-trigger protocol, but shares verbatim sentences (capture_prompt semantics, mem_review lifecycle rule, 3-step recovery) with channels 1/2.

## Model Capability Signal

`internal/model/capability.go` (`ModelCapability`) returns `"small"`/`"capable"` and is a real, tested, reusable abstraction — but wired ONLY to per-phase SDD sub-agent assignments (`internal/components/sdd/inject.go:448-460` → `WriteSharedPromptFiles`/`extractModelSection` in `prompts.go:47` and `profiles.go:602`, via `<!-- section:model-capable/small -->` markers). `model.Selection` has NO field for "the model the user's primary interactive session uses" for most agents — gentle-ai has no visibility into that for Claude Code, Cursor, Windsurf, etc. Codex's profile model assignments are the closest per-agent "default model" concept.

## Setup Gating / MCP Config

`engram setup <slug>` receives only the slug (no flags/env) — gentle-ai can gate but not configure hook verbosity today. MCP instructions are served entirely by the external binary; gentle-ai's only levers are command/args.

## Test Surface

`internal/components/engram/inject_test.go` (~44 tests), `setup_test.go`, `internal/components/golden_test.go` + 7 `testdata/golden/engram-*.golden` files, `internal/cli/run_integration_test.go`, `openclaw_orchestration_test.go`, `internal/components/uninstall/cleaners_test.go` (uninstaller must track renamed/removed files), `internal/assets/assets_test.go`.

## Recommendation

Two-phase: (1) internal dedup now — consolidate the Codex triplication via the existing `extractModelSection` section-marker mechanism into a single canonical asset; low risk, no external dependency. (2) Cross-agent capability tiering as a follow-up, blocked on (a) defining a capability signal for agents that lack one and (b) upstream coordination with the gentle-engram project for the SessionStart hook and MCP-instructions channels.

## Risks

- SessionStart hook + MCP instructions are externally owned — tiering those channels requires upstream gentle-engram changes.
- No "primary interactive model" signal for most agents; adding one risks bloating `Selection`/`SyncOverrides`.
- Codex `model_instructions_file` replaces built-in instructions — consolidation must not drop Codex-only content (PASSIVE CAPTURE section).
- 7 golden files + ~44 tests assert current text; strict TDD requires red→green updates.
- Uninstaller (`internal/components/uninstall/cleaners.go`) must track any renamed/removed files or upgrades leave orphans.

## Related

- Prior audit: every-session footprint ~13.6K tokens; Engram protocol ~3x per Claude session (system-prompt section + hook + MCP instructions).
- Change 1 (review-ledger-contract) archived; change 3 (persona canonical-channel) pending.
