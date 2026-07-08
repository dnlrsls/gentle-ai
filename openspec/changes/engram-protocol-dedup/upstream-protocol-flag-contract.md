# Upstream contract: `engram setup --protocol=<slim|full>`

This note documents the handoff to the sibling change in `gentle-engram`
(`/home/gentleman/work/engram`) that will implement the actual
`--protocol=<slim|full>` flag on `engram setup`. Implementing that flag is
explicitly **out of scope** for `engram-protocol-dedup` (see `proposal.md`
Out of Scope and Dependencies). This document only records the three
guarantees the upstream change MUST provide for gentle-ai's probe-and-forward
logic (design.md Decision 4, `internal/components/engram/setup.go`
`ProbeProtocolFlag`, `internal/cli/run.go` per-slug forwarding) to work.

## Guarantee 1 — `--protocol` MUST be discoverable via `--help`

`engram setup --help` stdout MUST contain the literal substring `--protocol`
once the flag ships, so `gentle-ai`'s side-effect-free capability probe
(`ProbeProtocolFlag`) can detect support via `strings.Contains(stdout,
"--protocol")`.

- If the flag exists but is undocumented in `--help` output, gentle-ai will
  never forward it — the probe is the ONLY detection mechanism (see design.md
  Decision 4: version-string parsing was rejected because beta/`@main` builds
  carry non-release versions, and try-with-fallback was rejected because
  `engram setup` has side effects and must never run twice).
- Until this guarantee ships, `--protocol` will correctly and permanently be
  treated as unsupported (safe default: full-text injection, current
  behavior, no breakage).

## Guarantee 2 — the probe MUST run with stdin detached, no TTY required

`gentle-ai` invokes `engram setup --help` with `cmd.Stdin` unset (no TTY
attached) and a hard 5-second `exec.CommandContext` deadline. The upstream
binary MUST NOT block waiting for interactive input when `--help` is parsed
with no TTY attached — reads from stdin in that state return immediate EOF.

- Research finding recorded in `design.md` Open Questions (live probe against
  engram 1.18.0, 2026-07-08): `engram setup --help` currently prints the
  interactive agent-selection menu with NO flags section (i.e., `--help` is
  not yet a recognized flag on that binary). This is exactly the scenario
  Guarantee 2 exists for — a menu-printing binary reading from a detached
  stdin must fail fast or exit cleanly, not hang, so gentle-ai's setup loop
  is never blocked by the probe.
- Whatever `--help` handling ships (dedicated flag parsing or otherwise)
  MUST preserve this non-blocking behavior on non-TTY stdin.

## Guarantee 3 — per-slug forwarding is safest-wins

`engram setup <slug>` runs at most once per unique slug (gentle-ai dedups via
`attemptedSlugs`), not once per adapter — `SetupAgentSlug` maps multiple
adapters onto the same slug in at least one case today (Gemini CLI and
Antigravity both resolve to `gemini-cli`). When gentle-ai forwards
`--protocol=<mode>` for a slug, `<mode>` is the **safest verdict across every
adapter sharing that slug** — `slim` is only forwarded when every adapter
mapped to the slug independently verifies slim; any adapter requiring `full`
forces `full` for the whole slug (design.md "Per-slug forwarding semantics").

- Upstream MUST treat `--protocol=slim` as scoped to the setup slug's hook
  output, not to a specific adapter identity — the flag has no adapter-level
  granularity on the gentle-ai side, by design.
- In gentle-ai's current adapter matrix (design.md Decision 1), this is a
  no-op in practice: Claude Code (the only adapter that ever verifies slim)
  does not share its `claude-code` slug with any other adapter. The
  safest-wins rule exists to keep behavior safe if a future adapter is added
  to a slug that Claude Code (or any future slim adapter) also shares.

## Cross-reference

- `proposal.md` → Dependencies: links back to this document.
- `design.md` → Decision 4 (Capability detection) and Decision 5 (Upstream
  flag contract — gentle-engram, specify only) for the full design rationale.
- `internal/components/engram/setup.go` → `ProbeProtocolFlag`.
- `internal/cli/run.go` → per-slug `--protocol` forwarding wiring.
