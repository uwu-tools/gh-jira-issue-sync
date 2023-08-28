package jira

import (
	gogh "github.com/google/go-github/v53/github"
	jira "github.com/uwu-tools/go-jira/v2/cloud"

	"github.com/uwu-tools/gh-jira-issue-sync/internal/github"
)

// Client is a wrapper around the Jira API clients library we
// use. It allows us to hide implementation details such as backoff
// as well as swap in other implementations, such as for dry run
// or test mocking.
type Client interface {
	ListIssues(ids []int) ([]jira.Issue, error)
	GetIssue(key string) (*jira.Issue, error)
	// TODO: Remove unnecessary return values; consider only returning error
	CreateIssue(issue *jira.Issue) (*jira.Issue, error)
	// TODO: Remove unnecessary return values; consider only returning error
	UpdateIssue(issue *jira.Issue) (*jira.Issue, error)
	// TODO: Remove unnecessary return values; consider only returning error
	CreateComment(
		issue *jira.Issue, comment *gogh.IssueComment, githubClient github.Client,
	) (*jira.Comment, error)
	// TODO: Remove unnecessary return values; consider only returning error
	// TODO: Re-arrange arguments
	UpdateComment(
		issue *jira.Issue, id string, comment *gogh.IssueComment, githubClient github.Client,
	) (*jira.Comment, error)
}
