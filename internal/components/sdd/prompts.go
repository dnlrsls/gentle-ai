package sdd

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/gentleman-programming/gentle-ai/internal/assets"
	"github.com/gentleman-programming/gentle-ai/internal/components/filemerge"
	"github.com/gentleman-programming/gentle-ai/internal/model"
)

const (
	claudeCodeGraphToolGrant = "mcp__codegraph__codegraph_explore"
	kiroCodeGraphToolGrant   = "@codegraph"
)

var renderNativeSubAgentAsset = renderBoundedReviewAsset

// readSkillContent reads the embedded skill content for the given phase.
func readSkillContent(phase string) (string, error) {
	return assets.Read("skills/" + phase + "/SKILL.md")
}

// SharedPromptDir returns the directory where shared SDD prompt files are stored.
// The path is {homeDir}/.config/opencode/prompts/sdd.
func SharedPromptDir(homeDir string) string {
	return filepath.Join(homeDir, ".config", "opencode", "prompts", "sdd")
}

// subAgentPhaseOrder is an alias for profilePhaseOrder (defined in profiles.go),
// kept for backward compatibility with any code in this file that references it.
// Both variables are in the same package and represent the same canonical list.
var subAgentPhaseOrder = profilePhaseOrder

// SharedPromptPhases returns the ordered list of phase names that have shared
// prompt files in SharedPromptDir(). Used by backup target enumeration and any
// caller that needs to enumerate all prompt files without importing internal vars.
func SharedPromptPhases() []string {
	return ProfilePhaseOrder()
}

// WriteSharedPromptFiles writes the 10 SDD sub-agent prompt files to
// {homeDir}/.config/opencode/prompts/sdd/. The content for each phase is extracted
// from the embedded skill file, filtered to the section matching the phase's
// model capability ("capable" or "small").
//
// The phaseCapabilities map controls which section is extracted per phase:
//   - "capable" sections are used for high-capability models
//   - "small" sections are used for small/fast models (e.g., flash, mini)
//   - If a phase is missing from the map, "capable" is used as default
//
// Returns (true, nil) if any file was created or changed, (false, nil) if all
// files already match (idempotent). Uses WriteFileAtomic so the operation is
// safe to repeat.
func WriteSharedPromptFiles(homeDir string, phaseCapabilities map[string]string, codeGraphGuidance ...string) (bool, error) {
	promptDir := SharedPromptDir(homeDir)
	anyChanged := false
	guidance := ""
	if len(codeGraphGuidance) > 0 {
		guidance = codeGraphGuidance[0]
	}

	for _, phase := range subAgentPhaseOrder {
		// Read the embedded skill content for this phase.
		skillContent, err := readSkillContent(phase)
		if err != nil {
			return false, err
		}

		// Determine which section to extract based on model capability.
		capability := "capable"
		if phaseCapabilities != nil {
			if cap, ok := phaseCapabilities[phase]; ok && cap != "" {
				capability = cap
			}
		}

		// Extract the section matching the capability (falls back to full content
		// if no matching section marker is found — correct behavior for phases
		// that don't yet have conditional sections).
		content := extractModelSection(skillContent, capability)
		content = injectCodeGraphGuidanceIntoPrompt(content, guidance)

		path := filepath.Join(promptDir, phase+".md")
		result, err := filemerge.WriteFileAtomic(path, []byte(content), 0o644)
		if err != nil {
			return false, err
		}

		if result.Changed {
			anyChanged = true
		}
	}

	return anyChanged, nil
}

func injectCodeGraphGuidanceIntoPrompt(prompt, guidance string) string {
	if strings.TrimSpace(guidance) == "" {
		return prompt
	}
	return filemerge.InjectMarkdownSection(prompt, "codegraph-guidance", guidance)
}

func injectCodeGraphToolGrantIntoPrompt(prompt string, agentID model.AgentID, guidance string) (string, error) {
	if strings.TrimSpace(guidance) == "" {
		return prompt, nil
	}

	var grant string
	switch agentID {
	case model.AgentClaudeCode:
		grant = claudeCodeGraphToolGrant
	case model.AgentKiroIDE:
		grant = kiroCodeGraphToolGrant
	default:
		return prompt, nil
	}

	newline := "\n"
	if strings.HasPrefix(prompt, "---\r\n") {
		newline = "\r\n"
	} else if !strings.HasPrefix(prompt, "---\n") {
		return prompt, fmt.Errorf("missing YAML frontmatter opening delimiter")
	}

	frontmatterStart := len("---" + newline)
	frontmatterEndOffset := strings.Index(prompt[frontmatterStart:], newline+"---"+newline)
	if frontmatterEndOffset < 0 {
		return prompt, fmt.Errorf("missing YAML frontmatter closing delimiter")
	}
	frontmatterEnd := frontmatterStart + frontmatterEndOffset
	lines := strings.Split(prompt[frontmatterStart:frontmatterEnd], newline)
	toolsLineIndex := -1
	for i, line := range lines {
		if strings.HasPrefix(line, "tools:") {
			if toolsLineIndex >= 0 {
				return prompt, fmt.Errorf("multiple tools declarations in YAML frontmatter")
			}
			toolsLineIndex = i
			continue
		}
		if strings.HasPrefix(strings.TrimLeft(line, " \t"), "tools:") {
			return prompt, fmt.Errorf("tools declaration must be unindented")
		}
	}
	if toolsLineIndex < 0 {
		return prompt, fmt.Errorf("missing tools declaration in YAML frontmatter")
	}

	toolsLine := lines[toolsLineIndex]
	value := strings.TrimSpace(strings.TrimPrefix(toolsLine, "tools:"))
	switch agentID {
	case model.AgentClaudeCode:
		if value == "" {
			lines[toolsLineIndex] = "tools: " + grant
			break
		}
		tools := strings.Split(value, ",")
		for _, tool := range tools {
			tool = strings.TrimSpace(tool)
			if tool == "" {
				return prompt, fmt.Errorf("malformed Claude tools declaration")
			}
			if tool == grant {
				return prompt, nil
			}
		}
		lines[toolsLineIndex] = toolsLine + ", " + grant
	case model.AgentKiroIDE:
		var tools []string
		if err := json.Unmarshal([]byte(value), &tools); err != nil || tools == nil {
			return prompt, fmt.Errorf("malformed Kiro tools declaration")
		}
		for _, tool := range tools {
			if tool == grant {
				return prompt, nil
			}
		}
		closingBracket := strings.LastIndex(toolsLine, "]")
		if len(tools) == 0 {
			lines[toolsLineIndex] = toolsLine[:closingBracket] + `"` + grant + `"` + toolsLine[closingBracket:]
		} else {
			lines[toolsLineIndex] = toolsLine[:closingBracket] + `, "` + grant + `"` + toolsLine[closingBracket:]
		}
	}

	updatedFrontmatter := strings.Join(lines, newline)
	return prompt[:frontmatterStart] + updatedFrontmatter + prompt[frontmatterEnd:], nil
}

func isMarkdownSubAgentPromptFile(fileName string) bool {
	if filepath.Ext(fileName) != ".md" {
		return false
	}
	return !strings.HasPrefix(filepath.Base(fileName), ".")
}

func injectCodeGraphGuidanceIntoOpenCodeSubagentPrompts(agentMap map[string]any, guidance string) {
	if strings.TrimSpace(guidance) == "" {
		return
	}
	for _, agentRaw := range agentMap {
		agent, ok := agentRaw.(map[string]any)
		if !ok {
			continue
		}
		if mode, _ := agent["mode"].(string); mode == "primary" {
			continue
		}
		prompt, ok := agent["prompt"].(string)
		if !ok || strings.HasPrefix(prompt, "{file:") {
			continue
		}
		agent["prompt"] = injectCodeGraphGuidanceIntoPrompt(prompt, guidance)
	}
}
