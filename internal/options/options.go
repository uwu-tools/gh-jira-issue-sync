// Copyright 2022 uwu-tools Authors
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

package options

import (
	"time"

	"github.com/sirupsen/logrus"
)

type Options struct {
	LogLevel     string
	ConfigFile   string
	GitHubToken  string
	JiraUser     string
	JiraPassword string
	RepoName     string
	JiraURI      string
	JiraProject  string
	// TODO(options): Should this be a time type?
	Since   string
	DryRun  bool
	Timeout time.Duration
	Period  time.Duration
}

const (
	AppName = "gh-jira-issue-sync"

	// DateFormat is the format used for the `since` configuration parameter.
	DateFormat = "2006-01-02T15:04:05-0700"

	// Application config keys.
	ConfigKeyLogLevel   = "log-level"
	ConfigKeyConfigFile = "config"
	ConfigKeySince      = "since"
	ConfigKeyDryRun     = "dry-run"
	ConfigKeyPeriod     = "period"
	ConfigKeyTimeout    = "timeout"

	// GitHub config keys.
	ConfigKeyRepoName    = "repo-name"
	ConfigKeyGitHubToken = "github-token"

	// Jira config keys.
	ConfigKeyJiraURI            = "jira-uri"
	ConfigKeyJiraProject        = "jira-project"
	ConfigKeyJiraUser           = "jira-user"
	ConfigKeyJiraPassword       = "jira-pass"
	ConfigKeyJiraToken          = "jira-token"
	ConfigKeyJiraSecret         = "jira-secret"
	ConfigKeyJiraConsumerKey    = "jira-consumer-key"
	ConfigKeyJiraPrivateKeyPath = "jira-private-key-path"

	// Default values
	//
	// DefaultLogLevel is the level logrus should default to if the configured
	// option can't be parsed.
	DefaultLogLevel   = logrus.InfoLevel
	DefaultConfigFile = "$HOME/.issue-sync.json"
	DefaultSince      = "1970-01-01T00:00:00+0000"
	DefaultDryRun     = false
	DefaultPeriod     = time.Hour
	DefaultTimeout    = 30 * time.Second
)

var DefaultLogLevelStr = DefaultLogLevel.String()
