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

package comment

import (
	"errors"
	"testing"
	"time"

	gogh "github.com/google/go-github/v53/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	gojira "github.com/uwu-tools/go-jira/v2/cloud"

	"github.com/uwu-tools/gh-jira-issue-sync/internal/config"
	"github.com/uwu-tools/gh-jira-issue-sync/internal/github"
	"github.com/uwu-tools/gh-jira-issue-sync/internal/jira"
	"github.com/uwu-tools/gh-jira-issue-sync/pkg"
)

//nolint:lll
const testComment = `Comment [(ID 484163403)|https://github.com] from GitHub user [bilbo-baggins|https://github.com/bilbo-baggins] (Bilbo Baggins) at 16:27 PM, April 17 2019:

Bla blibidy bloo bla`

var errMock = errors.New("mock error")

var (
	jiraClient *jira.JiraClientMock
	cfg        *config.ConfigMock
	ghClient   *github.GhClientMock
)

var ghComment1 = gogh.IssueComment{
	ID:        pkg.NewInt64(1),
	Body:      pkg.NewString("Comment body 1"),
	CreatedAt: &gogh.Timestamp{Time: time.Date(2000, 9, 26, 0, 0, 0, 0, time.FixedZone("UTC", 0))},
}

var ghComment2 = gogh.IssueComment{
	ID:        pkg.NewInt64(2),
	Body:      pkg.NewString("Comment body 2"),
	CreatedAt: &gogh.Timestamp{Time: time.Date(1996, 8, 1, 0, 0, 0, 0, time.FixedZone("UTC", 0))},
}

var ghComment3 = gogh.IssueComment{
	ID:        pkg.NewInt64(3),
	Body:      pkg.NewString("Comment body 3"),
	CreatedAt: &gogh.Timestamp{Time: time.Date(2001, 1, 1, 0, 0, 0, 0, time.FixedZone("UTC", 0))},
}

var jiraComment1 = gojira.Comment{
	ID: "1",
	//nolint:lll
	Body: `Comment [(ID 1)|https://github.com] from GitHub user [user1|https://github.com/user1] (First User) at 00:00 AM, September 26 2000:

Comment body 1`,
}

var jiraComment2 = gojira.Comment{
	ID: "2",
	//nolint:lll
	Body: `Comment [(ID 2)|https://github.com] from GitHub user [user2|https://github.com/user2] (Second User) at 00:00 AM, August 1 1996:

Comment body 2`,
}

func setup(t *testing.T) {
	t.Helper()

	jiraClient = new(jira.JiraClientMock)
	cfg = new(config.ConfigMock)
	ghClient = new(github.GhClientMock)
}

func TestJiraCommentRegex(t *testing.T) {
	fields := jCommentRegex.FindStringSubmatch(testComment)

	if len(fields) != 6 {
		t.Fatalf("Regex failed to parse fields %v", fields)
	}

	if fields[1] != "484163403" {
		t.Fatalf("Expected field[1] = 484163403; Got field[1] = %s", fields[1])
	}

	if fields[2] != "bilbo-baggins" {
		t.Fatalf("Expected field[2] = bilbo-baggins; Got field[2] = %s", fields[2])
	}

	if fields[3] != "Bilbo Baggins" {
		t.Fatalf("Expected field[3] = Bilbo Baggins; Got field[3] = %s", fields[3])
	}

	if fields[4] != "16:27 PM, April 17 2019" {
		t.Fatalf("Expected field[4] = 16:27 PM, April 17 2019; Got field[4] = %s", fields[4])
	}

	if fields[5] != "Bla blibidy bloo bla" {
		t.Fatalf("Expected field[5] = Bla blibidy bloo bla; Got field[5] = %s", fields[5])
	}
}

func TestCompare(t *testing.T) {
	tests := []struct { //nolint:govet
		name           string
		ghComments     []*gogh.IssueComment
		jiraComments   []*gojira.Comment
		expectedResult *ComparisonResult
	}{
		{
			"should return empty result if there are no GH comment",
			[]*gogh.IssueComment{},
			[]*gojira.Comment{},
			&ComparisonResult{make([]*gogh.IssueComment, 0), make([]*CommentPair, 0)},
		},
		{
			"should create all ghComments if there are no existing Jira comments",
			[]*gogh.IssueComment{&ghComment1, &ghComment2},
			[]*gojira.Comment{},
			&ComparisonResult{[]*gogh.IssueComment{&ghComment1, &ghComment2}, make([]*CommentPair, 0)},
		},
		{
			"should create GH comment if no matching Jira comment",
			[]*gogh.IssueComment{&ghComment3},
			[]*gojira.Comment{&jiraComment1, &jiraComment2},
			&ComparisonResult{[]*gogh.IssueComment{&ghComment3}, make([]*CommentPair, 0)},
		},
		{
			"should update GH comments if there are matching Jira comments",
			[]*gogh.IssueComment{&ghComment1, &ghComment2},
			[]*gojira.Comment{&jiraComment1, &jiraComment2},
			&ComparisonResult{
				make([]*gogh.IssueComment, 0),
				[]*CommentPair{
					{
						&ghComment1,
						&jiraComment1,
					},
					{
						&ghComment2,
						&jiraComment2,
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setup(t)

			result, err := Compare(tt.ghComments, tt.jiraComments)

			assert.Nil(t, err)
			assert.Equal(t, tt.expectedResult, result)
			mock.AssertExpectationsForObjects(t, jiraClient)
		})
	}
}

func TestUpdateComment(t *testing.T) {
	jiraIssue := &gojira.Issue{ID: "1"}

	tests := []struct { //nolint:govet
		name        string
		ghComment   *gogh.IssueComment
		jiraComment *gojira.Comment
		initMockFn  func()
		expectedErr string
	}{
		// {
		//	"should create if the corresponding Jira comment is not in the right format",
		//	&ghComment1,
		//	&jiraCommentWrongBody,
		//	func() {
		//		jiraClient.On("UpdateComment", jiraIssue, jiraCommentWrongBody.ID, &ghComment1, ghClient).Return(nil)
		//	},
		//	"",
		// }, // nolint
		{
			"should not update if the content of the comments are the same",
			&ghComment1,
			&jiraComment1,
			func() {
			},
			"",
		},
		{
			"should update if the content of the comments are different",
			&ghComment1,
			&jiraComment2,
			func() {
				jiraClient.On("UpdateComment", jiraIssue, jiraComment2.ID, &ghComment1, ghClient).Return(&jiraComment1, nil)
			},
			"",
		},
		{
			"should return error if the update failed",
			&ghComment1,
			&jiraComment2,
			func() {
				jiraClient.
					On("UpdateComment", jiraIssue, jiraComment2.ID, &ghComment1, ghClient).
					Return(&jiraComment1, errMock)
			},
			"updating Jira comment",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setup(t)
			tt.initMockFn()

			err := UpdateComment(cfg, tt.ghComment, tt.jiraComment, jiraIssue, ghClient, jiraClient)

			if tt.expectedErr != "" {
				assert.ErrorContains(t, err, tt.expectedErr)
			}
			mock.AssertExpectationsForObjects(t, jiraClient, cfg, ghClient)
		})
	}
}
