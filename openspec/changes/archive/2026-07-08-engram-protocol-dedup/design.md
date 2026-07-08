# Design: Engram protocol dedup

## TL;DR

Two independent levers, decoupled on purpose:

1. **Section slimming (gentle-ai-owned, ships now).** Replace the three source assets (`claude/engram-protocol.md`, `codex/engram-instructions.md`, `codex/engram-compact-prompt.md`) with ONE canonical marker-sectioned source, `internal/assets/engram/protocol.md`. `inject.go` selects the slim section ONLY for adapters with a design-verified redundant channel; for Claude Code that verification additionally requires the installed engram binary to be ≥ v1.4.0 (see Decision 1). Only **Claude Code** qualifies. Codex renders its three surfaces from the same source (dedup, not slimming). Written on-disk paths stay identical, so the uninstaller does not churn.
2. **Hook verbosity flag (upstream-gated).** Forward `engram setup <slug> --protocol=slim|full` only when a side-effect-free `--help` probe proves the installed binary supports it. Omission is always safe.

Slimming Claude Code is safe only when the installed engram binary is verified to serve the MCP `instructions` channel — it is NOT true for every engram binary ever released (see Decision 1 for the evidence-backed version floor). Once verified, that channel carries the full protocol regardless of the `--protocol` flag; the flag itself (Decision 4) is a separate, independently-gated optimization for the SessionStart hook, not a precondition for slimming.

---

## Decision 1 — Per-adapter redundant-channel verification matrix

Rule: an adapter may be **slim** only with ≥1 VERIFIED redundant channel that delivers the full protocol independent of the CLAUDE.md section. Inconclusive → **full** (safe default). For Claude Code specifically, verification additionally requires the installed engram binary to meet the version floor established below; below-floor or unparseable/unknown versions fall back to **full**.

**Version-floor evidence (researched 2026-07-08 in the sibling `gentle-engram` repo, read-only, via `git log`/`git show`/`git describe`/`git merge-base`):** `server.WithInstructions(serverInstructions)` — the call that actually wires the MCP `instructions` field — was introduced together with the `serverInstructions` const in commit `281bb1e` ("feat(mcp): add tool profiles and version to TUI dashboard"). `git describe --tags 281bb1e` resolves to `v1.3.1-4-g281bb1e` (4 commits after `v1.3.1`), and `git merge-base --is-ancestor 281bb1e v1.4.0` confirms the commit is included in `v1.4.0`, while `git merge-base --is-ancestor 281bb1e v1.3.1` fails — i.e. `v1.3.1` and earlier do NOT carry the instructions channel. **Version floor: engram ≥ v1.4.0.** Corroborating live evidence: the current design-review session, running engram **1.18.0**, demonstrably received the MCP instructions block. The gate reuses the `VerifyVersion()` shell-out mechanism (`internal/components/engram/verify.go`, which runs the `engram version` subcommand), but requires NEW plumbing: today `VerifyVersion()` is only called inside the post-install health check at `internal/cli/run.go:1587`, which discards the version string, and `InjectOptions` (`inject.go`) has no version field. See File Changes for the required `inject.go` wiring and test seam.

| # | Adapter | `engram setup` slug (hook) | MCP `instructions` surfaced to model | Verdict | Evidence |
|---|---------|---------------------------|--------------------------------------|---------|----------|
| 1 | Claude Code | `claude-code` (SessionStart hook) | **Verified (engram ≥ v1.4.0)** | **slim** | Both channels; hook installed by setup, MCP instructions rendered by Claude Code. Gated on `VerifyVersion` ≥ v1.4.0 (see evidence above); below floor or unknown/unparseable version → full (safe default). |
| 2 | Codex | `codex` | unverified | **defer/full** | Channel-1 AGENTS.md stays full; the change consolidates its 3 owned copies. |
| 3 | OpenCode | `opencode` | unverified | full | Hook target exists but no verified protocol-injection into context. |
| 4 | Kilocode | `kilocode` | unverified | full | Same as OpenCode. |
| 5 | Gemini CLI | `gemini-cli` | unverified | full | No verified SessionStart-equivalent that injects protocol prose. |
| 6 | Antigravity | `gemini-cli` + ephemeral hook | unverified | full | Ephemeral hook is a tool-checklist, NOT a protocol copy → not redundant. |
| 7 | Windsurf | `windsurf` | unverified | full | No verified context-injecting hook. |
| 8 | Qwen Code | none | unverified | full | Direct settings injection only. |
| 9 | Cursor | none | unverified | full | No setup slug. |
| 10 | VS Code Copilot | none | unverified | full | No setup slug. |
| 11 | Kiro | none | unverified | full | No setup slug. |
| 12 | Kimi | none (Jinja include) | unverified | full | Gets the same full asset via `{% include %}`. |
| 13 | Trae | none | unverified | full | No setup slug. |
| 14 | Hermes | none | unverified | full | YAML MCP injection only. |
| 15 | OpenClaw | none | unverified | full | Workspace injection, no slug. |
| 16 | Pi | n/a (early return) | n/a | **no protocol text (MCP-only precedent)** | `piEngramProvisioner` writes zero protocol text — relies entirely on the MCP `instructions` channel; existing precedent for the Decision 1 verification approach, not a "slim section" case. |

Net effect: exactly **one** adapter (Claude Code) flips to slim in this change.

## Decision 2 — Slim section content and size

Exact slim block (marker `<!-- section:slim -->`), injected into CLAUDE.md under the existing `engram-protocol` section id:

```markdown
## Engram Persistent Memory

Engram persistent memory is ACTIVE. The full protocol (save format, lifecycle,
search flow, after-compaction steps) is delivered every session by the Engram
MCP server instructions and the SessionStart hook. Always-on rules:

- Call `mem_save` PROACTIVELY after any decision, bugfix, discovery, convention,
  or config change — do not wait to be asked. Use `capture_prompt: false` for
  automated/SDD artifacts.
- On any reference to past work: `mem_context` → `mem_search` → `mem_get_observation`.
- Before saying "done", call `mem_session_summary`.
```

Size: ~11 lines / ~130 tokens vs. the 101-line / ~1,175-token full asset → **~1,045 tokens/session saved** on Claude Code, plus ~2.5K tokens from the upstream hook once the `--protocol` flag lands. This savings figure applies to Claude Code only — Codex's per-session token footprint does not shrink (see Decision 3): it goes from 3 near-duplicate gentle-ai-owned copies to 1 canonically-sourced copy, and the rendered `model_instructions_file` grows slightly (~6 bullets + self-check line) rather than shrinking.

## Decision 3 — Canonical asset structure for Codex

Single source `internal/assets/engram/protocol.md` with four **paired** markers (open + close, matching `extractModelSection`'s pairing requirement — a lone opening marker is not sufficient to bound a section), rendered by a local helper `extractProtocolSection(content, name)` in the `engram` package (marker logic identical to `sdd.extractModelSection`, kept local to avoid cross-package coupling):

| Marker pair | Feeds surface | Renderer |
|-------------|---------------|----------|
| `<!-- section:full -->` … `<!-- /section:full -->` (byte-identical to today's `claude/engram-protocol.md`) | CLAUDE.md/GEMINI.md/etc. full section; Kimi Jinja module | `protocolFull()` |
| `<!-- section:slim -->` … `<!-- /section:slim -->` | Claude Code CLAUDE.md | `protocolSlim()` |
| `<!-- section:passive-capture -->` … `<!-- /section:passive-capture -->` (Codex-only) | Codex `model_instructions_file` | `codexInstructions()` = `protocolFull()` content concatenated with the extracted `passive-capture` section (full + passive-capture, in that order) |
| `<!-- section:compact -->` … `<!-- /section:compact -->` | Codex `experimental_compact_prompt_file` | `codexCompact()` |

Keeping `full` byte-identical to the current Claude asset means every non-slim system-prompt golden (e.g. `engram-antigravity-rulesmd.golden`) stays unchanged. This does **not** mean Codex output is unchanged: today's `codex/engram-instructions.md` (104 lines, 6 "WHEN TO SAVE" bullets, no self-check line) is NOT byte-identical to the Claude `full` section (101 lines, 12 "PROACTIVE SAVE TRIGGERS" bullets, plus a self-check line). Rendering `codexInstructions()` from the canonical `full` text is a **real content change** for Codex — it gains ~6 additional save-trigger bullets and the self-check line, so the rendered `model_instructions_file` output grows, not shrinks. This is source-level dedup (one maintained copy instead of a near-duplicate), not a token-reduction for Codex; see Decision 2 sizing, which applies to Claude Code only. A new golden fixture for the rendered Codex `model_instructions_file` and `experimental_compact_prompt_file` output must be added (see Testing Strategy) since the existing `TestInjectCodexWritesInstructionFiles` only does loose `Contains` assertions and would not catch the content growth. PASSIVE CAPTURE is preserved by construction (concatenation, not replacement) and asserted in tests.

Note on ordering (JD-018): `codexInstructions()` intentionally renders the PASSIVE CAPTURE section **after** the full section's `AFTER COMPACTION` block (concatenation order — `protocolFull()` then `protocolPassiveCapture()`). In the original `codex/engram-instructions.md` asset, PASSIVE CAPTURE appeared *before* `AFTER COMPACTION`. The reorder is intentional — a byproduct of the append-only concatenation strategy chosen for simplicity and safety over reassembling the two source assets' original interleaving — and content byte-preserved: only the position of the (byte-identical) passive-capture block within the rendered output moves; no text is added, removed, or altered.

## Decision 4 — Capability detection

**Choice: side-effect-free `engram setup --help` probe**, grepping stdout for `--protocol`.

This probe cannot be implemented via `runCommand`/`executeCommand` (`internal/cli/run.go:1123-1141`, `executeCommand`): on the success path that seam discards `CombinedOutput()` and returns only `error`, so no stdout is ever available to the caller, and its `func(name string, args ...string) error` signature is faked at ~159 test call sites — changing it would ripple across the whole test suite. Instead, extend the existing output-capturing precedent in `internal/components/engram/verify.go` (`VerifyVersion` uses `execCommand(...).Output()`): add a new **`ProbeProtocolFlag(ctx context.Context) (string, error)`** helper in `internal/components/engram/setup.go`, built on the same package-level `execCommand` seam (fakeable in tests, same pattern as `VerifyVersion`). It runs `engram setup --help` via `exec.CommandContext(ctx, ...)` with a hard **5-second** deadline, leaves `cmd.Stdin` unset/detached (no TTY attached, so a menu-printing binary can never block the setup loop waiting for interactive input), and returns captured stdout via `.Output()`.

`run.go` calls `ProbeProtocolFlag()` **once** before the adapter loop, caches the result in a local `protocolFlagSupported bool` (`strings.Contains(stdout, "--protocol")`), and forwards `--protocol=<mode>` once per unique slug (see Per-slug forwarding semantics below). Timeout, non-zero exit, and any other error all degrade uniformly to `protocolFlagSupported = false` (omit the flag — safe default, today's behavior).

| Alternative | Rejected because |
|-------------|------------------|
| Version parse (`--version` + semver min) | Brittle; beta/`@main` builds carry non-release versions. |
| try-with-fallback (run flag, retry on error) | `engram setup` has side effects (installs hooks) — never run it twice. |

Behavior matrix:

| Binary | Adapter | Action |
|--------|---------|--------|
| new + slim adapter | Claude Code | `engram setup claude-code --protocol=slim` |
| new + full adapter | others | `--protocol=full` (or omit; `full` is default) |
| old (no `--protocol`) | any | omit flag → today's behavior, no breakage |

Section slimming in `inject.go` is **not** gated on this `--help` probe. It is gated separately, on the engram-version floor from Decision 1 (`VerifyVersion` ≥ v1.4.0) — the two gates are independent because the MCP instructions channel and the `--protocol` hook flag are unrelated features.

## Decision 5 — Upstream flag contract (gentle-engram — specify only)

- **Flag:** `engram setup <slug> --protocol=<slim|full>`, **default `full`** (absent flag = current behavior).
- **Slim hook output:** KEEP the dynamic status line (e.g. `N chunks imported`) plus a one-line proactive-save reminder; DROP the verbose protocol prose (already in MCP instructions).
- **MCP instructions:** UNCHANGED by this flag — they remain the canonical full-protocol channel.
- **Compatibility:** old gentle-ai + new engram → no flag passed → `full`. New gentle-ai + old engram → probe omits flag → runs as today. New + new → `--protocol=slim` for Claude Code.

## Per-slug forwarding semantics

`engram setup` runs at most once per **unique slug**, not once per adapter: `run.go:838-850` dedups via a local `attemptedSlugs map[string]struct{}`, and `SetupAgentSlug` (`internal/components/engram/setup.go:40`) maps multiple adapters onto the same slug in at least one case — Gemini CLI and Antigravity both resolve to `gemini-cli`. (OpenCode and Kilocode map to distinct slugs, `opencode` and `kilocode` respectively — no sharing there.) Because `--protocol` is forwarded once per slug (not once per adapter), the forwarded value must be the verdict for that SLUG, computed across every adapter mapped to it.

**Rule:** if adapters sharing a slug have divergent per-adapter verdicts, the safest verdict wins — any adapter requiring `full` forces `full` for the whole slug (`--protocol=slim` is only forwarded when every adapter sharing that slug independently verifies `slim`). In the current matrix (Decision 1) this is a no-op: Claude Code (the only `slim` adapter) does not share its `claude-code` slug with any other adapter. The rule exists to keep the behavior safe as new adapters are added to a shared slug.

## Sequence — setup → detect → forward → emission

```mermaid
sequenceDiagram
    participant Run as run.go (once)
    participant Bin as engram binary
    participant Inject as engram.Inject
    participant Model as Model session

    Run->>Bin: ProbeProtocolFlag(): engram setup --help (5s CommandContext deadline, stdin detached)
    Bin-->>Run: captured stdout (or timeout/non-zero exit)
    Run->>Run: protocolFlagSupported = contains(stdout, "--protocol")
    loop per adapter (setup call skipped once its slug has been attempted)
        alt slug not yet attempted (attemptedSlugs)
            Run->>Run: slugVerdict = safest (full-wins) verdict across adapters sharing this slug
            alt supported && slugVerdict known
                Run->>Bin: engram setup <slug> --protocol=<slim|full>
            else unsupported
                Run->>Bin: engram setup <slug>
            end
            Bin-->>Bin: install SessionStart hook (slim|full)
        end
        Run->>Inject: InjectWithOptions(adapter)
        Inject->>Inject: protocolFull()/protocolSlim() per adapter verdict
        Inject-->>Model: system-prompt section (slim only for Claude Code)
    end
    Note over Bin,Model: MCP instructions carry full protocol when engram >= v1.4.0 is verified (Decision 1); otherwise CLAUDE.md stays full
```

## File Changes

| File | Action | Description |
|------|--------|-------------|
| `internal/assets/engram/protocol.md` | Create | Canonical marker-sectioned source (full/slim/passive-capture/compact). |
| `internal/assets/claude/engram-protocol.md` | Delete | Superseded by canonical `full` section. |
| `internal/assets/codex/engram-instructions.md`, `engram-compact-prompt.md` | Delete | Rendered from canonical source. |
| `internal/components/engram/inject.go` | Modify | `extractProtocolSection` helper + `protocolFor(agent)` selection; Codex render helpers in `writeCodexInstructionFiles`. Add a `Version` field to `InjectOptions` (raw version value — a boolean cannot support the at-floor boundary comparison), threaded from a `VerifyVersion()` call in `run.go` before injection, so the Claude-slim decision can be gated on the version floor from Decision 1. |
| `internal/components/engram/setup.go` | Modify | New `ProbeProtocolFlag(ctx)` output-capturing seam (`execCommand(...).Output()`, `exec.CommandContext` 5s deadline, stdin detached) following the `VerifyVersion` precedent in `verify.go`; plus `--protocol` mode helper. |
| `internal/components/engram/verify.go` (or `inject.go`) | Modify | Add an exported test seam (e.g. `SetVersionForTest`, or fake via the existing `execCommand` variable) so golden/integration tests can pin the `engram version` result feeding the Decision 1 gate. |
| `internal/cli/run.go` (836-850, 1587) | Modify | Call `ProbeProtocolFlag()` once before the adapter loop; cache `protocolFlagSupported`; forward `--protocol` per slug (safest-wins across shared slugs, see Per-slug forwarding semantics). The existing `VerifyVersion()` call at line 1587 (post-install health check) currently discards its result — thread that (or a new) `VerifyVersion()` call's output into `InjectOptions.Version` before injection. |
| `internal/components/engram/download.go` | Modify (if needed) | Expose installed binary path/version signal only if probe needs it. |
| `internal/components/uninstall/{cleaners,service}.go` | Verify | On-disk paths unchanged (`~/.codex/engram-instructions.md`, `engram-compact-prompt.md`, CLAUDE.md `engram-protocol` marker); add regression assertion. |
| `testdata/golden/engram-claude-claudemd.golden` | Modify | Regenerate to slim content. |
| `testdata/golden/combined-claude-claudemd.golden` | Modify | Also embeds the full `engram-protocol` section (verified: 2 marker hits) — regenerates alongside `engram-claude-claudemd.golden`. |
| `testdata/golden/engram-codex-instructions.golden`, `engram-codex-compact-prompt.golden` (new) | Create | New goldens for the rendered Codex `model_instructions_file` and `experimental_compact_prompt_file` output, to catch the content growth from consolidating onto the canonical `full` text (see Decision 3). |

## Testing Strategy (strict TDD)

| Layer | What | Approach |
|-------|------|----------|
| Unit | Canonical renderers: `full` == old Claude text, `codexInstructions()` = `full` + passive-capture (content growth vs. today's Codex asset asserted explicitly, not just `Contains`), `compact` intact, `slim` matches spec | RED before creating source; assert paired markers. |
| Unit | Per-adapter slim/full table (16 rows): Claude Code → slim (gated on engram ≥ v1.4.0), all others → full, Pi → empty | New table-driven test in `inject_test.go`; include a below-floor/unknown-version case asserting fallback to full. |
| Unit | Version-gate wiring: `InjectOptions.Version` below-floor or unknown/unparseable → fall back to full; ≥ v1.4.0 (inclusive — at-floor v1.4.0 is the boundary case that MUST permit slim) → slim | New test in `inject_test.go` using the new test seam (`SetVersionForTest` or faked `execCommand`) to pin the `engram version` result; include exact-floor v1.4.0 as an explicit boundary assertion. |
| Unit | `ProbeProtocolFlag()`: canned `engram setup --help` outputs — with `--protocol`, without `--protocol`, context-deadline timeout, non-zero exit — all four must degrade to "flag unsupported" (omit) except the with-flag case | Fake the `execCommand` seam (same pattern as `verify_test.go`'s `VerifyVersion` fakes); assert `run.go`'s `protocolFlagSupported` value and forwarded argv. |
| Golden | Of the 7 `engram-*.golden` fixtures, exactly 1 changes (`engram-claude-claudemd.golden`) and 6 stay byte-stable; only 2 of the 7 contain protocol prose at all (`engram-claude-claudemd.golden`, `engram-antigravity-rulesmd.golden`) — the other 5 are MCP/settings-config-only JSON, unaffected by section content. `combined-claude-claudemd.golden` is a SEPARATE fixture family (not matched by the `engram-*` glob) that also regenerates, since it embeds the full `engram-protocol` section. | Regenerate `engram-claude-claudemd.golden` and `combined-claude-claudemd.golden` after clause tests pass. |
| Golden | `combined-windsurf-global-rules.golden` also embeds the full `engram-protocol` section (verified: 2 marker hits) but stays unchanged, since Windsurf remains `full` in this change | Re-verify byte-stable; no regeneration needed. |
| Golden | New Codex fixtures for `model_instructions_file` and `experimental_compact_prompt_file` capture the content growth from consolidating onto the canonical `full` text | Add `engram-codex-instructions.golden` / `engram-codex-compact-prompt.golden`. |
| Uninstall | Codex/Claude artifacts still removed after consolidation | Extend `cleaners_test.go`; assert no orphaned renamed files. |

Of the ~44 inject tests, those asserting Claude full text flip to slim (RED→GREEN); the rest re-verify full. Of the 7 existing `engram-*.golden` fixtures, 1 changes (`engram-claude-claudemd`), 6 re-verify unchanged; `combined-claude-claudemd.golden` (a separate fixture family) also changes; plus 2 new Codex goldens are added — the proposal's "7" undercounted the golden-fixture impact.

## Migration / Rollout

No data migration. File-level. Section slimming ships independently; the `--protocol` flag activates only against a supporting binary. Rollback = revert the canonical-asset + inject/run/setup commits and restore the three source assets and the claude golden; re-run `go test ./...` and `go vet ./...`.

## Open Questions

- [x] Confirm engram exposes `engram setup --help` listing subcommand flags — RESOLVED by live probe against installed engram 1.18.0 (2026-07-08): `engram setup --help` prints the interactive agent menu with NO flags section, and `engram version` returns `engram 1.18.0`. Consequences adopted: (a) the probe remains correct for old binaries — grepping the help output for `--protocol` finds nothing → flag omitted, which is the desired fallback; (b) the upstream sibling-change contract MUST require that `--protocol` appears in `engram setup --help` stdout, otherwise the probe can never detect support; (c) the probe MUST run with stdin detached/non-TTY so a menu-printing binary can never hang the setup loop waiting for interactive input.
- [ ] Antigravity `engram-convention.md` overlap — left as follow-up per proposal Out of Scope.
