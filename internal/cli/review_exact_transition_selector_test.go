package cli

import (
	"bytes"
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
