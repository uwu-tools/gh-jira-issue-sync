// Copyright 2017 CoreOS, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"
	"syscall"
	"time"

	jira "github.com/andygrunwald/go-jira/v2/cloud"
	"github.com/dghubble/oauth1"
	"github.com/fsnotify/fsnotify"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"golang.org/x/term"

	"github.com/uwu-tools/gh-jira-issue-sync/internal/options"
)

// fieldKey is an enum-like type to represent the customfield ID keys.
type fieldKey int

const (
	GitHubID       fieldKey = iota
	GitHubNumber   fieldKey = iota
	GitHubLabels   fieldKey = iota
	GitHubStatus   fieldKey = iota
	GitHubReporter fieldKey = iota
	LastISUpdate   fieldKey = iota

	// Custom field names.
	CustomFieldNameGitHubID       = "GitHub ID"
	CustomFieldNameGitHubNumber   = "GitHub Number"
	CustomFieldNameGitHubLabels   = "GitHub Labels"
	CustomFieldNameGitHubStatus   = "GitHub Status"
	CustomFieldNameGitHubReporter = "GitHub Reporter"
	CustomFieldNameGitHubLastSync = "Last Issue-Sync Update"
)

// fields represents the custom field IDs of the JIRA custom fields we care about.
type fields struct {
	githubID       string
	githubNumber   string
	githubLabels   string
	githubReporter string
	githubStatus   string
	lastUpdate     string
}

// Config is the root configuration object the application creates.
//
//nolint:govet
type Config struct {
	// cmdFile is the file Viper is using for its configuration (default $HOME/.issue-sync.json).
	cmdFile string

	// cmdConfig is the Viper configuration object created from the command line and config file.
	cmdConfig viper.Viper

	// ctx carries a deadline, a cancellation signal, and other values across
	// API boundaries.
	ctx context.Context

	// log is a logger set up with the configured log level, app name, etc.
	log logrus.Entry

	// basicAuth represents whether we're using HTTP Basic authentication or OAuth.
	basicAuth bool

	// fieldIDs is the list of custom fields we pulled from the `fields` JIRA endpoint.
	fieldIDs *fields

	// project represents the JIRA project the user has requested.
	project *jira.Project

	// since is the parsed value of the `since` configuration parameter, which is the earliest that
	// a GitHub issue can have been updated to be retrieved.
	since time.Time
}

// New creates a new, immutable configuration object. This object
// holds the Viper configuration and the logger, and is validated. The
// JIRA configuration is not yet initialized.
func New(ctx context.Context, cmd *cobra.Command) (*Config, error) {
	var cfg Config

	var err error
	cfg.cmdFile, err = cmd.Flags().GetString(options.ConfigKeyConfigFile)
	if err != nil {
		cfg.cmdFile = ""
	}

	cfg.cmdConfig = *newViper(options.AppName, cfg.cmdFile)
	cfg.cmdConfig.BindPFlags(cmd.Flags()) //nolint:errcheck

	cfg.cmdFile = cfg.cmdConfig.ConfigFileUsed()

	cfg.ctx = ctx

	cfg.log = *newLogger(
		options.AppName,
		cfg.cmdConfig.GetString(options.ConfigKeyLogLevel),
	)

	if err := cfg.validateConfig(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// LoadJIRAConfig loads the JIRA configuration (project key,
// custom field IDs) from a remote JIRA server.
func (c *Config) LoadJIRAConfig(client *jira.Client) error {
	proj, res, err := client.Project.Get(
		c.Context(),
		c.cmdConfig.GetString(options.ConfigKeyJiraProject),
	)
	if err != nil {
		c.log.Errorf("error retrieving JIRA project; check key and credentials. Error: %s", err)
		defer res.Body.Close()
		body, err := io.ReadAll(res.Body)
		if err != nil {
			c.log.Errorf("error occurred trying to read error body: %s", err)
			return fmt.Errorf("reading Jira project: %w", err)
		}

		c.log.Debugf("Error body: %s", body)
		return fmt.Errorf("reading error body: %s", string(body)) //nolint:goerr113
	}
	c.project = proj

	c.fieldIDs, err = c.getFieldIDs(client)
	if err != nil {
		return err
	}

	return nil
}

// Context returns the context.
func (c *Config) Context() context.Context {
	return c.ctx
}

// GetConfigFile returns the file that Viper loaded the configuration from.
func (c *Config) GetConfigFile() string {
	return c.cmdFile
}

// GetConfigString returns a string value from the Viper configuration.
func (c *Config) GetConfigString(key string) string {
	return c.cmdConfig.GetString(key)
}

// IsBasicAuth is true if we're using HTTP Basic Authentication, and false if
// we're using OAuth.
func (c *Config) IsBasicAuth() bool {
	return c.basicAuth
}

// GetSinceParam returns the `since` configuration parameter, parsed as a time.Time.
func (c *Config) GetSinceParam() time.Time {
	return c.since
}

// GetLogger returns the configured application logger.
func (c *Config) GetLogger() logrus.Entry {
	return c.log
}

// IsDryRun returns whether the application is running in dry-run mode or not.
func (c *Config) IsDryRun() bool {
	return c.cmdConfig.GetBool(options.ConfigKeyDryRun)
}

// IsDaemon returns whether the application is running as a daemon.
func (c *Config) IsDaemon() bool {
	return c.cmdConfig.GetDuration(options.ConfigKeyPeriod) != 0
}

// GetDaemonPeriod returns the period on which the tool runs if in daemon mode.
func (c *Config) GetDaemonPeriod() time.Duration {
	return c.cmdConfig.GetDuration(options.ConfigKeyPeriod)
}

// GetTimeout returns the configured timeout on all API calls, parsed as a time.Duration.
func (c *Config) GetTimeout() time.Duration {
	return c.cmdConfig.GetDuration(options.ConfigKeyTimeout)
}

// GetFieldID returns the customfield ID of a JIRA custom field.
func (c *Config) GetFieldID(key fieldKey) string {
	switch key {
	case GitHubID:
		return c.fieldIDs.githubID
	case GitHubNumber:
		return c.fieldIDs.githubNumber
	case GitHubLabels:
		return c.fieldIDs.githubLabels
	case GitHubReporter:
		return c.fieldIDs.githubReporter
	case GitHubStatus:
		return c.fieldIDs.githubStatus
	case LastISUpdate:
		return c.fieldIDs.lastUpdate
	default:
		return ""
	}
}

// GetFieldKey returns customfield_XXXXX, where XXXXX is the custom field ID (see GetFieldID).
func (c *Config) GetFieldKey(key fieldKey) string {
	return fmt.Sprintf("customfield_%s", c.GetFieldID(key))
}

// GetProject returns the JIRA project the user has configured.
func (c *Config) GetProject() *jira.Project {
	return c.project
}

// GetProjectKey returns the JIRA key of the configured project.
func (c *Config) GetProjectKey() string {
	return c.project.Key
}

// GetRepo returns the user/org name and the repo name of the configured GitHub repository.
func (c *Config) GetRepo() (string, string) {
	fullName := c.cmdConfig.GetString(options.ConfigKeyRepoName)
	parts := strings.Split(fullName, "/")
	// We check that repo-name is two parts separated by a slash in New, so this is safe
	return parts[0], parts[1]
}

// SetJIRAToken adds the JIRA OAuth tokens in the Viper configuration, ensuring that they
// are saved for future runs.
func (c *Config) SetJIRAToken(token *oauth1.Token) {
	c.cmdConfig.Set(options.ConfigKeyJiraToken, token.Token)
	c.cmdConfig.Set(options.ConfigKeyJiraSecret, token.TokenSecret)
}

// configFile is a serializable representation of the current Viper configuration.
type configFile struct {
	LogLevel    string        `json:"log-level" mapstructure:"log-level"`
	GithubToken string        `json:"github-token" mapstructure:"github-token"`
	JIRAUser    string        `json:"jira-user" mapstructure:"jira-user"`
	JIRAToken   string        `json:"jira-token" mapstructure:"jira-token"`
	JIRASecret  string        `json:"jira-secret" mapstructure:"jira-secret"`
	JIRAKey     string        `json:"jira-private-key-path" mapstructure:"jira-private-key-path"`
	JIRACKey    string        `json:"jira-consumer-key" mapstructure:"jira-consumer-key"`
	RepoName    string        `json:"repo-name" mapstructure:"repo-name"`
	JIRAURI     string        `json:"jira-uri" mapstructure:"jira-uri"`
	JIRAProject string        `json:"jira-project" mapstructure:"jira-project"`
	Since       string        `json:"since" mapstructure:"since"`
	Timeout     time.Duration `json:"timeout" mapstructure:"timeout"`
}

// SaveConfig updates the `since` parameter to now, then saves the configuration file.
func (c *Config) SaveConfig() error {
	c.cmdConfig.Set(
		options.ConfigKeySince,
		time.Now().Format(options.DateFormat),
	)

	var cf configFile
	if err := c.cmdConfig.Unmarshal(&cf); err != nil {
		return fmt.Errorf("unmarshalling config: %w", err)
	}

	b, err := json.MarshalIndent(cf, "", "  ")
	if err != nil {
		return fmt.Errorf("marshalling config: %w", err)
	}

	f, err := os.OpenFile(c.cmdConfig.ConfigFileUsed(), os.O_RDWR|os.O_TRUNC|os.O_CREATE, 0o644)
	if err != nil {
		return fmt.Errorf("opening config file %s: %w", c.cmdConfig.ConfigFileUsed(), err)
	}
	defer f.Close()

	_, err = f.WriteString(string(b))
	if err != nil {
		return fmt.Errorf("writing config: %w", err)
	}

	return nil
}

// newViper generates a viper configuration object which
// merges (in order from highest to lowest priority) the
// command line options, configuration file options, and
// default configuration values. This viper object becomes
// the single source of truth for the app configuration.
func newViper(appName, cfgFile string) *viper.Viper {
	log := logrus.New()
	v := viper.New()

	v.SetEnvPrefix(appName)
	v.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	v.AutomaticEnv()

	v.SetConfigName(fmt.Sprintf("config-%s", appName))
	v.AddConfigPath(".")
	if cfgFile != "" {
		v.SetConfigFile(cfgFile)
	}
	v.SetConfigType("json")

	if err := v.ReadInConfig(); err == nil {
		log.WithField("file", v.ConfigFileUsed()).Infof("config file loaded")
		v.WatchConfig()
		v.OnConfigChange(func(e fsnotify.Event) {
			log.WithField("file", e.Name).Info("config file changed")
		})
	} else if cfgFile != "" {
		log.WithError(err).Warningf("Error reading config file: %v", cfgFile)
	}

	if log.Level == logrus.DebugLevel {
		v.Debug()
	}

	return v
}

// parseLogLevel is a helper function to parse the log level passed in the
// configuration into a logrus Level, or to use the default log level set
// above if the log level can't be parsed.
func parseLogLevel(level string) logrus.Level {
	if level == "" {
		return options.DefaultLogLevel
	}

	ll, err := logrus.ParseLevel(level)
	if err != nil {
		fmt.Printf("Failed to parse log level, using default. Error: %v\n", err)
		return options.DefaultLogLevel
	}
	return ll
}

// newLogger uses the log level provided in the configuration
// to create a new logrus logger and set fields on it to make
// it easy to use.
func newLogger(app, level string) *logrus.Entry {
	logger := logrus.New()
	logger.Level = parseLogLevel(level)
	logEntry := logrus.NewEntry(logger).WithFields(logrus.Fields{
		"app": app,
	})
	logEntry.WithField("log-level", logger.Level).Info("log level set")
	return logEntry
}

// validateConfig checks the values provided to all of the configuration
// options, ensuring that e.g. `since` is a valid date, `jira-uri` is a
// real URI, etc. This is the first level of checking. It does not confirm
// if a JIRA cli is running at `jira-uri` for example; that is checked
// in getJIRAClient when we actually make a call to the API.
func (c *Config) validateConfig() error {
	// Log level and config file location are validated already

	c.log.Debug("Checking config variables...")
	token := c.cmdConfig.GetString(options.ConfigKeyGitHubToken)
	if token == "" {
		return errGitHubTokenRequired
	}

	c.basicAuth = (c.cmdConfig.GetString(options.ConfigKeyJiraUser) != "") &&
		(c.cmdConfig.GetString(options.ConfigKeyJiraPassword) != "")

	if c.basicAuth { //nolint:nestif // TODO(lint)
		c.log.Debug("Using HTTP Basic Authentication")

		jUser := c.cmdConfig.GetString(options.ConfigKeyJiraUser)
		if jUser == "" {
			return errJiraUsernameRequired
		}

		jPass := c.cmdConfig.GetString(options.ConfigKeyJiraPassword)
		if jPass == "" {
			fmt.Print("Enter your Jira password: ")
			bytePass, err := term.ReadPassword(syscall.Stdin)
			if err != nil {
				return errJiraPasswordRequired
			}
			fmt.Println()
			c.cmdConfig.Set(options.ConfigKeyJiraPassword, string(bytePass))
		}
	} else {
		c.log.Debug("Using OAuth 1.0a authentication")

		token := c.cmdConfig.GetString(options.ConfigKeyJiraToken)
		if token == "" {
			return errJiraAccessTokenRequired
		}

		secret := c.cmdConfig.GetString(options.ConfigKeyJiraSecret)
		if secret == "" {
			return errJiraAccessTokenSecretRequired
		}

		consumerKey := c.cmdConfig.GetString(options.ConfigKeyJiraConsumerKey)
		if consumerKey == "" {
			return errJiraConsumerKeyRequired
		}

		privateKey := c.cmdConfig.GetString(options.ConfigKeyJiraPrivateKeyPath)
		if privateKey == "" {
			return errJiraPrivateKeyRequired
		}

		_, err := os.Open(privateKey)
		if err != nil {
			return errJiraPEMFileInvalid
		}
	}

	repo := c.cmdConfig.GetString(options.ConfigKeyRepoName)
	if repo == "" {
		return errGitHubRepoRequired
	}
	if !strings.Contains(repo, "/") || len(strings.Split(repo, "/")) != 2 {
		return errGitHubRepoFormatInvalid
	}

	uri := c.cmdConfig.GetString(options.ConfigKeyJiraURI)
	if uri == "" {
		return errJiraURIRequired
	}
	if _, err := url.ParseRequestURI(uri); err != nil {
		return errJiraURIInvalid
	}

	project := c.cmdConfig.GetString(options.ConfigKeyJiraProject)
	if project == "" {
		return errJiraProjectRequired
	}

	sinceStr := c.cmdConfig.GetString(options.ConfigKeySince)
	if sinceStr == "" {
		c.cmdConfig.Set(options.ConfigKeySince, options.DefaultSince)
	}

	since, err := time.Parse(options.DateFormat, sinceStr)
	if err != nil {
		return errDateInvalid
	}
	c.since = since

	c.log.Debug("All config variables are valid!")

	return nil
}

// getFieldIDs requests the metadata of every issue field in the JIRA
// project, and saves the IDs of the custom fields used by issue-sync.
func (c *Config) getFieldIDs(client *jira.Client) (*fields, error) {
	c.log.Debug("Collecting field IDs.")
	req, err := client.NewRequest(c.Context(), "GET", "/rest/api/2/field", nil)
	if err != nil {
		return nil, fmt.Errorf("getting fields: %w", err)
	}

	jFieldsPtr := new([]jira.Field)
	_, err = client.Do(req, jFieldsPtr)
	if err != nil {
		return nil, fmt.Errorf("getting field IDs: %w", err)
	}

	jFields := *jFieldsPtr
	var fieldIDs fields

	for i := range jFields {
		field := jFields[i]
		switch field.Name {
		case CustomFieldNameGitHubID:
			fieldIDs.githubID = fmt.Sprint(field.Schema.CustomID)
		case CustomFieldNameGitHubNumber:
			fieldIDs.githubNumber = fmt.Sprint(field.Schema.CustomID)
		case CustomFieldNameGitHubLabels:
			fieldIDs.githubLabels = fmt.Sprint(field.Schema.CustomID)
		case CustomFieldNameGitHubStatus:
			fieldIDs.githubStatus = fmt.Sprint(field.Schema.CustomID)
		case CustomFieldNameGitHubReporter:
			fieldIDs.githubReporter = fmt.Sprint(field.Schema.CustomID)
		case CustomFieldNameGitHubLastSync:
			fieldIDs.lastUpdate = fmt.Sprint(field.Schema.CustomID)
		}
	}

	if fieldIDs.githubID == "" {
		return nil, errCustomFieldIDNotFound(CustomFieldNameGitHubID)
	}
	if fieldIDs.githubNumber == "" {
		return nil, errCustomFieldIDNotFound(CustomFieldNameGitHubNumber)
	}
	if fieldIDs.githubLabels == "" {
		return nil, errCustomFieldIDNotFound(CustomFieldNameGitHubLabels)
	}
	if fieldIDs.githubStatus == "" {
		return nil, errCustomFieldIDNotFound(CustomFieldNameGitHubStatus)
	}
	if fieldIDs.githubReporter == "" {
		return nil, errCustomFieldIDNotFound(CustomFieldNameGitHubReporter)
	}
	if fieldIDs.lastUpdate == "" {
		return nil, errCustomFieldIDNotFound(CustomFieldNameGitHubLastSync)
	}

	c.log.Debug("All fields have been checked.")

	return &fieldIDs, nil
}

// Errors

var (
	errGitHubTokenRequired           = errors.New("github token required")
	errJiraUsernameRequired          = errors.New("jira username required")
	errJiraPasswordRequired          = errors.New("jira password required")
	errJiraAccessTokenRequired       = errors.New("jira access token required")
	errJiraAccessTokenSecretRequired = errors.New("jira access token secret required")
	errJiraConsumerKeyRequired       = errors.New("jira consumer key required for OAuth handshake")
	errJiraPrivateKeyRequired        = errors.New("jira private key required for OAuth handshake")
	errJiraPEMFileInvalid            = errors.New("jira private key must point to existing PEM file")
	errGitHubRepoRequired            = errors.New("github repository required")
	errGitHubRepoFormatInvalid       = errors.New("github repository must be of form user/repo")
	errJiraURIRequired               = errors.New("jira URI required")
	errJiraURIInvalid                = errors.New("jira URI must be valid URI")
	errJiraProjectRequired           = errors.New("jira project required")
	errDateInvalid                   = errors.New("`since` date must be in ISO-8601 format")
)

func errCustomFieldIDNotFound(field string) error {
	return fmt.Errorf("could not find ID custom field '%s'; check that it is named correctly", field) //nolint:goerr113
}
