package assets

import (
	"strings"
	"testing"
)

func TestIssueCreationSkillDiscoversRepositoryPolicy(t *testing.T) {
	content := MustRead("skills/issue-creation/SKILL.md")

	for _, forbidden := range []string{
		"Gentleman-Programming",
		"agent-teams-lite",
		"bug_report.yml",
		"feature_request.yml",
		"status:needs-review",
		"status:approved",
		"Blank issues are disabled",
		"Every issue gets",
		"A maintainer MUST add",
	} {
		if strings.Contains(content, forbidden) {
			t.Errorf("consumer issue-creation skill contains repository-specific policy %q", forbidden)
		}
	}

	for _, required := range []string{
		"gh auth status",
		"gh repo view --json nameWithOwner,url,hasDiscussionsEnabled,hasIssuesEnabled,isBlankIssuesEnabled",
		".github/ISSUE_TEMPLATE",
		"REPO_URL=\"$(gh repo view --json url -q .url)\"",
		"HOST=\"${REPO_URL#*://}\"",
		"HOST=\"${HOST%%/*}\"",
		"gh api --hostname \"$HOST\" --paginate \"repos/$REPO/labels?per_page=100\" --jq '.[].name'",
		"REPO and HOST are non-empty",
		"hasIssuesEnabled is false",
		"required metadata is unavailable",
		"gh issue list --repo \"$HOST/$REPO\" --state all --search \"$QUERY\" --limit 1000",
		"If 1000 results are returned or completeness remains uncertain, narrow the search, use read-only API discovery, or stop and ask before publishing.",
		".yml and .yaml files are GitHub Issue Forms",
		"Do not parse or render their schema.",
		"gh issue create --repo \"$HOST/$REPO\" --web",
		"stop for human completion",
		".md files are Markdown templates",
		"BODY_FILE",
		"gh issue create --repo \"$HOST/$REPO\" --title \"$TITLE\" --body-file \"$BODY_FILE\"",
		"isBlankIssuesEnabled is explicitly true",
		"gh issue create --repo \"$HOST/$REPO\" --title \"$TITLE\" --body \"$BODY\"",
		"follow discovered contact links or stop and ask",
		"Never publish a no-template fallback.",
		"Stop and ask",
	} {
		if !strings.Contains(content, required) {
			t.Errorf("consumer issue-creation skill missing repository discovery or fallback %q", required)
		}
	}

	failedDiscoveryGuard := "Never continue from failed discovery into issue publication."
	guardIndex := strings.Index(content, failedDiscoveryGuard)
	if guardIndex == -1 {
		t.Errorf("consumer issue-creation skill missing failed-discovery guard %q", failedDiscoveryGuard)
	}

	publicationCommands := []string{
		"gh issue create --repo \"$HOST/$REPO\" --web",
		"gh issue create --repo \"$HOST/$REPO\" --title \"$TITLE\" --body-file \"$BODY_FILE\"",
		"gh issue create --repo \"$HOST/$REPO\" --title \"$TITLE\" --body \"$BODY\"",
	}
	requiredDiscoverySteps := []string{
		"gh auth status",
		"gh repo view --json nameWithOwner,url,hasDiscussionsEnabled,hasIssuesEnabled,isBlankIssuesEnabled",
		"gh api --hostname \"$HOST\" --paginate \"repos/$REPO/labels?per_page=100\" --jq '.[].name'",
		"gh issue list --repo \"$HOST/$REPO\" --state all --search \"$QUERY\" --limit 1000",
		"isBlankIssuesEnabled is explicitly true",
	}

	for _, issueCreationCommand := range publicationCommands {
		commandIndex := strings.Index(content, issueCreationCommand)
		if guardIndex >= commandIndex {
			t.Errorf("consumer issue-creation skill must place failed-discovery guard before %q", issueCreationCommand)
		}
		for _, discoveryStep := range requiredDiscoverySteps {
			if strings.Index(content, discoveryStep) >= commandIndex {
				t.Errorf("consumer issue-creation skill must place discovery step %q before %q", discoveryStep, issueCreationCommand)
			}
		}
	}
}
