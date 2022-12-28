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

package issue

import (
	"fmt"
	"strings"
	"time"

	gojira "github.com/andygrunwald/go-jira/v2/cloud"
	gh "github.com/google/go-github/v48/github"
	"github.com/trivago/tgo/tcontainer"

	"github.com/uwu-tools/gh-jira-issue-sync/config"
	"github.com/uwu-tools/gh-jira-issue-sync/github"
	"github.com/uwu-tools/gh-jira-issue-sync/jira"
	"github.com/uwu-tools/gh-jira-issue-sync/jira/comment"
)

// dateFormat is the format used for the Last IS Update field.
const dateFormat = "2006-01-02T15:04:05.0-0700"

// Compare gets the list of GitHub issues updated since the `since` date,
// gets the list of JIRA issues which have GitHub ID custom fields in that list,
// then matches each one. If a JIRA issue already exists for a given GitHub issue,
// it calls UpdateIssue; if no JIRA issue already exists, it calls CreateIssue.
func Compare(cfg *config.Config, ghClient github.Client, jiraClient jira.Client) error {
	log := cfg.GetLogger()

	log.Debug("Collecting issues")

	ghIssues, err := ghClient.ListIssues()
	if err != nil {
		return fmt.Errorf("listing GitHub issues: %w", err)
	}

	if len(ghIssues) == 0 {
		log.Info("There are no GitHub issues; exiting")
		return nil
	}

	ids := make([]int, len(ghIssues))
	for i, v := range ghIssues {
		ghID := v.GetID()
		ids[i] = int(ghID)
	}

	jiraIssues, err := jiraClient.ListIssues(ids)
	if err != nil {
		return fmt.Errorf("listing Jira issues: %w", err)
	}

	log.Debugf("Jira issues found: %v", len(jiraIssues))
	log.Debug("Collected all JIRA issues")

	fieldKey := cfg.GetFieldKey(config.GitHubID)
	log.Debugf("GitHub ID custom field key: %s", fieldKey)

	// TODO(compare): Consider move ID comparison logic into separate function
	for _, ghIssue := range ghIssues {
		found := false

		ghID := *ghIssue.ID

		for i := range jiraIssues {
			jIssue := jiraIssues[i]

			// TODO(fields): Getting a field with Unknowns will generate a nil
			//               pointer exception if the custom field is not defined in
			//               JIRA.
			//               ref: https://github.com/andygrunwald/go-jira/issues/322
			unknowns := jIssue.Fields.Unknowns
			id, exists := unknowns.Value(fieldKey)
			if !exists {
				log.Info("GitHub ID custom field is not set for issue")
			}

			jiraID, ok := id.(float64)
			if !ok {
				log.Debugf("GitHub ID custom field is not an float64; got %T", id)
				break
			}

			ghIDFloat64 := float64(ghID)
			if jiraID == ghIDFloat64 {
				found = true

				log.Infof("updating issue %s", jIssue.ID)
				if err := UpdateIssue(cfg, ghIssue, &jIssue, ghClient, jiraClient); err != nil {
					log.Errorf("Error updating issue %s. Error: %v", jIssue.Key, err)
				}
				break
			}
		}
		if !found {
			if err := CreateIssue(cfg, ghIssue, ghClient, jiraClient); err != nil {
				log.Errorf("Error creating issue for #%d. Error: %v", *ghIssue.Number, err)
			}
		}
	}

	return nil
}

// DidIssueChange tests each of the relevant fields on the provided JIRA and GitHub issue
// and returns whether or not they differ.
//
//nolint:gocognit // TODO(lint)
func DidIssueChange(cfg *config.Config, ghIssue *gh.Issue, jIssue *gojira.Issue) bool {
	log := cfg.GetLogger()

	log.Debugf("Comparing GitHub issue #%d and JIRA issue %s", ghIssue.GetNumber(), jIssue.Key)

	anyDifferent := false

	anyDifferent = anyDifferent || (ghIssue.GetTitle() != jIssue.Fields.Summary)
	anyDifferent = anyDifferent || (ghIssue.GetBody() != jIssue.Fields.Description)

	key := cfg.GetFieldKey(config.GitHubStatus)
	field, err := jIssue.Fields.Unknowns.String(key)
	if err != nil || *ghIssue.State != field {
		anyDifferent = true
	}

	key = cfg.GetFieldKey(config.GitHubReporter)
	field, err = jIssue.Fields.Unknowns.String(key)
	if err != nil || *ghIssue.User.Login != field {
		anyDifferent = true
	}

	if len(ghIssue.Labels) > 0 { //nolint:nestif // TODO(lint)
		ghLabels := githubLabelsToStrSlice(ghIssue.Labels)

		key = cfg.GetFieldKey(config.GitHubLabels)
		labelsField, exists := jIssue.Fields.Unknowns.Value(key)
		if !exists {
			log.Debug("`GitHub Labels` field is not populated")
		}

		jiraLabels, _ := labelsField.([]string) //nolint:errcheck // TODO(lint)

		for _, label := range ghLabels {
			if !anyDifferent {
				found := false
				for i, jiraLabel := range jiraLabels {
					if i < len(jiraLabels) && !found {
						if label == jiraLabel {
							found = true
							break
						}
					} else {
						anyDifferent = true
						break
					}
				}
			}
		}
	}

	log.Debugf("Issues have any differences: %t", anyDifferent)

	return anyDifferent
}

// UpdateIssue compares each field of a GitHub issue to a JIRA issue; if any of them
// differ, the differing fields of the JIRA issue are updated to match the GitHub
// issue.
func UpdateIssue(
	cfg *config.Config,
	ghIssue *gh.Issue,
	jIssue *gojira.Issue,
	ghClient github.Client,
	jClient jira.Client,
) error {
	log := cfg.GetLogger()

	log.Debugf("Updating JIRA %s with GitHub #%d", jIssue.Key, *ghIssue.Number)

	if DidIssueChange(cfg, ghIssue, jIssue) {
		fields := &gojira.IssueFields{}
		fields.Unknowns = tcontainer.NewMarshalMap()

		fields.Summary = ghIssue.GetTitle()
		fields.Description = ghIssue.GetBody()
		fields.Unknowns.Set(cfg.GetFieldKey(config.GitHubStatus), ghIssue.GetState())

		// TODO: Do we actually need to update this? It's not possible to change a
		//       GitHub issue's reporter.
		fields.Unknowns.Set(cfg.GetFieldKey(config.GitHubReporter), ghIssue.User.GetLogin())

		labels := githubLabelsToStrSlice(ghIssue.Labels)
		fields.Unknowns.Set(cfg.GetFieldKey(config.GitHubLabels), labels)

		fields.Unknowns.Set(cfg.GetFieldKey(config.LastISUpdate), time.Now().Format(dateFormat))

		fields.Type = jIssue.Fields.Type

		issue := &gojira.Issue{
			Fields: fields,
			Key:    jIssue.Key,
			ID:     jIssue.ID,
		}

		_, err := jClient.UpdateIssue(issue)
		if err != nil {
			return fmt.Errorf("updating Jira issue: %w", err)
		}

		log.Debugf("Successfully updated JIRA issue %s!", jIssue.Key)
	} else {
		log.Debugf("JIRA issue %s is already up to date!", jIssue.Key)
	}

	foundIssue, err := jClient.GetIssue(jIssue.Key)
	if err != nil {
		return fmt.Errorf("getting Jira issue %s: %w", jIssue.Key, err)
	}

	if err := comment.Compare(cfg, ghIssue, foundIssue, ghClient, jClient); err != nil {
		return fmt.Errorf("comparing comments for issue %s: %w", jIssue.Key, err)
	}

	return nil
}

// CreateIssue generates a JIRA issue from the various fields on the given GitHub issue, then
// sends it to the JIRA API.
func CreateIssue(cfg *config.Config, issue *gh.Issue, ghClient github.Client, jClient jira.Client) error {
	log := cfg.GetLogger()

	log.Debugf("Creating JIRA issue based on GitHub issue #%d", *issue.Number)

	unknowns := tcontainer.NewMarshalMap()

	unknowns.Set(cfg.GetFieldKey(config.GitHubID), issue.GetID())
	unknowns.Set(cfg.GetFieldKey(config.GitHubNumber), issue.GetNumber())
	unknowns.Set(cfg.GetFieldKey(config.GitHubStatus), issue.GetState())
	unknowns.Set(cfg.GetFieldKey(config.GitHubReporter), issue.User.GetLogin())

	labels := githubLabelsToStrSlice(issue.Labels)
	unknowns.Set(cfg.GetFieldKey(config.GitHubLabels), labels)

	unknowns.Set(cfg.GetFieldKey(config.LastISUpdate), time.Now().Format(dateFormat))

	fields := &gojira.IssueFields{
		Type: gojira.IssueType{
			Name: "Task", // TODO: Determine issue type
		},
		Project:     *cfg.GetProject(),
		Summary:     issue.GetTitle(),
		Description: issue.GetBody(),
		Unknowns:    unknowns,
	}

	jIssue := &gojira.Issue{
		Fields: fields,
	}

	newIssue, err := jClient.CreateIssue(jIssue)
	if err != nil {
		return fmt.Errorf("creating Jira issue: %w", err)
	}

	foundIssue, err := jClient.GetIssue(newIssue.Key)
	if err != nil {
		return fmt.Errorf("getting Jira issue %s: %w", newIssue.Key, err)
	}

	log.Debugf("Created JIRA issue %s!", newIssue.Key)

	if err := comment.Compare(cfg, issue, foundIssue, ghClient, jClient); err != nil {
		return fmt.Errorf("comparing comments for issue %s: %w", jIssue.Key, err)
	}

	return nil
}

// githubLabelsToStrSlice converts a slice of GitHub label pointers (the format
// which is returned by the GitHub API) to a slice of strings, which can be
// supplied as a value for the `GitHub Labels` custom field.
//
// It also converts spaces (' ') to hyphens ('-'), as the Jira `labels` custom
// field type does not support spaces.
//
// TODO(github): Consider github.IssueRequest.GetLabels() here.
func githubLabelsToStrSlice(ghLabels []*gh.Label) []string {
	labels := make([]string, len(ghLabels))
	for i, l := range ghLabels {
		jiraLabel := l.GetName()

		// Converts spaces (' ') to hyphens ('-'), as the Jira `labels` custom
		// field type does not support spaces.
		// TODO(labels): Consider a normalization function for all values not
		//               supported.
		jiraLabel = strings.ReplaceAll(jiraLabel, " ", "-")
		labels[i] = jiraLabel
	}

	return labels
}
