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
		"gh repo view --json nameWithOwner,url,hasDiscussionsEnabled",
		".github/ISSUE_TEMPLATE",
		"gh label list --repo \"$REPO\"",
		"gh issue list --repo \"$REPO\" --state all",
		"gh issue create --repo \"$REPO\" --template \"$TEMPLATE\"",
		"gh issue create --repo \"$REPO\" --title \"$TITLE\" --body \"$BODY\"",
		"Stop and ask",
	} {
		if !strings.Contains(content, required) {
			t.Errorf("consumer issue-creation skill missing repository discovery or fallback %q", required)
		}
	}
}
