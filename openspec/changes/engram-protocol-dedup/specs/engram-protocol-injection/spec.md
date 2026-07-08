# Engram protocol injection Specification

## Purpose

Define observable behavior for how gentle-ai renders and injects the Engram save-trigger protocol into each supported adapter's context, so the same normative content is not duplicated across channels within a single model session, while never breaking `engram setup` on older binaries or dropping adapter-specific content.

## Requirements

### Requirement: Single canonical Codex protocol asset

The system MUST render the Codex AGENTS.md protocol section, the `model_instructions_file`, and the compact prompt from one canonical source asset rather than three independently maintained files. Codex-only content (the PASSIVE CAPTURE section) MUST be preserved in the rendered `model_instructions_file` output. No normative rule present in today's rendered Codex outputs MUST be lost after consolidation.

#### Scenario: Codex outputs render from one source

- GIVEN the canonical protocol asset
- WHEN gentle-ai renders the Codex AGENTS.md section, `model_instructions_file`, and compact prompt
- THEN all three outputs MUST derive from that single canonical asset

#### Scenario: PASSIVE CAPTURE content survives consolidation

- GIVEN the canonical asset includes Codex-only PASSIVE CAPTURE content
- WHEN the `model_instructions_file` is rendered
- THEN the rendered output MUST include the PASSIVE CAPTURE section unchanged

#### Scenario: No normative rule regression

- GIVEN the pre-consolidation rendered Codex outputs (AGENTS.md section, `model_instructions_file`, compact prompt)
- WHEN the same outputs are rendered after consolidation
- THEN every normative rule (MUST/SHALL/SHOULD statement) present before consolidation MUST still be present after

---

### Requirement: Conditional per-adapter section slimming

The system MUST slim an adapter's system-prompt Engram protocol section ONLY when a redundant channel delivering equivalent protocol content is verified for that adapter's runtime. The set of adapters with a verified redundant channel is owned by design. For any adapter where no redundant channel is verified, the system MUST inject the full section unchanged. When a section is slimmed, the slim variant MUST include a pointer directing the model to where the full protocol content lives.

#### Scenario: Verified adapter gets the slim section

- GIVEN an adapter is on the design-verified redundant-channel list
- WHEN gentle-ai injects the Engram protocol section for that adapter
- THEN it MUST inject the slim variant
- AND the slim variant MUST contain a pointer to the full protocol location

#### Scenario: Unverified adapter keeps the full section

- GIVEN an adapter is NOT on the design-verified redundant-channel list
- WHEN gentle-ai injects the Engram protocol section for that adapter
- THEN it MUST inject the full section, unchanged from today's content

---

### Requirement: Version-gated protocol-verbosity forwarding to engram setup

When invoking `engram setup`, the system MUST detect whether the installed engram binary supports a protocol-verbosity option. When supported, it MUST pass the configured verbosity value. When not supported, it MUST invoke setup exactly as today, without any unknown flag, and MUST continue using full-text injection. Setup invocation MUST NOT fail as a result of this detection or forwarding.

#### Scenario: Supported binary receives the verbosity flag

- GIVEN the installed engram binary supports the protocol-verbosity option
- WHEN gentle-ai invokes `engram setup`
- THEN it MUST pass the configured verbosity value to the invocation

#### Scenario: Unsupported binary is invoked unchanged

- GIVEN the installed engram binary does not support the protocol-verbosity option
- WHEN gentle-ai invokes `engram setup`
- THEN it MUST invoke setup with no unknown flags, identical to current behavior
- AND full-text injection MUST remain in effect

#### Scenario: Setup never fails due to verbosity detection

- GIVEN any installed engram binary, supported or unsupported
- WHEN gentle-ai performs verbosity detection and forwarding around `engram setup`
- THEN the setup invocation MUST NOT fail as a result of that detection or forwarding logic

---

### Requirement: Idempotent injection and clean uninstall across upgrades

Re-running injection over a previously-injected section (full or slim) MUST converge to the currently-targeted state using the existing marker-based mechanism, without duplicating or corrupting content. The uninstaller MUST remove any file introduced or renamed by this change, leaving no orphaned assets after an upgrade or uninstall.

#### Scenario: Re-inject converges to target state

- GIVEN a section previously injected as full is now targeted to render as slim (or vice versa)
- WHEN injection runs again
- THEN the resulting section MUST match the newly targeted state via the existing marker mechanism
- AND no duplicate or corrupted markers MUST remain

#### Scenario: Uninstall removes renamed and new assets

- GIVEN files were renamed or newly introduced by the canonical-asset consolidation
- WHEN the uninstaller runs
- THEN it MUST remove those renamed/new files
- AND it MUST NOT leave orphaned protocol asset files behind

---

### Requirement: Test and golden fixture coverage per adapter

Behavior for canonical rendering, per-adapter slim/full selection, and verbosity forwarding MUST be asserted by table-driven tests covering each affected adapter, and MUST be backed by updated golden fixtures reflecting the new rendered output.

#### Scenario: Table-driven coverage per adapter

- GIVEN the set of adapters affected by consolidation or slimming
- WHEN the test suite runs
- THEN each affected adapter MUST have a table-driven test case asserting its expected rendered output

#### Scenario: Golden fixtures reflect new output

- GIVEN a golden fixture exists for an adapter's rendered protocol content
- WHEN that adapter's rendering logic changes under this capability
- THEN the corresponding golden fixture MUST be updated to match and MUST pass in CI
