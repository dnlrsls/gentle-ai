# Delta for persona-behavior-contract

## ADDED Requirements

### Requirement: Canonical Tone Channel for Output-Style-Capable Adapters

For any adapter with an active output-style channel — Claude Code (gated by `SupportsOutputStyles()`) and Kimi (explicit carve-out) — the output-style asset MUST be the exclusive source of persona tone, language, and philosophy. The system-prompt persona section for these adapters MUST be reduced to a residual block containing only `## Rules`, `## Expertise`, `## Contextual Skill Loading (MANDATORY)`, a one-line pointer to the output style, plus any agent-native tooling section identified in the design's disposition tables (Kimi: `## Kimi-native notes`). The residual MUST NOT duplicate tone/language/philosophy content.

#### Scenario: Claude and Kimi residual sections carry no tone content

- GIVEN Claude or Kimi assets are generated with persona `gentleman` or `neutral`
- WHEN the CLAUDE.md or KIMI.md-included persona section is inspected
- THEN it contains only Rules, Expertise, Contextual Skill Loading, a pointer to the output style, and any agent-native tooling section identified in the design's disposition tables (Kimi: `## Kimi-native notes`)
- AND it contains no tone, language, or philosophy prose

---

### Requirement: Persona Section Parity for Non-Output-Style Adapters

Adapters without an active output-style channel MUST continue to receive the full persona section — tone, language, philosophy, rules, expertise, and skill loading — unchanged by this change.

#### Scenario: Non-output-style adapter keeps full persona section

- GIVEN an adapter other than Claude Code or Kimi is generated with a persona
- WHEN its persona asset is inspected
- THEN it contains the full persona section, including tone/language/philosophy, as before this change

---

### Requirement: Persona Drift Reconciliation

Where the system-prompt and output-style assets for a persona variant have drifted into independently authored paraphrases, the output-style asset MUST be reconciled to contain the union of normative tone rules found in either copy. Reconciliation MUST NOT drop a rule present in only one of the two drifted copies.

#### Scenario: Gentleman and Neutral reconciliation preserve rules from both copies

- GIVEN `claude/persona-{variant}.md` and `claude/output-style-{variant}.md` contain divergent normative tone rules, with Neutral using a different section structure than Gentleman
- WHEN the reconciled output-style asset for each variant is produced
- THEN it includes every normative rule that existed in either source for that variant
- AND no rule present in either source is silently dropped

---

### Requirement: Idempotent Convergence on Re-Injection

Upgrading an installation that has an existing full (pre-slim) persona section MUST converge the on-disk system-prompt persona section to the residual block via the marker-delimited section-replace mechanism. Repeated injection MUST be idempotent.

#### Scenario: Existing full section converges to residual, then stays stable

- GIVEN an installation has a full, pre-slim persona section on disk
- WHEN injection runs after this change, then runs again
- THEN the first run replaces the section with the residual block, leaving no orphaned tone content outside the marker boundaries
- AND the second run leaves the on-disk content unchanged

---

### Requirement: Legacy Fingerprint Continuity

Uninstall and auto-heal fingerprint checks that rely on literal markers from the persona section (e.g., `## Rules`, `Senior Architect`) MUST continue to match on-disk content after tone/philosophy content is removed, or the fingerprint constants MUST be updated so uninstall and auto-heal continue to function correctly.

#### Scenario: Uninstall and auto-heal fingerprints still match after slimming

- GIVEN a Claude or Kimi installation has the residual persona section on disk
- WHEN the uninstall cleaner and auto-heal each check their persona-section fingerprint
- THEN both checks match the residual on-disk content
- AND auto-heal does not false-positive a repair

---

### Requirement: Kimi Non-Duplication and Documentation Accuracy

Kimi's `KIMI.md` module includes MUST NOT deliver tone/language/philosophy content twice per session. `docs/agents.md` MUST accurately describe Kimi's output-style status as an active canonical tone channel, not as unsupported.

#### Scenario: Kimi delivers tone content once per session

- GIVEN Kimi assets are installed with an active persona
- WHEN the rendered session content is inspected
- THEN tone/language/philosophy content appears exactly once, sourced from the output-style module

#### Scenario: docs/agents.md reflects Kimi's real output-style status

- GIVEN `docs/agents.md` documents the Output Styles column per adapter
- WHEN the Kimi row is inspected
- THEN it accurately reflects that Kimi has an active output-style channel used as the canonical tone source

---

### Requirement: Per-Adapter Test and Golden Coverage

Automated tests and golden fixtures MUST cover the residual persona section, the reconciled canonical output-style content, and unaffected non-output-style adapter behavior, and MUST be updated in strict-TDD lockstep with implementation changes.

#### Scenario: Claude goldens capture the residual section; Kimi is covered by unit assertions

- GIVEN Claude golden fixtures are regenerated after this change
- WHEN the persona section of the golden is inspected
- THEN it matches the residual block, not the prior full section
- AND, since no Kimi golden fixtures exist, Kimi's residual persona section and reconciled output-style content are covered by `inject_test.go` unit assertions instead

#### Scenario: Non-output-style adapter goldens remain unchanged

- GIVEN a golden fixture for a non-output-style adapter existed before this change
- WHEN the same fixture is regenerated after this change
- THEN its persona section content is unchanged

---

## MODIFIED Requirements

### Requirement: Generic Neutral Asset Parity

All neutral consumers that are not covered by an agent-specific override MUST receive parity through the generic neutral persona or output-style asset. Agent-specific assets MAY adapt wording to platform mechanics, but MUST NOT weaken the neutral behavior contract. For adapters with an active output-style channel (Claude Code, Kimi), parity with the generic neutral requirements MUST be evaluated over the COMBINED persona-residual + output-style channel, not over the persona file in isolation — the residual file alone deliberately omits tone/language/philosophy content that now lives exclusively in the output style.

#### Scenario: Non-agent-specific consumers receive generic neutral parity

- GIVEN an agent or surface consumes the generic neutral persona asset
- WHEN neutral instructions are rendered for that consumer
- THEN the rendered content includes the neutral mentor behavior contract
- AND it includes brevity, one-question, no-menu, verification-first, and artifact-language constraints

#### Scenario: Agent-specific neutral assets do not weaken generic behavior

- GIVEN an agent has its own neutral persona or output-style asset
- WHEN that asset is compared against the generic neutral behavior contract
- THEN it preserves all generic neutral requirements
- AND any agent-specific differences are limited to platform-accurate wording or installation mechanics

#### Scenario: Output-style-capable adapters are evaluated on the combined channel

- GIVEN Claude Code or Kimi has a residual neutral persona section plus a reconciled neutral output-style asset
- WHEN that adapter's neutral behavior contract is compared against the generic neutral requirements
- THEN the combined content of the residual persona section and the output-style asset preserves all generic neutral requirements
- AND the residual persona section alone is not required to independently preserve tone/language/philosophy content
