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

package comments

import (
	"fmt"
	"regexp"
	"strconv"

	gojira "github.com/andygrunwald/go-jira/v2/cloud"
	gh "github.com/google/go-github/v48/github"

	"github.com/uwu-tools/gh-jira-issue-sync/config"
	"github.com/uwu-tools/gh-jira-issue-sync/github"
	"github.com/uwu-tools/gh-jira-issue-sync/jira"
)

// jCommentRegex matches a generated JIRA comment. It has matching groups to retrieve the
// GitHub Comment ID (\1), the GitHub username (\2), the GitHub real name (\3, if it exists),
// the time the comment was posted (\3 or \4), and the body of the comment (\4 or \5).
var jCommentRegex = regexp.MustCompile(
	`^Comment \[\(ID (\d+)\)\|.*?] from GitHub user \[(.+)\|.*?] \((.+)\) at (.+):\n\n(.+)$`,
)

// jCommentIDRegex just matches the beginning of a generated JIRA comment. It's a smaller,
// simpler, and more efficient regex, to quickly filter only generated comments and retrieve
// just their GitHub ID for matching.
var jCommentIDRegex = regexp.MustCompile(`^Comment \[\(ID (\d+)\)\|`)

// Compare takes a GitHub issue, and retrieves all of its comments. It then
// matches each one to a comment in `existing`. If it finds a match, it calls
// UpdateComment; if it doesn't, it calls CreateComment.
func Compare(cfg config.Config, ghIssue gh.Issue, jIssue gojira.Issue, ghClient github.Client, jClient jira.Client) error {
	log := cfg.GetLogger()

	if ghIssue.GetComments() == 0 {
		log.Debugf("Issue #%d has no comments, skipping.", *ghIssue.Number)
		return nil
	}

	ghComments, err := ghClient.ListComments(ghIssue)
	if err != nil {
		return fmt.Errorf("listing GitHub comments: %w", err)
	}

	var jComments []gojira.Comment
	if jIssue.Fields.Comments == nil {
		log.Debugf("JIRA issue %s has no comments.", jIssue.Key)
	} else {
		commentPtrs := jIssue.Fields.Comments.Comments
		jComments = make([]gojira.Comment, len(commentPtrs))
		for i, v := range commentPtrs {
			jComments[i] = *v
		}
		log.Debugf("JIRA issue %s has %d comments", jIssue.Key, len(jComments))
	}

	for _, ghComment := range ghComments {
		found := false
		for _, jComment := range jComments {
			if !jCommentIDRegex.MatchString(jComment.Body) {
				continue
			}
			// matches[0] is the whole string, matches[1] is the ID
			matches := jCommentIDRegex.FindStringSubmatch(jComment.Body)
			intID, err := strconv.Atoi(matches[1])
			if err != nil {
				return fmt.Errorf("converting comment ID to int: %w", err)
			}

			id := int64(intID)
			if *ghComment.ID != id {
				continue
			}
			found = true

			err = UpdateComment(cfg, *ghComment, jComment, jIssue, ghClient, jClient)
			if err != nil {
				return err
			}

			break
		}
		if found {
			continue
		}

		comment, err := jClient.CreateComment(jIssue, *ghComment, ghClient)
		if err != nil {
			return fmt.Errorf("creating Jira comment: %w", err)
		}

		log.Debugf("Created JIRA comment %s.", comment.ID)
	}

	log.Debugf("Copied comments from GH issue #%d to JIRA issue %s.", *ghIssue.Number, jIssue.Key)
	return nil
}

// UpdateComment compares the body of a GitHub comment with the body (minus header)
// of the JIRA comment, and updates the JIRA comment if necessary.
func UpdateComment(cfg config.Config, ghComment gh.IssueComment, jComment gojira.Comment, jIssue gojira.Issue, ghClient github.Client, jClient jira.Client) error {
	log := cfg.GetLogger()

	// fields[0] is the whole body, 1 is the ID, 2 is the username, 3 is the real name (or "" if none)
	// 4 is the date, and 5 is the real body
	fields := jCommentRegex.FindStringSubmatch(jComment.Body)

	if fields[5] == ghComment.GetBody() {
		return nil
	}

	comment, err := jClient.UpdateComment(jIssue, jComment.ID, ghComment, ghClient)
	if err != nil {
		return fmt.Errorf("updating Jira comment: %w", err)
	}

	log.Debug("Updated JIRA comment %s.", comment.ID)

	return nil
}
