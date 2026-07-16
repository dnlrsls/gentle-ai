package reviewtransaction

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

func TestCorrectionRequiredScopeRecoveryCreatesFreshAuditableSuccessor(t *testing.T) {
	repo, predecessor, store, predecessorRecord := correctionScopeRecoveryFixture(t, "correction-scope-predecessor")
	stateBefore, err := os.ReadFile(store.StatePath())
	if err != nil {
		t.Fatal(err)
	}
	receiptBefore := []byte("preserve existing receipt bytes\n")
	if err := os.WriteFile(store.ReceiptPath(), receiptBefore, 0o644); err != nil {
		t.Fatal(err)
	}
	writeSnapshotFile(t, repo, "process_helper.go", "package processhelper\n")
	successor := newCompactTestStateWithIntended(t, repo, "correction-scope-successor", []string{"process_helper.go"})
	successor.Generation = predecessor.Generation + 1
	recoveredAt := time.Date(2026, 7, 16, 15, 0, 0, 0, time.UTC)
	request := CompactRecoveryRequest{
		PredecessorLineageID: predecessor.LineageID, ExpectedPredecessorRevision: predecessorRecord.Revision,
		Successor: successor, Disposition: RecoveryScopeChanged, Reason: "correction requires a process helper",
		Actor: "maintainer", MaintainerAuthorization: "approved issue #1257", RecoveredAt: recoveredAt,
	}

	recovered, err := RecoverCompactAuthority(context.Background(), repo, request)
	if err != nil {
		t.Fatal(err)
	}
	if recovered.State.Recovery == nil || recovered.State.Recovery.MaintainerAuthorization != request.MaintainerAuthorization ||
		recovered.State.Generation != predecessor.Generation+1 || !compactPristineReviewing(recovered.State) {
		t.Fatalf("recovered successor is not fresh: %#v", recovered.State)
	}
	if recovered.State.RiskLevel != successor.RiskLevel || recovered.State.OriginalChangedLines != successor.OriginalChangedLines ||
		recovered.State.CorrectionBudget != successor.CorrectionBudget || !equalStrings(recovered.State.GenesisPaths, successor.GenesisPaths) ||
		len(recovered.State.CorrectionAttempts) != 0 || recovered.State.CumulativeCorrectionLines != 0 {
		t.Fatalf("successor did not retain freshly derived inputs: %#v", recovered.State)
	}
	replayed, err := RecoverCompactAuthority(context.Background(), repo, request)
	if err != nil || replayed.Revision != recovered.Revision || !compactStateEqual(replayed.State, recovered.State) {
		t.Fatalf("exact replay = %#v, %v", replayed, err)
	}
	fork := newCompactTestStateWithIntended(t, repo, "correction-scope-fork", []string{"process_helper.go"})
	fork.Generation = predecessor.Generation + 1
	request.Successor = fork
	if _, err := RecoverCompactAuthority(context.Background(), repo, request); err == nil || !strings.Contains(err.Error(), "already has successor") {
		t.Fatalf("conflicting successor error = %v", err)
	}
	stateAfter, _ := os.ReadFile(store.StatePath())
	receiptAfter, _ := os.ReadFile(store.ReceiptPath())
	if !bytes.Equal(stateBefore, stateAfter) || !bytes.Equal(receiptBefore, receiptAfter) {
		t.Fatal("recovery changed predecessor state or receipt bytes")
	}
}

func TestCorrectionRequiredScopeRecoveryRejectsInvalidRequests(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*CompactRecoveryRequest, CompactState)
		want   string
	}{
		{name: "missing authorization", mutate: func(request *CompactRecoveryRequest, _ CompactState) { request.MaintainerAuthorization = "" }, want: "maintainer authorization"},
		{name: "wrong revision", mutate: func(request *CompactRecoveryRequest, _ CompactState) {
			request.ExpectedPredecessorRevision = hash("wrong")
		}, want: "expected predecessor revision"},
		{name: "same lineage", mutate: func(request *CompactRecoveryRequest, predecessor CompactState) {
			request.Successor.LineageID = predecessor.LineageID
		}, want: "distinct successor"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, predecessor, _, record := correctionScopeRecoveryFixture(t, "correction-invalid-predecessor")
			writeSnapshotFile(t, repo, "new_helper.go", "package newhelper\n")
			successor := newCompactTestStateWithIntended(t, repo, "correction-invalid-successor", []string{"new_helper.go"})
			successor.Generation = predecessor.Generation + 1
			request := CompactRecoveryRequest{PredecessorLineageID: predecessor.LineageID, ExpectedPredecessorRevision: record.Revision,
				Successor: successor, Disposition: RecoveryScopeChanged, Reason: "scope expanded", Actor: "maintainer", MaintainerAuthorization: "approved"}
			tt.mutate(&request, predecessor)
			if _, err := RecoverCompactAuthority(context.Background(), repo, request); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("recovery error = %v, want %q", err, tt.want)
			}
		})
	}

	t.Run("no outside-genesis path", func(t *testing.T) {
		repo, predecessor, _, record := correctionScopeRecoveryFixture(t, "correction-byte-predecessor")
		writeSnapshotFile(t, repo, "tracked.txt", "byte-only correction\n")
		successor := newCompactTestState(t, repo, "correction-byte-successor")
		successor.Generation = predecessor.Generation + 1
		_, err := RecoverCompactAuthority(context.Background(), repo, CompactRecoveryRequest{
			PredecessorLineageID: predecessor.LineageID, ExpectedPredecessorRevision: record.Revision,
			Successor: successor, Disposition: RecoveryScopeChanged, Reason: "only bytes changed", Actor: "maintainer", MaintainerAuthorization: "approved",
		})
		if err == nil || !strings.Contains(err.Error(), "path expansion") {
			t.Fatalf("byte-only recovery error = %v", err)
		}
	})
}

func TestCompactAuthorityGraphRejectsForgedCorrectionScopeRecovery(t *testing.T) {
	repo, predecessor, _, record := correctionScopeRecoveryFixture(t, "correction-graph-predecessor")
	writeSnapshotFile(t, repo, "tracked.txt", "in-genesis change\n")
	successor := newCompactTestState(t, repo, "correction-graph-successor")
	successor.Generation = predecessor.Generation + 1
	successor.Recovery = &CompactRecoveryProvenance{PredecessorLineageID: predecessor.LineageID, PredecessorRevision: record.Revision,
		Disposition: RecoveryScopeChanged, Reason: "forged reset", Actor: "maintainer", MaintainerAuthorization: "approved", RecoveredAt: time.Now().UTC()}
	successorStore, _ := CompactAuthoritativeStore(context.Background(), repo, successor.LineageID)
	_, payload, err := makeCompactRecord(successor)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(successorStore.Dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(successorStore.StatePath(), payload, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := CompactAuthorityLeaves(context.Background(), repo); err == nil || !strings.Contains(err.Error(), "path expansion") {
		t.Fatalf("forged graph error = %v", err)
	}
}

func correctionScopeRecoveryFixture(t *testing.T, lineage string) (string, CompactState, CompactStore, CompactRecord) {
	t.Helper()
	repo := initSnapshotRepo(t)
	writeSnapshotFile(t, repo, "tracked.txt", "base\none\ntwo\nthree\nwrong\n")
	state := newCompactTestState(t, repo, lineage)
	store := storeCompactStartAuthority(t, repo, state)
	started, _ := store.Load()
	finding := Finding{ID: "R3-001", Lens: "reliability", Location: "tracked.txt:5", Severity: "CRITICAL", Claim: "wrong value", ProofRefs: []string{"candidate-only failure"}}
	results := make([]LensResult, len(state.SelectedLenses))
	for index, lens := range state.SelectedLenses {
		results[index] = LensResult{Lens: lens, Findings: []Finding{}, Evidence: []string{"reviewed"}}
	}
	if len(results) == 0 {
		t.Fatal("correction fixture unexpectedly selected no lenses")
	}
	results[0].Findings = []Finding{finding}
	if err := state.CompleteReview(CompactReviewInput{LensResults: results,
		Classifications: []FindingEvidence{{FindingID: finding.ID, Class: EvidenceDeterministic, Causality: CausalIntroduced, Proof: "changed hunk"}}, RefuterOutcomes: []EvidenceResult{}}); err != nil {
		t.Fatal(err)
	}
	if state.State != StateCorrectionRequired {
		t.Fatalf("fixture state = %s", state.State)
	}
	if _, err := store.Replace(started.Revision, "review/complete-review", state); err != nil {
		t.Fatal(err)
	}
	record, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	return repo, state, store, record
}
