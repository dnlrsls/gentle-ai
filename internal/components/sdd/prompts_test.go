package sdd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/gentleman-programming/gentle-ai/internal/agents"
	"github.com/gentleman-programming/gentle-ai/internal/assets"
	"github.com/gentleman-programming/gentle-ai/internal/components/communitytool"
	"github.com/gentleman-programming/gentle-ai/internal/model"
)

// TestSharedPromptDir verifies the expected directory path is returned.
func TestSharedPromptDir(t *testing.T) {
	want := filepath.FromSlash("/home/testuser/.config/opencode/prompts/sdd")
	got := SharedPromptDir(filepath.FromSlash("/home/testuser"))
	if got != want {
		t.Fatalf("SharedPromptDir(%q) = %q, want %q", "/home/testuser", got, want)
	}
}

func readOpenCodeAgents(t *testing.T, settingsPath string) map[string]any {
	t.Helper()
	content, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", settingsPath, err)
	}
	var settings struct {
		Agent map[string]any `json:"agent"`
	}
	if err := json.Unmarshal(content, &settings); err != nil {
		t.Fatalf("Unmarshal(%q) error = %v", settingsPath, err)
	}
	return settings.Agent
}

func agentPrompt(t *testing.T, agentsMap map[string]any, agentName string) string {
	t.Helper()
	agentRaw, ok := agentsMap[agentName]
	if !ok {
		t.Fatalf("agent %q missing", agentName)
	}
	agentMap, ok := agentRaw.(map[string]any)
	if !ok {
		t.Fatalf("agent %q has type %T, want object", agentName, agentRaw)
	}
	prompt, ok := agentMap["prompt"].(string)
	if !ok {
		t.Fatalf("agent %q prompt has type %T, want string", agentName, agentMap["prompt"])
	}
	return prompt
}

func inlineOpenCodeSubAgentsForCodeGraphTest(t *testing.T, agentsMap map[string]any) []string {
	t.Helper()
	names := make([]string, 0, len(agentsMap))
	for name, agentRaw := range agentsMap {
		agentMap, ok := agentRaw.(map[string]any)
		if !ok {
			continue
		}
		if mode, _ := agentMap["mode"].(string); mode == "primary" {
			continue
		}
		prompt, ok := agentMap["prompt"].(string)
		if !ok || strings.HasPrefix(prompt, "{file:") {
			continue
		}
		names = append(names, name)
	}
	sort.Strings(names)
	if len(names) == 0 {
		t.Fatal("generated OpenCode settings contain no inline subagents")
	}
	return names
}

func nativeMarkdownSubAgentFilesForCodeGraphTest(t *testing.T, adapter agents.Adapter) []string {
	t.Helper()
	entries, err := assets.FS.ReadDir(adapter.EmbeddedSubAgentsDir())
	if err != nil {
		t.Fatalf("ReadDir(%q) error = %v", adapter.EmbeddedSubAgentsDir(), err)
	}
	files := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() && isMarkdownSubAgentPromptFile(entry.Name()) {
			files = append(files, entry.Name())
		}
	}
	return files
}

func frontmatterToolsLineForCodeGraphTest(t *testing.T, content string) string {
	t.Helper()
	frontmatterEnd := strings.Index(strings.TrimPrefix(content, "---\n"), "\n---\n")
	if !strings.HasPrefix(content, "---\n") || frontmatterEnd < 0 {
		t.Fatal("content missing YAML frontmatter")
	}
	for _, line := range strings.Split(content[:len("---\n")+frontmatterEnd], "\n") {
		if strings.HasPrefix(line, "tools:") {
			return line
		}
	}
	t.Fatal("frontmatter missing tools line")
	return ""
}

func kimiYAMLSubagentFilesForCodeGraphTest() []string {
	return []string{"sdd-apply.yaml", "review-risk.yaml"}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func withoutStrings(values []string, excluded ...string) []string {
	kept := make([]string, 0, len(values))
	for _, value := range values {
		if containsString(excluded, value) {
			continue
		}
		kept = append(kept, value)
	}
	return kept
}

// TestWriteSharedPromptFilesCreates10Files verifies that WriteSharedPromptFiles
// creates exactly the 10 expected prompt files under {homeDir}/.config/opencode/prompts/sdd/.
func TestWriteSharedPromptFilesCreates10Files(t *testing.T) {
	home := t.TempDir()

	changed, err := WriteSharedPromptFiles(home, nil)
	if err != nil {
		t.Fatalf("WriteSharedPromptFiles() error = %v", err)
	}
	if !changed {
		t.Fatal("WriteSharedPromptFiles() first call changed = false, want true")
	}

	expectedFiles := []string{
		"sdd-init.md",
		"sdd-explore.md",
		"sdd-propose.md",
		"sdd-spec.md",
		"sdd-design.md",
		"sdd-tasks.md",
		"sdd-apply.md",
		"sdd-verify.md",
		"sdd-archive.md",
		"sdd-onboard.md",
	}

	promptDir := SharedPromptDir(home)
	for _, fileName := range expectedFiles {
		path := filepath.Join(promptDir, fileName)
		info, statErr := os.Stat(path)
		if statErr != nil {
			t.Errorf("prompt file %q not found: %v", path, statErr)
			continue
		}
		if info.Size() == 0 {
			t.Errorf("prompt file %q is empty", path)
		}
	}
}

// TestWriteSharedPromptFilesIdempotent verifies that calling WriteSharedPromptFiles
// twice returns changed=false on the second call.
func TestWriteSharedPromptFilesIdempotent(t *testing.T) {
	home := t.TempDir()

	first, err := WriteSharedPromptFiles(home, nil)
	if err != nil {
		t.Fatalf("WriteSharedPromptFiles() first error = %v", err)
	}
	if !first {
		t.Fatal("WriteSharedPromptFiles() first call changed = false, want true")
	}

	second, err := WriteSharedPromptFiles(home, nil)
	if err != nil {
		t.Fatalf("WriteSharedPromptFiles() second error = %v", err)
	}
	if second {
		t.Fatal("WriteSharedPromptFiles() second call changed = true, want false (idempotent)")
	}
}

// TestWriteSharedPromptFilesContent verifies each prompt file contains the
// executor-scoped sub-agent prompt content for the correct phase.
func TestWriteSharedPromptFilesContent(t *testing.T) {
	home := t.TempDir()

	if _, err := WriteSharedPromptFiles(home, nil); err != nil {
		t.Fatalf("WriteSharedPromptFiles() error = %v", err)
	}

	promptDir := SharedPromptDir(home)

	phases := []struct {
		file  string
		phase string
	}{
		{"sdd-init.md", "init"},
		{"sdd-explore.md", "explore"},
		{"sdd-propose.md", "propose"},
		{"sdd-spec.md", "spec"},
		{"sdd-design.md", "design"},
		{"sdd-tasks.md", "tasks"},
		{"sdd-apply.md", "apply"},
		{"sdd-verify.md", "verify"},
		{"sdd-archive.md", "archive"},
		{"sdd-onboard.md", "onboard"},
	}

	for _, tc := range phases {
		path := filepath.Join(promptDir, tc.file)
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Errorf("ReadFile(%q) error = %v", path, readErr)
			continue
		}

		content := string(data)

		// Each file must contain the phase name (executor-scoped prompt).
		if !strings.Contains(content, tc.phase) {
			t.Errorf("prompt file %q missing phase %q in content", tc.file, tc.phase)
		}

		// Each file must have substantial content (not the old one-liner).
		if len(content) < 200 {
			t.Errorf("prompt file %q content too short (%d bytes), want >= 200", tc.file, len(content))
		}

		// Each file must contain the ORCHESTRATOR gate/note (present in all skill files)
		// or "do not delegate" (present in some skill files).
		hasGate := strings.Contains(content, "ORCHESTRATOR GATE") || strings.Contains(content, "ORCHESTRATOR NOTE")
		hasDoNotDelegate := strings.Contains(strings.ToLower(content), "do not delegate")
		if !hasGate && !hasDoNotDelegate {
			t.Errorf("prompt file %q missing expected skill content (ORCHESTRATOR GATE/NOTE or do not delegate)", tc.file)
		}
	}
}

func TestWriteSharedPromptFilesLanguageContract(t *testing.T) {
	home := t.TempDir()

	if _, err := WriteSharedPromptFiles(home, nil); err != nil {
		t.Fatalf("WriteSharedPromptFiles() error = %v", err)
	}

	for _, fileName := range []string{
		"sdd-init.md",
		"sdd-explore.md",
		"sdd-propose.md",
		"sdd-spec.md",
		"sdd-design.md",
		"sdd-tasks.md",
		"sdd-apply.md",
		"sdd-verify.md",
		"sdd-archive.md",
		"sdd-onboard.md",
	} {
		t.Run(fileName, func(t *testing.T) {
			path := filepath.Join(SharedPromptDir(home), fileName)
			content, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("ReadFile(%q) error = %v", path, err)
			}
			text := string(content)
			for _, required := range []string{
				"Generated technical artifacts default to English",
				"If Spanish technical artifacts are explicitly requested, use neutral/professional Spanish",
			} {
				if !strings.Contains(text, required) {
					t.Fatalf("%s missing delegated prompt language contract %q", fileName, required)
				}
			}
		})
	}
}

// TestWriteSharedPromptFilesWithCapabilities verifies that prompt file content
// differs based on model capability (small vs capable).
func TestWriteSharedPromptFilesWithCapabilities(t *testing.T) {
	home := t.TempDir()

	// Write with "capable" for sdd-apply.
	capableMap := map[string]string{"sdd-apply": "capable"}
	_, err := WriteSharedPromptFiles(home, capableMap)
	if err != nil {
		t.Fatalf("WriteSharedPromptFiles(capable) error = %v", err)
	}

	capablePath := filepath.Join(SharedPromptDir(home), "sdd-apply.md")
	capableContent, err := os.ReadFile(capablePath)
	if err != nil {
		t.Fatalf("ReadFile(capable) error = %v", err)
	}

	// Now write with "small" for sdd-apply.
	smallMap := map[string]string{"sdd-apply": "small"}
	_, err = WriteSharedPromptFiles(home, smallMap)
	if err != nil {
		t.Fatalf("WriteSharedPromptFiles(small) error = %v", err)
	}

	smallPath := filepath.Join(SharedPromptDir(home), "sdd-apply.md")
	smallContent, err := os.ReadFile(smallPath)
	if err != nil {
		t.Fatalf("ReadFile(small) error = %v", err)
	}

	// The two contents should differ (different skill sections).
	if string(capableContent) == string(smallContent) {
		t.Fatal("sdd-apply.md content should differ between 'capable' and 'small' sections")
	}

	// Small section should mention "max 3 files" (small model constraint).
	if !strings.Contains(string(smallContent), "max 3 files") {
		t.Error("small section should contain 'max 3 files'")
	}

	// Capable section should NOT mention "max 3 files" (no such constraint).
	if strings.Contains(string(capableContent), "max 3 files") {
		t.Error("capable section should NOT contain 'max 3 files'")
	}
}

// TestInjectOpenCodeMultiModeSubagentPromptsUseFilePaths verifies that after
// injection in multi-mode, each sub-agent's prompt field in opencode.json
// contains a {file:...} reference (not an inline string).
func TestInjectOpenCodeMultiModeSubagentPromptsUseFilePaths(t *testing.T) {
	home := t.TempDir()
	mockNoPackageManager(t)

	if _, err := Inject(home, opencodeAdapter(), "multi"); err != nil {
		t.Fatalf("Inject(multi) error = %v", err)
	}

	settingsPath := filepath.Join(home, ".config", "opencode", "opencode.json")
	content, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("ReadFile(opencode.json) error = %v", err)
	}

	promptDir := SharedPromptDir(home)

	text := strings.ReplaceAll(string(content), `\\`, `/`)
	for _, phase := range []string{"sdd-init", "sdd-explore", "sdd-propose", "sdd-spec", "sdd-design", "sdd-tasks", "sdd-apply", "sdd-verify", "sdd-archive", "sdd-onboard"} {
		expectedRef := "{file:" + filepath.Join(promptDir, phase+".md") + "}"
		expectedRef = strings.ReplaceAll(expectedRef, `\`, `/`)
		if !strings.Contains(text, expectedRef) {
			t.Errorf("opencode.json sub-agent %q missing {file:...} reference %q", phase, expectedRef)
		}
	}
}

func TestWriteSharedPromptFilesOmitCodeGraphGuidanceByDefault(t *testing.T) {
	home := t.TempDir()

	if _, err := WriteSharedPromptFiles(home, nil); err != nil {
		t.Fatalf("WriteSharedPromptFiles() error = %v", err)
	}

	for _, phase := range SharedPromptPhases() {
		path := filepath.Join(SharedPromptDir(home), phase+".md")
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("ReadFile(%q) error = %v", path, err)
		}
		text := string(content)
		if strings.Contains(text, "<!-- gentle-ai:codegraph-guidance -->") || strings.Contains(text, "gentle-ai codegraph init --cwd <project-root>") {
			t.Fatalf("%s unexpectedly contains CodeGraph guidance by default", phase)
		}
	}
}

func TestWriteSharedPromptFilesIncludeCodeGraphGuidanceWhenEnabled(t *testing.T) {
	home := t.TempDir()
	guidance := communitytool.CodeGraphGuidanceMarkdown()

	if _, err := WriteSharedPromptFiles(home, nil, guidance); err != nil {
		t.Fatalf("WriteSharedPromptFiles() error = %v", err)
	}

	for _, phase := range SharedPromptPhases() {
		path := filepath.Join(SharedPromptDir(home), phase+".md")
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("ReadFile(%q) error = %v", path, err)
		}
		text := string(content)
		if !strings.Contains(text, "<!-- gentle-ai:codegraph-guidance -->") || !strings.Contains(text, "gentle-ai codegraph init --cwd <project-root>") {
			t.Fatalf("%s missing CodeGraph guidance when enabled", phase)
		}
		if count := strings.Count(text, "<!-- gentle-ai:codegraph-guidance -->"); count != 1 {
			t.Fatalf("%s has %d CodeGraph guidance sections, want 1", phase, count)
		}
	}
}

func TestInjectOpenCodeSingleModeSubagentPromptsOmitCodeGraphGuidanceByDefault(t *testing.T) {
	home := t.TempDir()
	mockNoPackageManager(t)

	if _, err := Inject(home, opencodeAdapter(), model.SDDModeSingle); err != nil {
		t.Fatalf("Inject(single) error = %v", err)
	}

	agentsMap := readOpenCodeAgents(t, filepath.Join(home, ".config", "opencode", "opencode.json"))
	for _, agentName := range inlineOpenCodeSubAgentsForCodeGraphTest(t, agentsMap) {
		prompt := agentPrompt(t, agentsMap, agentName)
		if strings.Contains(prompt, "<!-- gentle-ai:codegraph-guidance -->") || strings.Contains(prompt, "gentle-ai codegraph init --cwd <project-root>") {
			t.Fatalf("%s unexpectedly contains CodeGraph guidance by default", agentName)
		}
	}
}

func TestInjectOpenCodeSingleModeSubagentPromptsIncludeCodeGraphGuidanceWhenEnabled(t *testing.T) {
	home := t.TempDir()
	mockNoPackageManager(t)
	options := InjectOptions{CodeGraphGuidanceMarkdown: communitytool.CodeGraphGuidanceMarkdown()}

	if _, err := Inject(home, opencodeAdapter(), model.SDDModeSingle, options); err != nil {
		t.Fatalf("Inject(single) error = %v", err)
	}

	settingsPath := filepath.Join(home, ".config", "opencode", "opencode.json")
	agentsMap := readOpenCodeAgents(t, settingsPath)
	covered := inlineOpenCodeSubAgentsForCodeGraphTest(t, agentsMap)
	for _, agentName := range covered {
		prompt := agentPrompt(t, agentsMap, agentName)
		if !strings.Contains(prompt, "<!-- gentle-ai:codegraph-guidance -->") || !strings.Contains(prompt, "gentle-ai codegraph init --cwd <project-root>") {
			t.Fatalf("%s missing CodeGraph guidance when enabled", agentName)
		}
		if count := strings.Count(prompt, "<!-- gentle-ai:codegraph-guidance -->"); count != 1 {
			t.Fatalf("%s has %d CodeGraph guidance sections, want 1", agentName, count)
		}
	}
	if !containsString(covered, reviewRefuterAgentName) {
		t.Fatalf("generated OpenCode subagent coverage missing %s", reviewRefuterAgentName)
	}

	beforeSecond, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("ReadFile(%q) before second injection error = %v", settingsPath, err)
	}
	second, err := Inject(home, opencodeAdapter(), model.SDDModeSingle, options)
	if err != nil {
		t.Fatalf("Inject(single) second error = %v", err)
	}
	if second.Changed {
		t.Fatal("Inject(single) second changed = true, want idempotent output")
	}
	afterSecond, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("ReadFile(%q) after second injection error = %v", settingsPath, err)
	}
	if string(afterSecond) != string(beforeSecond) {
		t.Fatal("Inject(single) second invocation altered generated OpenCode agent content")
	}
}

func TestInjectOpenCodeMultiModeSubagentPromptFilesIncludeCodeGraphGuidanceWhenEnabled(t *testing.T) {
	home := t.TempDir()
	mockNoPackageManager(t)
	options := InjectOptions{CodeGraphGuidanceMarkdown: communitytool.CodeGraphGuidanceMarkdown()}

	if _, err := Inject(home, opencodeAdapter(), model.SDDModeMulti, options); err != nil {
		t.Fatalf("Inject(multi) error = %v", err)
	}

	promptContents := make(map[string]string, len(SharedPromptPhases()))
	for _, phase := range SharedPromptPhases() {
		path := filepath.Join(SharedPromptDir(home), phase+".md")
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("ReadFile(%q) error = %v", path, err)
		}
		text := string(content)
		if !strings.Contains(text, "<!-- gentle-ai:codegraph-guidance -->") || !strings.Contains(text, "gentle-ai codegraph init --cwd <project-root>") {
			t.Fatalf("%s missing CodeGraph guidance when enabled", phase)
		}
		if count := strings.Count(text, "<!-- gentle-ai:codegraph-guidance -->"); count != 1 {
			t.Fatalf("%s has %d CodeGraph guidance sections, want 1", phase, count)
		}
		promptContents[path] = text
	}

	settingsPath := filepath.Join(home, ".config", "opencode", "opencode.json")
	agentsMap := readOpenCodeAgents(t, settingsPath)
	covered := inlineOpenCodeSubAgentsForCodeGraphTest(t, agentsMap)
	for _, agentName := range covered {
		prompt := agentPrompt(t, agentsMap, agentName)
		if !strings.Contains(prompt, "<!-- gentle-ai:codegraph-guidance -->") || !strings.Contains(prompt, "gentle-ai codegraph init --cwd <project-root>") {
			t.Fatalf("%s missing CodeGraph guidance in multi-mode inline prompt when enabled", agentName)
		}
		if count := strings.Count(prompt, "<!-- gentle-ai:codegraph-guidance -->"); count != 1 {
			t.Fatalf("%s has %d CodeGraph guidance sections in multi-mode inline prompt, want 1", agentName, count)
		}
	}
	if !containsString(covered, reviewRefuterAgentName) {
		t.Fatalf("generated OpenCode subagent coverage missing %s", reviewRefuterAgentName)
	}

	settingsBeforeSecond, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("ReadFile(%q) before second injection error = %v", settingsPath, err)
	}
	second, err := Inject(home, opencodeAdapter(), model.SDDModeMulti, options)
	if err != nil {
		t.Fatalf("Inject(multi) second error = %v", err)
	}
	if second.Changed {
		t.Fatal("Inject(multi) second changed = true, want idempotent output")
	}
	settingsAfterSecond, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("ReadFile(%q) after second injection error = %v", settingsPath, err)
	}
	if string(settingsAfterSecond) != string(settingsBeforeSecond) {
		t.Fatal("Inject(multi) second invocation altered generated OpenCode agent content")
	}
	for path, beforeSecond := range promptContents {
		afterSecond, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("ReadFile(%q) after second injection error = %v", path, err)
		}
		if string(afterSecond) != beforeSecond {
			t.Fatalf("Inject(multi) second invocation altered generated prompt %q", path)
		}
	}
}

func TestInjectCodeGraphToolGrantIntoPrompt(t *testing.T) {
	guidance := "CodeGraph guidance"
	tests := []struct {
		name    string
		agentID model.AgentID
		prompt  string
		want    string
		wantErr string
	}{
		{
			name:    "Claude preserves LF frontmatter and existing tools",
			agentID: model.AgentClaudeCode,
			prompt:  "---\nname: test\ntools: Read, Grep\nmodel: sonnet\n---\nBody\n",
			want:    "---\nname: test\ntools: Read, Grep, mcp__codegraph__codegraph_explore\nmodel: sonnet\n---\nBody\n",
		},
		{
			name:    "Claude preserves CRLF frontmatter and body",
			agentID: model.AgentClaudeCode,
			prompt:  "---\r\nname: test\r\ntools: Read\r\n---\r\nBody\r\n",
			want:    "---\r\nname: test\r\ntools: Read, mcp__codegraph__codegraph_explore\r\n---\r\nBody\r\n",
		},
		{
			name:    "Claude accepts an empty tools declaration",
			agentID: model.AgentClaudeCode,
			prompt:  "---\ntools:\n---\nBody\n",
			want:    "---\ntools: mcp__codegraph__codegraph_explore\n---\nBody\n",
		},
		{
			name:    "Kiro accepts an empty tools list",
			agentID: model.AgentKiroIDE,
			prompt:  "---\ntools: []\n---\nBody\n",
			want:    "---\ntools: [\"@codegraph\"]\n---\nBody\n",
		},
		{
			name:    "Kiro preserves existing tools and frontmatter",
			agentID: model.AgentKiroIDE,
			prompt:  "---\nname: test\ntools: [\"read\"]\nmodel: auto\n---\nBody\n",
			want:    "---\nname: test\ntools: [\"read\", \"@codegraph\"]\nmodel: auto\n---\nBody\n",
		},
		{
			name:    "missing frontmatter fails explicitly",
			agentID: model.AgentClaudeCode,
			prompt:  "tools: Read\nBody\n",
			wantErr: "missing YAML frontmatter opening delimiter",
		},
		{
			name:    "malformed frontmatter fails explicitly",
			agentID: model.AgentClaudeCode,
			prompt:  "---\ntools: Read\nBody\n",
			wantErr: "missing YAML frontmatter closing delimiter",
		},
		{
			name:    "missing tools fails explicitly",
			agentID: model.AgentClaudeCode,
			prompt:  "---\nname: test\n---\nBody\n",
			wantErr: "missing tools declaration",
		},
		{
			name:    "malformed Claude tools fails explicitly",
			agentID: model.AgentClaudeCode,
			prompt:  "---\ntools: Read,\n---\nBody\n",
			wantErr: "malformed Claude tools declaration",
		},
		{
			name:    "malformed Kiro tools fails explicitly",
			agentID: model.AgentKiroIDE,
			prompt:  "---\ntools: [\"read\"\n---\nBody\n",
			wantErr: "malformed Kiro tools declaration",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := injectCodeGraphToolGrantIntoPrompt(tc.prompt, tc.agentID, guidance)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("error = %v, want containing %q", err, tc.wantErr)
				}
				if got != tc.prompt {
					t.Fatalf("failed mutation changed prompt:\n%s", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("result mismatch:\ngot:  %q\nwant: %q", got, tc.want)
			}
			gotAgain, err := injectCodeGraphToolGrantIntoPrompt(got, tc.agentID, guidance)
			if err != nil {
				t.Fatalf("idempotence call error: %v", err)
			}
			if gotAgain != got {
				t.Fatalf("second mutation changed prompt:\n%s", gotAgain)
			}
		})
	}
}

func TestInjectNativeSDDSubagentsIncludeCodeGraphGuidanceWhenEnabled(t *testing.T) {
	tests := []struct {
		name      string
		agentID   model.AgentID
		toolGrant string
	}{
		{name: "claude", agentID: model.AgentClaudeCode, toolGrant: "mcp__codegraph__codegraph_explore"},
		{name: "cursor", agentID: model.AgentCursor},
		{name: "kiro", agentID: model.AgentKiroIDE, toolGrant: "@codegraph"},
		{name: "kimi", agentID: model.AgentKimi},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			home := t.TempDir()
			adapter := mustAdapter(t, tc.agentID)

			if _, err := Inject(home, adapter, model.SDDModeSingle, InjectOptions{CodeGraphGuidanceMarkdown: communitytool.CodeGraphGuidanceMarkdown()}); err != nil {
				t.Fatalf("Inject(%s) error = %v", tc.name, err)
			}

			for _, fileName := range nativeMarkdownSubAgentFilesForCodeGraphTest(t, adapter) {
				path := filepath.Join(adapter.SubAgentsDir(home), fileName)
				content, err := os.ReadFile(path)
				if err != nil {
					t.Fatalf("ReadFile(%q) error = %v", path, err)
				}
				text := string(content)
				if !strings.Contains(text, "<!-- gentle-ai:codegraph-guidance -->") || !strings.Contains(text, "gentle-ai codegraph init --cwd <project-root>") {
					t.Fatalf("%s native subagent missing CodeGraph guidance when enabled", fileName)
				}
				if count := strings.Count(text, "<!-- gentle-ai:codegraph-guidance -->"); count != 1 {
					t.Fatalf("%s has %d CodeGraph guidance sections, want 1", fileName, count)
				}

				toolsLine := ""
				if tc.toolGrant != "" {
					toolsLine = frontmatterToolsLineForCodeGraphTest(t, text)
				}
				for _, grant := range []string{"mcp__codegraph__codegraph_explore", "@codegraph"} {
					wantCount := 0
					if grant == tc.toolGrant {
						wantCount = 1
					}
					if count := strings.Count(text, grant); count != wantCount {
						t.Fatalf("%s tools line has %d %q grants, want %d: %s", fileName, count, grant, wantCount, toolsLine)
					}
				}

				if tc.toolGrant != "" {
					source := renderBoundedReviewAsset(adapter.EmbeddedSubAgentsDir() + "/" + fileName)
					sourceToolsLine := frontmatterToolsLineForCodeGraphTest(t, source)
					wantToolsLine := sourceToolsLine + ", " + tc.toolGrant
					if tc.agentID == model.AgentKiroIDE {
						wantToolsLine = strings.TrimSuffix(sourceToolsLine, "]") + `, "` + tc.toolGrant + `"]`
					}
					if toolsLine != wantToolsLine {
						t.Fatalf("%s tools line = %q, want preserved tools plus grant %q", fileName, toolsLine, wantToolsLine)
					}
					if strings.Count(toolsLine, "Bash") != strings.Count(sourceToolsLine, "Bash") {
						t.Fatalf("%s CodeGraph grant changed Bash access: before %q, after %q", fileName, sourceToolsLine, toolsLine)
					}
				}
			}

			second, err := Inject(home, adapter, model.SDDModeSingle, InjectOptions{CodeGraphGuidanceMarkdown: communitytool.CodeGraphGuidanceMarkdown()})
			if err != nil {
				t.Fatalf("Inject(%s) second error = %v", tc.name, err)
			}
			if second.Changed {
				t.Fatalf("Inject(%s) second changed = true, want idempotent output", tc.name)
			}
		})
	}
}

func TestInjectNativeSDDSubagentsMalformedLaterPromptLeavesBatchUnchanged(t *testing.T) {
	home := t.TempDir()
	adapter := claudeAdapter()
	if _, err := Inject(home, adapter, model.SDDModeSingle); err != nil {
		t.Fatalf("Inject(claude) baseline error = %v", err)
	}

	fileNames := nativeMarkdownSubAgentFilesForCodeGraphTest(t, adapter)
	if len(fileNames) < 2 {
		t.Fatalf("native Markdown prompt count = %d, want at least 2", len(fileNames))
	}
	before := make(map[string]string, len(fileNames))
	for _, fileName := range fileNames {
		path := filepath.Join(adapter.SubAgentsDir(home), fileName)
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("ReadFile(%q) before failed batch error = %v", path, err)
		}
		before[path] = string(content)
	}

	laterFile := fileNames[1]
	originalRenderer := renderNativeSubAgentAsset
	renderNativeSubAgentAsset = func(path string) string {
		if filepath.Base(path) == laterFile {
			return "---\ntools: Read\nmissing closing delimiter\n"
		}
		return originalRenderer(path)
	}
	t.Cleanup(func() { renderNativeSubAgentAsset = originalRenderer })

	result, err := Inject(home, adapter, model.SDDModeSingle, InjectOptions{CodeGraphGuidanceMarkdown: communitytool.CodeGraphGuidanceMarkdown()})
	if err == nil {
		t.Fatal("Inject(claude) malformed later prompt error = nil")
	}
	for _, want := range []string{"grant CodeGraph tool in agent " + laterFile, "missing YAML frontmatter closing delimiter"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("Inject(claude) error = %q, want containing %q", err, want)
		}
	}
	if result.Changed {
		t.Fatalf("Inject(claude) changed = true, want false: %+v", result)
	}
	for _, path := range result.Files {
		if filepath.Dir(path) == adapter.SubAgentsDir(home) {
			t.Fatalf("Inject(claude) result contains partially updated native agent %q", path)
		}
	}

	earlyPath := filepath.Join(adapter.SubAgentsDir(home), fileNames[0])
	if strings.Contains(before[earlyPath], "<!-- gentle-ai:codegraph-guidance -->") {
		t.Fatalf("early prompt %q unexpectedly had CodeGraph guidance before failed batch", earlyPath)
	}
	for path, want := range before {
		content, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatalf("ReadFile(%q) after failed batch error = %v", path, readErr)
		}
		if string(content) != want {
			t.Fatalf("failed native batch changed %q", path)
		}
	}
}

func TestInjectNativeSDDSubagentsWriteFailureReturnsChangedInventory(t *testing.T) {
	home := t.TempDir()
	adapter := claudeAdapter()
	if _, err := Inject(home, adapter, model.SDDModeSingle); err != nil {
		t.Fatalf("Inject(claude) baseline error = %v", err)
	}

	fileNames := nativeMarkdownSubAgentFilesForCodeGraphTest(t, adapter)
	if len(fileNames) < 3 {
		t.Fatalf("native Markdown prompt count = %d, want at least 3", len(fileNames))
	}
	earlyPath := filepath.Join(adapter.SubAgentsDir(home), fileNames[0])
	failingPath := filepath.Join(adapter.SubAgentsDir(home), fileNames[2])
	if err := os.Remove(failingPath); err != nil {
		t.Fatalf("Remove(%q) error = %v", failingPath, err)
	}
	if err := os.Mkdir(failingPath, 0o755); err != nil {
		t.Fatalf("Mkdir(%q) error = %v", failingPath, err)
	}

	result, err := Inject(home, adapter, model.SDDModeSingle, InjectOptions{CodeGraphGuidanceMarkdown: communitytool.CodeGraphGuidanceMarkdown()})
	if err == nil || !strings.Contains(err.Error(), "write agent "+fileNames[2]) {
		t.Fatalf("Inject(claude) error = %v, want write context for %s", err, fileNames[2])
	}
	if !result.Changed {
		t.Fatalf("Inject(claude) changed = false, want partial write inventory: %+v", result)
	}
	if !containsString(result.Files, earlyPath) {
		t.Fatalf("Inject(claude) changed files = %v, want earlier changed file %q", result.Files, earlyPath)
	}
	if containsString(result.Files, failingPath) {
		t.Fatalf("Inject(claude) changed files includes failed target %q", failingPath)
	}
}

func TestInjectNativeSDDSubagentsPreserveExistingModeWhenUpdated(t *testing.T) {
	home := t.TempDir()
	adapter := claudeAdapter()
	if _, err := Inject(home, adapter, model.SDDModeSingle); err != nil {
		t.Fatalf("Inject(claude) baseline error = %v", err)
	}

	fileNames := nativeMarkdownSubAgentFilesForCodeGraphTest(t, adapter)
	path := filepath.Join(adapter.SubAgentsDir(home), fileNames[0])
	if err := os.Chmod(path, 0o600); err != nil {
		t.Fatalf("Chmod(%q) error = %v", path, err)
	}
	result, err := Inject(home, adapter, model.SDDModeSingle, InjectOptions{CodeGraphGuidanceMarkdown: communitytool.CodeGraphGuidanceMarkdown()})
	if err != nil {
		t.Fatalf("Inject(claude) update error = %v", err)
	}
	if !containsString(result.Files, path) {
		t.Fatalf("Inject(claude) changed files = %v, want %q", result.Files, path)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat(%q) error = %v", path, err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("updated mode = %o, want 600", got)
	}
}

func TestInjectNativeSDDSubagentsOmitCodeGraphGuidanceByDefault(t *testing.T) {
	for _, agentID := range []model.AgentID{model.AgentClaudeCode, model.AgentKiroIDE} {
		t.Run(string(agentID), func(t *testing.T) {
			home := t.TempDir()
			adapter := mustAdapter(t, agentID)
			if _, err := Inject(home, adapter, model.SDDModeSingle); err != nil {
				t.Fatalf("Inject(%s) error = %v", agentID, err)
			}

			for _, fileName := range nativeMarkdownSubAgentFilesForCodeGraphTest(t, adapter) {
				path := filepath.Join(adapter.SubAgentsDir(home), fileName)
				content, err := os.ReadFile(path)
				if err != nil {
					t.Fatalf("ReadFile(%q) error = %v", path, err)
				}
				text := string(content)
				if strings.Contains(text, "<!-- gentle-ai:codegraph-guidance -->") || strings.Contains(text, "gentle-ai codegraph init --cwd <project-root>") {
					t.Fatalf("%s native subagent unexpectedly contains CodeGraph guidance by default", fileName)
				}
				for _, grant := range []string{"mcp__codegraph__codegraph_explore", "@codegraph"} {
					if strings.Contains(frontmatterToolsLineForCodeGraphTest(t, text), grant) {
						t.Fatalf("%s native subagent unexpectedly grants %q by default", fileName, grant)
					}
				}

				source := renderBoundedReviewAsset(adapter.EmbeddedSubAgentsDir() + "/" + fileName)
				if got, want := frontmatterToolsLineForCodeGraphTest(t, text), frontmatterToolsLineForCodeGraphTest(t, source); got != want {
					t.Fatalf("%s default tools changed: got %q, want %q", fileName, got, want)
				}
			}
		})
	}
}

func TestInjectKimiYAMLSubagentsOmitCodeGraphGuidanceByDefault(t *testing.T) {
	home := t.TempDir()

	if _, err := Inject(home, kimiAdapter(), model.SDDModeSingle); err != nil {
		t.Fatalf("Inject(kimi) error = %v", err)
	}

	for _, fileName := range kimiYAMLSubagentFilesForCodeGraphTest() {
		path := filepath.Join(home, ".kimi", "agents", fileName)
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("ReadFile(%q) error = %v", path, err)
		}
		text := string(content)
		if strings.Contains(text, "  instructions: |-") || strings.Contains(text, "gentle-ai codegraph init --cwd <project-root>") {
			t.Fatalf("%s YAML unexpectedly contains CodeGraph guidance by default", fileName)
		}
	}
}

func TestInjectKimiYAMLSubagentsRemainControlFilesWhenCodeGraphEnabled(t *testing.T) {
	home := t.TempDir()

	if _, err := Inject(home, kimiAdapter(), model.SDDModeSingle, InjectOptions{CodeGraphGuidanceMarkdown: communitytool.CodeGraphGuidanceMarkdown()}); err != nil {
		t.Fatalf("Inject(kimi) error = %v", err)
	}

	for _, fileName := range kimiYAMLSubagentFilesForCodeGraphTest() {
		path := filepath.Join(home, ".kimi", "agents", fileName)
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("ReadFile(%q) error = %v", path, err)
		}
		text := string(content)
		for _, want := range []string{"  system_prompt_path: ./", "  exclude_tools:"} {
			if !strings.Contains(text, want) {
				t.Fatalf("%s YAML missing %q:\n%s", fileName, want, text)
			}
		}
		for _, forbidden := range []string{"  instructions: |-", "<!-- gentle-ai:codegraph-guidance -->", "gentle-ai codegraph init --cwd <project-root>"} {
			if strings.Contains(text, forbidden) {
				t.Fatalf("%s YAML unexpectedly contains %q:\n%s", fileName, forbidden, text)
			}
		}

		markdownPath := strings.TrimSuffix(path, ".yaml") + ".md"
		markdownContent, err := os.ReadFile(markdownPath)
		if err != nil {
			t.Fatalf("ReadFile(%q) error = %v", markdownPath, err)
		}
		markdownText := string(markdownContent)
		if !strings.Contains(markdownText, "<!-- gentle-ai:codegraph-guidance -->") || !strings.Contains(markdownText, "gentle-ai codegraph init --cwd <project-root>") {
			t.Fatalf("%s referenced Markdown prompt missing CodeGraph guidance when enabled", markdownPath)
		}
	}
}

// TestInjectOpenCodeMultiModeOrchestratorPromptIsStillInlined verifies that
// the orchestrator prompt is still inlined (not a file reference) after injection.
func TestInjectOpenCodeMultiModeOrchestratorPromptIsStillInlined(t *testing.T) {
	home := t.TempDir()
	mockNoPackageManager(t)

	if _, err := Inject(home, opencodeAdapter(), "multi"); err != nil {
		t.Fatalf("Inject(multi) error = %v", err)
	}

	settingsPath := filepath.Join(home, ".config", "opencode", "opencode.json")
	content, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("ReadFile(opencode.json) error = %v", err)
	}

	text := string(content)

	// The orchestrator still uses {file:./AGENTS.md} from the overlay (not from prompts/).
	// We check that there's NO file reference inside the prompts/sdd/ dir for orchestrator.
	promptDir := SharedPromptDir(home)
	if strings.Contains(text, filepath.Join(promptDir, "sdd-orchestrator.md")) {
		t.Fatal("orchestrator should NOT use a file reference from prompts/sdd/")
	}
}

// TestInjectOpenCodeMultiModeIdempotentWithPromptFiles verifies that the second
// Inject call returns changed=false when prompt files are already on disk.
func TestInjectOpenCodeMultiModeIdempotentWithPromptFiles(t *testing.T) {
	home := t.TempDir()
	mockNoPackageManager(t)

	first, err := Inject(home, opencodeAdapter(), "multi")
	if err != nil {
		t.Fatalf("Inject(multi) first error = %v", err)
	}
	if !first.Changed {
		t.Fatal("Inject(multi) first changed = false")
	}

	second, err := Inject(home, opencodeAdapter(), "multi")
	if err != nil {
		t.Fatalf("Inject(multi) second error = %v", err)
	}
	if second.Changed {
		t.Fatal("Inject(multi) second changed = true — should be idempotent with prompt files")
	}
}
