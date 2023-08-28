package issue

import (
	"errors"
	"os"
	"testing"

	gogh "github.com/google/go-github/v53/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/trivago/tgo/tcontainer"
	gojira "github.com/uwu-tools/go-jira/v2/cloud"

	clockMock "github.com/uwu-tools/gh-jira-issue-sync/internal/clock"
	"github.com/uwu-tools/gh-jira-issue-sync/internal/config"
	ghmock "github.com/uwu-tools/gh-jira-issue-sync/internal/github"
	jmock "github.com/uwu-tools/gh-jira-issue-sync/internal/jira"
	commentMock "github.com/uwu-tools/gh-jira-issue-sync/internal/utils/comment"
	"github.com/uwu-tools/gh-jira-issue-sync/pkg"
)

const (
	testGitHubIdFieldName       = "customfield_1000"
	testGitHubNumberFieldName   = "customfield_2000"
	testGitHubStatusFieldName   = "customfield_3000"
	testGitHubReporterFieldName = "customfield_4000"
	testGitHubLabelsFieldName   = "customfield_5000"
	testGitHubLastSyncFieldName = "customfield_6000"
)

var errMock = errors.New("mock error")

var project gojira.Project

var (
	jClient       *jmock.JiraClientMock
	ghClient      *ghmock.GhClientMock
	cfg           *config.ConfigMock
	commentFnMock *commentMock.CommentFnMock
)

var ghIssue1 = gogh.Issue{
	ID:     pkg.NewInt64(1),
	Number: pkg.NewInt(1),
	State:  pkg.NewString("Under review"),
	Title:  pkg.NewString("Title 1"),
	Body:   pkg.NewString("Issue body 1"),
	User: &gogh.User{
		Login: pkg.NewString("user1"),
	},
	Labels: []*gogh.Label{
		{
			Name: pkg.NewString("label 11"),
		},
		{
			Name: pkg.NewString("label 12"),
		},
	},
}

var ghIssue2 = gogh.Issue{
	ID:     pkg.NewInt64(2),
	Number: pkg.NewInt(2),
	State:  pkg.NewString("Completed"),
	Title:  pkg.NewString("Title 2"),
	Body:   pkg.NewString("Issue body 2"),
	User: &gogh.User{
		Login: pkg.NewString("user2"),
	},
	Labels: []*gogh.Label{
		{
			Name: pkg.NewString("label 21"),
		},
		{
			Name: pkg.NewString("label 22"),
		},
	},
}

var jiraIssue1 = gojira.Issue{
	Fields: &gojira.IssueFields{
		Type:        gojira.IssueType{Name: "Task"},
		Project:     project,
		Summary:     "Title 1",
		Description: "Issue body 1",
		Unknowns: tcontainer.MarshalMap{
			testGitHubIdFieldName:       float64(1),
			testGitHubNumberFieldName:   1,
			testGitHubStatusFieldName:   "Under review",
			testGitHubReporterFieldName: "user1",
			testGitHubLabelsFieldName:   []string{"label-11", "label-12"},
			testGitHubLastSyncFieldName: "1996-08-01T00:00:00.0+0200",
		},
	},
}

var jiraIssue2 = gojira.Issue{
	Fields: &gojira.IssueFields{
		Type:        gojira.IssueType{Name: "Task"},
		Project:     project,
		Summary:     "Title 2",
		Description: "Issue body 2",
		Unknowns: tcontainer.MarshalMap{
			testGitHubIdFieldName:       float64(2),
			testGitHubNumberFieldName:   2,
			testGitHubStatusFieldName:   "Completed",
			testGitHubReporterFieldName: "user2",
			testGitHubLabelsFieldName:   []string{"label-21", "label-22"},
			testGitHubLastSyncFieldName: "1996-08-01T00:00:00.0+0200",
		},
	},
}

var jiraIssue1Id = gojira.Issue{
	ID:  "1",
	Key: "jira-issue-1",
	Fields: &gojira.IssueFields{
		Type:        gojira.IssueType{Name: "Task"},
		Project:     project,
		Summary:     "Title 1",
		Description: "Issue body 1",
		Unknowns: tcontainer.MarshalMap{
			testGitHubIdFieldName:       float64(1),
			testGitHubNumberFieldName:   1,
			testGitHubStatusFieldName:   "Under review",
			testGitHubReporterFieldName: "user1",
			testGitHubLabelsFieldName:   []string{"label-11", "label-12"},
			testGitHubLastSyncFieldName: "1996-08-01T00:00:00.0+0200",
		},
	},
}

var jiraIssueUpdate1 = gojira.Issue{
	ID:  "1",
	Key: "jira-issue-1",
	Fields: &gojira.IssueFields{
		Type:        gojira.IssueType{Name: "Task"},
		Project:     project,
		Summary:     "Title 1",
		Description: "Issue body 1",
		Unknowns: tcontainer.MarshalMap{
			testGitHubStatusFieldName:   "Under review",
			testGitHubReporterFieldName: "user1",
			testGitHubLabelsFieldName:   []string{"label-11", "label-12"},
			testGitHubLastSyncFieldName: "1996-08-01T00:00:00.0+0200",
		},
	},
}

var jiraIssueNoGhID = gojira.Issue{
	Fields: &gojira.IssueFields{
		Type:     gojira.IssueType{Name: "Task"},
		Unknowns: tcontainer.MarshalMap{},
	},
}

func setup(t *testing.T) {
	t.Helper()

	commentFnMock = &commentMock.CommentFnMock{}
	compareCommentFn = commentFnMock.Reconcile

	jClient = new(jmock.JiraClientMock)
	ghClient = new(ghmock.GhClientMock)
	cfg = new(config.ConfigMock)
}

func TestMain(m *testing.M) {
	time = clockMock.NewClockMock()

	exitCode := m.Run()
	os.Exit(exitCode)
}

func TestCompare(t *testing.T) {
	tests := []struct { //nolint:govet
		name           string
		ghIssues       []*gogh.Issue
		jiraIssues     []gojira.Issue
		expectedResult *ComparisonResult
		expectedError  error
	}{
		{
			"should return empty results if no gh issue",
			[]*gogh.Issue{},
			[]gojira.Issue{},
			&ComparisonResult{
				[]*gogh.Issue{},
				[]*IssuePair{},
			},
			nil,
		},
		{
			"one issue should return for creation in Jira when there is no Jira issue",
			[]*gogh.Issue{&ghIssue1},
			[]gojira.Issue{},
			&ComparisonResult{
				ShouldCreate: []*gogh.Issue{&ghIssue1},
				ShouldUpdate: []*IssuePair{},
			},
			nil,
		},
		{
			"one issue should return for creation in Jira when there is no matching Jira issue for GH issue",
			[]*gogh.Issue{&ghIssue1},
			[]gojira.Issue{jiraIssue2},
			&ComparisonResult{
				ShouldCreate: []*gogh.Issue{&ghIssue1},
				ShouldUpdate: []*IssuePair{},
			},
			nil,
		},
		{
			"one issue should return for update in Jira when there is existing Jira issue as pair of GH issue",
			[]*gogh.Issue{&ghIssue1},
			[]gojira.Issue{jiraIssue1},
			&ComparisonResult{
				[]*gogh.Issue{},
				[]*IssuePair{{&ghIssue1, &jiraIssue1}},
			},
			nil,
		},
		{
			"two issue should return for creation when there is no existing Jira issue as pair of GH issues",
			[]*gogh.Issue{&ghIssue1, &ghIssue2},
			[]gojira.Issue{},
			&ComparisonResult{
				ShouldCreate: []*gogh.Issue{&ghIssue1, &ghIssue2},
				ShouldUpdate: []*IssuePair{},
			},
			nil,
		},
		{
			"two issue should return for update when there is matching Jira issue for all GH issues",
			[]*gogh.Issue{&ghIssue1, &ghIssue2},
			[]gojira.Issue{jiraIssue1, jiraIssue2},
			&ComparisonResult{
				[]*gogh.Issue{},
				[]*IssuePair{
					{
						&ghIssue1,
						&jiraIssue1,
					},
					{
						&ghIssue2,
						&jiraIssue2,
					},
				},
			},
			nil,
		},
		{
			"one issue should be created and one should be updated in Jira if only one matching jira issue exists as pair of gh issue",
			[]*gogh.Issue{&ghIssue1, &ghIssue2},
			[]gojira.Issue{jiraIssue1},
			&ComparisonResult{
				[]*gogh.Issue{&ghIssue2},
				[]*IssuePair{{&ghIssue1, &jiraIssue1}},
			},
			nil,
		},
		{
			"gh issue should be recreated if the corresponding jira issue does not contain github-id",
			[]*gogh.Issue{&ghIssue1},
			[]gojira.Issue{jiraIssueNoGhID},
			&ComparisonResult{ShouldCreate: []*gogh.Issue{&ghIssue1}, ShouldUpdate: make([]*IssuePair, 0)},
			nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setup(t)
			cfg.On("GetFieldKey", config.GitHubID).Return(testGitHubIdFieldName)

			result, err := Compare(cfg, tt.ghIssues, tt.jiraIssues)

			assert.Equal(t, tt.expectedError, err)
			assert.Equal(t, tt.expectedResult, result)
			mock.AssertExpectationsForObjects(t, cfg)
		})
	}
}

// func TestCreateIssue(t *testing.T) {
//	tests := []struct { //nolint:govet
//		name           string
//		ghIssue        *gogh.Issue
//		initMockFn     func()
//		expectedErrStr string
//	}{
//		{
//			"should create an issue if there is no external error",
//			&ghIssue1,
//			func() {
//				cfg.On("GetFieldKey", config.GitHubID).Return(testGitHubIdFieldName)
//				cfg.On("GetFieldKey", config.GitHubNumber).Return(testGitHubNumberFieldName)
//				cfg.On("GetFieldKey", config.GitHubStatus).Return(testGitHubStatusFieldName)
//				cfg.On("GetFieldKey", config.GitHubReporter).Return(testGitHubReporterFieldName)
//				cfg.On("GetFieldKey", config.GitHubLabels).Return(testGitHubLabelsFieldName)
//				cfg.On("GetFieldKey", config.GitHubLastSync).Return(testGitHubLastSyncFieldName)
//				cfg.On("GetProject").Return(&project)
//				jClient.On("CreateIssue", &jiraIssue1).Return(&jiraIssue1Id, nil)
//				jClient.On("GetIssue", "jira-issue-1").Return(&jiraIssue1Id, nil)
//				commentFnMock.On("Reconcile", cfg, &ghIssue1, &jiraIssue1Id, ghClient, jClient).Return(nil).Once()
//			},
//			"",
//		},
//		{
//			"should return error if the creation of issue failed",
//			&ghIssue1,
//			func() {
//				cfg.On("GetFieldKey", config.GitHubID).Return(testGitHubIdFieldName)
//				cfg.On("GetFieldKey", config.GitHubNumber).Return(testGitHubNumberFieldName)
//				cfg.On("GetFieldKey", config.GitHubStatus).Return(testGitHubStatusFieldName)
//				cfg.On("GetFieldKey", config.GitHubReporter).Return(testGitHubReporterFieldName)
//				cfg.On("GetFieldKey", config.GitHubLabels).Return(testGitHubLabelsFieldName)
//				cfg.On("GetFieldKey", config.GitHubLastSync).Return(testGitHubLastSyncFieldName)
//				cfg.On("GetProject").Return(&project)
//				jClient.On("CreateIssue", &jiraIssue1).Return(&gojira.Issue{}, errors.New("creation error"))
//			},
//			"creating Jira issue",
//		},
//		{
//			"should return error if checking of creation failed",
//			&ghIssue1,
//			func() {
//				cfg.On("GetFieldKey", config.GitHubID).Return(testGitHubIdFieldName)
//				cfg.On("GetFieldKey", config.GitHubNumber).Return(testGitHubNumberFieldName)
//				cfg.On("GetFieldKey", config.GitHubStatus).Return(testGitHubStatusFieldName)
//				cfg.On("GetFieldKey", config.GitHubReporter).Return(testGitHubReporterFieldName)
//				cfg.On("GetFieldKey", config.GitHubLabels).Return(testGitHubLabelsFieldName)
//				cfg.On("GetFieldKey", config.GitHubLastSync).Return(testGitHubLastSyncFieldName)
//				cfg.On("GetProject").Return(&project)
//				jClient.On("CreateIssue", &jiraIssue1).Return(&jiraIssue1Id, nil)
//				jClient.On("GetIssue", "jira-issue-1").Return(&jiraIssue1Id, errors.New("getting issue error"))
//			},
//			"getting Jira issue",
//		},
//		{
//			"should return error if the reconcile of the comments failed",
//			&ghIssue1,
//			func() {
//				cfg.On("GetFieldKey", config.GitHubID).Return(testGitHubIdFieldName)
//				cfg.On("GetFieldKey", config.GitHubNumber).Return(testGitHubNumberFieldName)
//				cfg.On("GetFieldKey", config.GitHubStatus).Return(testGitHubStatusFieldName)
//				cfg.On("GetFieldKey", config.GitHubReporter).Return(testGitHubReporterFieldName)
//				cfg.On("GetFieldKey", config.GitHubLabels).Return(testGitHubLabelsFieldName)
//				cfg.On("GetFieldKey", config.GitHubLastSync).Return(testGitHubLastSyncFieldName)
//				cfg.On("GetProject").Return(&project)
//				jClient.On("CreateIssue", &jiraIssue1).Return(&jiraIssue1Id, nil)
//				jClient.On("GetIssue", "jira-issue-1").Return(&jiraIssue1Id, nil)
//				commentFnMock.On("Reconcile", cfg, &ghIssue1, &jiraIssue1Id, ghClient, jClient).Return(errors.New("compare error fails")).Once()
//			},
//			"comparing comments for issue failed",
//		},
//	}
//
//	for _, tt := range tests {
//		t.Run(tt.name, func(t *testing.T) {
//			setup(t)
//			tt.initMockFn()
//
//			err := CreateIssue(cfg, tt.ghIssue, ghClient, jClient)
//
//			if tt.expectedErrStr != "" {
//				assert.ErrorContains(t, err, tt.expectedErrStr)
//			}
//			mock.AssertExpectationsForObjects(t, cfg, jClient)
//		})
//	}
// }

func TestUpdateIssue(t *testing.T) {
	var ghIssue *gogh.Issue
	var newJiraIssue *gojira.Issue

	const updatedTitle = "Updated title"
	const updatedBody = "Updated body"
	const updatedStatus = "Updated status"
	const updatedUser = "Updated user"

	tests := []struct { //nolint:govet
		name              string
		getGhIssueFn      func() *gogh.Issue
		getJiraIssueFn    func() *gojira.Issue
		getNewJiraIssueFn func() *gojira.Issue
		initMockFn        func()
		expectedErr       string
	}{
		{
			"should not update if no change but comments should be compared",
			func() *gogh.Issue {
				return &ghIssue1
			},
			func() *gojira.Issue {
				return &jiraIssue1Id
			},
			func() *gojira.Issue {
				return &jiraIssue1Id
			},
			func() {
				cfg.On("GetFieldKey", config.GitHubStatus).Return(testGitHubStatusFieldName)
				cfg.On("GetFieldKey", config.GitHubReporter).Return(testGitHubReporterFieldName)
				cfg.On("GetFieldKey", config.GitHubLabels).Return(testGitHubLabelsFieldName)
				jClient.On("GetIssue", "jira-issue-1").Return(newJiraIssue, nil)
				commentFnMock.On("Reconcile", cfg, ghIssue, newJiraIssue, ghClient, jClient).Return(nil)
			},
			"",
		},
		{
			"should update the issue if GH title was changed",
			func() *gogh.Issue {
				iss := ghIssue1
				iss.Title = pkg.NewString(updatedTitle)
				return &iss
			},
			func() *gojira.Issue {
				return &jiraIssue1Id
			},
			func() *gojira.Issue {
				iss := jiraIssueUpdate1
				issFields := *iss.Fields
				issFields.Summary = updatedTitle //nolint:goconst
				iss.Fields = &issFields
				return &iss
			},
			func() {
				cfg.On("GetFieldKey", config.GitHubStatus).Return(testGitHubStatusFieldName)
				cfg.On("GetFieldKey", config.GitHubReporter).Return(testGitHubReporterFieldName)
				cfg.On("GetFieldKey", config.GitHubLabels).Return(testGitHubLabelsFieldName)
				cfg.On("GetFieldKey", config.GitHubLastSync).Return(testGitHubLastSyncFieldName)
				jClient.On("UpdateIssue", newJiraIssue).Return(&gojira.Issue{}, nil)
				jClient.On("GetIssue", "jira-issue-1").Return(newJiraIssue, nil)
				commentFnMock.On("Reconcile", cfg, ghIssue, newJiraIssue, ghClient, jClient).Return(nil)
			},
			"",
		},
		{
			"should update the issue if GH body was changed",
			func() *gogh.Issue {
				iss := ghIssue1
				iss.Body = pkg.NewString(updatedBody)
				return &iss
			},
			func() *gojira.Issue {
				return &jiraIssue1Id
			},
			func() *gojira.Issue {
				iss := jiraIssueUpdate1
				issFields := *iss.Fields
				issFields.Description = updatedBody
				iss.Fields = &issFields
				return &iss
			},
			func() {
				cfg.On("GetFieldKey", config.GitHubStatus).Return(testGitHubStatusFieldName)
				cfg.On("GetFieldKey", config.GitHubReporter).Return(testGitHubReporterFieldName)
				cfg.On("GetFieldKey", config.GitHubLabels).Return(testGitHubLabelsFieldName)
				cfg.On("GetFieldKey", config.GitHubLastSync).Return(testGitHubLastSyncFieldName)
				jClient.On("UpdateIssue", newJiraIssue).Return(&gojira.Issue{}, nil)
				jClient.On("GetIssue", "jira-issue-1").Return(newJiraIssue, nil)
				commentFnMock.On("Reconcile", cfg, ghIssue, newJiraIssue, ghClient, jClient).Return(nil)
			},
			"",
		},
		{
			"should update the issue if GH status was changed",
			func() *gogh.Issue {
				iss := ghIssue1
				iss.State = pkg.NewString(updatedStatus)
				return &iss
			},
			func() *gojira.Issue {
				return &jiraIssue1Id
			},
			func() *gojira.Issue {
				iss := jiraIssueUpdate1
				issFields := *iss.Fields
				issFields.Unknowns = issFields.Unknowns.Clone()
				issFields.Unknowns.Set(testGitHubStatusFieldName, updatedStatus)
				iss.Fields = &issFields
				return &iss
			},
			func() {
				cfg.On("GetFieldKey", config.GitHubStatus).Return(testGitHubStatusFieldName)
				cfg.On("GetFieldKey", config.GitHubReporter).Return(testGitHubReporterFieldName)
				cfg.On("GetFieldKey", config.GitHubLabels).Return(testGitHubLabelsFieldName)
				cfg.On("GetFieldKey", config.GitHubLastSync).Return(testGitHubLastSyncFieldName)
				jClient.On("UpdateIssue", newJiraIssue).Return(&gojira.Issue{}, nil)
				jClient.On("GetIssue", "jira-issue-1").Return(newJiraIssue, nil)
				commentFnMock.On("Reconcile", cfg, ghIssue, newJiraIssue, ghClient, jClient).Return(nil)
			},
			"",
		},
		{
			"should update the issue if GH status missing in Jira issue",
			func() *gogh.Issue {
				iss := ghIssue1
				iss.State = pkg.NewString(updatedStatus)
				return &iss
			},
			func() *gojira.Issue {
				iss := jiraIssue1Id
				issFields := *iss.Fields
				issFields.Unknowns = issFields.Unknowns.Clone()
				issFields.Unknowns.Delete(testGitHubStatusFieldName)
				iss.Fields = &issFields
				return &iss
			},
			func() *gojira.Issue {
				iss := jiraIssueUpdate1
				issFields := *iss.Fields
				issFields.Unknowns = issFields.Unknowns.Clone()
				issFields.Unknowns.Set(testGitHubStatusFieldName, updatedStatus)
				iss.Fields = &issFields
				return &iss
			},
			func() {
				cfg.On("GetFieldKey", config.GitHubStatus).Return(testGitHubStatusFieldName)
				cfg.On("GetFieldKey", config.GitHubReporter).Return(testGitHubReporterFieldName)
				cfg.On("GetFieldKey", config.GitHubLabels).Return(testGitHubLabelsFieldName)
				cfg.On("GetFieldKey", config.GitHubLastSync).Return(testGitHubLastSyncFieldName)
				jClient.On("UpdateIssue", newJiraIssue).Return(&gojira.Issue{}, nil)
				jClient.On("GetIssue", "jira-issue-1").Return(newJiraIssue, nil)
				commentFnMock.On("Reconcile", cfg, ghIssue, newJiraIssue, ghClient, jClient).Return(nil)
			},
			"",
		},
		{
			"should update if GH reporter was changed",
			func() *gogh.Issue {
				iss := ghIssue1
				issUser := *iss.User
				issUser.Login = pkg.NewString(updatedUser)
				iss.User = &issUser
				return &iss
			},
			func() *gojira.Issue {
				return &jiraIssue1Id
			},
			func() *gojira.Issue {
				iss := jiraIssueUpdate1
				issFields := *iss.Fields
				issFields.Unknowns = issFields.Unknowns.Clone()
				issFields.Unknowns.Set(testGitHubReporterFieldName, updatedUser)
				iss.Fields = &issFields
				return &iss
			},
			func() {
				cfg.On("GetFieldKey", config.GitHubStatus).Return(testGitHubStatusFieldName)
				cfg.On("GetFieldKey", config.GitHubReporter).Return(testGitHubReporterFieldName)
				cfg.On("GetFieldKey", config.GitHubLabels).Return(testGitHubLabelsFieldName)
				cfg.On("GetFieldKey", config.GitHubLastSync).Return(testGitHubLastSyncFieldName)
				jClient.On("UpdateIssue", newJiraIssue).Return(&gojira.Issue{}, nil)
				jClient.On("GetIssue", "jira-issue-1").Return(newJiraIssue, nil)
				commentFnMock.On("Reconcile", cfg, ghIssue, newJiraIssue, ghClient, jClient).Return(nil)
			},
			"",
		},
		{
			"should update if GH reporter is missing in Jira issue",
			func() *gogh.Issue {
				iss := ghIssue1
				issUser := *iss.User
				issUser.Login = pkg.NewString(updatedUser)
				iss.User = &issUser
				return &iss
			},
			func() *gojira.Issue {
				iss := jiraIssue1Id
				issFields := *iss.Fields
				issFields.Unknowns = issFields.Unknowns.Clone()
				issFields.Unknowns.Delete(testGitHubReporterFieldName)
				iss.Fields = &issFields
				return &iss
			},
			func() *gojira.Issue {
				iss := jiraIssueUpdate1
				issFields := *iss.Fields
				issFields.Unknowns = issFields.Unknowns.Clone()
				issFields.Unknowns.Set(testGitHubReporterFieldName, updatedUser)
				iss.Fields = &issFields
				return &iss
			},
			func() {
				cfg.On("GetFieldKey", config.GitHubStatus).Return(testGitHubStatusFieldName)
				cfg.On("GetFieldKey", config.GitHubReporter).Return(testGitHubReporterFieldName)
				cfg.On("GetFieldKey", config.GitHubLabels).Return(testGitHubLabelsFieldName)
				cfg.On("GetFieldKey", config.GitHubLastSync).Return(testGitHubLastSyncFieldName)
				jClient.On("UpdateIssue", newJiraIssue).Return(&gojira.Issue{}, nil)
				jClient.On("GetIssue", "jira-issue-1").Return(newJiraIssue, nil)
				commentFnMock.On("Reconcile", cfg, ghIssue, newJiraIssue, ghClient, jClient).Return(nil)
			},
			"",
		},
		// {
		//	"should update if a GH label was deleted",
		//	func() *gogh.Issue {
		//		iss := ghIssue1
		//		newLabels := []*gogh.Label{
		//			{
		//				Name: pkg.NewString("label 11"),
		//			},
		//		}
		//		iss.Labels = newLabels
		//		return &iss
		//	},
		//	func() *gojira.Issue {
		//		return &jiraIssue1Id
		//	},
		//	func() *gojira.Issue {
		//		iss := jiraIssueUpdate1
		//		issFields := *iss.Fields
		//		issFields.Unknowns = issFields.Unknowns.Clone()
		//		issFields.Unknowns.Set(testGitHubLabelsFieldName, []string{"label-11"})
		//		iss.Fields = &issFields
		//		return &iss
		//	},
		//	func() {
		//		cfg.On("GetFieldKey", config.GitHubStatus).Return(testGitHubStatusFieldName)
		//		cfg.On("GetFieldKey", config.GitHubReporter).Return(testGitHubReporterFieldName)
		//		cfg.On("GetFieldKey", config.GitHubLabels).Return(testGitHubLabelsFieldName)
		//		cfg.On("GetFieldKey", config.GitHubLastSync).Return(testGitHubLastSyncFieldName)
		//		jClient.On("UpdateIssue", newJiraIssue).Return(&gojira.Issue{}, nil)
		//		jClient.On("GetIssue", "jira-issue-1").Return(newJiraIssue, nil)
		//		commentFnMock.On("Reconcile", cfg, ghIssue, newJiraIssue, ghClient, jClient).Return(nil)
		//	},
		//	"",
		// },
		// {
		//	"should update if a new GH label was added",
		//	func() *gogh.Issue {
		//		iss := ghIssue1
		//		newLabels := []*gogh.Label{
		//			{
		//				Name: pkg.NewString("label 11"),
		//			},
		//			{
		//				Name: pkg.NewString("label 12"),
		//			},
		//			{
		//				Name: pkg.NewString("label 13"),
		//			},
		//		}
		//		iss.Labels = newLabels
		//		return &iss
		//	},
		//	func() *gojira.Issue {
		//		return &jiraIssue1Id
		//	},
		//	func() *gojira.Issue {
		//		iss := jiraIssueUpdate1
		//		issFields := *iss.Fields
		//		issFields.Unknowns = issFields.Unknowns.Clone()
		//		issFields.Unknowns.Set(testGitHubLabelsFieldName, []string{"label-11", "label-12", "label-13"})
		//		iss.Fields = &issFields
		//		return &iss
		//	},
		//	func() {
		//		cfg.On("GetFieldKey", config.GitHubStatus).Return(testGitHubStatusFieldName)
		//		cfg.On("GetFieldKey", config.GitHubReporter).Return(testGitHubReporterFieldName)
		//		cfg.On("GetFieldKey", config.GitHubLabels).Return(testGitHubLabelsFieldName)
		//		cfg.On("GetFieldKey", config.GitHubLastSync).Return(testGitHubLastSyncFieldName)
		//		jClient.On("UpdateIssue", newJiraIssue).Return(&gojira.Issue{}, nil)
		//		jClient.On("GetIssue", "jira-issue-1").Return(newJiraIssue, nil)
		//		commentFnMock.On("Reconcile", cfg, ghIssue, newJiraIssue, ghClient, jClient).Return(nil)
		//	},
		//	"",
		// },
		{
			"should return error if update failed",
			func() *gogh.Issue {
				iss := ghIssue1
				iss.Title = pkg.NewString(updatedTitle)
				return &iss
			},
			func() *gojira.Issue {
				return &jiraIssue1Id
			},
			func() *gojira.Issue {
				iss := jiraIssueUpdate1
				issFields := *iss.Fields
				issFields.Summary = updatedTitle
				iss.Fields = &issFields
				return &iss
			},
			func() {
				cfg.On("GetFieldKey", config.GitHubStatus).Return(testGitHubStatusFieldName)
				cfg.On("GetFieldKey", config.GitHubReporter).Return(testGitHubReporterFieldName)
				cfg.On("GetFieldKey", config.GitHubLabels).Return(testGitHubLabelsFieldName)
				cfg.On("GetFieldKey", config.GitHubLastSync).Return(testGitHubLastSyncFieldName)
				jClient.On("UpdateIssue", newJiraIssue).Return(&gojira.Issue{}, nil)
				jClient.On("GetIssue", "jira-issue-1").Return(&gojira.Issue{}, errMock)
			},
			"getting Jira issue",
		},
		{
			"should return error if comparison comments failed",
			func() *gogh.Issue {
				iss := ghIssue1
				iss.Title = pkg.NewString(updatedTitle)
				return &iss
			},
			func() *gojira.Issue {
				return &jiraIssue1Id
			},
			func() *gojira.Issue {
				iss := jiraIssueUpdate1
				issFields := *iss.Fields
				issFields.Summary = updatedTitle
				iss.Fields = &issFields
				return &iss
			},
			func() {
				cfg.On("GetFieldKey", config.GitHubStatus).Return(testGitHubStatusFieldName)
				cfg.On("GetFieldKey", config.GitHubReporter).Return(testGitHubReporterFieldName)
				cfg.On("GetFieldKey", config.GitHubLabels).Return(testGitHubLabelsFieldName)
				cfg.On("GetFieldKey", config.GitHubLastSync).Return(testGitHubLastSyncFieldName)
				jClient.On("UpdateIssue", newJiraIssue).Return(&gojira.Issue{}, nil)
				jClient.On("GetIssue", "jira-issue-1").Return(newJiraIssue, nil)
				commentFnMock.On("Reconcile", cfg, ghIssue, newJiraIssue, ghClient, jClient).Return(errMock)
			},
			"comparing comments",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setup(t)

			ghIssue = tt.getGhIssueFn()
			newJiraIssue = tt.getNewJiraIssueFn()
			tt.initMockFn()

			err := UpdateIssue(cfg, ghIssue, tt.getJiraIssueFn(), ghClient, jClient)

			if tt.expectedErr != "" {
				assert.ErrorContains(t, err, tt.expectedErr)
			}
			mock.AssertExpectationsForObjects(t, cfg, jClient, commentFnMock)
		})
	}
}
