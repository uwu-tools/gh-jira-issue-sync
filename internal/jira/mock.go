package jira

import (
	gogh "github.com/google/go-github/v53/github"
	"github.com/stretchr/testify/mock"
	"github.com/uwu-tools/gh-jira-issue-sync/internal/github"
	jira "github.com/uwu-tools/go-jira/v2/cloud"
)

type JiraClientMock struct {
	mock.Mock
}

func (j *JiraClientMock) ListIssues(ids []int) ([]jira.Issue, error) {
	args := j.Called(ids)
	return args.Get(0).([]jira.Issue), args.Error(1)
}

func (j *JiraClientMock) GetIssue(key string) (*jira.Issue, error) {
	args := j.Called(key)
	return args.Get(0).(*jira.Issue), args.Error(1)
}

func (j *JiraClientMock) CreateIssue(issue *jira.Issue) (*jira.Issue, error) {
	args := j.Called(issue)
	return args.Get(0).(*jira.Issue), args.Error(1)
}

func (j *JiraClientMock) UpdateIssue(issue *jira.Issue) (*jira.Issue, error) {
	args := j.Called(issue)
	return args.Get(0).(*jira.Issue), args.Error(1)
}

func (j *JiraClientMock) CreateComment(issue *jira.Issue, comment *gogh.IssueComment, githubClient github.Client) (*jira.Comment, error) {
	args := j.Called(issue, comment, githubClient)
	return args.Get(0).(*jira.Comment), args.Error(1)
}

func (j *JiraClientMock) UpdateComment(issue *jira.Issue, id string, comment *gogh.IssueComment, githubClient github.Client) (*jira.Comment, error) {
	args := j.Called(issue, id, comment, githubClient)
	return args.Get(0).(*jira.Comment), args.Error(1)
}
