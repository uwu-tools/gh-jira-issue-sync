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

package cmd

import (
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"sigs.k8s.io/release-utils/version"

	"github.com/uwu-tools/gh-jira-issue-sync/config"
	"github.com/uwu-tools/gh-jira-issue-sync/github"
	"github.com/uwu-tools/gh-jira-issue-sync/jira"
	"github.com/uwu-tools/gh-jira-issue-sync/jira/issue"
)

// Execute provides a single function to run the root command and handle errors.
func Execute() {
	// Create a temporary logger that we can use if an error occurs before the real one is instantiated.
	log := logrus.New()
	if err := RootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

// RootCmd represents the command itself and configures it.
var RootCmd = &cobra.Command{
	Use:   "gh-jira-issue-sync [options]",
	Short: "A tool to synchronize GitHub and JIRA issues",
	Long:  "Full docs coming later; see https://github.com/uwu-tools/gh-jira-issue-sync",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.New(cmd)
		if err != nil {
			return fmt.Errorf("creating new config: %w", err)
		}

		log := cfg.GetLogger()

		jiraClient, err := jira.New(cfg)
		if err != nil {
			return fmt.Errorf("creating Jira client: %w", err)
		}
		ghClient, err := github.New(cfg)
		if err != nil {
			return fmt.Errorf("creating GitHub client: %w", err)
		}

		for {
			if err := issue.Compare(cfg, ghClient, jiraClient); err != nil {
				log.Error(err)
			}
			if !cfg.IsDryRun() {
				if err := cfg.SaveConfig(); err != nil {
					log.Error(err)
				}
			}
			if !cfg.IsDaemon() {
				return nil
			}
			<-time.After(cfg.GetDaemonPeriod())
		}
	},
}

func init() {
	// TODO(cmd): Parameterize default values
	RootCmd.PersistentFlags().String(
		"log-level",
		logrus.InfoLevel.String(),
		"Set the global log level",
	)

	RootCmd.PersistentFlags().String(
		"config",
		"",
		"Config file (default is $HOME/.issue-sync.json)",
	)

	RootCmd.PersistentFlags().StringP(
		"github-token",
		"t",
		"",
		"Set the API Token used to access the GitHub repo",
	)

	RootCmd.PersistentFlags().StringP(
		"jira-user",
		"u",
		"",
		"Set the JIRA username to authenticate with",
	)

	RootCmd.PersistentFlags().StringP(
		"jira-pass",
		"p",
		"",
		"Set the JIRA password to authenticate with",
	)

	RootCmd.PersistentFlags().StringP(
		"repo-name",
		"r",
		"",
		"Set the repository path (should be form owner/repo)",
	)

	RootCmd.PersistentFlags().StringP(
		"jira-uri",
		"U",
		"",
		"Set the base uri of the JIRA instance",
	)

	RootCmd.PersistentFlags().StringP(
		"jira-project",
		"P",
		"",
		"Set the key of the JIRA project",
	)

	RootCmd.PersistentFlags().StringP(
		"since",
		"s",
		"1970-01-01T00:00:00+0000",
		"Set the day that the update should run forward from",
	)

	RootCmd.PersistentFlags().BoolP(
		"dry-run",
		"d",
		false,
		"Print out actions to be taken, but do not execute them",
	)

	RootCmd.PersistentFlags().DurationP(
		"timeout",
		"T",
		time.Minute,
		"Set the maximum timeout on all API calls",
	)

	RootCmd.PersistentFlags().Duration(
		"period",
		1*time.Hour,
		"How often to synchronize; set to 0 for one-shot mode",
	)

	RootCmd.AddCommand(version.Version())
}
