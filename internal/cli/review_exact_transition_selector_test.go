package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gentleman-programming/gentle-ai/internal/reviewtransaction"
)

func TestNegotiatedValidateTransitionPreservesExactBaseRef(t *testing.T) {
	repo := initReviewCLIRepo(t)
	branch := strings.TrimSpace(runReviewCLIGit(t, repo, "symbolic-ref", "--short", "HEAD"))
	runReviewCLIGit(t, repo, "branch", "release")
	configureCLIReviewPublicationRemote(t, repo, branch)
	runReviewCLIGit(t, repo, "push", "origin", "release")
	runReviewCLIGit(t, repo, "fetch", "origin", "release:refs/remotes/origin/release")
	if err := os.WriteFile(filepath.Join(repo, "tracked.txt"), []byte("candidate\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runReviewCLIGit(t, repo, "add", "tracked.txt")
	runReviewCLIGit(t, repo, "commit", "-qm", "candidate")

	var startedOutput bytes.Buffer
	if err := RunReviewFacadeStart([]string{"--cwd", repo, "--base-ref", "origin/release", "--committed-only"}, &startedOutput); err != nil {
		t.Fatal(err)
	}
	var started ReviewFacadeStartResult
	if err := json.Unmarshal(startedOutput.Bytes(), &started); err != nil {
		t.Fatal(err)
	}
	resultPath := filepath.Join(t.TempDir(), "review.json")
	evidencePath := filepath.Join(t.TempDir(), "evidence.txt")
	writeReviewCLIJSON(t, resultPath, facadeReviewerResult{Findings: []facadeFinding{}, Evidence: []string{"candidate reviewed"}})
	if err := os.WriteFile(evidencePath, []byte("verification passed\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := RunReviewFacadeFinalize([]string{"--cwd", repo, "--lineage", started.LineageID, "--result", resultPath}, io.Discard); err != nil {
		t.Fatal(err)
	}
	if err := RunReviewFacadeFinalize([]string{"--cwd", repo, "--lineage", started.LineageID, "--evidence", evidencePath}, io.Discard); err != nil {
		t.Fatal(err)
	}

	var statusOutput bytes.Buffer
	if err := RunReview([]string{"status", "--contract", ReviewIntegrationContractV1, "--next-transition", "--gate", string(reviewtransaction.GatePrePR), "--base-ref", "origin/release", "--cwd", repo, "--lineage", started.LineageID}, &statusOutput); err != nil {
		t.Fatal(err)
	}
	var status ReviewTargetStatusResult
	decodeStrictReviewJSON(t, statusOutput.Bytes(), &status)
	if status.NextTransition == nil || status.NextTransition.Execute == nil || status.NextTransition.Execute.Operation != "review.validate" {
		t.Fatalf("approved status transition = %#v", status.NextTransition)
	}

	arguments := status.NextTransition.Execute.Arguments
	validateArgs := []string{"validate", "--cwd", repo}
	foundBaseRef := false
	for _, argument := range arguments {
		if argument.Name == "base-ref" && argument.Value == "origin/release" {
			foundBaseRef = true
		}
		validateArgs = append(validateArgs, "--"+argument.Name, argument.Value)
	}
	if !foundBaseRef {
		t.Fatalf("validate transition arguments = %#v, want base-ref=origin/release", arguments)
	}
	if err := RunReview(validateArgs, io.Discard); err != nil {
		t.Fatalf("execute emitted validate transition: %v", err)
	}
}

func TestNegotiatedRecoverTransitionPreservesExactBaseDiffSelectors(t *testing.T) {
	repo := initReviewCLIRepo(t)
	runReviewCLIGit(t, repo, "branch", "review-base")
	if err := os.WriteFile(filepath.Join(repo, "tracked.txt"), []byte("candidate\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runReviewCLIGit(t, repo, "add", "tracked.txt")
	runReviewCLIGit(t, repo, "commit", "-qm", "candidate")

	var startedOutput bytes.Buffer
	if err := RunReviewFacadeStart([]string{"--cwd", repo, "--base-ref", "review-base", "--committed-only"}, &startedOutput); err != nil {
		t.Fatal(err)
	}
	var started ReviewFacadeStartResult
	if err := json.Unmarshal(startedOutput.Bytes(), &started); err != nil {
		t.Fatal(err)
	}
	predecessorStore, err := reviewtransaction.CompactAuthoritativeStore(context.Background(), repo, started.LineageID)
	if err != nil {
		t.Fatal(err)
	}
	predecessor, err := predecessorStore.Load()
	if err != nil {
		t.Fatal(err)
	}
	if err := RunReviewInvalidate([]string{"--cwd", repo, "--lineage", started.LineageID, "--expected-revision", predecessor.Revision, "--reason", "candidate invalidated"}, io.Discard); err != nil {
		t.Fatal(err)
	}
	predecessor, err = predecessorStore.Load()
	if err != nil {
		t.Fatal(err)
	}

	status := func(args ...string) ReviewTargetStatusResult {
		t.Helper()
		var output bytes.Buffer
		if err := RunReview(append([]string{"status", "--contract", ReviewIntegrationContractV1, "--base-ref", "review-base", "--cwd", repo, "--lineage", started.LineageID}, args...), &output); err != nil {
			t.Fatal(err)
		}
		var result ReviewTargetStatusResult
		decodeStrictReviewJSON(t, output.Bytes(), &result)
		return result
	}
	authorization := func(targetIdentity, successor string) []string {
		return []string{
			"--next-transition", "--recovery-successor-lineage", successor,
			"--recovery-actor", "maintainer", "--recovery-reason", "changed base-diff scope",
			"--recovery-authorization", reviewRecoveryAuthorization(started.LineageID, predecessor.Revision, targetIdentity, "maintainer", "changed base-diff scope"),
		}
	}

	probe := status()
	unchanged := status(authorization(probe.TargetIdentity, "review-unchanged")...)
	if unchanged.NextTransition == nil || unchanged.NextTransition.Kind != reviewNextTransitionStop || unchanged.NextTransition.Execute != nil || unchanged.NextTransition.Collect != nil {
		t.Fatalf("unchanged recovery transition = %#v", unchanged.NextTransition)
	}
	unchangedStore, err := reviewtransaction.CompactAuthoritativeStore(context.Background(), repo, "review-unchanged")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(unchangedStore.Dir); !os.IsNotExist(err) {
		t.Fatalf("unchanged successor store exists or cannot be inspected: %v", err)
	}

	if err := os.WriteFile(filepath.Join(repo, "tracked.txt"), []byte("changed candidate\nline two\nline three\nline four\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runReviewCLIGit(t, repo, "add", "tracked.txt")
	runReviewCLIGit(t, repo, "commit", "-qm", "changed candidate")

	var recoveryStartedOutput bytes.Buffer
	if err := RunReviewFacadeStart([]string{"--cwd", repo, "--base-ref", "review-base", "--committed-only"}, &recoveryStartedOutput); err != nil {
		t.Fatal(err)
	}
	var recoveryPredecessor ReviewFacadeStartResult
	if err := json.Unmarshal(recoveryStartedOutput.Bytes(), &recoveryPredecessor); err != nil {
		t.Fatal(err)
	}
	resultPath := filepath.Join(t.TempDir(), "blocking-result.json")
	writeReviewCLIJSON(t, resultPath, facadeReviewerResult{Lens: recoveryPredecessor.SelectedLenses[0], Findings: []facadeFinding{{Location: "tracked.txt:1", Severity: "CRITICAL", Claim: "candidate regression", ProofRefs: []string{"tracked.txt:1 changed hunk"}, EvidenceClass: reviewtransaction.EvidenceDeterministic, CausalDisposition: reviewtransaction.CausalIntroduced}}, Evidence: []string{"candidate reviewed"}})
	if err := RunReviewFacadeFinalize([]string{"--cwd", repo, "--lineage", recoveryPredecessor.LineageID, "--result", resultPath, "--correction-lines", "1"}, io.Discard); err != nil {
		t.Fatal(err)
	}
	recoveryStore, err := reviewtransaction.CompactAuthoritativeStore(context.Background(), repo, recoveryPredecessor.LineageID)
	if err != nil {
		t.Fatal(err)
	}
	recoveryRecord, err := recoveryStore.Load()
	if err != nil {
		t.Fatal(err)
	}
	if recoveryRecord.State.State != reviewtransaction.StateCorrectionRequired {
		t.Fatalf("recovery predecessor state = %q", recoveryRecord.State.State)
	}

	if err := os.WriteFile(filepath.Join(repo, "tracked.txt"), []byte("corrected candidate\nline two\nline three\nline four\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	validationPath := filepath.Join(t.TempDir(), "validation.json")
	writeReviewCLIJSON(t, validationPath, facadeValidationResult{
		OriginalCriteria:     facadeValidationCheck{Evidence: []string{"acceptance still fails"}},
		CorrectionRegression: facadeValidationCheck{Evidence: []string{"regression still fails"}},
		FollowUps:            []reviewtransaction.FollowUp{},
	})
	if err := RunReviewFacadeFinalize([]string{"--cwd", repo, "--lineage", recoveryPredecessor.LineageID, "--validation", validationPath}, io.Discard); err != nil {
		t.Fatal(err)
	}
	recoveryRecord, err = recoveryStore.Load()
	if err != nil {
		t.Fatal(err)
	}
	forecast := 1
	recoveryRecord.State.State, recoveryRecord.State.ProposedCorrectionLines, recoveryRecord.State.ActualCorrectionLines = reviewtransaction.StateCorrectionRequired, &forecast, nil
	recoveryRecord.State.FixDeltaHash, recoveryRecord.State.OriginalCriteria, recoveryRecord.State.CorrectionRegression = reviewtransaction.EmptyFixDeltaHash, nil, nil
	if err := recoveryRecord.State.Validate(); err != nil {
		t.Fatal(err)
	}
	recoveryRecord.Revision, err = reviewtransaction.CompactRevisionForState(recoveryRecord.State)
	if err != nil {
		t.Fatal(err)
	}
	recoveryRecord.Schema = "gentle-ai.review-state-record/v2"
	payload, err := json.MarshalIndent(recoveryRecord, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(recoveryStore.StatePath(), append(payload, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(recoveryStore.ReceiptPath()); err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(recoveryStore.Dir, "finalize-attempt-journal.json")); err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	runReviewCLIGit(t, repo, "add", "tracked.txt")
	runReviewCLIGit(t, repo, "commit", "-qm", "corrected candidate")

	if err := os.WriteFile(filepath.Join(repo, "tracked.txt"), []byte("recovered candidate\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runReviewCLIGit(t, repo, "add", "tracked.txt")
	runReviewCLIGit(t, repo, "commit", "-qm", "recovered candidate")
	committedTree := strings.TrimSpace(runReviewCLIGit(t, repo, "rev-parse", "HEAD^{tree}"))
	var probeOutput bytes.Buffer
	if err := RunReview([]string{"status", "--contract", ReviewIntegrationContractV1, "--base-ref", "review-base", "--cwd", repo, "--lineage", recoveryPredecessor.LineageID}, &probeOutput); err != nil {
		t.Fatal(err)
	}
	decodeStrictReviewJSON(t, probeOutput.Bytes(), &probe)
	var changedOutput bytes.Buffer
	if err := RunReview([]string{
		"status", "--contract", ReviewIntegrationContractV1, "--next-transition", "--base-ref", "review-base", "--cwd", repo, "--lineage", recoveryPredecessor.LineageID,
		"--recovery-successor-lineage", "review-changed", "--recovery-actor", "maintainer", "--recovery-reason", "changed base-diff scope",
		"--recovery-authorization", reviewRecoveryAuthorization(recoveryPredecessor.LineageID, recoveryRecord.Revision, probe.TargetIdentity, "maintainer", "changed base-diff scope"),
	}, &changedOutput); err != nil {
		t.Fatal(err)
	}
	var changed ReviewTargetStatusResult
	decodeStrictReviewJSON(t, changedOutput.Bytes(), &changed)
	if changed.NextTransition == nil || changed.NextTransition.Execute == nil || changed.NextTransition.Execute.Operation != "review.recover" {
		t.Fatalf("changed recovery transition = %#v", changed.NextTransition)
	}

	recoverArgs := []string{"recover", "--cwd", repo}
	found := map[string]string{}
	for _, argument := range changed.NextTransition.Execute.Arguments {
		found[argument.Name] = argument.Value
		recoverArgs = append(recoverArgs, "--"+argument.Name+"="+argument.Value)
	}
	if found["base-ref"] != "review-base" || found["committed-only"] != "true" || found["projection"] != string(reviewtransaction.ProjectionWorkspace) {
		t.Fatalf("recover transition arguments = %#v", changed.NextTransition.Execute.Arguments)
	}
	if err := RunReview(recoverArgs, io.Discard); err != nil {
		t.Fatalf("execute emitted recover transition: %v", err)
	}
	successorStore, err := reviewtransaction.CompactAuthoritativeStore(context.Background(), repo, "review-changed")
	if err != nil {
		t.Fatal(err)
	}
	successor, err := successorStore.Load()
	if err != nil {
		t.Fatal(err)
	}
	if successor.State.InitialSnapshot.Kind != reviewtransaction.TargetBaseDiff || successor.State.InitialSnapshot.CandidateTree != committedTree {
		t.Fatalf("recovered committed base-diff target = %#v, want candidate tree %s", successor.State.InitialSnapshot, committedTree)
	}
}

func TestTransitionConsumersRejectDuplicateSelectors(t *testing.T) {
	recoveryBindings := []string{
		"--predecessor-lineage=review-duplicate-predecessor",
		"--expected-predecessor-revision=sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		"--successor-lineage=review-duplicate-successor", "--disposition=invalidated",
		"--reason=duplicate selector probe", "--actor=maintainer",
	}
	tests := []struct {
		name string
		run  func([]string, io.Writer) error
		args []string
		want string
	}{
		{"validate base-ref", RunReviewFacadeValidate, []string{"--cwd=/missing", "--gate=pre-pr", "--base-ref=HEAD", "-base-ref=HEAD^"}, "review.validate repeats --base-ref"},
		{"recover base-ref", RunReviewRecover, append(recoveryBindings, "--cwd=/missing", "--base-ref=HEAD", "-base-ref=HEAD^", "--committed-only"), "review.recover repeats --base-ref"},
		{"recover committed-only", RunReviewRecover, append(recoveryBindings, "--cwd=/missing", "--base-ref=HEAD", "--committed-only=true", "-committed-only=false"), "review.recover repeats --committed-only"},
		{"recover projection", RunReviewRecover, append(recoveryBindings, "--cwd=/missing", "--projection=workspace", "-projection=staged"), "review.recover repeats --projection"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.run(tt.args, io.Discard)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want %q", err, tt.want)
			}
		})
	}
}
