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
	// Flag / Viper field names.
	ConfigKeyLogLevel     = "log-level"
	ConfigKeyConfigFile   = "config"
	ConfigKeyGitHubToken  = "github-token"
	ConfigKeyJiraUser     = "jira-user"
	ConfigKeyJiraPassword = "jira-pass"
	ConfigKeyRepoName     = "repo-name"
	ConfigKeyJiraURI      = "jira-uri"
	ConfigKeyJiraProject  = "jira-project"
	ConfigKeySince        = "since"
	ConfigKeyDryRun       = "dry-run"
	ConfigKeyTimeout      = "timeout"
	ConfigKeyPeriod       = "period"

	// Default values.
	DefaultConfigFile = "$HOME/.issue-sync.json"
	DefaultSince      = "1970-01-01T00:00:00+0000"
	DefaultDryRun     = false
	DefaultTimeout    = time.Minute
	DefaultPeriod     = time.Hour
)

var DefaultLogLevel = logrus.InfoLevel.String()
