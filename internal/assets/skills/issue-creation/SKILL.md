---
name: issue-creation
description: "Create and triage GitHub issues from repository evidence. Trigger: issue creation, bug reports, feature requests, or issue approval."
license: Apache-2.0
metadata:
  author: gentleman-programming
  version: "1.1"
---

# Issue Creation

## When To Use

Use this skill when creating, drafting, triaging, or approving an issue in the current GitHub repository.

## Core Rule

Discover the repository's actual contribution workflow before proposing or publishing an issue. Templates, labels, approval gates, and Discussions support are repository policy, not universal GitHub behavior.

## Safe Discovery

Run read-only checks first:

```bash
gh auth status
REPO="$(gh repo view --json nameWithOwner -q .nameWithOwner)"
gh repo view --json nameWithOwner,url,hasDiscussionsEnabled
git ls-files CONTRIBUTING.md CONTRIBUTING.* .github/CONTRIBUTING.md .github/ISSUE_TEMPLATE
gh api --paginate "repos/$REPO/labels?per_page=100" --jq '.[].name'
```

Also inspect:

- repository instructions such as `CONTRIBUTING.md` and `README.md`;
- files under `.github/ISSUE_TEMPLATE`;
- `.github/ISSUE_TEMPLATE/config.yml` when present;
- issue forms, required fields, and labels declared by each template;
- existing open and closed issues for duplicates and established wording.

Stop and ask for repository context if authentication, repository resolution, or policy discovery fails. Never continue from failed discovery into issue publication.

## Workflow

1. Describe the problem or request in one sentence and derive a short search query.
2. Search open and closed issues:

   ```bash
   gh issue list --repo "$REPO" --state all --search "$QUERY"
   ```

3. If an issue already covers the same behavior, comment there instead of creating a duplicate.
4. Choose a repository-provided template only when its purpose matches the report.
5. Fill every required template field from known evidence. Ask for missing facts rather than inventing them.
6. Apply labels only when they exist and repository guidance establishes who should apply them.
7. Publish only after the title, body, target repository, and selected template or fallback have been reviewed.

## Template Path

When a matching template exists, use its discovered filename:

```bash
gh issue create --repo "$REPO" --template "$TEMPLATE" --title "$TITLE"
```

Do not guess a template filename. If multiple templates could apply and repository guidance does not distinguish them, stop and ask which one to use.

## No-Template Fallback

When the repository permits issue creation but provides no matching template, prepare a structured body with these sections:

- problem or requested outcome;
- reproduction or motivating example;
- expected behavior;
- actual behavior or current limitation;
- environment and relevant evidence;
- alternatives or workarounds, when applicable.

Publish the reviewed fallback explicitly:

```bash
gh issue create --repo "$REPO" --title "$TITLE" --body "$BODY"
```

If repository configuration disables blank issues, follow its configured contact links or stop and ask. Do not bypass repository policy.

## Labels And Approval

Treat labels and approval gates as conditional:

- use only labels returned by repository discovery;
- follow contribution guidance for who may apply each label;
- wait when repository policy requires maintainer approval before implementation;
- do not invent a status or priority taxonomy when none is documented.

## Questions And Discussions

Use Discussions only when `hasDiscussionsEnabled` is true and repository guidance routes the question there. Otherwise follow documented support/contact links or ask the user where the question belongs. Never link to another repository's Discussions page.

## Triage Decision

Before approving or closing an issue, verify:

- it describes a concrete bug or scoped improvement rather than an unsupported question;
- it is not a duplicate;
- the report contains enough evidence for an implementation decision;
- the requested behavior is in repository scope;
- labels and status changes follow the current repository's policy.

If any point is uncertain, keep the issue in the repository's review state and request the smallest missing evidence.
