package issue

import (
	gogh "github.com/google/go-github/v53/github"
	"github.com/stretchr/testify/mock"
	gojira "github.com/uwu-tools/go-jira/v2/cloud"

	"github.com/uwu-tools/gh-jira-issue-sync/internal/config"
	"github.com/uwu-tools/gh-jira-issue-sync/internal/github"
	"github.com/uwu-tools/gh-jira-issue-sync/internal/jira"
)

type IssueFnMock struct {
	mock.Mock
}

func (ifn *IssueFnMock) CreateIssue(
	cfg config.IConfig,
	issue *gogh.Issue,
	ghClient github.Client,
	jClient jira.Client,
) error {
	args := ifn.Called(cfg, issue, ghClient, jClient)
	return args.Error(0) //nolint:wrapcheck
}

func (ifn *IssueFnMock) UpdateIssue(
	cfg config.IConfig,
	ghIssue *gogh.Issue,
	jIssue *gojira.Issue,
	ghClient github.Client,
	jClient jira.Client,
) error {
	args := ifn.Called(cfg, ghIssue, jIssue, ghClient, jClient)
	return args.Error(0) //nolint:wrapcheck
}
