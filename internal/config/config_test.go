package config

import (
	"errors"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/uwu-tools/gh-jira-issue-sync/internal/filesystem"
	"github.com/uwu-tools/gh-jira-issue-sync/internal/options"
	jira "github.com/uwu-tools/go-jira/v2/cloud"
	"strings"
	"testing"
)

var mockError = errors.New("mock error")

var rCmd *cobra.Command
var config2 *config

func setup(t *testing.T) {
	t.Helper()

	fs = &filesystem.MockFs{}
}

func setupGetConfigPath(t *testing.T) {
	t.Helper()

	rCmd = &cobra.Command{}
	rCmd.Flags().String(options.ConfigKeyConfigFile, "", "config file path")
}

func setupValidation(t *testing.T, content string) {
	t.Helper()

	config2 = &config{}
	viper.Reset()
	v := viper.GetViper()
	v.SetConfigType("json")
	if err := v.ReadConfig(strings.NewReader(content)); err != nil {
		t.Errorf("error to read config: %s", err)
	}
	config2.cmdConfig = *v
}

func setupParseField(t *testing.T) {
	t.Helper()

	config2 = &config{}
}

func TestGetConfigPath(t *testing.T) {

	tests := []struct { //nolint:govet
		name         string
		cmdArgs      []string
		initMock     func()
		expectedPath string
		expectedErr  error
	}{
		{
			"successful with provided cli parameter",
			[]string{"--config", "./test/config.json"},
			func() {
				fs.(*filesystem.MockFs).On("Stat", "./test/config.json").Return(&filesystem.FakeFileInfo{}, nil)
			},
			"./test/config.json",
			nil,
		},
		{
			"successful without provided cli parameter",
			[]string{},
			func() {
				fs.(*filesystem.MockFs).On("Getwd").Return("/currentwd", nil)
				fs.(*filesystem.MockFs).On("Stat", "/currentwd/.issue-sync.json").Return(&filesystem.FakeFileInfo{}, nil)
			},
			"/currentwd/.issue-sync.json",
			nil,
		},
		{
			"getting error because file doesn't exist",
			[]string{"--config", "./test/config.json"},
			func() {
				fs.(*filesystem.MockFs).On("Stat", "./test/config.json").Return(&filesystem.FakeFileInfo{}, mockError)
			},
			"",
			mockError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setup(t)
			setupGetConfigPath(t)
			tt.initMock()

			if err := rCmd.ParseFlags(tt.cmdArgs); err != nil {
				t.Errorf("Error during pasing flags: %s", err.Error())
			}

			path, err := getConfigFilePath(rCmd)

			assert.Equal(t, tt.expectedPath, path)
			if tt.expectedErr != nil {
				if err == nil {
					t.Errorf("error should be not null")
				}
				assert.Contains(t, err.Error(), tt.expectedErr.Error())
			}
		})
	}
}

// Only basic auth is tested because OAuth 1.0 is deprecated.
func TestValidateConfig(t *testing.T) {
	tests := []struct { //nolint:govet
		name        string
		config      string
		expectedErr error
	}{
		{
			"all field is valid",
			`{
	"jira-user": "user",
	"jira-pass": "jiratoken",
	"github-token": "ghtoken",
	"repo-name": "example/repo",
	"jira-uri": "https://jira.com",
	"jira-project": "example",
	"since": "2023-08-16T15:04:05-0000"
}`,
			nil,
		},
		{
			"should return error because github token is missing",
			`{
	"jira-user": "user",
	"jira-pass": "jiratoken",
	"repo-name": "example/repo",
	"jira-uri": "https://jira.com",
	"jira-project": "example",
	"since": "2023-08-16T14:54:14.712Z"
}`,
			errGitHubTokenRequired,
		},
		{
			"should return error because repo-name is missing",
			`{
	"jira-user": "user",
	"jira-pass": "jiratoken",
	"github-token": "ghtoken",
	"jira-uri": "https://jira.com",
	"jira-project": "example",
	"since": "2023-08-16T14:54:14.712Z"
}`,
			errGitHubRepoRequired,
		},
		{
			"should return error because repo-name is not in the right format",
			`{
	"jira-user": "user",
	"jira-pass": "jiratoken",
	"github-token": "ghtoken",
	"repo-name": "repo",
	"jira-uri": "https://jira.com",
	"jira-project": "example",
	"since": "2023-08-16T14:54:14.712Z"
}`,
			errGitHubRepoFormatInvalid,
		},
		{
			"should return error because jira-uri is missing",
			`{
	"jira-user": "user",
	"jira-pass": "jiratoken",
	"github-token": "ghtoken",
	"repo-name": "example/repo",
	"jira-project": "example",
	"since": "2023-08-16T14:54:14.712Z"
}`,
			errJiraURIRequired,
		},
		{
			"should return error because jira-uri is invalid",
			`{
	"jira-user": "user",
	"jira-pass": "jiratoken",
	"github-token": "ghtoken",
	"repo-name": "example/repo",
	"jira-uri": "jira",
	"jira-project": "example",
	"since": "2023-08-16T14:54:14.712Z"
}`,
			errJiraURIInvalid,
		},
		{
			"should return error because jira project is missing",
			`{
	"jira-user": "user",
	"jira-pass": "jiratoken",
	"github-token": "ghtoken",
	"repo-name": "example/repo",
	"jira-uri": "https://jira.com",
	"since": "2023-08-16T14:54:14.712Z"
}`,
			errJiraProjectRequired,
		},
		{
			"should return error because since field is in wrong format",
			`{
	"jira-user": "user",
	"jira-pass": "jiratoken",
	"github-token": "ghtoken",
	"repo-name": "example/repo",
	"jira-uri": "https://jira.com",
	"jira-project": "example",
	"since": "20230816"
}`,
			errDateInvalid,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setup(t)
			setupValidation(t, tt.config)

			err := config2.validateConfig()

			assert.Equal(t, tt.expectedErr, err)
		})
	}
}

func TestParseFields(t *testing.T) {
	tests := []struct { //nolint:govet
		name           string
		jiraFields     *[]jira.Field
		expectedFields *fields
		expectedError  error
	}{
		{
			"successful if all fields are filled",
			&[]jira.Field{
				{
					Name:   "github-id",
					Schema: jira.FieldSchema{CustomID: 1},
				},
				{
					Name:   "github-number",
					Schema: jira.FieldSchema{CustomID: 2},
				},
				{
					Name:   "github-labels",
					Schema: jira.FieldSchema{CustomID: 3},
				},
				{
					Name:   "github-status",
					Schema: jira.FieldSchema{CustomID: 4},
				},
				{
					Name:   "github-reporter",
					Schema: jira.FieldSchema{CustomID: 5},
				},
				{
					Name:   "github-last-sync",
					Schema: jira.FieldSchema{CustomID: 6},
				},
			},
			&fields{
				githubID:       "1",
				githubNumber:   "2",
				githubLabels:   "3",
				githubReporter: "5",
				githubStatus:   "4",
				lastUpdate:     "6",
			},
			nil,
		},
		{
			"github-id is missing",
			&[]jira.Field{
				{
					Name:   "github-number",
					Schema: jira.FieldSchema{CustomID: 2},
				},
				{
					Name:   "github-labels",
					Schema: jira.FieldSchema{CustomID: 3},
				},
				{
					Name:   "github-status",
					Schema: jira.FieldSchema{CustomID: 4},
				},
				{
					Name:   "github-reporter",
					Schema: jira.FieldSchema{CustomID: 5},
				},
				{
					Name:   "github-last-sync",
					Schema: jira.FieldSchema{CustomID: 6},
				},
			},
			nil,
			errCustomFieldIDNotFound("github-id"),
		},
		{
			"github-number is missing",
			&[]jira.Field{
				{
					Name:   "github-id",
					Schema: jira.FieldSchema{CustomID: 1},
				},
				{
					Name:   "github-labels",
					Schema: jira.FieldSchema{CustomID: 3},
				},
				{
					Name:   "github-status",
					Schema: jira.FieldSchema{CustomID: 4},
				},
				{
					Name:   "github-reporter",
					Schema: jira.FieldSchema{CustomID: 5},
				},
				{
					Name:   "github-last-sync",
					Schema: jira.FieldSchema{CustomID: 6},
				},
			},
			nil,
			errCustomFieldIDNotFound("github-number"),
		},
		{
			"github-labels is missing",
			&[]jira.Field{
				{
					Name:   "github-number",
					Schema: jira.FieldSchema{CustomID: 2},
				},
				{
					Name:   "github-id",
					Schema: jira.FieldSchema{CustomID: 1},
				},
				{
					Name:   "github-status",
					Schema: jira.FieldSchema{CustomID: 4},
				},
				{
					Name:   "github-reporter",
					Schema: jira.FieldSchema{CustomID: 5},
				},
				{
					Name:   "github-last-sync",
					Schema: jira.FieldSchema{CustomID: 6},
				},
			},
			nil,
			errCustomFieldIDNotFound("github-labels"),
		},
		{
			"github-status is missing",
			&[]jira.Field{
				{
					Name:   "github-number",
					Schema: jira.FieldSchema{CustomID: 2},
				},
				{
					Name:   "github-labels",
					Schema: jira.FieldSchema{CustomID: 3},
				},
				{
					Name:   "github-id",
					Schema: jira.FieldSchema{CustomID: 1},
				},
				{
					Name:   "github-reporter",
					Schema: jira.FieldSchema{CustomID: 5},
				},
				{
					Name:   "github-last-sync",
					Schema: jira.FieldSchema{CustomID: 6},
				},
			},
			nil,
			errCustomFieldIDNotFound("github-status"),
		},
		{
			"github-reporter is missing",
			&[]jira.Field{
				{
					Name:   "github-number",
					Schema: jira.FieldSchema{CustomID: 2},
				},
				{
					Name:   "github-labels",
					Schema: jira.FieldSchema{CustomID: 3},
				},
				{
					Name:   "github-status",
					Schema: jira.FieldSchema{CustomID: 4},
				},
				{
					Name:   "github-id",
					Schema: jira.FieldSchema{CustomID: 1},
				},
				{
					Name:   "github-last-sync",
					Schema: jira.FieldSchema{CustomID: 6},
				},
			},
			nil,
			errCustomFieldIDNotFound("github-reporter"),
		},
		{
			"github-last-sync is missing",
			&[]jira.Field{
				{
					Name:   "github-number",
					Schema: jira.FieldSchema{CustomID: 2},
				},
				{
					Name:   "github-labels",
					Schema: jira.FieldSchema{CustomID: 3},
				},
				{
					Name:   "github-status",
					Schema: jira.FieldSchema{CustomID: 4},
				},
				{
					Name:   "github-reporter",
					Schema: jira.FieldSchema{CustomID: 5},
				},
				{
					Name:   "github-id",
					Schema: jira.FieldSchema{CustomID: 1},
				},
			},
			nil,
			errCustomFieldIDNotFound("github-last-sync"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setup(t)
			setupParseField(t)

			field, err := config2.parseFieldIDs(tt.jiraFields)

			assert.Equal(t, tt.expectedFields, field)
			assert.Equal(t, tt.expectedError, err)
		})
	}
}
