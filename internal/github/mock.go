package github

import (
	gogh "github.com/google/go-github/v53/github"
	"github.com/stretchr/testify/mock"
	"time"
)

type GhClientMock struct {
	mock.Mock
}

func (m *GhClientMock) ListIssues(owner, repo string) ([]*gogh.Issue, error) {
	args := m.Called(owner, repo)
	return args.Get(0).([]*gogh.Issue), args.Error(1)
}

func (m *GhClientMock) ListComments(owner, repo string, issue *gogh.Issue, since time.Time) ([]*gogh.IssueComment, error) {
	args := m.Called(owner, repo, issue, since)
	return args.Get(0).([]*gogh.IssueComment), args.Error(1)
}

func (m *GhClientMock) GetUser(login string) (*gogh.User, error) {
	args := m.Called(login)
	return args.Get(0).(*gogh.User), args.Error(1)
}
