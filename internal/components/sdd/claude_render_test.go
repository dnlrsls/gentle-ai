package sdd

import (
	"strings"
	"testing"

	"github.com/gentleman-programming/gentle-ai/internal/model"
)

var claudePaths = []string{
	"claude/sdd-orchestrator.md", "claude/sdd-orchestrator-workflow.md",
	"claude/commands/sdd-apply.md", "claude/commands/sdd-archive.md", "claude/commands/sdd-continue.md",
	"claude/commands/sdd-explore.md", "claude/commands/sdd-ff.md", "claude/commands/sdd-init.md",
	"claude/commands/sdd-new.md", "claude/commands/sdd-onboard.md", "claude/commands/sdd-status.md",
	"claude/commands/sdd-verify.md",
}

func TestClaudeModesPlansTargets(t *testing.T) {
	for _, tc := range []struct{ in, want model.SDDModeID }{{"", model.SDDModeSingle}, {model.SDDModeSingle, model.SDDModeSingle}, {model.SDDModeMulti, model.SDDModeMulti}} {
		plan, err := BuildClaudeRenderPlan(tc.in)
		if err != nil || plan.Mode != tc.want || len(plan.Files) != len(claudePaths) {
			t.Fatalf("mode %q: plan=%+v err=%v", tc.in, plan, err)
		}
		seen := map[string]bool{}
		for i, file := range plan.Files {
			if file.Path != claudePaths[i] || seen[file.Path] {
				t.Fatalf("path %d = %q", i, file.Path)
			}
			if tc.want == model.SDDModeMulti && file.Path == "claude/commands/sdd-apply.md" && (strings.Contains(file.Content, authorityFirstProcedurePlaceholder) || !strings.Contains(file.Content, "`gentle-ai review start`")) {
				t.Fatalf("multi sdd-apply did not expand authority procedure")
			}
			seen[file.Path] = true
		}
	}
	for _, mode := range []model.SDDModeID{"invalid"} {
		if _, err := BuildClaudeRenderPlan(mode); err == nil || !strings.Contains(err.Error(), string(mode)) {
			t.Errorf("plan invalid mode: %v", err)
		}
		if got, err := RenderClaudeAsset(mode, claudePaths[0]); err == nil || got != "" || !strings.Contains(err.Error(), string(mode)) {
			t.Errorf("asset invalid mode: %q %v", got, err)
		}
	}
	for _, target := range []string{"requirements.txt", "CMakeLists.txt", "README.sh", "other.md", "other.mdx"} {
		for _, mode := range []model.SDDModeID{model.SDDModeSingle, model.SDDModeMulti} {
			if got, err := RenderClaudeAsset(mode, target); err == nil || got != "" || !strings.Contains(err.Error(), "unknown") {
				t.Errorf("%s %s: %q %v", mode, target, got, err)
			}
		}
	}
}

func TestClaudeSingleContracts(t *testing.T) {
	for _, target := range claudePaths {
		got, err := RenderClaudeAsset(model.SDDModeSingle, target)
		if err != nil {
			t.Errorf("%s: %v", target, err)
			continue
		}
		for _, want := range []string{"current conversation", "English", "Strict TDD", "400", "approval", "archive", "do not use a sub-agent or Agent/Task"} {
			if !strings.Contains(got, want) {
				t.Errorf("%s missing %q", target, want)
			}
		}
		if strings.Contains(got, "Launch `") || strings.Contains(got, "delegate this command") {
			t.Errorf("%s delegates", target)
		}
	}
	for _, tc := range []struct {
		name  string
		wants []string
	}{
		{"sdd-new", []string{"explore", "propose", "proposal summary"}},
		{"sdd-continue", []string{"nextRecommended", "blockedReasons"}},
		{"sdd-ff", []string{"interactive", "auto", "planning"}},
		{"sdd-status", []string{"read-only", "blockedReasons"}},
		{"sdd-orchestrator-workflow", []string{"four-choice", "sdd-init/{project}", "Engram", "testing-capabilities", "persistence"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			target := "claude/commands/" + tc.name + ".md"
			if tc.name == "sdd-orchestrator-workflow" {
				target = "claude/" + tc.name + ".md"
			}
			got, err := RenderClaudeAsset(model.SDDModeSingle, target)
			if err != nil {
				t.Fatal(err)
			}
			for _, want := range tc.wants {
				if !strings.Contains(got, want) {
					t.Errorf("missing %q", want)
				}
			}
		})
	}
}

func TestClaudeMultiAndRegions(t *testing.T) {
	wants := map[string][]string{
		"claude/sdd-orchestrator.md":          {"delegate ALL real work to sub-agents", "Agent/Task"},
		"claude/sdd-orchestrator-workflow.md": {"Sub-Agent Launch Protocol", "nextRecommended", "blockedReasons"},
		"claude/commands/sdd-apply.md":        {"delegate this command", "RED-GREEN-REFACTOR"}, "claude/commands/sdd-archive.md": {"delegate this command", "Read the verification report first"},
		"claude/commands/sdd-continue.md": {"Launch the appropriate sub-agent(s)", "nextRecommended", "blockedReasons"}, "claude/commands/sdd-explore.md": {"delegate this command", "exploration only"},
		"claude/commands/sdd-ff.md": {"interactive", "auto", "Do NOT execute phase work inline"}, "claude/commands/sdd-init.md": {"delegate this command", "Initialize Spec-Driven Development"},
		"claude/commands/sdd-new.md": {"Launch `sdd-explore`", "Launch `sdd-propose`", "Present the proposal summary"}, "claude/commands/sdd-onboard.md": {"delegate this command", "complete SDD cycle"},
		"claude/commands/sdd-status.md": {"read-only", "blockedReasons"}, "claude/commands/sdd-verify.md": {"delegate this command", "independent requirements/runtime final verification"},
	}
	for target, required := range wants {
		got, err := RenderClaudeAsset(model.SDDModeMulti, target)
		if err != nil {
			t.Errorf("%s: %v", target, err)
			continue
		}
		for _, want := range required {
			if !strings.Contains(got, want) {
				t.Errorf("%s missing %q", target, want)
			}
		}
		for _, forbidden := range []string{"Run this workflow in the current conversation.", "gentle-ai:claude-render", "{{CLAUDE_", "{{COMMAND}}"} {
			if strings.Contains(got, forbidden) {
				t.Errorf("%s exposes %q", target, forbidden)
			}
		}
	}
	for _, tc := range []struct{ name, template, want string }{
		{"orphan", "<!-- gentle-ai:claude-render:single:end -->", "orphan"}, {"missing", "<!-- gentle-ai:claude-render:single:start -->", "missing"},
		{"unknown", "<!-- gentle-ai:claude-render:bad:start -->x", "unknown"}, {"nested", "<!-- gentle-ai:claude-render:single:start --><!-- gentle-ai:claude-render:multi:start -->", "nested"}, {"placeholder", "{{CLAUDE_VALUE}}", "unresolved"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := renderClaudeModeRegions(tc.template, model.SDDModeSingle)
			if got != "" || err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("%q %v", got, err)
			}
		})
	}
}
