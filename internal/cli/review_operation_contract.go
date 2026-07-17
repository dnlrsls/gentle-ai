package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"github.com/gentleman-programming/gentle-ai/internal/reviewtransaction"
	"github.com/gentleman-programming/gentle-ai/internal/sddstatus"
)

const ReviewIntegrationOperationSchema = "gentle-ai.review-integration.operation/v1"
const ReviewIntegrationOperationSchemaID = "https://gentle-ai.dev/contracts/review-integration/v1/schemas/operation.schema.json"
const ReviewIntegrationFailureSchema = "gentle-ai.review-integration.failure/v1"
const ReviewIntegrationFailureSchemaID = "https://gentle-ai.dev/contracts/review-integration/v1/schemas/failure.schema.json"

const (
	ReviewIntegrationOperationFinalize = "review.finalize"
	ReviewIntegrationOperationValidate = "review.validate"
	ReviewIntegrationOperationBindSDD  = "review.bind_sdd"
)

type ReviewMutationOutcome string

const (
	ReviewMutationNotStarted ReviewMutationOutcome = "not_started"
	ReviewMutationUnknown    ReviewMutationOutcome = "unknown"
	ReviewMutationCommitted  ReviewMutationOutcome = "committed"
)

type ReviewIntegrationFailure struct {
	Schema                 string                          `json:"schema"`
	Contract               string                          `json:"contract"`
	Operation              string                          `json:"operation"`
	Phase                  string                          `json:"phase"`
	Code                   string                          `json:"code"`
	Message                string                          `json:"message"`
	MutationOutcome        ReviewMutationOutcome           `json:"mutation_outcome"`
	AuthorityApplicability string                          `json:"authority_applicability"`
	RetrySafe              bool                            `json:"retry_safe"`
	Replayability          reviewtransaction.Replayability `json:"replayability"`
	LineageID              string                          `json:"lineage_id,omitempty"`
	RequestDigest          string                          `json:"request_digest,omitempty"`
	RequiredInputs         []string                        `json:"required_inputs"`
	NextAction             string                          `json:"next_action"`
	Context                *reviewtransaction.GateContext  `json:"context,omitempty"`
	CauseCategory          string                          `json:"cause_category,omitempty"`
}

type ReviewIntegrationFailureError struct {
	Failure ReviewIntegrationFailure
	cause   error
}

func (err *ReviewIntegrationFailureError) Error() string {
	return fmt.Sprintf("%s [%s]", err.Failure.Message, err.Failure.Code)
}

func (err *ReviewIntegrationFailureError) Unwrap() error { return err.cause }

func newReviewIntegrationFailureError(failure ReviewIntegrationFailure, cause error) *ReviewIntegrationFailureError {
	return &ReviewIntegrationFailureError{Failure: failure, cause: cause}
}

type reviewIntegrationPreflightError struct{ cause error }

func (err *reviewIntegrationPreflightError) Error() string { return err.cause.Error() }
func (err *reviewIntegrationPreflightError) Unwrap() error { return err.cause }

func reviewPreflightError(err error) error {
	if err == nil {
		return nil
	}
	return &reviewIntegrationPreflightError{cause: err}
}

func reviewIntegrationFailureRoute(args []string) (string, bool, *ReviewIntegrationFailure) {
	if len(args) == 0 {
		return "", false, nil
	}
	operation := map[string]string{
		"capabilities": "review.capabilities",
		"start":        "review.start",
		"status":       "review.status",
		"finalize":     "review.finalize",
		"validate":     "review.validate",
		"bind-sdd":     "review.bind_sdd",
	}[args[0]]
	if operation == "" {
		return "", false, nil
	}
	provided, contract, missing := reviewIntegrationContractArgument(args[1:])
	if args[0] != "capabilities" && !provided {
		return operation, false, nil
	}
	if !provided {
		contract = ReviewIntegrationContractV1
	}
	if missing {
		failure := newReviewIntegrationPreflightFailure(operation, "invalid_request", "The negotiated review request is invalid.")
		failure.LineageID = safeReviewIntegrationLineage(operation, args[1:])
		return operation, true, &failure
	}
	if contract == "" {
		failure := newReviewIntegrationPreflightFailure(operation, "empty_contract", "The review integration contract cannot be empty.")
		failure.LineageID = safeReviewIntegrationLineage(operation, args[1:])
		return operation, true, &failure
	}
	if contract != ReviewIntegrationContractV1 {
		failure := newReviewIntegrationPreflightFailure(operation, "unsupported_contract", "The requested review integration contract is not supported.")
		failure.LineageID = safeReviewIntegrationLineage(operation, args[1:])
		return operation, true, &failure
	}
	return operation, true, nil
}

func reviewIntegrationContractArgument(args []string) (provided bool, value string, missing bool) {
	for index := 0; index < len(args); index++ {
		arg := args[index]
		if strings.HasPrefix(arg, "--contract=") {
			provided, value, missing = true, strings.TrimPrefix(arg, "--contract="), false
			continue
		}
		if arg != "--contract" {
			continue
		}
		provided = true
		if index+1 >= len(args) {
			return true, "", true
		}
		value, missing = args[index+1], false
		index++
	}
	return provided, value, missing
}

func newReviewIntegrationPreflightFailure(operation, code, message string) ReviewIntegrationFailure {
	return ReviewIntegrationFailure{
		Schema: ReviewIntegrationFailureSchema, Contract: ReviewIntegrationContractV1, Operation: operation,
		Phase: "preflight", Code: code, Message: message, MutationOutcome: ReviewMutationNotStarted,
		AuthorityApplicability: "not_evaluated", RetrySafe: true,
		Replayability: reviewtransaction.ReplayabilityNotReplayable, RequiredInputs: []string{}, NextAction: "correct_request",
	}
}

func newReviewIntegrationFailure(operation string, args []string, runErr error) ReviewIntegrationFailure {
	failure := ReviewIntegrationFailure{
		Schema: ReviewIntegrationFailureSchema, Contract: ReviewIntegrationContractV1, Operation: operation,
		Phase: "native_running", Code: "operation_outcome_unknown",
		Message:         "The negotiated review operation failed without authoritative mutation evidence.",
		MutationOutcome: ReviewMutationUnknown, AuthorityApplicability: "not_evaluated", RetrySafe: false,
		Replayability: reviewtransaction.ReplayabilityStatusRequired, RequiredInputs: []string{}, NextAction: "review.status",
	}
	failure.LineageID = safeReviewIntegrationLineage(operation, args)
	var publication *ReviewFacadeReceiptPublicationError
	if errors.As(runErr, &publication) {
		failure.Phase = "native_committed"
		failure.Code = "receipt_publication_pending"
		failure.Message = "Receipt publication did not complete after terminal authority was committed."
		failure.MutationOutcome = ReviewMutationCommitted
		failure.AuthorityApplicability = "current_target"
		failure.Replayability = reviewtransaction.ReplayabilityExactReplaySafe
		failure.LineageID = publication.LineageID
		failure.RequestDigest = publication.RequestDigest
		failure.RequiredInputs = []string{"lineage_id"}
		failure.NextAction = "review.finalize"
		return failure
	}
	var progress *reviewFacadeOperationProgressError
	if errors.As(runErr, &progress) {
		failure.Phase = "native_committed"
		failure.MutationOutcome = ReviewMutationUnknown
		failure.AuthorityApplicability = "current_target"
		failure.RetrySafe = false
		failure.Replayability = reviewtransaction.ReplayabilityStatusRequired
		failure.LineageID = progress.LineageID
		failure.RequiredInputs = []string{}
		failure.NextAction = "review.status"
		var progressedGitTimeout *reviewtransaction.GitCommandTimeoutError
		var progressedGitFailure *reviewtransaction.GitCommandError
		switch {
		case errors.As(runErr, &progressedGitTimeout):
			failure.Code = "git_command_timeout"
			failure.Message = "A bounded Git subprocess timed out after review authority committed a native transition."
		case errors.As(runErr, &progressedGitFailure):
			failure.Code = "git_command_failed"
			failure.Message = "A Git subprocess failed after review authority committed a native transition."
		case errors.Is(runErr, context.DeadlineExceeded):
			failure.Code = "operation_timeout"
			failure.Message = "The negotiated review operation timed out after review authority committed a native transition."
		}
		return failure
	}
	var gitTimeout *reviewtransaction.GitCommandTimeoutError
	if errors.As(runErr, &gitTimeout) {
		if gitTimeout.Aggregate {
			return reviewOperationTimeoutFailure(failure, operation)
		}
		failure.Phase = "pre_native"
		failure.Code = "git_command_timeout"
		failure.Message = "A bounded Git subprocess timed out before review authority mutation."
		failure.MutationOutcome = ReviewMutationNotStarted
		failure.AuthorityApplicability = "not_evaluated"
		failure.RetrySafe = false
		failure.Replayability = reviewtransaction.ReplayabilityManualActionRequired
		failure.NextAction = "stop"
		return failure
	}
	var gitFailure *reviewtransaction.GitCommandError
	if errors.As(runErr, &gitFailure) {
		failure.Phase = "pre_native"
		failure.Code = "git_command_failed"
		failure.Message = "A Git subprocess failed before review authority mutation."
		failure.MutationOutcome = ReviewMutationNotStarted
		failure.AuthorityApplicability = "not_evaluated"
		failure.RetrySafe = false
		failure.Replayability = reviewtransaction.ReplayabilityManualActionRequired
		failure.NextAction = "stop"
		return failure
	}
	if errors.Is(runErr, context.DeadlineExceeded) {
		return reviewOperationTimeoutFailure(failure, operation)
	}
	var preflight *reviewIntegrationPreflightError
	if errors.As(runErr, &preflight) {
		preflightFailure := newReviewIntegrationPreflightFailure(operation, "invalid_request", "The negotiated review request is invalid.")
		preflightFailure.LineageID = failure.LineageID
		return preflightFailure
	}
	var legacy *reviewtransaction.LegacyReadOnlyError
	if errors.As(runErr, &legacy) {
		failure.Code = reviewtransaction.LegacyReadOnlyErrorCode
		failure.Message = "Legacy v1 review authority is read-only and cannot be mutated."
		failure.MutationOutcome = ReviewMutationNotStarted
		failure.AuthorityApplicability = "current_target"
		failure.Replayability = reviewtransaction.ReplayabilityNotReplayable
		failure.NextAction = "stop"
		return failure
	}
	var lockTimeout *reviewtransaction.AuthorityLockTimeoutError
	var lockCancelled *reviewtransaction.AuthorityLockCancelledError
	if errors.As(runErr, &lockTimeout) || errors.As(runErr, &lockCancelled) {
		failure.Phase = "pre_native"
		failure.Code = "authority_lock_timeout"
		failure.Message = "Review START could not acquire the authority lock within the bounded wait."
		if lockCancelled != nil {
			failure.Code = "authority_lock_cancelled"
			failure.Message = "Review START authority lock acquisition was cancelled."
		}
		failure.MutationOutcome = ReviewMutationNotStarted
		failure.AuthorityApplicability = "not_evaluated"
		failure.RetrySafe = false
		failure.Replayability = reviewtransaction.ReplayabilityManualActionRequired
		failure.NextAction = "stop"
		return failure
	}
	var denied ReviewGateDeniedError
	if errors.As(runErr, &denied) {
		failure.Phase = "preflight"
		failure.Code = "gate_" + strings.ReplaceAll(string(denied.Result), "-", "_")
		failure.Message = "The review delivery gate denied the current target."
		failure.MutationOutcome = ReviewMutationNotStarted
		failure.AuthorityApplicability = "current_target"
		failure.RetrySafe = true
		failure.Replayability = reviewtransaction.ReplayabilityManualActionRequired
		failure.NextAction = reviewGateAction(denied.Result)
		if denied.Context.Gate != "" && denied.Context.ScopeChange != nil {
			contextCopy := denied.Context
			failure.Context = &contextCopy
		}
		if denied.Result == reviewtransaction.GateScopeChanged && denied.Context.ScopeChange != nil {
			failure.RetrySafe = false
			failure.RequiredInputs = append([]string{}, denied.Context.ScopeChange.RecoveryRequiredInputs...)
		}
		if lineage := safeReviewIntegrationLineage(operation, args); lineage != "" {
			failure.LineageID = lineage
		}
		return failure
	}
	var discovery *ReviewReceiptDiscoveryError
	if errors.As(runErr, &discovery) {
		failure.Phase = "pre_native"
		failure.Code = string(discovery.Kind)
		failure.Message = "No unique exact review receipt applies to the live gate target."
		failure.MutationOutcome = ReviewMutationNotStarted
		failure.AuthorityApplicability = "unrelated"
		failure.RetrySafe = false
		failure.Replayability = reviewtransaction.ReplayabilityManualActionRequired
		failure.NextAction = "stop"
		switch discovery.Kind {
		case ReviewReceiptMissing:
			failure.AuthorityApplicability = "not_evaluated"
		case ReviewReceiptScopeChanged:
			if discovery.Context != nil {
				contextCopy := *discovery.Context
				failure.Context = &contextCopy
				failure.AuthorityApplicability = "current_target"
				if contextCopy.ScopeChange != nil {
					failure.RequiredInputs = append([]string{}, contextCopy.ScopeChange.RecoveryRequiredInputs...)
				}
			}
			failure.NextAction = "explicit-maintainer-action"
		case ReviewReceiptAmbiguous:
			failure.AuthorityApplicability = "ambiguous"
			failure.RequiredInputs = []string{"lineage_id"}
			failure.NextAction = "review.status"
		case ReviewAuthorityCorrupted:
			failure.AuthorityApplicability = "corrupted"
			failure.CauseCategory = discovery.Category
		}
		return failure
	}
	if operation == "review.capabilities" || operation == "review.status" || operation == "review.validate" {
		failure.Phase = "pre_native"
		failure.Code = "operation_failed"
		failure.Message = "The negotiated read-only review operation failed safely."
		failure.MutationOutcome = ReviewMutationNotStarted
		failure.RetrySafe = true
		failure.Replayability = reviewtransaction.ReplayabilityNotReplayable
		failure.NextAction = "retry"
	}
	return failure
}

func reviewOperationTimeoutFailure(failure ReviewIntegrationFailure, operation string) ReviewIntegrationFailure {
	failure.Code = "operation_timeout"
	failure.Message = "The negotiated review operation exceeded its aggregate time budget."
	if operation == ReviewIntegrationOperationBindSDD {
		failure.Phase = "pre_native"
		failure.MutationOutcome = ReviewMutationNotStarted
		failure.AuthorityApplicability = "not_evaluated"
		failure.RetrySafe = true
		failure.Replayability = reviewtransaction.ReplayabilityNotReplayable
		failure.NextAction = "retry"
		return failure
	}
	failure.RetrySafe = false
	if operation == "review.start" || operation == ReviewIntegrationOperationFinalize {
		failure.Phase = "native_running"
		failure.MutationOutcome = ReviewMutationUnknown
		failure.AuthorityApplicability = "not_evaluated"
		failure.Replayability = reviewtransaction.ReplayabilityStatusRequired
		failure.NextAction = "review.status"
		return failure
	}
	failure.Phase = "pre_native"
	failure.MutationOutcome = ReviewMutationNotStarted
	failure.AuthorityApplicability = "not_evaluated"
	failure.Replayability = reviewtransaction.ReplayabilityManualActionRequired
	failure.NextAction = "stop"
	return failure
}

type reviewIntegrationFlagKind uint8

const (
	reviewIntegrationValueFlag reviewIntegrationFlagKind = iota
	reviewIntegrationBoolFlag
	reviewIntegrationIntFlag
)

func safeReviewIntegrationLineage(operation string, args []string) string {
	shape := reviewIntegrationOperationFlagShape(operation)
	value := ""
	for index := 0; index < len(args); index++ {
		arg := args[index]
		if arg == "--" {
			break
		}
		if arg == "" || arg == "-" || arg[0] != '-' {
			break
		}
		nameValue := strings.TrimPrefix(arg, "-")
		nameValue = strings.TrimPrefix(nameValue, "-")
		name, flagValue, hasValue := nameValue, "", false
		if separator := strings.IndexByte(nameValue, '='); separator >= 0 {
			name, flagValue, hasValue = nameValue[:separator], nameValue[separator+1:], true
		}
		kind, known := shape[name]
		if !known {
			break
		}
		switch kind {
		case reviewIntegrationBoolFlag:
			if hasValue {
				if _, err := strconv.ParseBool(flagValue); err != nil {
					index = len(args)
				}
			}
			continue
		case reviewIntegrationValueFlag, reviewIntegrationIntFlag:
			if !hasValue {
				if index+1 >= len(args) {
					index = len(args)
					continue
				}
				index++
				flagValue = args[index]
			}
			if kind == reviewIntegrationIntFlag {
				if _, err := strconv.Atoi(flagValue); err != nil {
					index = len(args)
				}
				continue
			}
		}
		if name == "lineage" {
			value = flagValue
		}
	}
	if !validReviewIntegrationLineage(value) {
		return ""
	}
	return value
}

func reviewIntegrationOperationFlagShape(operation string) map[string]reviewIntegrationFlagKind {
	valueFlags := []string{"contract"}
	boolFlags := []string{}
	intFlags := []string{}
	switch operation {
	case "review.capabilities":
	case "review.start":
		valueFlags = append(valueFlags, "cwd", "lineage", "policy", "focus", "base-ref", "projection", "trace")
		boolFlags = append(boolFlags, "committed-only", "workspace-overlay")
	case "review.status":
		valueFlags = append(valueFlags, "cwd", "lineage", "projection", "base-ref")
	case ReviewIntegrationOperationFinalize:
		valueFlags = append(valueFlags, "cwd", "lineage", "validation", "refuter", "evidence", "trace", "result")
		boolFlags = append(boolFlags, "failed")
		intFlags = append(intFlags, "correction-lines")
	case ReviewIntegrationOperationValidate:
		valueFlags = append(valueFlags, "cwd", "lineage", "gate", "base-ref", "pre-pr-ci-attestation", "policy",
			"release-configuration", "release-generated", "release-provenance", "release-publication-boundary", "release-evidence-freshness")
	case ReviewIntegrationOperationBindSDD:
		valueFlags = append(valueFlags, "cwd", "change", "lineage", "expected-binding-revision")
	}
	shape := make(map[string]reviewIntegrationFlagKind, len(valueFlags)+len(boolFlags)+len(intFlags)+2)
	for _, name := range valueFlags {
		shape[name] = reviewIntegrationValueFlag
	}
	for _, name := range boolFlags {
		shape[name] = reviewIntegrationBoolFlag
	}
	for _, name := range intFlags {
		shape[name] = reviewIntegrationIntFlag
	}
	shape["h"] = reviewIntegrationBoolFlag
	shape["help"] = reviewIntegrationBoolFlag
	return shape
}

func reviewNamedArgument(args []string, name string) (provided bool, value string, missing bool) {
	prefix := "--" + name + "="
	flagName := "--" + name
	for index := 0; index < len(args); index++ {
		if strings.HasPrefix(args[index], prefix) {
			provided, value, missing = true, strings.TrimPrefix(args[index], prefix), false
			continue
		}
		if args[index] != flagName {
			continue
		}
		provided = true
		if index+1 >= len(args) {
			return true, "", true
		}
		value, missing = args[index+1], false
		index++
	}
	return provided, value, missing
}

func validReviewIntegrationLineage(value string) bool {
	if value == "" || len(value) > 128 || value[0] == '-' || value[len(value)-1] == '-' {
		return false
	}
	previousHyphen := false
	for _, char := range value {
		if char != '-' && (char < 'a' || char > 'z') && (char < '0' || char > '9') {
			return false
		}
		if char == '-' && previousHyphen {
			return false
		}
		previousHyphen = char == '-'
	}
	return true
}

func (failure ReviewIntegrationFailure) Validate() error {
	if failure.Schema != ReviewIntegrationFailureSchema || failure.Contract != ReviewIntegrationContractV1 ||
		!validReviewIntegrationFailureOperation(failure.Operation) {
		return errors.New("invalid negotiated review failure identity")
	}
	if !validReviewIntegrationFailureCode(failure.Code) || strings.TrimSpace(failure.Message) != failure.Message ||
		failure.Message == "" || len(failure.Message) > 240 || strings.ContainsAny(failure.Message, "\r\n") {
		return errors.New("invalid negotiated review failure message")
	}
	switch failure.Phase {
	case "preflight", "pre_native", "native_running", "native_committed", "reconciliation":
	default:
		return errors.New("invalid negotiated review failure phase")
	}
	switch failure.MutationOutcome {
	case ReviewMutationNotStarted, ReviewMutationUnknown, ReviewMutationCommitted:
	default:
		return errors.New("invalid negotiated review mutation outcome")
	}
	switch failure.AuthorityApplicability {
	case "current_target", "unrelated", "ambiguous", "corrupted", "not_evaluated":
	default:
		return errors.New("invalid negotiated review authority applicability")
	}
	switch failure.Replayability {
	case reviewtransaction.ReplayabilityNotReplayable, reviewtransaction.ReplayabilityExactReplaySafe,
		reviewtransaction.ReplayabilityStatusRequired, reviewtransaction.ReplayabilityManualActionRequired:
	default:
		return errors.New("invalid negotiated review failure replayability")
	}
	if failure.RequiredInputs == nil || strings.TrimSpace(failure.NextAction) == "" {
		return errors.New("negotiated review failure action is incomplete")
	}
	for _, input := range failure.RequiredInputs {
		if !supportedReviewIntegrationFailureInput(input) {
			return errors.New("unsupported negotiated review failure input")
		}
	}
	if failure.Context != nil {
		if failure.Operation != ReviewIntegrationOperationValidate || failure.Context.Gate == "" {
			return errors.New("negotiated review failure context is not a gate denial")
		}
		if failure.Context.ScopeChange != nil {
			scope := failure.Context.ScopeChange
			if failure.Code != "gate_scope_changed" && failure.Code != "receipt_scope_changed" || failure.Context.Denial == nil || scope.DifferingPaths == nil ||
				scope.Expected.Paths == nil || scope.Actual.Paths == nil || scope.DifferingPathCount != len(scope.DifferingPaths) ||
				!sort.StringsAreSorted(scope.DifferingPaths) || !validReviewCapabilitySHA256(scope.DifferingPathsDigest) ||
				!validReviewIntegrationLineage(scope.PredecessorLineageID) || !validReviewCapabilitySHA256(scope.PredecessorRevision) ||
				scope.RecoveryOperation != "review.recover" || !reflect.DeepEqual(failure.RequiredInputs, scope.RecoveryRequiredInputs) {
				return errors.New("negotiated review scope-change diagnostics are incomplete")
			}
		}
	}
	if failure.LineageID != "" && !validReviewIntegrationLineage(failure.LineageID) ||
		failure.RequestDigest != "" && !validReviewCapabilitySHA256(failure.RequestDigest) ||
		failure.RequestDigest != "" && failure.LineageID == "" {
		return errors.New("invalid negotiated review failure replay identity")
	}
	if failure.MutationOutcome == ReviewMutationUnknown && (failure.RetrySafe || failure.Replayability != reviewtransaction.ReplayabilityStatusRequired || failure.NextAction != "review.status") {
		return errors.New("unknown negotiated review mutation must require status")
	}
	if failure.Replayability == reviewtransaction.ReplayabilityExactReplaySafe &&
		(failure.MutationOutcome != ReviewMutationCommitted || failure.LineageID == "" || failure.RequestDigest == "" ||
			len(failure.RequiredInputs) != 1 || failure.RequiredInputs[0] != "lineage_id" || failure.NextAction != "review.finalize") {
		return errors.New("exact negotiated review replay is incomplete")
	}
	return nil
}

func supportedReviewIntegrationFailureInput(input string) bool {
	switch input {
	case "lineage_id", "predecessor_lineage_id", "expected_predecessor_revision", "successor_lineage_id", "disposition", "reason", "actor":
		return true
	default:
		return false
	}
}

func validReviewIntegrationFailureOperation(operation string) bool {
	switch operation {
	case "review.capabilities", "review.start", "review.status", "review.finalize", "review.validate", "review.bind_sdd":
		return true
	default:
		return false
	}
}

func validReviewIntegrationFailureCode(code string) bool {
	if code == "" {
		return false
	}
	for _, char := range code {
		if char != '_' && (char < 'a' || char > 'z') && (char < '0' || char > '9') {
			return false
		}
	}
	return true
}

func emitReviewIntegrationFailure(stdout io.Writer, failure ReviewIntegrationFailure) error {
	if err := failure.Validate(); err != nil {
		return fmt.Errorf("validate negotiated review failure: %w", err)
	}
	return encodeReviewJSON(stdout, failure)
}

type ReviewIntegrationOperationResult struct {
	Schema    string          `json:"schema"`
	Contract  string          `json:"contract"`
	Operation string          `json:"operation"`
	Result    json.RawMessage `json:"result"`
}

// ReviewIntegrationFinalizeResult preserves the existing finalize semantics
// while excluding the provider-private receipt path from negotiated output.
type ReviewIntegrationFinalizeResult struct {
	Operation     string                  `json:"operation"`
	LineageID     string                  `json:"lineage_id"`
	State         reviewtransaction.State `json:"state"`
	Action        string                  `json:"action"`
	StoreRevision string                  `json:"store_revision"`
}

func reviewIntegrationNegotiation(flags *flag.FlagSet, contract string) (bool, error) {
	if !reviewFlagWasProvided(flags, "contract") {
		return false, nil
	}
	if err := validateReviewIntegrationContract(contract); err != nil {
		return false, err
	}
	return true, nil
}

func reviewFlagWasProvided(flags *flag.FlagSet, name string) bool {
	provided := false
	flags.Visit(func(value *flag.Flag) {
		provided = provided || value.Name == name
	})
	return provided
}

func encodeReviewIntegrationOperation(stdout io.Writer, negotiated bool, operation string, legacyResult, publicResult any) error {
	if !negotiated {
		return encodeReviewJSON(stdout, legacyResult)
	}
	payload, err := json.Marshal(publicResult)
	if err != nil {
		return fmt.Errorf("encode negotiated %s result: %w", operation, err)
	}
	envelope := ReviewIntegrationOperationResult{
		Schema: ReviewIntegrationOperationSchema, Contract: ReviewIntegrationContractV1,
		Operation: operation, Result: payload,
	}
	if err := envelope.Validate(); err != nil {
		return fmt.Errorf("validate negotiated %s result: %w", operation, err)
	}
	return encodeReviewJSON(stdout, envelope)
}

func (result ReviewIntegrationOperationResult) Validate() error {
	if result.Schema != ReviewIntegrationOperationSchema || result.Contract != ReviewIntegrationContractV1 || len(result.Result) == 0 {
		return errors.New("invalid negotiated review operation identity")
	}
	var document any
	if err := json.Unmarshal(result.Result, &document); err != nil {
		return fmt.Errorf("parse negotiated review operation result: %w", err)
	}
	if _, object := document.(map[string]any); !object {
		return errors.New("negotiated review operation result must be an object")
	}
	if field := forbiddenReviewIntegrationResultField(document); field != "" {
		return fmt.Errorf("negotiated review operation result contains private field %q", field)
	}
	switch result.Operation {
	case ReviewIntegrationOperationFinalize:
		var finalized ReviewIntegrationFinalizeResult
		if err := decodeStrictReviewIntegrationResult(result.Result, &finalized); err != nil {
			return err
		}
		if finalized.Operation != "review/finalize" || strings.TrimSpace(finalized.LineageID) == "" ||
			strings.TrimSpace(finalized.Action) == "" || !validReviewCapabilitySHA256(finalized.StoreRevision) || strings.TrimSpace(string(finalized.State)) == "" {
			return errors.New("negotiated finalize result is incomplete")
		}
	case ReviewIntegrationOperationValidate:
		var validated ReviewValidateResult
		if err := decodeStrictReviewIntegrationResult(result.Result, &validated); err != nil {
			return err
		}
		if validated.Schema != ReviewValidateSchema || validated.Allowed != (validated.Result == reviewtransaction.GateAllow) ||
			strings.TrimSpace(validated.Action) == "" || strings.TrimSpace(validated.Reason) == "" ||
			(validated.Context.Gate != "" && !validReviewIntegrationGate(validated.Context.Gate)) ||
			(validated.Allowed && !validReviewIntegrationGate(validated.Context.Gate)) {
			return errors.New("negotiated validate result is inconsistent")
		}
	case ReviewIntegrationOperationBindSDD:
		var binding sddstatus.ReviewBinding
		if err := decodeStrictReviewIntegrationResult(result.Result, &binding); err != nil {
			return err
		}
		if binding.Schema != "gentle-ai.sdd-review-binding/v1" || strings.TrimSpace(binding.Change) == "" || strings.TrimSpace(binding.Lineage) == "" ||
			!validReviewCapabilitySHA256(binding.Revision) || !validReviewCapabilitySHA256(binding.AuthorityRevision) ||
			!validReviewCapabilitySHA256(binding.ReceiptHash) || binding.GateContext.Gate != reviewtransaction.GatePostApply {
			return errors.New("negotiated bind-sdd result is incomplete")
		}
	default:
		return fmt.Errorf("unsupported negotiated review operation %q", result.Operation)
	}
	return nil
}

func decodeStrictReviewIntegrationResult(payload []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("decode negotiated review operation result: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return errors.New("negotiated review operation result contains multiple JSON values")
	}
	return nil
}

func forbiddenReviewIntegrationResultField(value any) string {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			lower := strings.ToLower(key)
			if lower == "model" || lower == "provider" || lower == "profile" || lower == "cwd" || lower == "repository" ||
				lower == "path" || strings.HasSuffix(lower, "_path") {
				return key
			}
			if found := forbiddenReviewIntegrationResultField(child); found != "" {
				return found
			}
		}
	case []any:
		for _, child := range typed {
			if found := forbiddenReviewIntegrationResultField(child); found != "" {
				return found
			}
		}
	}
	return ""
}

func validReviewIntegrationGate(gate reviewtransaction.GateKind) bool {
	switch gate {
	case reviewtransaction.GatePostApply, reviewtransaction.GatePreCommit, reviewtransaction.GatePrePush,
		reviewtransaction.GatePrePR, reviewtransaction.GateRelease:
		return true
	default:
		return false
	}
}
