package comment

import (
	gogh "github.com/google/go-github/v53/github"
	"github.com/stretchr/testify/mock"
	"github.com/uwu-tools/gh-jira-issue-sync/internal/config"
	"github.com/uwu-tools/gh-jira-issue-sync/internal/github"
	"github.com/uwu-tools/gh-jira-issue-sync/internal/jira"
	gojira "github.com/uwu-tools/go-jira/v2/cloud"
)

type CommentFnMock struct {
	mock.Mock
}

func (c *CommentFnMock) Reconcile(cfg config.IConfig, ghIssue *gogh.Issue, jIssue *gojira.Issue, ghClient github.Client, jClient jira.Client) error {
	args := c.Called(cfg, ghIssue, jIssue, ghClient, jClient)
	return args.Error(0)
}
