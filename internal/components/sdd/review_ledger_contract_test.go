package sdd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gentleman-programming/gentle-ai/internal/agents"
	"github.com/gentleman-programming/gentle-ai/internal/assets"
	"github.com/gentleman-programming/gentle-ai/internal/model"
)

// requiredLedgerClauses are the normative sentence fragments that MUST appear
// verbatim in every review-*, jd-*, orchestrator "Review Execution Contract",
// and judgment-day skill asset. They encode the exhaustive first-pass loop,
// the persisted findings-ledger schema, the artifact-store persistence
// branches, and the scoped re-review contract defined in
// openspec/changes/review-ledger-contract/{spec,design}.md.
//
// Every clause is chosen so it does NOT already exist in any unmodified asset
// (verified against the pre-change repo state) — this keeps the first run of
// TestRequiredLedgerClauses a true RED before any asset is edited.
var requiredLedgerClauses = []string{
	// Exhaustive first-pass loop-until-dry (spec: "First pass loops until dry", "Loop is bounded").
	"Loop until dry: sweep the diff repeatedly until N consecutive sweeps yield zero new findings",
	"Default N = 2 consecutive dry sweeps",
	"R2 Readability MAY use N = 1",
	"Hard ceiling: 4 sweeps regardless of N",
	// Ledger schema fields (design ADR 2). Full enum strings, not truncated
	// prefixes: a prefix match would still pass if a replicated asset dropped
	// a trailing enum value (JD-004).
	"`id` | `{LENS}-{NNN}`",
	"`lens` | risk \\| readability \\| reliability \\| resilience \\| judgment-day |",
	"`location` | `path/to/file.ext:line` or `:start-end`",
	"`severity` | BLOCKER \\| CRITICAL \\| WARNING \\| SUGGESTION |",
	"`status` | open \\| fixed \\| verified \\| wont-fix \\| info |",
	"`evidence` | why it matters |",
	"persist an empty ledger record rather than skip persistence",
	// Persistence branches on the artifact store (design ADR 4).
	"write `openspec/changes/{change-name}/review-ledger.md`",
	"upsert topic `sdd/{change-name}/review-ledger`",
	"ad-hoc judgment-day without a change: `review/{target-slug}/ledger`",
	// target-slug derivation rule (JD-002): deterministic so ad-hoc sessions
	// don't guess divergent keys.
	"`target-slug` = `pr-{number}` when reviewing a PR, else the current branch name kebab-cased, else a kebab-case slug of the user-stated review target",
	"do not write files or Engram artifacts",
	"the ledger lives only in this conversation",
	// Compaction caveat for the `none` store (JD-005), folded into the
	// hand-copied `none` bullet instead of living only in a non-copied note.
	"complete the review → fix → re-review loop within the session because it is not persisted across compaction",
	// Scoped re-review contract (spec: "Scoped re-review contract").
	"MUST verify each ledger finding's resolution and MUST review only fix-touched lines",
	"MUST NOT re-read the full original diff",
	"MUST be logged with status `info` as a first-pass quality signal",
	"MUST NOT by itself trigger another full round",
}

// requiredSubagentReviewModeClause is the execution-mode sentence every
// subagent-mode review-* lens asset must carry (design ADR 5).
const requiredSubagentReviewModeClause = "This is a subagent-mode review lens: emit your own ledger rows above; the orchestrator merges them into the persisted ledger."

// requiredOrchestratorMergeModeClause is the execution-mode sentence the four
// subagent-family orchestrators (Claude, Cursor, Kimi, Kiro) must carry: even
// though the lenses run as subagents, the orchestrator still owns merge/persist.
const requiredOrchestratorMergeModeClause = "Subagent mode (Claude, Cursor, Kimi, Kiro): each review-*/jd-* subagent runs its own exhaustive pass and returns its own ledger rows; merge those subagent ledger rows into the persisted ledger and persist per the branch above."

// requiredOrchestratorInlineModeClause is the execution-mode sentence every
// inline-mode orchestrator (no dedicated review-*/jd-* subagents) must carry.
const requiredOrchestratorInlineModeClause = "Inline mode (this adapter has no dedicated review-*/jd-* subagents): run each lens sequentially inside your own orchestrator context and maintain the merged ledger directly."

// requiredJDSubagentModeClause is the execution-mode sentence every jd-judge-*.md
// subagent asset (Claude, Kiro) must carry. jd-fix-agent.md files do NOT carry
// this judge-oriented clause — see requiredFixAgentClauses (JD-001).
const requiredJDSubagentModeClause = "Judgment-day judges run as delegated agents; when this agent is a named sub-agent (Claude, Kiro), emit your own ledger rows and hand them to the orchestrator, which merges both judges' rows into the persisted ledger."

// requiredFixAgentClauses are the fix-specific clauses jd-fix-agent.md files
// must carry instead of the judge-oriented requiredLedgerClauses/
// requiredJDSubagentModeClause. The fix agent applies confirmed fixes; it
// does not run the exhaustive first pass and does not emit a findings
// ledger, so pasting the judge contract verbatim contradicts its own
// "fix ONLY confirmed issues" rules (JD-001).
var requiredFixAgentClauses = []string{
	"does NOT run the exhaustive first-pass sweep and does NOT emit a findings ledger",
	"Read the ledger entries the orchestrator confirmed and passed in the delegate prompt",
	"set that entry's `status` to `fixed`",
	"Never add new ledger rows: if fixing surfaces a new problem, report it back to the orchestrator instead of fixing it or logging it yourself",
	"it receives confirmed findings from the orchestrator, applies them, and hands control back to the orchestrator, which runs the scoped re-judge",
}

// requiredJudgePromptClauses is the subset of requiredLedgerClauses that
// belongs INSIDE the Judge Prompt fenced template itself: the exhaustive
// first-pass loop, the findings-ledger schema, and the ledger-persistence
// branches. The scoped re-review contract clauses (the trailing 4 entries of
// requiredLedgerClauses) govern the re-judge round that follows the fix
// agent, not the judge's own prompt, so they are excluded here (JD-013).
var requiredJudgePromptClauses = requiredLedgerClauses[:len(requiredLedgerClauses)-4]

// assertContainsClauses fails the test for every clause missing from the
// embedded asset at path.
func assertContainsClauses(t *testing.T, path string, clauses []string) {
	t.Helper()
	content, err := assets.Read(path)
	if err != nil {
		t.Fatalf("assets.Read(%q) error = %v", path, err)
	}
	assertTextContainsClauses(t, path, content, clauses)
}

func assertTextContainsClauses(t *testing.T, label, content string, clauses []string) {
	t.Helper()
	for _, clause := range clauses {
		if !strings.Contains(content, clause) {
			t.Errorf("%s missing required ledger clause: %q", label, clause)
		}
	}
}

// reviewLedgerSubagentAssets enumerates the 16 review-* subagent files across
// the 4 adapter families with dedicated review-*/jd-* subagents, plus the 4
// jd-judge-* files for the 2 adapters that carry named judgment-day judges.
// jd-fix-agent.md files are enumerated separately in reviewLedgerFixAgentAssets
// because they carry a fix-specific clause set, not the judge-oriented one
// (JD-001).
var reviewLedgerSubagentAssets = map[string][]string{
	"claude": {
		"claude/agents/review-risk.md",
		"claude/agents/review-readability.md",
		"claude/agents/review-reliability.md",
		"claude/agents/review-resilience.md",
		"claude/agents/jd-judge-a.md",
		"claude/agents/jd-judge-b.md",
	},
	"cursor": {
		"cursor/agents/review-risk.md",
		"cursor/agents/review-readability.md",
		"cursor/agents/review-reliability.md",
		"cursor/agents/review-resilience.md",
	},
	"kimi": {
		"kimi/agents/review-risk.md",
		"kimi/agents/review-readability.md",
		"kimi/agents/review-reliability.md",
		"kimi/agents/review-resilience.md",
	},
	"kiro": {
		"kiro/agents/review-risk.md",
		"kiro/agents/review-readability.md",
		"kiro/agents/review-reliability.md",
		"kiro/agents/review-resilience.md",
		"kiro/agents/jd-judge-a.md",
		"kiro/agents/jd-judge-b.md",
	},
}

// reviewLedgerFixAgentAssets enumerates the jd-fix-agent.md files for the 2
// adapters that carry named judgment-day agents. These carry
// requiredFixAgentClauses instead of requiredLedgerClauses/
// requiredJDSubagentModeClause (JD-001).
var reviewLedgerFixAgentAssets = []string{
	"claude/agents/jd-fix-agent.md",
	"kiro/agents/jd-fix-agent.md",
}

// judgeOnlyMarkers are judge-role clauses that must NOT appear in fix-agent
// assets. If the judge contract block (exhaustive first pass, findings
// ledger emission, judge execution mode) is ever pasted back into a
// fix-agent file alongside the fix clauses, these markers catch it (JD-011).
var judgeOnlyMarkers = []string{
	"**Exhaustive first pass.**",
	"Emit a findings ledger with this schema for every entry",
	requiredJDSubagentModeClause,
}

// assertNotContainsMarkers fails the test if any marker is present in content.
func assertNotContainsMarkers(t *testing.T, path, content string, markers []string) {
	t.Helper()
	for _, marker := range markers {
		if strings.Contains(content, marker) {
			t.Errorf("%s must NOT contain judge-only marker: %q", path, marker)
		}
	}
}

// extractFencedBlockAfterHeading returns the contents of the first fenced
// code block that follows the given markdown heading in content. It fails
// the test if the heading or a subsequent fence cannot be found, so a clause
// that lives in prose outside the fence (placement drift, JD-013) cannot
// silently satisfy a whole-file strings.Contains check.
func extractFencedBlockAfterHeading(t *testing.T, label, content, heading string) string {
	t.Helper()
	headingIdx := strings.Index(content, heading)
	if headingIdx == -1 {
		t.Fatalf("%s: heading %q not found", label, heading)
	}
	rest := content[headingIdx+len(heading):]
	fenceStart := strings.Index(rest, "```")
	if fenceStart == -1 {
		t.Fatalf("%s: no fenced block found after heading %q", label, heading)
	}
	afterFenceOpen := rest[fenceStart+3:]
	newlineIdx := strings.Index(afterFenceOpen, "\n")
	if newlineIdx == -1 {
		t.Fatalf("%s: malformed fence after heading %q", label, heading)
	}
	body := afterFenceOpen[newlineIdx+1:]
	fenceEnd := strings.Index(body, "```")
	if fenceEnd == -1 {
		t.Fatalf("%s: unterminated fenced block after heading %q", label, heading)
	}
	return body[:fenceEnd]
}

// reviewLedgerOrchestratorAssets enumerates the 12 sdd-orchestrator.md source
// files (OpenCode's single source file also renders the Kilocode variant —
// see TestRequiredLedgerClauses/kilocode below).
var reviewLedgerOrchestratorAssets = map[string]string{
	"claude":      "claude/sdd-orchestrator.md",
	"cursor":      "cursor/sdd-orchestrator.md",
	"kimi":        "kimi/sdd-orchestrator.md",
	"kiro":        "kiro/sdd-orchestrator.md",
	"codex":       "codex/sdd-orchestrator.md",
	"gemini":      "gemini/sdd-orchestrator.md",
	"qwen":        "qwen/sdd-orchestrator.md",
	"windsurf":    "windsurf/sdd-orchestrator.md",
	"antigravity": "antigravity/sdd-orchestrator.md",
	"hermes":      "hermes/sdd-orchestrator.md",
	"generic":     "generic/sdd-orchestrator.md",
	"opencode":    "opencode/sdd-orchestrator.md",
}

// subagentFamilyOrchestrators are the orchestrators that also need the
// merge-side clause because their lenses run as subagents.
var subagentFamilyOrchestrators = map[string]bool{
	"claude": true,
	"cursor": true,
	"kimi":   true,
	"kiro":   true,
}

// reviewLedgerJudgmentDaySkillAssets enumerates the 2 judgment-day skill docs
// shared by every adapter's judgment-day flow.
var reviewLedgerJudgmentDaySkillAssets = []string{
	"skills/judgment-day/SKILL.md",
	"skills/judgment-day/references/prompts-and-formats.md",
}

// TestRequiredLedgerClauses is the RED->GREEN consistency test for this
// change: it asserts every one of the 16 review-* subagent files, 13
// orchestrator variants (12 source files + the Kilocode-rendered variant),
// and 8 judgment-day files carries the full requiredLedgerClauses set.
//
// RED evidence: on the first run (before any asset edit), every subtest below
// fails because no asset yet carries the contract — see apply-progress.md for
// the captured failing-test output.
func TestRequiredLedgerClauses(t *testing.T) {
	t.Run("subagent_review_and_jd_assets", func(t *testing.T) {
		for family, paths := range reviewLedgerSubagentAssets {
			t.Run(family, func(t *testing.T) {
				for _, p := range paths {
					t.Run(filepath.Base(p), func(t *testing.T) {
						assertContainsClauses(t, p, requiredLedgerClauses)
						if strings.HasPrefix(filepath.Base(p), "review-") {
							content := assets.MustRead(p)
							assertTextContainsClauses(t, p, content, []string{requiredSubagentReviewModeClause})
						}
						if strings.HasPrefix(filepath.Base(p), "jd-") {
							content := assets.MustRead(p)
							assertTextContainsClauses(t, p, content, []string{requiredJDSubagentModeClause})
						}
					})
				}
			})
		}
	})

	t.Run("fix_agent_assets", func(t *testing.T) {
		for _, p := range reviewLedgerFixAgentAssets {
			t.Run(filepath.Base(p), func(t *testing.T) {
				content := assets.MustRead(p)
				assertTextContainsClauses(t, p, content, requiredFixAgentClauses)
				assertNotContainsMarkers(t, p, content, judgeOnlyMarkers)
			})
		}
	})

	t.Run("orchestrator_assets", func(t *testing.T) {
		for family, path := range reviewLedgerOrchestratorAssets {
			t.Run(family, func(t *testing.T) {
				assertContainsClauses(t, path, requiredLedgerClauses)
				content := assets.MustRead(path)
				if subagentFamilyOrchestrators[family] {
					assertTextContainsClauses(t, path, content, []string{requiredOrchestratorMergeModeClause})
				} else {
					assertTextContainsClauses(t, path, content, []string{requiredOrchestratorInlineModeClause})
				}
			})
		}
	})

	// Kilocode/OpenCode have no dedicated source asset: sddOrchestratorAsset maps
	// both AgentOpenCode and AgentKilocode to "opencode/sdd-orchestrator.md".
	// Render through the real Inject() path to prove the shared source actually
	// reaches each adapter's Kilocode/OpenCode-rendered gentle-orchestrator
	// prompt, not just a static file read.
	for _, tc := range []struct {
		name    string
		adapter agents.Adapter
	}{
		{name: "opencode", adapter: opencodeAdapter()},
		{name: "kilocode", adapter: kilocodeAdapter()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			home := t.TempDir()
			if _, err := Inject(home, tc.adapter, model.SDDModeMulti); err != nil {
				t.Fatalf("Inject(%s) error = %v", tc.name, err)
			}
			settingsPath := tc.adapter.SettingsPath(home)
			prompt := readGentleOrchestratorPrompt(t, settingsPath)
			assertTextContainsClauses(t, tc.name+" gentle-orchestrator prompt", prompt, requiredLedgerClauses)
			assertTextContainsClauses(t, tc.name+" gentle-orchestrator prompt", prompt, []string{requiredOrchestratorInlineModeClause})
		})
	}

	t.Run("judgment_day_skill_assets", func(t *testing.T) {
		for _, p := range reviewLedgerJudgmentDaySkillAssets {
			t.Run(filepath.Base(p), func(t *testing.T) {
				assertContainsClauses(t, p, requiredLedgerClauses)
			})
		}
	})

	// prompts-and-formats.md carries both templates: the Judge Prompt template
	// and the Fix Agent Prompt template. Both assertions below extract the
	// actual fenced block under each heading and assert clauses against that
	// EXTRACTED FENCE, not the whole file — a whole-file strings.Contains
	// check would still pass if a clause lived in prose outside its fence
	// (JD-013), which is exactly the placement-drift class this guards
	// against. Fix clauses must match requiredFixAgentClauses verbatim so the
	// template can't silently drift from jd-fix-agent.md (JD-009).
	t.Run("judgment_day_skill_fix_agent_clauses", func(t *testing.T) {
		const p = "skills/judgment-day/references/prompts-and-formats.md"
		content := assets.MustRead(p)

		fixAgentBlock := extractFencedBlockAfterHeading(t, p, content, "## Fix Agent Prompt")
		assertTextContainsClauses(t, p+" Fix Agent Prompt fence", fixAgentBlock, requiredFixAgentClauses)

		judgeBlock := extractFencedBlockAfterHeading(t, p, content, "## Judge Prompt")
		assertTextContainsClauses(t, p+" Judge Prompt fence", judgeBlock, requiredJudgePromptClauses)
	})
}

// readGentleOrchestratorPrompt reads back the rendered gentle-orchestrator
// agent prompt from an OpenCode/Kilocode opencode.json settings file.
func readGentleOrchestratorPrompt(t *testing.T, settingsPath string) string {
	t.Helper()
	raw, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", settingsPath, err)
	}
	var root map[string]any
	if err := json.Unmarshal(raw, &root); err != nil {
		t.Fatalf("Unmarshal(%q) error = %v", settingsPath, err)
	}
	agentsMap, ok := root["agent"].(map[string]any)
	if !ok {
		t.Fatalf("%q: missing top-level \"agent\" map", settingsPath)
	}
	orchestrator, ok := agentsMap["gentle-orchestrator"].(map[string]any)
	if !ok {
		t.Fatalf("%q: missing \"agent.gentle-orchestrator\"", settingsPath)
	}
	prompt, ok := orchestrator["prompt"].(string)
	if !ok {
		t.Fatalf("%q: \"agent.gentle-orchestrator.prompt\" is not a string", settingsPath)
	}
	return prompt
}

// TestReviewLedgerContractSchema is the golden-string assertion test (task
// 1.2) against the canonical _shared source file: id format, severity enum,
// and status enum must be present exactly as the schema table defines them.
//
// RED evidence: fails until internal/assets/skills/_shared/review-ledger-contract.md
// is created (task 1.3).
func TestReviewLedgerContractSchema(t *testing.T) {
	const path = "skills/_shared/review-ledger-contract.md"
	content, err := assets.Read(path)
	if err != nil {
		t.Fatalf("assets.Read(%q) error = %v", path, err)
	}

	assertTextContainsClauses(t, path, content, requiredLedgerClauses)

	for _, want := range []string{
		"`{LENS}-{NNN}`",
		"BLOCKER \\| CRITICAL \\| WARNING \\| SUGGESTION",
		"open \\| fixed \\| verified \\| wont-fix \\| info",
	} {
		if !strings.Contains(content, want) {
			t.Errorf("%s missing schema fixture %q", path, want)
		}
	}
}
