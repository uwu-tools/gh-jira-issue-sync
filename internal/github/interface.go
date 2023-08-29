package github

import (
	"time"

	gogh "github.com/google/go-github/v53/github"
)

// Client is a wrapper around the GitHub API Client library we
// use. It allows us to swap in other implementations, such as a dry run
// clients, or mock clients for testing.
type Client interface {
	ListIssues(owner, repo string) ([]*gogh.Issue, error)
	ListComments(
		owner, repo string, issue *gogh.Issue, since time.Time,
	) ([]*gogh.IssueComment, error)
	GetUser(login string) (*gogh.User, error)
}
