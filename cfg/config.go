package cfg

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/andygrunwald/go-jira"
	"github.com/dghubble/oauth1"
	"github.com/fsnotify/fsnotify"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"golang.org/x/term"
)

// dateFormat is the format used for the `since` configuration parameter.
const dateFormat = "2006-01-02T15:04:05-0700"

// defaultLogLevel is the level logrus should default to if the configured option can't be parsed.
const defaultLogLevel = logrus.InfoLevel

// fieldKey is an enum-like type to represent the customfield ID keys.
type fieldKey int

const (
	GitHubID       fieldKey = iota
	GitHubNumber   fieldKey = iota
	GitHubLabels   fieldKey = iota
	GitHubStatus   fieldKey = iota
	GitHubReporter fieldKey = iota
	LastISUpdate   fieldKey = iota
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
type Config struct {
	// cmdFile is the file Viper is using for its configuration (default $HOME/.issue-sync.json).
	cmdFile string
	// cmdConfig is the Viper configuration object created from the command line and config file.
	cmdConfig viper.Viper

	// log is a logger set up with the configured log level, app name, etc.
	log logrus.Entry

	// basicAuth represents whether we're using HTTP Basic authentication or OAuth.
	basicAuth bool

	// fieldIDs is the list of custom fields we pulled from the `fields` JIRA endpoint.
	fieldIDs fields

	// project represents the JIRA project the user has requested.
	project jira.Project

	// since is the parsed value of the `since` configuration parameter, which is the earliest that
	// a GitHub issue can have been updated to be retrieved.
	since time.Time
}

// NewConfig creates a new, immutable configuration object. This object
// holds the Viper configuration and the logger, and is validated. The
// JIRA configuration is not yet initialized.
func NewConfig(cmd *cobra.Command) (Config, error) {
	config := Config{}

	var err error
	config.cmdFile, err = cmd.Flags().GetString("config")
	if err != nil {
		config.cmdFile = ""
	}

	config.cmdConfig = *newViper("issue-sync", config.cmdFile)
	config.cmdConfig.BindPFlags(cmd.Flags())

	config.cmdFile = config.cmdConfig.ConfigFileUsed()

	config.log = *newLogger("issue-sync", config.cmdConfig.GetString("log-level"))

	if err := config.validateConfig(); err != nil {
		return Config{}, err
	}

	return config, nil
}

// LoadJIRAConfig loads the JIRA configuration (project key,
// custom field IDs) from a remote JIRA server.
func (c *Config) LoadJIRAConfig(client jira.Client) error {
	proj, res, err := client.Project.Get(c.cmdConfig.GetString("jira-project"))
	if err != nil {
		c.log.Errorf("error retrieving JIRA project; check key and credentials. Error: %s", err)
		defer res.Body.Close()
		body, err := io.ReadAll(res.Body)
		if err != nil {
			c.log.Errorf("error occurred trying to read error body: %s", err)
			return err
		}

		c.log.Debugf("Error body: %s", body)
		return errors.New(string(body))
	}
	c.project = *proj

	c.fieldIDs, err = c.getFieldIDs(client)
	if err != nil {
		return err
	}

	return nil
}

// GetConfigFile returns the file that Viper loaded the configuration from.
func (c Config) GetConfigFile() string {
	return c.cmdFile
}

// GetConfigString returns a string value from the Viper configuration.
func (c Config) GetConfigString(key string) string {
	return c.cmdConfig.GetString(key)
}

// IsBasicAuth is true if we're using HTTP Basic Authentication, and false if
// we're using OAuth.
func (c Config) IsBasicAuth() bool {
	return c.basicAuth
}

// GetSinceParam returns the `since` configuration parameter, parsed as a time.Time.
func (c Config) GetSinceParam() time.Time {
	return c.since
}

// GetLogger returns the configured application logger.
func (c Config) GetLogger() logrus.Entry {
	return c.log
}

// IsDryRun returns whether the application is running in dry-run mode or not.
func (c Config) IsDryRun() bool {
	return c.cmdConfig.GetBool("dry-run")
}

// IsDaemon returns whether the application is running as a daemon.
func (c Config) IsDaemon() bool {
	return c.cmdConfig.GetDuration("period") != 0
}

// GetDaemonPeriod returns the period on which the tool runs if in daemon mode.
func (c Config) GetDaemonPeriod() time.Duration {
	return c.cmdConfig.GetDuration("period")
}

// GetTimeout returns the configured timeout on all API calls, parsed as a time.Duration.
func (c Config) GetTimeout() time.Duration {
	return c.cmdConfig.GetDuration("timeout")
}

// GetFieldID returns the customfield ID of a JIRA custom field.
func (c Config) GetFieldID(key fieldKey) string {
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
func (c Config) GetFieldKey(key fieldKey) string {
	return fmt.Sprintf("customfield_%s", c.GetFieldID(key))
}

// GetProject returns the JIRA project the user has configured.
func (c Config) GetProject() jira.Project {
	return c.project
}

// GetProjectKey returns the JIRA key of the configured project.
func (c Config) GetProjectKey() string {
	return c.project.Key
}

// GetRepo returns the user/org name and the repo name of the configured GitHub repository.
func (c Config) GetRepo() (string, string) {
	fullName := c.cmdConfig.GetString("repo-name")
	parts := strings.Split(fullName, "/")
	// We check that repo-name is two parts separated by a slash in NewConfig, so this is safe
	return parts[0], parts[1]
}

// SetJIRAToken adds the JIRA OAuth tokens in the Viper configuration, ensuring that they
// are saved for future runs.
func (c Config) SetJIRAToken(token *oauth1.Token) {
	c.cmdConfig.Set("jira-token", token.Token)
	c.cmdConfig.Set("jira-secret", token.TokenSecret)
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
	c.cmdConfig.Set("since", time.Now().Format(dateFormat))

	var cf configFile
	c.cmdConfig.Unmarshal(&cf)

	b, err := json.MarshalIndent(cf, "", "  ")
	if err != nil {
		return err
	}

	f, err := os.OpenFile(c.cmdConfig.ConfigFileUsed(), os.O_RDWR|os.O_TRUNC|os.O_CREATE, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()

	f.WriteString(string(b))

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
	} else {
		if cfgFile != "" {
			log.WithError(err).Warningf("Error reading config file: %v", cfgFile)
		}
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
		return defaultLogLevel
	}

	ll, err := logrus.ParseLevel(level)
	if err != nil {
		fmt.Printf("Failed to parse log level, using default. Error: %v\n", err)
		return defaultLogLevel
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
	token := c.cmdConfig.GetString("github-token")
	if token == "" {
		return errors.New("GitHub token required")
	}

	c.basicAuth = (c.cmdConfig.GetString("jira-user") != "") && (c.cmdConfig.GetString("jira-pass") != "")

	if c.basicAuth {
		c.log.Debug("Using HTTP Basic Authentication")

		jUser := c.cmdConfig.GetString("jira-user")
		if jUser == "" {
			return errors.New("jira username required")
		}

		jPass := c.cmdConfig.GetString("jira-pass")
		if jPass == "" {
			fmt.Print("Enter your Jira password: ")
			bytePass, err := term.ReadPassword(syscall.Stdin)
			if err != nil {
				return errors.New("jira password required")
			}
			fmt.Println()
			c.cmdConfig.Set("jira-pass", string(bytePass))
		}
	} else {
		c.log.Debug("Using OAuth 1.0a authentication")

		token := c.cmdConfig.GetString("jira-token")
		if token == "" {
			return errors.New("jira access token required")
		}

		secret := c.cmdConfig.GetString("jira-secret")
		if secret == "" {
			return errors.New("jira access token secret required")
		}

		consumerKey := c.cmdConfig.GetString("jira-consumer-key")
		if consumerKey == "" {
			return errors.New("JIRA consumer key required for OAuth handshake")
		}

		privateKey := c.cmdConfig.GetString("jira-private-key-path")
		if privateKey == "" {
			return errors.New("JIRA private key required for OAuth handshake")
		}

		_, err := os.Open(privateKey)
		if err != nil {
			return errors.New("JIRA private key must point to existing PEM file")
		}
	}

	repo := c.cmdConfig.GetString("repo-name")
	if repo == "" {
		return errors.New("GitHub repository required")
	}
	if !strings.Contains(repo, "/") || len(strings.Split(repo, "/")) != 2 {
		return errors.New("GitHub repository must be of form user/repo")
	}

	uri := c.cmdConfig.GetString("jira-uri")
	if uri == "" {
		return errors.New("JIRA URI required")
	}
	if _, err := url.ParseRequestURI(uri); err != nil {
		return errors.New("JIRA URI must be valid URI")
	}

	project := c.cmdConfig.GetString("jira-project")
	if project == "" {
		return errors.New("JIRA project required")
	}

	sinceStr := c.cmdConfig.GetString("since")
	if sinceStr == "" {
		c.cmdConfig.Set("since", "1970-01-01T00:00:00+0000")
	}

	since, err := time.Parse(dateFormat, sinceStr)
	if err != nil {
		return errors.New("Since date must be in ISO-8601 format")
	}
	c.since = since

	c.log.Debug("All config variables are valid!")

	return nil
}

// jiraField represents field metadata in JIRA. For an example of its
// structure, make a request to `${jira-uri}/rest/api/2/field`.
type jiraField struct {
	ID          string   `json:"id"`
	Key         string   `json:"key"`
	Name        string   `json:"name"`
	Custom      bool     `json:"custom"`
	Orderable   bool     `json:"orderable"`
	Navigable   bool     `json:"navigable"`
	Searchable  bool     `json:"searchable"`
	ClauseNames []string `json:"clauseNames"`
	Schema      struct {
		Type     string `json:"type"`
		System   string `json:"system,omitempty"`
		Items    string `json:"items,omitempty"`
		Custom   string `json:"custom,omitempty"`
		CustomID int    `json:"customId,omitempty"`
	} `json:"schema,omitempty"`
}

// getFieldIDs requests the metadata of every issue field in the JIRA
// project, and saves the IDs of the custom fields used by issue-sync.
func (c Config) getFieldIDs(client jira.Client) (fields, error) {
	c.log.Debug("Collecting field IDs.")
	req, err := client.NewRequest("GET", "/rest/api/2/field", nil)
	if err != nil {
		return fields{}, err
	}
	jFields := new([]jiraField)

	_, err = client.Do(req, jFields)
	if err != nil {
		return fields{}, err
	}

	fieldIDs := fields{}

	for _, field := range *jFields {
		switch field.Name {
		case "GitHub ID":
			fieldIDs.githubID = fmt.Sprint(field.Schema.CustomID)
		case "GitHub Number":
			fieldIDs.githubNumber = fmt.Sprint(field.Schema.CustomID)
		case "GitHub Labels":
			fieldIDs.githubLabels = fmt.Sprint(field.Schema.CustomID)
		case "GitHub Status":
			fieldIDs.githubStatus = fmt.Sprint(field.Schema.CustomID)
		case "GitHub Reporter":
			fieldIDs.githubReporter = fmt.Sprint(field.Schema.CustomID)
		case "Last Issue-Sync Update":
			fieldIDs.lastUpdate = fmt.Sprint(field.Schema.CustomID)
		}
	}

	if fieldIDs.githubID == "" {
		return fieldIDs, errors.New("could not find ID of 'GitHub ID' custom field; check that it is named correctly")
	} else if fieldIDs.githubNumber == "" {
		return fieldIDs, errors.New("could not find ID of 'GitHub Number' custom field; check that it is named correctly")
	} else if fieldIDs.githubLabels == "" {
		return fieldIDs, errors.New("could not find ID of 'Github Labels' custom field; check that it is named correctly")
	} else if fieldIDs.githubStatus == "" {
		return fieldIDs, errors.New("could not find ID of 'Github Status' custom field; check that it is named correctly")
	} else if fieldIDs.githubReporter == "" {
		return fieldIDs, errors.New("could not find ID of 'Github Reporter' custom field; check that it is named correctly")
	} else if fieldIDs.lastUpdate == "" {
		return fieldIDs, errors.New("could not find ID of 'Last Issue-Sync Update' custom field; check that it is named correctly")
	}

	c.log.Debug("All fields have been checked.")

	return fieldIDs, nil
}
