# Review findings ledger contract

## Purpose

Define the exhaustive first-pass, persisted findings ledger, and scoped re-review contract shared by the 4R review lenses (review-risk, review-resilience, review-readability, review-reliability) and judgment-day, across all supported adapter variants and execution modes.

## ADDED Requirements

### Requirement: Exhaustive first-pass loop-until-dry termination

Each 4R lens and judgment-day judge pass MUST run its first review as a bounded loop that keeps sweeping the diff until a design-defined number of consecutive sweeps (N) yield zero new findings, instead of a single read. The loop MUST be finite.

#### Scenario: First pass loops until dry

- GIVEN a lens begins its first pass on a diff
- WHEN each sweep runs
- THEN the lens MUST keep sweeping until N consecutive sweeps yield no new findings, then stop and finalize the ledger

#### Scenario: Loop is bounded

- GIVEN a first pass reaches the design-defined dry-sweep threshold
- WHEN termination is evaluated
- THEN the lens MUST stop and MUST NOT sweep indefinitely

---

### Requirement: Persisted findings ledger

The first pass MUST emit a structured findings ledger. Each entry MUST identify the finding (id), file:line location, severity, originating lens, and resolution status. Exact field names and the dry-sweep value N are deferred to design.

#### Scenario: Ledger captures required fields

- GIVEN a lens completes an exhaustive first pass
- WHEN it emits the ledger
- THEN each entry MUST include an id, file:line location, severity, lens, and status

#### Scenario: Zero findings still produce a ledger record

- GIVEN a first pass finds nothing
- WHEN the ledger is finalized
- THEN the system MUST persist an empty ledger record rather than skip persistence

---

### Requirement: Ledger persistence honors the artifact store

Ledger persistence MUST follow the session's configured artifact store: an OpenSpec change artifact when the store is `openspec`, an Engram topic when the store is `engram`, or in-context only (no file or topic write) when the store is `none`.

#### Scenario: Store selects persistence target

- GIVEN the artifact store is `openspec`, `engram`, or `none`
- WHEN a lens finalizes its ledger
- THEN it MUST be persisted respectively as a change artifact, an Engram topic scoped to change and lens, or kept in-context only

#### Scenario: None store writes nothing

- GIVEN the artifact store is `none`
- WHEN a lens finalizes its ledger
- THEN no file or Engram artifact MUST be written

---

### Requirement: Scoped re-review contract

A re-review pass MUST take the persisted ledger and the fix diff as input and scope its work to (a) verifying each ledger finding's resolution and (b) reviewing only fix-touched lines. It MUST NOT re-read the full original diff. A finding on an untouched line MUST be logged as a first-pass quality signal and MUST NOT by itself trigger another full round.

#### Scenario: Re-review verifies ledger findings within scope

- GIVEN a persisted ledger with open findings and a fix diff addressing them
- WHEN the re-review pass runs
- THEN it MUST update each finding's status to resolved or still-open
- AND it MUST NOT perform a fresh full read of lines outside the fix diff

#### Scenario: Untouched-line finding is logged, not escalated

- GIVEN a re-review observes an issue on an untouched line
- WHEN the finding is recorded
- THEN it MUST be appended to the ledger tagged as a first-pass quality signal
- AND it MUST NOT by itself cause a full review round to restart

---

### Requirement: Judgment-day ledger and scoped re-judge

Judgment-day's judge agents (jd-judge-a, jd-judge-b) MUST apply the same exhaustive first-pass and persisted-ledger contract, and the re-judge pass following jd-fix-agent MUST follow the same scoped re-review contract as the 4R lenses.

#### Scenario: Judgment-day first pass is exhaustive and ledgered

- GIVEN jd-judge-a or jd-judge-b runs a first judgment pass
- WHEN the pass completes
- THEN it MUST loop until dry and persist a findings ledger per the artifact-store contract

#### Scenario: Re-judge is scoped

- GIVEN jd-fix-agent has applied fixes for ledgered findings
- WHEN the re-judge pass runs
- THEN it MUST verify ledger findings and review only fix-touched lines

---

### Requirement: Contract coverage across adapter variants and execution modes

The exhaustive-pass, ledger, and scoped re-review contract MUST be present across all 13 supported adapter variants and both execution modes: subagent mode (adapters with dedicated review-* and jd-* subagents: Claude, Kiro, Cursor, Kimi) and inline mode (all other supported adapters, running review lenses inline in the orchestrator).

#### Scenario: Both execution modes carry the contract

- GIVEN an adapter with dedicated subagents, or one without
- WHEN its review-*/jd-* assets, or its orchestrator's inline-lens section, are inspected
- THEN they MUST contain the first-pass, ledger, and scoped re-review contract worded for that execution mode

#### Scenario: No adapter variant is left uncovered

- GIVEN all 13 adapter variants are enumerated
- WHEN contract coverage is evaluated
- THEN every variant MUST include the contract in its review and judgment-day-related assets
