# Design: Review ledger contract

## TL;DR

Codify one canonical review contract — exhaustive first pass, a persisted findings ledger, and a scoped re-review/re-judge loop — as a single authored source, then hand-replicate its normative clauses into every review-* subagent asset, every orchestrator inline-lens section, and the judgment-day assets across all 13 adapter families. A table-driven consistency test asserts each variant carries the canonical clauses, so drift across 13 adapters and two execution modes fails the build instead of shipping. This is a prompt-and-test change centered on `internal/assets` and `internal/components/sdd`; no runtime model/API semantics change.

Lens codes are the existing ones: **R1** risk, **R2** readability, **R3** reliability, **R4** resilience, **JD** judgment-day.

---

## Architecture Decisions

### ADR 1 — One canonical contract, hand-replicated, test-enforced (not build-time generated)

**Choice.** Author the contract once in `internal/assets/skills/_shared/review-ledger-contract.md` (source of truth). Hand-copy its normative clauses into each static asset surface. Enforce parity with a Go table-driven test over a `requiredLedgerClauses` constant slice.

**Alternatives considered.** (a) Render the block from Go and inject it via a marker section, mirroring `RenderTriggerRules` (`inject.go:303-349`). (b) Fully hand-maintain per adapter with no enforcement.

**Rationale.** The contract lands on heterogeneous surfaces: agent front-matter files that are copied whole (review-*.md, jd-*.md are never marker-injected), orchestrator prompt sections, and a skill reference doc. A single injected block does not fit all three cleanly, and only trigger-rules currently uses the Go-render path. Hand-replication matches the repo's existing asset-invariant discipline (the persona-language change guarded phrases by test, not generation). Adding a generator for static prompt prose is over-engineering; the risk it mitigates (drift) is fully covered by a presence test. Option (b) is rejected because 13-adapter drift is the top risk in the proposal.

### ADR 2 — Canonical ledger schema (single rendering, all variants)

**Choice.** One schema, rendered identically everywhere:

| Field | Type / values | Notes |
|-------|---------------|-------|
| `id` | `{LENS}-{NNN}` e.g. `R1-001`, `JD-004` | Zero-padded, sequential per lens per session. |
| `lens` | `risk \| readability \| reliability \| resilience \| judgment-day` | Matches R1/R2/R3/R4/JD codes. |
| `location` | `path/to/file.ext:line` or `:start-end` | Required; anchors scoped re-review. |
| `severity` | `BLOCKER \| CRITICAL \| WARNING \| SUGGESTION` | Reuses the existing review Output contract enum. |
| `status` | `open \| fixed \| verified \| wont-fix \| info` | Lifecycle; see below. |
| `evidence` | free text | Why it matters (kept from current contract). |

**Status lifecycle.** `open` (first-pass finding) → `fixed` (fix agent changed code) → `verified` (re-review confirmed resolved). `wont-fix` = accepted/deferred with reason. `info` = a new finding on untouched lines (first-pass quality signal, NOT a re-round trigger) and JD `WARNING (theoretical)` items.

**Alternatives considered.** Per-lens bespoke schemas; a separate JD severity vocabulary (`WARNING (real|theoretical)`).

**Rationale.** A shared schema is the only way the orchestrator can merge subagent and inline findings into one ledger. JD's real/theoretical distinction collapses onto `severity=WARNING` + `status` (`open` vs `info`), so JD and 4R write the same table.

### ADR 3 — Bounded loop-until-dry: N and overrides

**Choice.** Default termination `N = 2` consecutive dry sweeps (a sweep that finds nothing new). Hard ceiling `4` sweeps per lens regardless of N. Per-lens overrides: **R1 Risk** stays at N=2 (security stakes, no reduction); **R2 Readability** MAY use N=1 (suggestion-heavy, cheap to re-run). JD judges use N=2 to match the dual-blind design.

**Alternatives considered.** Single pass (status quo); unbounded loop-until-dry.

**Rationale.** N=2 catches the single-pass sampling gap the exploration identified while the ceiling caps the cost risk flagged in the proposal. R2 relaxation trims cost where findings are lowest-severity.

### ADR 4 — Persistence branches on the artifact store

**Choice.**
- **openspec:** `openspec/changes/{change-name}/review-ledger.md` (markdown table). JD tied to a change writes the same file.
- **engram:** topic key `sdd/{change-name}/review-ledger`, type `review` (upsert). Ad-hoc JD not tied to a change: `review/{target-slug}/ledger`.
- **none:** keep the ledger inline in the review response; do NOT write files or Engram. Exact wording: "Artifact store `none`: the ledger lives only in this conversation; complete the review → fix → re-review loop within the session because it is not persisted across compaction."

**Rationale.** Mirrors the existing store policy (`config.yaml` artifact-store modes) and the persistence-contract shared asset. Branching explicitly is the mitigation named in the proposal's persistence-mismatch risk.

### ADR 5 — One conceptual contract, two execution-mode clauses

**Choice.** The canonical block states normative clauses once, then a mode switch names WHO holds the ledger:
- **Subagent mode** (Claude, Kiro, Cursor, Kimi): each review-* / jd-* agent runs its lens exhaustively, returns its own ledger rows; the orchestrator merges into the persisted ledger.
- **Inline mode** (Codex, Gemini, Qwen, OpenCode/Kilocode, Windsurf, Antigravity, Hermes, generic): the orchestrator runs each lens sequentially in its own context and maintains the merged ledger directly.

**Rationale.** The proposal requires one contract asserted for both modes. Only ledger ownership differs; exhaustive-pass, schema, persistence, and scoped re-review are identical, which keeps the `requiredLedgerClauses` test able to assert the same sentences on both surfaces.

---

## Data Flow — review → ledger → fix → re-review loop

```mermaid
sequenceDiagram
    participant Orch as Orchestrator
    participant Lens as Review lens (subagent or inline)
    participant Ledger as Findings ledger (store-branched)
    participant Fix as Fix agent
    participant Re as Scoped re-review

    Orch->>Lens: Run lens on full diff
    loop until N consecutive dry sweeps (ceiling 4)
        Lens->>Lens: Sweep for new findings
    end
    Lens->>Ledger: Emit rows {id, lens, location, severity, status=open, evidence}
    Orch->>Ledger: Persist per artifact store (openspec file | engram topic | none/inline)
    Orch->>Fix: Fix confirmed open findings (ask first per JD rules)
    Fix->>Ledger: Set status=fixed on addressed ids
    Orch->>Re: Input = ledger + fix diff
    Re->>Ledger: Each open/fixed id → verified | still-open
    Re->>Re: Review ONLY fix-touched lines
    alt New finding on untouched line
        Re->>Ledger: Log status=info (first-pass signal, NOT a new round)
    end
    Re-->>Orch: Verdict; loop only while confirmed ids remain unverified
```

---

## File Changes

| File | Action | Description |
|------|--------|-------------|
| `internal/assets/skills/_shared/review-ledger-contract.md` | Create | Canonical contract: exhaustive pass, schema, persistence branches, scoped re-review, two-mode clause. Authoring source of truth. |
| `internal/assets/{claude,cursor,kimi,kiro}/agents/review-{risk,readability,reliability,resilience}.md` | Modify (16) | Extend Output contract with ledger emission + exhaustive-pass + scoped re-review (subagent mode). |
| `internal/assets/{claude,codex,cursor,gemini,kimi,kiro,qwen,windsurf,antigravity,hermes,generic}/sdd-orchestrator.md` + OpenCode/Kilocode inline path | Modify (12 + inject) | Add a "Review execution contract" section (inline mode + ledger merge/persist). |
| `internal/assets/{claude,kiro}/agents/jd-{judge-a,judge-b,fix-agent}.md` | Modify (6) | Ledger emission + scoped re-judge. |
| `internal/assets/skills/judgment-day/SKILL.md`, `references/prompts-and-formats.md` | Modify | Same ledger + scoped re-judge in judge/fix prompt templates and hard rules. |
| `internal/components/sdd/*_test.go` | Create/Modify | Table-driven `requiredLedgerClauses` consistency test across all variant assets + `severity`/`status`/`id`-format golden. |
| `internal/testdata/golden/*` | Modify | Only if a rendered orchestrator/prompt output surfaces the new wording; trigger-rules golden is unchanged. |

## Interfaces / Contracts

Canonical ledger row (rendered identically in every asset):

```
| id     | lens        | location            | severity | status | evidence            |
|--------|-------------|---------------------|----------|--------|---------------------|
| R1-001 | risk        | internal/x.go:42    | CRITICAL | open   | secret hardcoded    |
| JD-004 | judgment-day| internal/y.go:88    | WARNING  | info   | theoretical path    |
```

## Testing Strategy (strict TDD active)

| Layer | What to Test | Approach |
|-------|-------------|----------|
| Unit | Every variant asset contains each `requiredLedgerClauses` clause | Table-driven test iterating the 13 families × surfaces; RED before edits |
| Unit | Schema fixture (id format `{LENS}-{NNN}`, severity enum, status enum, N=2/ceiling wording) | Golden string assertions against the canonical `_shared` file |
| Unit | Two-mode parity: subagent and inline surfaces both assert exhaustive-pass, ledger, scoped re-review | Shared clause slice reused for both surface groups |
| Integration | Persistence branch wording present for openspec / engram / none | Assert store-conditional sentences in orchestrator asset |
| Golden | Rendered prompt/trigger output unchanged where not intended | Regenerate only affected goldens after clause tests pass |

RED→GREEN→REFACTOR: write the failing clause/consistency tests first (they fail because no asset carries the contract), author the `_shared` source, replicate into assets until green, regenerate goldens last.

## Migration / Rollout

No data migration. File-level only. Forecast diff is broad (24+ assets); if it exceeds the 400-line review budget, split into work units: (1) tests + `_shared` source, (2) review-* subagent assets, (3) orchestrator inline sections, (4) judgment-day assets, (5) golden regeneration. Rollback = revert the asset-normalization and test/golden commits per work unit; re-run `internal/components/sdd` and golden tests to confirm baseline.

## Open Questions

- [x] For `none` store on a very long review, is inline-only acceptable, or should we recommend forcing engram? Accepted as-is: inline-only with the explicit compaction caveat, now propagated to every adopting asset via the hand-copied `none` bullet (JD-005 fix).
- [x] Ad-hoc JD (no change dir) Engram key `review/{target-slug}/ledger` — resolved: deterministic rule `target-slug` = `pr-{number}` when reviewing a PR, else the current branch name kebab-cased, else a kebab-case slug of the user-stated review target, now normative in all assets (JD-002 fix).
