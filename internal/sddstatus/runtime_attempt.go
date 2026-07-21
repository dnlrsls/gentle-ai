package sddstatus

import (
	"crypto/sha256"
	"fmt"
	"regexp"
	"strings"
)

const runtimeAttemptSchema = "gentle-ai.runtime-attempt/v1"
const runtimeAttemptResetSchema = "gentle-ai.runtime-attempt-reset/v1"

type RuntimeAttemptState struct {
	Present          bool   `json:"present"`
	Valid            bool   `json:"valid"`
	DecisionRequired bool   `json:"decisionRequired"`
	Exhausted        bool   `json:"exhausted"`
	Interrupted      int    `json:"interrupted"`
	Reason           string `json:"reason"`
}

type attemptGeneration struct {
	budgetID string
	attempts int
	harness  string
	lastHash string
	passed   bool
}

var yamlEnvelopePattern = regexp.MustCompile("(?s)```yaml[ \\t]*\\n(.*?)\\n```")

func parseRuntimeAttempts(text, expectedChange string) RuntimeAttemptState {
	state := RuntimeAttemptState{Valid: true}
	current := map[string]*attemptGeneration{}
	text = strings.ReplaceAll(text, "\r\n", "\n")
	envelopes := yamlEnvelopePattern.FindAllStringSubmatch(text, -1)
	runtimeEnvelopeCount := 0
	for _, envelope := range envelopes {
		if strings.Contains(envelope[1], "gentle-ai.runtime-attempt") {
			runtimeEnvelopeCount++
		}
	}
	if strings.Count(text, "gentle-ai.runtime-attempt") != runtimeEnvelopeCount {
		return invalidRuntimeAttempts(RuntimeAttemptState{Present: true}, "malformed runtime attempt envelope")
	}
	for _, match := range envelopes {
		if !strings.Contains(match[1], "gentle-ai.runtime-attempt") {
			continue
		}
		state.Present = true
		fields, reason := parseScalarFields(strings.Split(match[1], "\n"), runtimeAttemptFields(), "runtime attempt")
		if reason != "" {
			return invalidRuntimeAttempts(state, reason)
		}
		if reason := requireRuntimeFields(fields); reason != "" {
			return invalidRuntimeAttempts(state, reason)
		}
		identity, generation, reason := runtimeIdentity(fields, expectedChange)
		if reason != "" {
			return invalidRuntimeAttempts(state, reason)
		}
		key := strings.Join(identity[:4], "\x00")
		switch fields["schema"] {
		case runtimeAttemptSchema:
			generationState := current[key]
			if generationState == nil {
				if generation != 0 {
					return invalidRuntimeAttempts(state, "scope generation requires an explicit reset")
				}
				generationState = &attemptGeneration{budgetID: runtimeBudgetID(identity)}
				current[key] = generationState
			}
			if fields["budget_id"] != generationState.budgetID || fields["budget_id"] != runtimeBudgetID(identity) {
				return invalidRuntimeAttempts(state, "inconsistent runtime attempt budget_id")
			}
			attempt, ok := parseNonnegativeInt(fields["attempt"])
			if !ok || attempt != generationState.attempts+1 || attempt < 1 || attempt > 2 || generationState.passed {
				return invalidRuntimeAttempts(state, "invalid runtime attempt counter")
			}
			if reason := validateAttempt(fields, attempt, generationState); reason != "" {
				return invalidRuntimeAttempts(state, reason)
			}
			generationState.attempts = attempt
			generationState.harness = fields["harness"]
			generationState.lastHash = fields["record_hash"]
			generationState.passed = fields["state"] == "passed"
			if fields["state"] == "running" {
				state.Interrupted++
			}
		case runtimeAttemptResetSchema:
			previous := current[key]
			from, fromOK := parseNonnegativeInt(fields["from_generation"])
			to, toOK := parseNonnegativeInt(fields["to_generation"])
			if previous == nil || !fromOK || !toOK || generation != to || to != from+1 || previous.attempts != 2 || previous.passed || fields["predecessor_budget_id"] != previous.budgetID || !isConcreteEvidence(fields["reason"]) {
				return invalidRuntimeAttempts(state, "invalid runtime attempt scope reset")
			}
			if fields["record_hash"] != runtimeHash(resetHashValues(fields)) {
				return invalidRuntimeAttempts(state, "invalid runtime attempt reset hash")
			}
			current[key] = &attemptGeneration{budgetID: runtimeBudgetID(identity)}
		default:
			return invalidRuntimeAttempts(state, "unsupported runtime attempt schema")
		}
	}
	for _, generation := range current {
		if generation.attempts == 2 && !generation.passed {
			state.Exhausted, state.DecisionRequired = true, true
			state.Reason = "runtime attempt budget exhausted; explicit decision or scope reset required"
		}
	}
	return state
}

func requireRuntimeFields(fields map[string]string) string {
	common := []string{"schema", "change", "work_unit_id", "evidence_goal", "candidate_id", "scope_generation", "record_hash"}
	var specific []string
	switch fields["schema"] {
	case runtimeAttemptSchema:
		specific = []string{"budget_id", "attempt", "state", "diagnosis", "harness", "predecessor_hash", "predecessor_invalidation"}
	case runtimeAttemptResetSchema:
		specific = []string{"from_generation", "to_generation", "predecessor_budget_id", "reason"}
	default:
		return "unsupported runtime attempt schema"
	}
	required := append(common, specific...)
	if len(fields) != len(required) {
		return "malformed runtime attempt field set"
	}
	for _, field := range required {
		if fields[field] == "" {
			return "missing " + field + " in runtime attempt envelope"
		}
	}
	return ""
}

func runtimeAttemptFields() map[string]bool {
	fields := map[string]bool{}
	for _, field := range []string{"schema", "change", "work_unit_id", "evidence_goal", "candidate_id", "scope_generation", "budget_id", "attempt", "state", "diagnosis", "harness", "predecessor_hash", "predecessor_invalidation", "record_hash", "from_generation", "to_generation", "predecessor_budget_id", "reason"} {
		fields[field] = true
	}
	return fields
}

func runtimeIdentity(fields map[string]string, expectedChange string) ([5]string, int, string) {
	identity := [5]string{fields["change"], fields["work_unit_id"], fields["evidence_goal"], fields["candidate_id"], fields["scope_generation"]}
	generation, ok := parseNonnegativeInt(identity[4])
	if identity[0] == "" || identity[0] != expectedChange || !isConcreteEvidence(identity[1]) || !isConcreteEvidence(identity[2]) || !sha256IdentityPattern.MatchString(identity[3]) || !ok {
		return identity, 0, "invalid runtime attempt identity"
	}
	return identity, generation, ""
}

func validateAttempt(fields map[string]string, attempt int, previous *attemptGeneration) string {
	for _, field := range []string{"state", "diagnosis", "harness", "predecessor_hash", "predecessor_invalidation", "record_hash"} {
		if fields[field] == "" {
			return "missing " + field + " in runtime attempt envelope"
		}
	}
	if fields["state"] != "running" && fields["state"] != "failed" && fields["state"] != "interrupted" && fields["state"] != "passed" {
		return "invalid runtime attempt state"
	}
	if !isConcreteEvidence(fields["harness"]) || fields["record_hash"] != runtimeHash(attemptHashValues(fields)) {
		return "invalid runtime attempt record hash"
	}
	if attempt == 1 {
		if fields["diagnosis"] != "none" || fields["predecessor_hash"] != "none" || fields["predecessor_invalidation"] != "none" {
			return "initial runtime attempt cannot claim correction provenance"
		}
		return ""
	}
	if !isConcreteEvidence(fields["diagnosis"]) || fields["predecessor_hash"] != previous.lastHash {
		return "corrective runtime attempt requires diagnosis and predecessor provenance"
	}
	if fields["harness"] != previous.harness && !isConcreteEvidence(fields["predecessor_invalidation"]) {
		return "corrective runtime attempt requires harness reuse or explicit predecessor invalidation"
	}
	return ""
}

func runtimeBudgetID(identity [5]string) string { return runtimeHash(identity[:]) }

func attemptHashValues(fields map[string]string) []string {
	return []string{fields["schema"], fields["change"], fields["work_unit_id"], fields["evidence_goal"], fields["candidate_id"], fields["scope_generation"], fields["budget_id"], fields["attempt"], fields["state"], fields["diagnosis"], fields["harness"], fields["predecessor_hash"], fields["predecessor_invalidation"]}
}

func resetHashValues(fields map[string]string) []string {
	return []string{fields["schema"], fields["change"], fields["work_unit_id"], fields["evidence_goal"], fields["candidate_id"], fields["scope_generation"], fields["from_generation"], fields["to_generation"], fields["predecessor_budget_id"], fields["reason"]}
}

func runtimeHash(values []string) string {
	sum := sha256.Sum256([]byte(strings.Join(values, "\x00")))
	return fmt.Sprintf("sha256:%x", sum)
}

func invalidRuntimeAttempts(state RuntimeAttemptState, reason string) RuntimeAttemptState {
	state.Valid, state.DecisionRequired, state.Reason = false, true, reason
	return state
}
