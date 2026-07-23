# Generic Issue Creation Skill

Issue #1374 will make the consumer-distributed `issue-creation` skill discover each repository's actual GitHub workflow instead of presenting Gentle AI conventions as universal rules. The repository-specific Gentle AI contributor skill remains unchanged.

## Outcome

An installed agent can create or triage issues in an arbitrary GitHub repository without assuming that the repository has Gentle AI template names, labels, approval gates, or discussion URLs.

## Scope

| Area | Decision |
|------|----------|
| Consumer asset | Rewrite `internal/assets/skills/issue-creation/SKILL.md`. |
| Gentle AI workflow | Keep `skills/issue-creation/SKILL.md` unchanged because it intentionally documents this repository. |
| Runtime | Do not change Go installation or injection behavior. The existing embedded asset mechanism already distributes the file. |
| Compatibility | Preserve the skill name and frontmatter so existing installations update in place. |

## Skill Flow

1. Confirm the current GitHub repository with `gh repo view`.
2. Search open and closed issues before creating a new one.
3. Inspect `.github/ISSUE_TEMPLATE/`, its `config.yml`, and repository contribution guidance.
4. Inspect available labels instead of assuming status, type, or priority names.
5. Use an existing matching template when one is available.
6. Otherwise create a structured title and body without a template.
7. Apply labels or approval gates only when repository evidence establishes them.
8. Route questions to Discussions only when the current repository enables or documents Discussions.

## Failure Handling

The skill must stop and ask for context rather than guess when:

- `gh` is unavailable or unauthenticated;
- the current directory cannot be resolved to a GitHub repository;
- repository metadata cannot be inspected;
- multiple templates are plausible and their intended use is unclear;
- contribution guidance conflicts with inferred defaults.

Read-only discovery failures must never fall through to issue publication.

## Content Rules

- No hardcoded Gentle AI or `agent-teams-lite` repository URLs.
- No universal claim that blank issues are disabled.
- No universal `status:needs-review` or `status:approved` gate.
- No fixed bug or feature template filenames.
- No project-specific agent/client field catalog.
- Examples use placeholders or values discovered from the current repository.

## Testing

Add a focused asset contract test that reads the embedded skill and verifies:

- repository-specific URLs and workflow claims are absent;
- repository, template, label, and Discussions discovery are present;
- a no-template fallback is documented;
- publication is blocked when discovery cannot establish safe inputs.

Run `go test ./internal/assets -count=1` as the authoritative focused suite. Also run formatting and repository diff checks. The full baseline currently has unrelated Windows symlink, WSL, cache-sensitive, and Git-timeout failures, so those results must be reported separately rather than attributed to this change.

## Out Of Scope

- Changing Gentle AI's own issue or PR approval policy.
- Adding GitHub API calls to the Go runtime.
- Creating templates or labels in consumer repositories.
- Automatically enabling Discussions.
- Modifying the separate `branch-pr` skill.

## Acceptance Checklist

- [ ] A consumer repository with custom templates receives instructions based on those templates.
- [ ] A repository without templates receives a safe structured-body fallback.
- [ ] Approval labels and Discussions are used only when discovered.
- [ ] No Gentle AI-specific URL or gate remains in the consumer asset.
- [ ] Existing skill frontmatter and installation paths remain stable.
- [ ] Focused asset tests pass.

## Rollback

Revert the consumer skill asset and its focused contract test. No persisted format, runtime state, or migration is involved.
