package sdd

import (
	"fmt"
	"path"
	"strings"

	"github.com/gentleman-programming/gentle-ai/internal/assets"
	"github.com/gentleman-programming/gentle-ai/internal/model"
)

type ClaudeRenderFile struct{ Path, Content string }
type ClaudeRenderPlan struct {
	Mode  model.SDDModeID
	Files []ClaudeRenderFile
}

var claudeRenderPaths = []string{
	"claude/sdd-orchestrator.md", "claude/sdd-orchestrator-workflow.md",
	"claude/commands/sdd-apply.md", "claude/commands/sdd-archive.md", "claude/commands/sdd-continue.md",
	"claude/commands/sdd-explore.md", "claude/commands/sdd-ff.md", "claude/commands/sdd-init.md",
	"claude/commands/sdd-new.md", "claude/commands/sdd-onboard.md", "claude/commands/sdd-status.md", "claude/commands/sdd-verify.md",
}

func BuildClaudeRenderPlan(mode model.SDDModeID) (ClaudeRenderPlan, error) {
	mode, err := normalizeClaudeMode(mode)
	if err != nil {
		return ClaudeRenderPlan{}, err
	}
	plan := ClaudeRenderPlan{Mode: mode, Files: make([]ClaudeRenderFile, 0, len(claudeRenderPaths))}
	for _, target := range claudeRenderPaths {
		content, err := RenderClaudeAsset(mode, target)
		if err != nil {
			return ClaudeRenderPlan{}, err
		}
		plan.Files = append(plan.Files, ClaudeRenderFile{target, content})
	}
	return plan, nil
}

func RenderClaudeAsset(mode model.SDDModeID, target string) (string, error) {
	mode, err := normalizeClaudeMode(mode)
	if err != nil {
		return "", err
	}
	if !claudeTarget(target) {
		return "", fmt.Errorf("unknown Claude render target %q", target)
	}
	if mode == model.SDDModeMulti {
		return renderClaudeModeRegions(renderBoundedReviewAsset(target), mode)
	}
	switch target {
	case "claude/sdd-orchestrator.md":
		return assets.MustRead("claude/single/sdd-orchestrator.md"), nil
	case "claude/sdd-orchestrator-workflow.md":
		return assets.MustRead("claude/single/sdd-orchestrator-workflow.md"), nil
	}
	name := strings.TrimSuffix(path.Base(target), ".md")
	extra := map[string]string{"sdd-new": " explore then propose inline; present proposal summary.", "sdd-continue": " Follow nextRecommended inline and expose blockedReasons.", "sdd-ff": " Choose interactive or auto planning.", "sdd-status": " This command is read-only and reports blockedReasons."}[name]
	return strings.ReplaceAll(assets.MustRead("claude/single/sdd-command.md"), "{{COMMAND}}", name) + extra, nil
}

func normalizeClaudeMode(mode model.SDDModeID) (model.SDDModeID, error) {
	if mode == "" {
		return model.SDDModeSingle, nil
	}
	if mode != model.SDDModeSingle && mode != model.SDDModeMulti {
		return "", fmt.Errorf("unsupported Claude SDD mode %q", mode)
	}
	return mode, nil
}

func claudeTarget(target string) bool {
	for _, candidate := range claudeRenderPaths {
		if target == candidate {
			return true
		}
	}
	return false
}

func renderClaudeModeRegions(template string, mode model.SDDModeID) (string, error) {
	if strings.Contains(template, "{{CLAUDE_") {
		return "", fmt.Errorf("unresolved Claude placeholder")
	}
	const prefix = "<!-- gentle-ai:claude-render:"
	starts := map[model.SDDModeID]string{model.SDDModeSingle: prefix + "single:start -->", model.SDDModeMulti: prefix + "multi:start -->"}
	var out strings.Builder
	for template != "" {
		i := strings.Index(template, prefix)
		if i < 0 {
			out.WriteString(template)
			break
		}
		out.WriteString(template[:i])
		rest := template[i:]
		var region model.SDDModeID
		var start string
		for candidate, marker := range starts {
			if strings.HasPrefix(rest, marker) {
				region, start = candidate, marker
			}
		}
		if start == "" {
			if strings.Contains(rest, ":end -->") {
				return "", fmt.Errorf("orphan Claude render end")
			}
			return "", fmt.Errorf("unknown Claude render region")
		}
		end := prefix + string(region) + ":end -->"
		body := rest[len(start):]
		j := strings.Index(body, end)
		if j < 0 && strings.Contains(body, prefix) {
			return "", fmt.Errorf("nested Claude render region")
		}
		if j < 0 {
			return "", fmt.Errorf("missing Claude render end")
		}
		body, template = body[:j], body[j+len(end):]
		if strings.Contains(body, prefix) {
			return "", fmt.Errorf("nested Claude render region")
		}
		if region == mode {
			out.WriteString(body)
		}
	}
	return out.String(), nil
}
