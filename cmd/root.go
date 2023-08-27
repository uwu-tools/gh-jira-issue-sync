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
	"context"
	"fmt"
	"github.com/uwu-tools/gh-jira-issue-sync/internal/utils/issue"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"sigs.k8s.io/release-utils/log"
	"sigs.k8s.io/release-utils/version"

	"github.com/uwu-tools/gh-jira-issue-sync/internal/config"
	"github.com/uwu-tools/gh-jira-issue-sync/internal/github"
	"github.com/uwu-tools/gh-jira-issue-sync/internal/jira"
	"github.com/uwu-tools/gh-jira-issue-sync/internal/options"
)

var opts = &options.Options{}

// Execute provides a single function to run the root command and handle errors.
func Execute() {
	if err := RootCmd.Execute(); err != nil {
		logrus.Fatal(err)
	}
}

// RootCmd represents the command itself and configures it.
var RootCmd = &cobra.Command{
	Use:               fmt.Sprintf("%s [options]", options.AppName),
	Short:             "A tool to synchronize GitHub and Jira issues",
	Long:              "Full docs coming later; see https://github.com/uwu-tools/gh-jira-issue-sync",
	PersistentPreRunE: initLogging,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		cfg, err := config.New(ctx, cmd)
		if err != nil {
			return fmt.Errorf("creating new config: %w", err)
		}

		jiraClient, err := jira.New(cfg)
		if err != nil {
			return fmt.Errorf("creating Jira client: %w", err)
		}

		ghToken := cfg.GetConfigString(options.ConfigKeyGitHubToken)
		ghClient, err := github.New(ghToken)
		if err != nil {
			return fmt.Errorf("creating GitHub client: %w", err)
		}

		for {
			if err := issue.Reconcile(cfg, ghClient, jiraClient); err != nil {
				// TODO(log): Better error message
				logrus.Error(err)
			}
			if !cfg.IsDryRun() {
				if err := cfg.SaveConfig(); err != nil {
					// TODO(log): Better error message
					logrus.Error(err)
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
	RootCmd.PersistentFlags().StringVar(
		&opts.LogLevel,
		options.ConfigKeyLogLevel,
		options.DefaultLogLevelStr,
		fmt.Sprintf("the logging verbosity, either %s", log.LevelNames()),
	)

	RootCmd.PersistentFlags().StringVar(
		&opts.ConfigFile,
		options.ConfigKeyConfigFile,
		"",
		"viper config file location",
	)

	RootCmd.PersistentFlags().StringVarP(
		&opts.GitHubToken,
		options.ConfigKeyGitHubToken,
		"t",
		"",
		"set the API token used to access the GitHub repo",
	)

	RootCmd.PersistentFlags().StringVarP(
		&opts.JiraUser,
		options.ConfigKeyJiraUser,
		"u",
		"",
		"set the Jira username to authenticate with",
	)

	RootCmd.PersistentFlags().StringVarP(
		&opts.JiraPassword,
		options.ConfigKeyJiraPassword,
		"p",
		"",
		"set the Jira password to authenticate with",
	)

	RootCmd.PersistentFlags().StringVarP(
		&opts.RepoName,
		options.ConfigKeyRepoName,
		"r",
		"",
		"set the repository path (should be form owner/repo)",
	)

	RootCmd.PersistentFlags().StringVarP(
		&opts.JiraURI,
		options.ConfigKeyJiraURI,
		"U",
		"",
		"set the base URI of the Jira instance",
	)

	RootCmd.PersistentFlags().StringVarP(
		&opts.JiraProject,
		options.ConfigKeyJiraProject,
		"P",
		"",
		"set the key of the Jira project",
	)

	RootCmd.PersistentFlags().StringVarP(
		&opts.Since,
		options.ConfigKeySince,
		"s",
		options.DefaultSince,
		"set the day that the update should run forward from",
	)

	RootCmd.PersistentFlags().BoolVarP(
		&opts.Confirm,
		options.ConfigKeyConfirm,
		"c",
		options.DefaultConfirm,
		"if set to true, all actions will be executed, otherwise they are just printed out (dry run)",
	)

	RootCmd.PersistentFlags().DurationVarP(
		&opts.Timeout,
		options.ConfigKeyTimeout,
		"T",
		options.DefaultTimeout,
		"set the maximum timeout on all API calls",
	)

	RootCmd.PersistentFlags().DurationVar(
		&opts.Period,
		options.ConfigKeyPeriod,
		options.DefaultPeriod,
		"how often to synchronize; set to 0 for one-shot mode",
	)

	RootCmd.AddCommand(version.Version())
}

func initLogging(*cobra.Command, []string) error {
	err := log.SetupGlobalLogger(opts.LogLevel)
	if err != nil {
		return fmt.Errorf("setting up global logger: %w", err)
	}
	return nil
}
