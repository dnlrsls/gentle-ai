package engram

import (
	"os"
	"strings"
	"testing"
)

// readTestdata loads a fixture file captured from the pre-consolidation
// source assets (see testdata/*.md). Fixtures are byte-exact snapshots so
// tests can assert canonical-renderer output without depending on the old
// per-adapter asset files, which are deleted once every renderer is green
// (task 2.8).
func readTestdata(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatalf("ReadFile(testdata/%s) error = %v", name, err)
	}
	return string(data)
}

func TestProtocolFullByteEqualsPreConsolidationClaudeAsset(t *testing.T) {
	want := readTestdata(t, "protocol-full.md")
	got := protocolFull()
	if got != want {
		t.Fatalf("protocolFull() not byte-identical to pre-consolidation claude/engram-protocol.md\nwant (%d bytes):\n%s\ngot (%d bytes):\n%s", len(want), want, len(got), got)
	}
}

func TestProtocolSlimMatchesDecision2Block(t *testing.T) {
	want := readTestdata(t, "protocol-slim.md")
	got := protocolSlim()
	if got != want {
		t.Fatalf("protocolSlim() mismatch\nwant:\n%s\ngot:\n%s", want, got)
	}
	// Scenario: "Verified adapter gets the slim section" — slim variant MUST
	// contain a pointer to the full protocol location.
	if !strings.Contains(got, "MCP server instructions and the SessionStart hook") {
		t.Fatal("protocolSlim() missing pointer to the full protocol location")
	}
}

func TestCodexInstructionsIsFullPlusPassiveCaptureInOrder(t *testing.T) {
	fullContent := protocolFull()
	passiveCapture := readTestdata(t, "protocol-passive-capture.md")

	got := codexInstructions()
	want := fullContent + "\n" + passiveCapture

	if got != want {
		t.Fatalf("codexInstructions() != protocolFull() + passive-capture\nwant (%d bytes):\n%s\ngot (%d bytes):\n%s", len(want), want, len(got), got)
	}

	// PASSIVE CAPTURE content MUST survive consolidation, unchanged.
	if !strings.Contains(got, "### PASSIVE CAPTURE — automatic learning extraction") {
		t.Fatal("codexInstructions() missing PASSIVE CAPTURE heading")
	}
	if !strings.Contains(got, "mem_capture_passive(content)") {
		t.Fatal("codexInstructions() missing PASSIVE CAPTURE mem_capture_passive reference")
	}

	// Explicit content-growth assertion (not a loose Contains check): the
	// canonical full text (12 PROACTIVE SAVE TRIGGERS bullets + self-check
	// line) is strictly larger than the old Codex-owned copy (6 WHEN TO SAVE
	// bullets, no self-check line) it replaces.
	oldCodexInstructions := readTestdata(t, "protocol-old-codex-instructions.md")
	if len(got) <= len(oldCodexInstructions) {
		t.Fatalf("codexInstructions() must grow vs. pre-consolidation asset: got %d bytes, old asset was %d bytes", len(got), len(oldCodexInstructions))
	}
	if !strings.Contains(got, `Self-check after EVERY task`) {
		t.Fatal("codexInstructions() missing the self-check line that today's Codex asset lacks (expected content growth)")
	}
}

func TestCodexCompactMatchesPreConsolidationCompactPrompt(t *testing.T) {
	want := readTestdata(t, "protocol-compact.md")
	got := codexCompact()
	if got != want {
		t.Fatalf("codexCompact() mismatch\nwant:\n%s\ngot:\n%s", want, got)
	}
}

func TestExtractProtocolSectionBoundsAllFourMarkerPairs(t *testing.T) {
	content := protocolAssetContent()

	tests := []struct {
		name           string
		mustHavePrefix string
	}{
		{"full", "## Engram Persistent Memory — Protocol"},
		{"slim", "## Engram Persistent Memory"},
		{"passive-capture", "### PASSIVE CAPTURE"},
		{"compact", "You are compacting a coding session"},
	}

	for _, tt := range tests {
		openMarker := "<!-- section:" + tt.name + " -->"
		closeMarker := "<!-- /section:" + tt.name + " -->"
		if !strings.Contains(content, openMarker) {
			t.Fatalf("canonical asset missing open marker %q", openMarker)
		}
		if !strings.Contains(content, closeMarker) {
			t.Fatalf("canonical asset missing close marker %q", closeMarker)
		}

		section := extractProtocolSection(content, tt.name)
		if section == content {
			t.Fatalf("extractProtocolSection(%q) fell back to full content — markers not bounded correctly", tt.name)
		}
		if !strings.HasPrefix(section, tt.mustHavePrefix) {
			t.Fatalf("extractProtocolSection(%q) = %q, want prefix %q", tt.name, section[:min(40, len(section))], tt.mustHavePrefix)
		}
	}
}
