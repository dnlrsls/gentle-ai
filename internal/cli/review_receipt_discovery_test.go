package cli

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/gentleman-programming/gentle-ai/internal/reviewtransaction"
)

func assertScopeChangeRecovery(t *testing.T, failure ReviewIntegrationFailure, lineage, path string) {
	if failure.Context == nil || failure.Context.ScopeChange == nil {
		t.Fatalf("scope-change context = %#v", failure)
	}
	scope := failure.Context.ScopeChange
	if scope.PredecessorLineageID != lineage || scope.PredecessorRevision == "" || !reflect.DeepEqual(scope.DifferingPaths, []string{path}) || !reflect.DeepEqual(failure.RequiredInputs, []string{"predecessor_lineage_id", "expected_predecessor_revision", "successor_lineage_id", "disposition", "reason", "actor"}) || !reflect.DeepEqual(scope.RecoveryRequiredInputs, failure.RequiredInputs) {
		t.Fatalf("scope-change provenance = %#v", failure)
	}
}

func TestUnqualifiedGateDiscoverySelectsOneExactReceiptAcrossUnrelatedHistory(t *testing.T) {
	repo := initReviewCLIRepo(t)
	first, _ := approveDiscoveryMarkdown(t, repo, "review-discovery-first", "docs/first.md", "first\n")
	runReviewCLIGit(t, repo, "add", "-A")
	runReviewCLIGit(t, repo, "commit", "-qm", "first reviewed target")
	second, store := approveDiscoveryMarkdown(t, repo, "review-discovery-second", "docs/second.md", "second\n")
	receipt, err := os.ReadFile(store.ReceiptPath())
	if err != nil {
		t.Fatal(err)
	}
	receipt = bytes.Replace(receipt, []byte(`"selected_lenses": []`), []byte(`"selected_lenses": null`), 1)
	if err := os.WriteFile(store.ReceiptPath(), receipt, 0o644); err != nil {
		t.Fatal(err)
	}

	var output bytes.Buffer
	if err := RunReview([]string{
		"validate", "--contract", ReviewIntegrationContractV1, "--cwd", repo,
		"--gate", string(reviewtransaction.GatePostApply),
	}, &output); err != nil {
		t.Fatalf("one exact receipt among unrelated history: %v\n%s", err, output.String())
	}
	var result ReviewValidateResult
	decodeStrictReviewJSON(t, decodeReviewOperationEnvelope(t, output.Bytes()).Result, &result)
	if !result.Allowed || result.Context.LineageID != second.LineageID || result.Context.LineageID == first.LineageID {
		t.Fatalf("exact receipt selection = %#v", result)
	}
}

func TestUnqualifiedGateDiscoveryReturnsTypedMissingAndScopeChanged(t *testing.T) {
	t.Run("missing", func(t *testing.T) {
		repo := initReviewCLIRepo(t)
		var output bytes.Buffer
		err := RunReview([]string{
			"validate", "--contract", ReviewIntegrationContractV1, "--cwd", repo,
			"--gate", string(reviewtransaction.GatePostApply),
		}, &output)
		if err == nil {
			t.Fatal("missing receipt validation succeeded")
		}
		failure := decodeReviewIntegrationFailure(t, output.Bytes())
		if failure.Code != "receipt_missing" || failure.MutationOutcome != ReviewMutationNotStarted || failure.RetrySafe || failure.NextAction != "stop" {
			t.Fatalf("missing receipt failure = %#v", failure)
		}
	})

	t.Run("scope changed", func(t *testing.T) {
		repo := initReviewCLIRepo(t)
		_, _ = approveDiscoveryMarkdown(t, repo, "review-discovery-scope", "docs/reviewed.md", "reviewed\n")
		if err := os.WriteFile(filepath.Join(repo, "docs", "reviewed.md"), []byte("drifted\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		var output bytes.Buffer
		err := RunReview([]string{
			"validate", "--contract", ReviewIntegrationContractV1, "--cwd", repo,
			"--gate", string(reviewtransaction.GatePostApply),
		}, &output)
		if err == nil {
			t.Fatal("scope-changed receipt validation succeeded")
		}
		failure := decodeReviewIntegrationFailure(t, output.Bytes())
		if failure.Code != "receipt_scope_changed" || failure.AuthorityApplicability != "current_target" || failure.RetrySafe ||
			failure.Replayability != reviewtransaction.ReplayabilityManualActionRequired || failure.NextAction != "explicit-maintainer-action" {
			t.Fatalf("scope-changed receipt failure = %#v", failure)
		}
		assertScopeChangeRecovery(t, failure, "review-discovery-scope", "docs/reviewed.md")
	})

	t.Run("unrelated", func(t *testing.T) {
		repo := initReviewCLIRepo(t)
		_, store := approveDiscoveryMarkdown(t, repo, "review-discovery-unrelated", "docs/reviewed.md", "reviewed\n")
		runReviewCLIGit(t, repo, "add", "-A")
		runReviewCLIGit(t, repo, "commit", "-qm", "reviewed target")
		if err := os.WriteFile(filepath.Join(repo, "unrelated.txt"), []byte("unrelated\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		runReviewCLIGit(t, repo, "add", "-A")
		runReviewCLIGit(t, repo, "commit", "-qm", "unrelated target")
		record, err := store.Load()
		if err != nil {
			t.Fatal(err)
		}
		assessment, err := reviewtransaction.AssessCompactGateTarget(context.Background(), repo, record.State, reviewtransaction.NativeGateRequestInput{
			Gate:      reviewtransaction.GateRelease,
			LineageID: record.State.LineageID,
		})
		if err != nil {
			t.Fatalf("assess unrelated release target: %v", err)
		}
		if assessment.Applicability != reviewtransaction.CompactGateTargetUnrelated {
			t.Fatalf("unrelated release applicability = %q; expected=%#v actual=%#v", assessment.Applicability, assessment.Expected, assessment.Actual)
		}
		var output bytes.Buffer
		err = RunReview([]string{
			"validate", "--contract", ReviewIntegrationContractV1, "--cwd", repo,
			"--gate", string(reviewtransaction.GateRelease),
		}, &output)
		if err == nil {
			t.Fatal("unrelated receipt validation succeeded")
		}
		failure := decodeReviewIntegrationFailure(t, output.Bytes())
		if failure.Code != "receipt_unrelated" || failure.AuthorityApplicability != "unrelated" || failure.RetrySafe || failure.NextAction != "stop" {
			t.Fatalf("unrelated receipt failure = %#v", failure)
		}
	})
}

func TestUnqualifiedGateDiscoveryRejectsMultipleExactReceiptsButExplicitLineageIsDirect(t *testing.T) {
	repo := initReviewCLIRepo(t)
	started, store := approveDiscoveryMarkdown(t, repo, "review-discovery-exact-a", "docs/exact.md", "exact\n")
	record, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	lines := record.State.OriginalChangedLines
	clone, err := reviewtransaction.NewCompactState(reviewtransaction.Start{
		LineageID: "review-discovery-exact-b", Mode: reviewtransaction.ModeOrdinaryBounded, Generation: record.State.Generation,
		Snapshot: record.State.InitialSnapshot, PolicyHash: record.State.PolicyHash, RiskLevel: record.State.RiskLevel,
		SelectedLenses: []string{}, OriginalChangedLines: &lines,
	})
	if err != nil {
		t.Fatal(err)
	}
	cloneStore, err := reviewtransaction.CompactAuthoritativeStore(context.Background(), repo, clone.LineageID)
	if err != nil {
		t.Fatal(err)
	}
	revision, err := cloneStore.Replace("", "review/start", clone)
	if err != nil {
		t.Fatal(err)
	}
	if err := clone.CompleteReview(reviewtransaction.CompactReviewInput{LensResults: []reviewtransaction.LensResult{}}); err != nil {
		t.Fatal(err)
	}
	revision, err = cloneStore.Replace(revision, "review/complete-review", clone)
	if err != nil {
		t.Fatal(err)
	}
	if err := clone.CompleteVerification([]byte("independent duplicate fixture evidence"), true); err != nil {
		t.Fatal(err)
	}
	if _, err := cloneStore.Replace(revision, "review/complete-verification", clone); err != nil {
		t.Fatal(err)
	}
	cloneReceipt, err := clone.Receipt()
	if err != nil {
		t.Fatal(err)
	}
	if err := reviewtransaction.WriteCompactReceiptAtomic(cloneStore.ReceiptPath(), cloneReceipt); err != nil {
		t.Fatal(err)
	}

	var output bytes.Buffer
	err = RunReview([]string{
		"validate", "--contract", ReviewIntegrationContractV1, "--cwd", repo,
		"--gate", string(reviewtransaction.GatePostApply),
	}, &output)
	if err == nil {
		t.Fatal("multiple exact receipts were selected implicitly")
	}
	failure := decodeReviewIntegrationFailure(t, output.Bytes())
	if failure.Code != "receipt_ambiguous" || failure.AuthorityApplicability != "ambiguous" || failure.RetrySafe ||
		len(failure.RequiredInputs) != 1 || failure.RequiredInputs[0] != "lineage_id" {
		t.Fatalf("ambiguous receipt failure = %#v", failure)
	}

	output.Reset()
	if err := RunReview([]string{
		"validate", "--contract", ReviewIntegrationContractV1, "--cwd", repo, "--lineage", started.LineageID,
		"--gate", string(reviewtransaction.GatePostApply),
	}, &output); err != nil {
		t.Fatalf("explicit exact lineage failed: %v\n%s", err, output.String())
	}
}

func TestUnqualifiedGateDiscoveryRequiresSelectionForMultipleScopeChangedReceipts(t *testing.T) {
	for _, tt := range []struct {
		name       string
		projection reviewtransaction.Projection
		gate       reviewtransaction.GateKind
	}{
		{name: "workspace", projection: reviewtransaction.ProjectionWorkspace, gate: reviewtransaction.GatePostApply},
		{name: "staged", projection: reviewtransaction.ProjectionStaged, gate: reviewtransaction.GatePreCommit},
	} {
		t.Run(tt.name, func(t *testing.T) {
			repo := initReviewCLIRepo(t)
			lineages := []string{"review-discovery-scope-a-" + tt.name, "review-discovery-scope-b-" + tt.name}
			logicalPath := "docs/scope-" + tt.name + ".md"
			_, first := approveDiscoveryMarkdownProjection(t, repo, lineages[0], logicalPath, "reviewed\n", tt.projection)
			cloneApprovedDiscoveryAuthority(t, repo, first, lineages[1])
			if err := os.WriteFile(filepath.Join(repo, filepath.FromSlash(logicalPath)), []byte("drifted\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			if tt.projection == reviewtransaction.ProjectionStaged {
				runReviewCLIGit(t, repo, "add", "-A")
			}

			gateInput := reviewtransaction.NativeGateRequestInput{Gate: tt.gate}
			_, _, discoveryErr := discoverCompactFacadeGateReview(context.Background(), repo, "", gateInput)
			var discovery *ReviewReceiptDiscoveryError
			if !errors.As(discoveryErr, &discovery) || discovery.Kind != ReviewReceiptAmbiguous || !reflect.DeepEqual(discovery.Candidates, lineages) || discovery.Context != nil {
				t.Fatalf("multiple scope-changed discovery = %#v, %v", discovery, discoveryErr)
			}

			var output bytes.Buffer
			err := RunReview([]string{
				"validate", "--contract", ReviewIntegrationContractV1, "--cwd", repo, "--gate", string(tt.gate),
			}, &output)
			if err == nil {
				t.Fatal("multiple scope-changed receipts were selected implicitly")
			}
			failure := decodeReviewIntegrationFailure(t, output.Bytes())
			if failure.Code != "receipt_ambiguous" || failure.AuthorityApplicability != "ambiguous" || failure.Context != nil || failure.RetrySafe ||
				failure.Replayability != reviewtransaction.ReplayabilityManualActionRequired || failure.NextAction != "review.status" ||
				!reflect.DeepEqual(failure.RequiredInputs, []string{"lineage_id"}) {
				t.Fatalf("multiple scope-changed failure = %#v", failure)
			}

			output.Reset()
			if err := RunReview([]string{
				"status", "--contract", ReviewIntegrationContractV1, "--cwd", repo, "--projection", string(tt.projection),
			}, &output); err != nil {
				t.Fatalf("target-scoped status: %v\n%s", err, output.String())
			}
			var status ReviewTargetStatusResult
			decodeStrictReviewJSON(t, output.Bytes(), &status)
			if status.Applicability != reviewtransaction.TargetApplicabilityAmbiguous || status.Action != reviewtransaction.TargetStatusActionSelectLineage ||
				status.Replayability != reviewtransaction.ReplayabilityStatusRequired || !reflect.DeepEqual(status.Candidates, lineages) {
				t.Fatalf("multiple scope-changed status = %#v", status)
			}

			for _, lineage := range lineages {
				output.Reset()
				err := RunReview([]string{
					"validate", "--contract", ReviewIntegrationContractV1, "--cwd", repo, "--lineage", lineage, "--gate", string(tt.gate),
				}, &output)
				if err == nil {
					t.Fatalf("explicit scope-changed lineage %s unexpectedly passed", lineage)
				}
				explicit := decodeReviewIntegrationFailure(t, output.Bytes())
				if explicit.Code != "gate_scope_changed" || explicit.NextAction != "explicit-maintainer-action" {
					t.Fatalf("explicit scope-changed lineage %s failure = %#v", lineage, explicit)
				}
				assertScopeChangeRecovery(t, explicit, lineage, logicalPath)
			}
		})
	}
}

func TestUnscopedGateDiscoveryFailsClosedOnMalformedUnrelatedInventory(t *testing.T) {
	repo := initReviewCLIRepo(t)
	started, _ := approveDiscoveryMarkdown(t, repo, "review-discovery-valid", "docs/valid.md", "valid\n")
	commonDir := filepath.Clean(string(bytes.TrimSpace([]byte(runReviewCLIGit(t, repo, "rev-parse", "--path-format=absolute", "--git-common-dir")))))
	broken := filepath.Join(commonDir, "gentle-ai", "review-transactions", "v1", "unrelated-broken")
	if err := os.MkdirAll(broken, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(broken, "HEAD"), []byte("not-a-revision\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var explicit bytes.Buffer
	if err := RunReview([]string{
		"validate", "--contract", ReviewIntegrationContractV1, "--cwd", repo, "--lineage", started.LineageID,
		"--gate", string(reviewtransaction.GatePostApply),
	}, &explicit); err != nil {
		t.Fatalf("explicit lineage was poisoned by unrelated inventory: %v\n%s", err, explicit.String())
	}

	var unscoped bytes.Buffer
	err := RunReview([]string{
		"validate", "--contract", ReviewIntegrationContractV1, "--cwd", repo,
		"--gate", string(reviewtransaction.GatePostApply),
	}, &unscoped)
	if err == nil {
		t.Fatal("unscoped discovery ignored malformed unrelated inventory")
	}
	failure := decodeReviewIntegrationFailure(t, unscoped.Bytes())
	if failure.Code != "authority_corrupted" || failure.AuthorityApplicability != "corrupted" || failure.RetrySafe || failure.NextAction != "stop" {
		t.Fatalf("corrupted inventory failure = %#v", failure)
	}
}

func approveDiscoveryMarkdown(t *testing.T, repo, lineage, logicalPath, content string) (ReviewFacadeStartResult, reviewtransaction.CompactStore) {
	return approveDiscoveryMarkdownProjection(t, repo, lineage, logicalPath, content, reviewtransaction.ProjectionWorkspace)
}

func approveDiscoveryMarkdownProjection(t *testing.T, repo, lineage, logicalPath, content string, projection reviewtransaction.Projection) (ReviewFacadeStartResult, reviewtransaction.CompactStore) {
	t.Helper()
	path := filepath.Join(repo, filepath.FromSlash(logicalPath))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	if projection == reviewtransaction.ProjectionStaged {
		runReviewCLIGit(t, repo, "add", "-A")
	}
	var output bytes.Buffer
	if err := RunReviewFacadeStart([]string{"--cwd", repo, "--lineage", lineage, "--projection", string(projection)}, &output); err != nil {
		t.Fatal(err)
	}
	var started ReviewFacadeStartResult
	decodeStrictReviewJSON(t, output.Bytes(), &started)
	if started.RiskLevel != reviewtransaction.RiskLow {
		t.Fatalf("discovery fixture risk = %q", started.RiskLevel)
	}
	if err := RunReviewFacadeFinalize([]string{"--cwd", repo, "--lineage", lineage}, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	store, err := reviewtransaction.CompactAuthoritativeStore(context.Background(), repo, lineage)
	if err != nil {
		t.Fatal(err)
	}
	return started, store
}

func cloneApprovedDiscoveryAuthority(t *testing.T, repo string, source reviewtransaction.CompactStore, lineage string) reviewtransaction.CompactStore {
	t.Helper()
	record, err := source.Load()
	if err != nil {
		t.Fatal(err)
	}
	lines := record.State.OriginalChangedLines
	clone, err := reviewtransaction.NewCompactState(reviewtransaction.Start{
		LineageID: lineage, Mode: reviewtransaction.ModeOrdinaryBounded, Generation: record.State.Generation,
		Snapshot: record.State.InitialSnapshot, PolicyHash: record.State.PolicyHash, RiskLevel: record.State.RiskLevel,
		SelectedLenses: append([]string{}, record.State.SelectedLenses...), OriginalChangedLines: &lines,
	})
	if err != nil {
		t.Fatal(err)
	}
	cloneStore, err := reviewtransaction.CompactAuthoritativeStore(context.Background(), repo, clone.LineageID)
	if err != nil {
		t.Fatal(err)
	}
	revision, err := cloneStore.Replace("", "review/start", clone)
	if err != nil {
		t.Fatal(err)
	}
	if err := clone.CompleteReview(reviewtransaction.CompactReviewInput{LensResults: []reviewtransaction.LensResult{}}); err != nil {
		t.Fatal(err)
	}
	revision, err = cloneStore.Replace(revision, "review/complete-review", clone)
	if err != nil {
		t.Fatal(err)
	}
	if err := clone.CompleteVerification([]byte("independent duplicate fixture evidence"), true); err != nil {
		t.Fatal(err)
	}
	if _, err := cloneStore.Replace(revision, "review/complete-verification", clone); err != nil {
		t.Fatal(err)
	}
	receipt, err := clone.Receipt()
	if err != nil {
		t.Fatal(err)
	}
	if err := reviewtransaction.WriteCompactReceiptAtomic(cloneStore.ReceiptPath(), receipt); err != nil {
		t.Fatal(err)
	}
	return cloneStore
}
