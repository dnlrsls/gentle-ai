package sddstatus

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

const testCandidate = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

func TestRuntimeAttemptLedgerEnforcesCumulativeBudget(t *testing.T) {
	first := testAttemptEnvelope(0, 1, "failed", "none", "go test ./e2e", "none", "none", "")
	firstHash := envelopeField(first, "record_hash")
	second := testAttemptEnvelope(0, 2, "running", "timeout diagnosed from captured logs", "go test ./e2e", firstHash, "none", "")
	ledger := first + "\nTask fingerprint: old\nSession: alpha\n" + second
	tests := []struct {
		name, content, reason      string
		valid, exhausted, decision bool
		interrupted                int
	}{
		{name: "budget persists across task fingerprint and session prose", content: ledger, valid: true, exhausted: true, decision: true, interrupted: 1},
		{name: "duplicate scalar rejected", content: strings.Replace(first, "attempt: 1", "attempt: 1\nattempt: 1", 1), reason: "duplicate"},
		{name: "malformed envelope rejected", content: strings.TrimSuffix(first, "\n```"), reason: "malformed"},
		{name: "schema-specific extra field rejected", content: strings.Replace(first, "attempt: 1", "attempt: 1\nreason: not allowed", 1), reason: "field set"},
		{name: "unsupported version rejected", content: strings.Replace(first, runtimeAttemptSchema, "gentle-ai.runtime-attempt/v2", 1), reason: "unsupported"},
		{name: "inconsistent counter rejected", content: first + "\n" + strings.Replace(second, "attempt: 2", "attempt: 3", 1), reason: "counter"},
		{name: "inconsistent hash rejected", content: strings.Replace(first, "record_hash: sha256:", "record_hash: sha256:0", 1), reason: "hash"},
		{name: "correction requires diagnosis", content: first + "\n" + testAttemptEnvelope(0, 2, "failed", "none", "go test ./e2e", firstHash, "none", ""), reason: "diagnosis"},
		{name: "changed harness requires invalidation", content: first + "\n" + testAttemptEnvelope(0, 2, "failed", "root cause confirmed in logs", "go test ./integration", firstHash, "none", ""), reason: "harness reuse"},
		{name: "changed harness accepts explicit invalidation", content: first + "\n" + testAttemptEnvelope(0, 2, "failed", "root cause confirmed in logs", "go test ./integration", firstHash, "predecessor harness omitted required service startup", ""), valid: true, exhausted: true, decision: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseRuntimeAttempts(tt.content, "change-a")
			wantDecision := tt.decision || tt.reason != ""
			if got.Valid != tt.valid || got.Exhausted != tt.exhausted || got.DecisionRequired != wantDecision || got.Interrupted != tt.interrupted {
				t.Fatalf("state = %#v", got)
			}
			if tt.reason != "" && !strings.Contains(got.Reason, tt.reason) {
				t.Fatalf("reason = %q, want containing %q", got.Reason, tt.reason)
			}
		})
	}
	identity0 := [5]string{"change-a", "wu-1", "runtime evidence", testCandidate, "0"}
	identity1 := [5]string{"change-a", "wu-1", "runtime evidence", testCandidate, "1"}
	if runtimeBudgetID(identity0) == runtimeBudgetID(identity1) {
		t.Fatal("scope generation must change budget identity")
	}
}

func TestRuntimeAttemptScopeReset(t *testing.T) {
	first := testAttemptEnvelope(0, 1, "failed", "none", "go test ./e2e", "none", "none", "")
	firstHash := envelopeField(first, "record_hash")
	second := testAttemptEnvelope(0, 2, "failed", "failure diagnosed from service logs", "go test ./e2e", firstHash, "none", "")
	reset := testResetEnvelope(0, 1, runtimeBudgetID([5]string{"change-a", "wu-1", "runtime evidence", testCandidate, "0"}), "maintainer approved a materially new evidence scope")
	valid := first + "\n" + second + "\n" + reset + "\n" + testAttemptEnvelope(1, 1, "running", "none", "go test ./e2e", "none", "none", "")
	tests := []struct {
		name, content, reason string
		valid, decision       bool
	}{
		{name: "explicit reset opens next generation", content: valid, valid: true},
		{name: "generation cannot advance without reset", content: first + "\n" + second + "\n" + testAttemptEnvelope(1, 1, "running", "none", "go test ./e2e", "none", "none", ""), reason: "budget_id"},
		{name: "reset requires exhausted predecessor", content: first + "\n" + reset, reason: "scope reset"},
		{name: "reset predecessor must match", content: first + "\n" + second + "\n" + strings.Replace(reset, "predecessor_budget_id: sha256:", "predecessor_budget_id: sha256:0", 1), reason: "scope reset"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseRuntimeAttempts(tt.content, "change-a")
			wantDecision := tt.decision || tt.reason != ""
			if got.Valid != tt.valid || got.DecisionRequired != wantDecision {
				t.Fatalf("state = %#v", got)
			}
			if tt.reason != "" && !strings.Contains(got.Reason, tt.reason) {
				t.Fatalf("reason = %q", got.Reason)
			}
		})
	}
}

func TestResolveRoutesExhaustedRuntimeAttemptsForNativeStores(t *testing.T) {
	ledger := testAttemptEnvelope(0, 1, "failed", "none", "go test ./e2e", "none", "none", "")
	ledger += "\n" + testAttemptEnvelope(0, 2, "failed", "failure diagnosed from service logs", "go test ./e2e", envelopeField(ledger, "record_hash"), "none", "")
	t.Run("OpenSpec", func(t *testing.T) {
		root := t.TempDir()
		changeRoot := seedReadyChange(t, root, "change-a", "- [ ] 1.1 Work\n")
		write(t, filepath.Join(changeRoot, "apply-progress.md"), ledger)
		status, err := Resolve(ResolveOptions{CWD: root})
		if err != nil || status.NextRecommended != "decision-required" || !status.RuntimeAttempts.Exhausted {
			t.Fatalf("status = %#v err = %v", status, err)
		}
	})
	t.Run("Engram", func(t *testing.T) {
		root := t.TempDir()
		mkdir(t, filepath.Join(root, ".engram"))
		write(t, filepath.Join(root, ".git", "config"), "[remote \"origin\"]\n url = git@github.com:Gentleman-Programming/gentle-ai.git\n")
		restore := stubEngramExport(t, []engramObservation{{Title: "sdd/change-a/proposal", Content: "ready", Project: "gentle-ai", Scope: "project"}, {Title: "sdd/change-a/spec", Content: "### Requirement: A\n#### Scenario: B", Project: "gentle-ai", Scope: "project"}, {Title: "sdd/change-a/design", Content: "ready", Project: "gentle-ai", Scope: "project"}, {Title: "sdd/change-a/tasks", Content: "- [ ] 1.1 Work", Project: "gentle-ai", Scope: "project"}, {Title: "sdd/change-a/apply-progress", Content: ledger, Project: "gentle-ai", Scope: "project"}})
		defer restore()
		status, err := Resolve(ResolveOptions{CWD: root})
		if err != nil || status.ArtifactStore != ArtifactStoreEngram || status.NextRecommended != "decision-required" || !status.RuntimeAttempts.DecisionRequired {
			t.Fatalf("status = %#v err = %v", status, err)
		}
	})
}

func testAttemptEnvelope(generation, attempt int, state, diagnosis, harness, predecessor, invalidation, recordHash string) string {
	identity := [5]string{"change-a", "wu-1", "runtime evidence", testCandidate, fmt.Sprint(generation)}
	fields := map[string]string{"schema": runtimeAttemptSchema, "change": identity[0], "work_unit_id": identity[1], "evidence_goal": identity[2], "candidate_id": identity[3], "scope_generation": identity[4], "budget_id": runtimeBudgetID(identity), "attempt": fmt.Sprint(attempt), "state": state, "diagnosis": diagnosis, "harness": harness, "predecessor_hash": predecessor, "predecessor_invalidation": invalidation}
	if recordHash == "" {
		fields["record_hash"] = runtimeHash(attemptHashValues(fields))
	} else {
		fields["record_hash"] = recordHash
	}
	return testRuntimeEnvelope(fields, []string{"schema", "change", "work_unit_id", "evidence_goal", "candidate_id", "scope_generation", "budget_id", "attempt", "state", "diagnosis", "harness", "predecessor_hash", "predecessor_invalidation", "record_hash"})
}

func testResetEnvelope(from, to int, predecessor, reason string) string {
	fields := map[string]string{"schema": runtimeAttemptResetSchema, "change": "change-a", "work_unit_id": "wu-1", "evidence_goal": "runtime evidence", "candidate_id": testCandidate, "scope_generation": fmt.Sprint(to), "from_generation": fmt.Sprint(from), "to_generation": fmt.Sprint(to), "predecessor_budget_id": predecessor, "reason": reason}
	fields["record_hash"] = runtimeHash(resetHashValues(fields))
	return testRuntimeEnvelope(fields, []string{"schema", "change", "work_unit_id", "evidence_goal", "candidate_id", "scope_generation", "from_generation", "to_generation", "predecessor_budget_id", "reason", "record_hash"})
}

func testRuntimeEnvelope(fields map[string]string, order []string) string {
	lines := []string{"```yaml"}
	for _, key := range order {
		lines = append(lines, key+": "+fields[key])
	}
	return strings.Join(append(lines, "```"), "\n")
}
func envelopeField(envelope, key string) string {
	for _, line := range strings.Split(envelope, "\n") {
		if value, ok := strings.CutPrefix(line, key+": "); ok {
			return value
		}
	}
	return ""
}
