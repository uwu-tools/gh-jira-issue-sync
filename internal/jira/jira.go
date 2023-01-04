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

package jira

import (
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"

	jira "github.com/andygrunwald/go-jira/v2/cloud"
	gogh "github.com/google/go-github/v48/github"
	log "github.com/sirupsen/logrus"

	"github.com/uwu-tools/gh-jira-issue-sync/internal/config"
	"github.com/uwu-tools/gh-jira-issue-sync/internal/github"
	synchttp "github.com/uwu-tools/gh-jira-issue-sync/internal/http"
	"github.com/uwu-tools/gh-jira-issue-sync/internal/jira/auth"
	"github.com/uwu-tools/gh-jira-issue-sync/internal/options"
)

const (
	// commentDateFormat is the format used in the headers of JIRA comments.
	commentDateFormat = "15:04 PM, January 2 2006"

	// maxJQLIssueLength is the maximum number of GitHub issues we can
	// use before we need to stop using JQL and filter issues ourself.
	maxJQLIssueLength = 100

	// maxIssueSearchResults is the maximum number of items that a page can
	// return. Each operation can have a different limit for the number of items
	// returned, and these limits may change without notice. To find the maximum
	// number of items that an operation could return, set maxResults to a large
	// number—for example, over 1000—and if the returned value of maxResults is
	// less than the requested value, the returned value is the maximum.
	//
	// ref: https://developer.atlassian.com/cloud/jira/platform/rest/v2/intro/#pagination
	maxIssueSearchResults = 1000
)

// getErrorBody reads the HTTP response body of a JIRA API response,
// logs it as an error, and returns an error object with the contents
// of the body. If an error occurs during reading, that error is
// instead printed and returned. This function closes the body for
// further reading.
func getErrorBody(res *jira.Response) error {
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		log.Errorf("Error occurred trying to read error body: %+v", err)
		return fmt.Errorf("reading error body: %w", err)
	}

	log.Debugf("Error body: %+v", body)
	return fmt.Errorf("reading error body: %s", string(body)) //nolint:goerr113
}

// Client is a wrapper around the JIRA API clients library we
// use. It allows us to hide implementation details such as backoff
// as well as swap in other implementations, such as for dry run
// or test mocking.
type Client interface {
	ListIssues(ids []int) ([]jira.Issue, error)
	GetIssue(key string) (*jira.Issue, error)
	// TODO: Remove unnecessary return values; consider only returning error
	CreateIssue(issue *jira.Issue) (jira.Issue, error)
	// TODO: Remove unnecessary return values; consider only returning error
	UpdateIssue(issue *jira.Issue) (jira.Issue, error)
	// TODO: Remove unnecessary return values; consider only returning error
	CreateComment(
		issue *jira.Issue, comment *gogh.IssueComment, githubClient github.Client,
	) (jira.Comment, error)
	// TODO: Remove unnecessary return values; consider only returning error
	// TODO: Re-arrange arguments
	UpdateComment(
		issue *jira.Issue, id string, comment *gogh.IssueComment, githubClient github.Client,
	) (jira.Comment, error)
}

// New creates a new Client and configures it with
// the config object provided. The type of clients created depends
// on the configuration; currently, it creates either a standard
// clients, or a dry-run clients.
func New(cfg *config.Config) (Client, error) {
	var j Client
	var tp http.Client
	var err error

	if !cfg.IsBasicAuth() {
		oauth, err := auth.NewJiraHTTPClient(cfg)
		if err != nil {
			log.Errorf("Error getting OAuth config: %+v", err)
			return nil, fmt.Errorf("initializing Jira client: %w", err)
		}

		tp = *oauth
	} else {
		basicAuth := jira.BasicAuthTransport{
			Username: cfg.GetConfigString(options.ConfigKeyJiraUser),
			APIToken: strings.TrimSpace(cfg.GetConfigString(options.ConfigKeyJiraPassword)),
		}

		tp.Transport = &basicAuth
	}

	client, err := jira.NewClient(strings.TrimSpace(cfg.GetConfigString(options.ConfigKeyJiraURI)), &tp)
	if err != nil {
		log.Errorf("Error initializing JIRA clients; check your base URI. Error: %+v", err)
		return nil, fmt.Errorf("initializing Jira client: %w", err)
	}

	log.Debug("JIRA clients initialized")

	err = cfg.LoadJIRAConfig(client)
	if err != nil {
		return nil, fmt.Errorf("loading Jira configuration: %w", err)
	}

	if cfg.IsDryRun() {
		j = &dryrunJIRAClient{
			cfg:    cfg,
			client: client,
		}
	} else {
		j = &realJIRAClient{
			cfg:    cfg,
			client: client,
		}
	}

	return j, nil
}

func getJQLQuery(projectKey, fieldID string, ids []int) string {
	idStrs := make([]string, len(ids))
	for i, v := range ids {
		idStrs[i] = fmt.Sprint(v)
	}

	// If the list of IDs is too long, we get a 414 Request-URI Too Large, so in that case,
	// we'll need to do the filtering ourselves.
	var jql string
	if len(ids) < maxJQLIssueLength {
		jql = fmt.Sprintf(
			"project='%s' AND cf[%s] in (%s)",
			projectKey,
			fieldID,
			strings.Join(idStrs, ","),
		)
	} else {
		jql = fmt.Sprintf("project='%s'", projectKey)
	}

	log.Debugf("JQL query used: %s", jql)
	return jql
}

// realJIRAClient is a standard JIRA clients, which actually makes
// of the requests against the JIRA REST API. It is the canonical
// implementation of JIRAClient.
type realJIRAClient struct {
	cfg    *config.Config
	client *jira.Client
}

// ListIssues returns a list of JIRA issues on the configured project which
// have GitHub IDs in the provided list. `ids` should be a comma-separated
// list of GitHub IDs.
func (j *realJIRAClient) ListIssues(ids []int) ([]jira.Issue, error) {
	jql := getJQLQuery(
		j.cfg.GetProjectKey(),
		j.cfg.GetFieldID(config.GitHubID),
		ids,
	)

	// TODO(backoff): Consider restoring backoff logic here
	// TODO(j-v2): Parameterize all query options
	searchOpts := &jira.SearchOptions{
		MaxResults: maxIssueSearchResults,
	}
	jiraIssues, res, err := j.client.Issue.Search(j.cfg.Context(), jql, searchOpts)
	if err != nil {
		log.Errorf("Error retrieving JIRA issues: %+v", err)
		return nil, getErrorBody(res)
	}

	var issues []jira.Issue
	if len(ids) < maxJQLIssueLength {
		// The issues were already filtered by our JQL, so use as is
		issues = jiraIssues
	} else {
		// Filter only issues which have a defined GitHub ID in the list of IDs
		for _, v := range jiraIssues {
			if id, err := v.Fields.Unknowns.Int(j.cfg.GetFieldKey(config.GitHubID)); err == nil {
				for _, idOpt := range ids {
					if id == int64(idOpt) {
						issues = append(issues, v)
						break
					}
				}
			}
		}
	}

	return issues, nil
}

// GetIssue returns a single JIRA issue within the configured project
// according to the issue key (e.g. "PROJ-13").
func (j *realJIRAClient) GetIssue(key string) (*jira.Issue, error) {
	i, res, err := j.request(func() (interface{}, *jira.Response, error) {
		// TODO(j-v2): Add query options
		return j.client.Issue.Get(j.cfg.Context(), key, nil) //nolint:wrapcheck
	})
	if err != nil {
		log.Errorf("Error retrieving JIRA issue: %+v", err)
		return nil, getErrorBody(res)
	}
	issue, ok := i.(*jira.Issue)
	if !ok {
		log.Errorf("Get JIRA issue did not return issue! Got %v", i)
		return nil, fmt.Errorf("get JIRA issue failed: expected *jira.Issue; got %T", i) //nolint:goerr113
	}

	return issue, nil
}

// CreateIssue creates a new JIRA issue according to the fields provided in
// the provided issue object. It returns the created issue, with all the
// fields provided (including e.g. ID and Key).
func (j *realJIRAClient) CreateIssue(issue *jira.Issue) (jira.Issue, error) {
	i, res, err := j.request(func() (interface{}, *jira.Response, error) {
		return j.client.Issue.Create(j.cfg.Context(), issue) //nolint:wrapcheck
	})
	if err != nil {
		log.Errorf("Error creating JIRA issue: %+v", err)
		return jira.Issue{}, getErrorBody(res)
	}
	is, ok := i.(*jira.Issue)
	if !ok {
		log.Errorf("Create JIRA issue did not return issue! Got: %v", i)
		return jira.Issue{}, fmt.Errorf("create JIRA issue failed: expected *jira.Issue; got %T", i) //nolint:goerr113
	}

	return *is, nil
}

// UpdateIssue updates a given issue (identified by the Key field of the provided
// issue object) with the fields on the provided issue. It returns the updated
// issue as it exists on JIRA.
func (j *realJIRAClient) UpdateIssue(issue *jira.Issue) (jira.Issue, error) {
	i, res, err := j.request(func() (interface{}, *jira.Response, error) {
		// TODO(j-v2): Add query options
		return j.client.Issue.Update(j.cfg.Context(), issue, nil) //nolint:wrapcheck
	})
	if err != nil {
		log.Errorf("Error updating JIRA issue %s: %v", issue.Key, err)
		return jira.Issue{}, getErrorBody(res)
	}
	is, ok := i.(*jira.Issue)
	if !ok {
		log.Errorf("Update JIRA issue did not return issue! Got: %v", i)
		return jira.Issue{}, fmt.Errorf("update JIRA issue failed: expected *jira.Issue; got %T", i) //nolint:goerr113
	}

	return *is, nil
}

// maxBodyLength is the maximum length of a JIRA comment body, which is currently
// 2^15-1.
const maxBodyLength = 1 << 15

// CreateComment adds a comment to the provided JIRA issue using the fields from
// the provided GitHub comment. It then returns the created comment.
func (j *realJIRAClient) CreateComment(
	issue *jira.Issue,
	comment *gogh.IssueComment,
	githubClient github.Client,
) (jira.Comment, error) {
	user, err := githubClient.GetUser(comment.User.GetLogin())
	if err != nil {
		return jira.Comment{}, fmt.Errorf("getting GitHub user: %w", err)
	}

	body := fmt.Sprintf("Comment [(ID %d)|%s]", comment.GetID(), comment.GetHTMLURL())
	body = fmt.Sprintf("%s from GitHub user [%s|%s]", body, user.GetLogin(), user.GetHTMLURL())
	if user.GetName() != "" {
		body = fmt.Sprintf("%s (%s)", body, user.GetName())
	}
	body = fmt.Sprintf(
		"%s at %s:\n\n%s",
		body,
		comment.CreatedAt.Format(commentDateFormat),
		comment.GetBody(),
	)

	if len(body) > maxBodyLength {
		body = body[:maxBodyLength]
	}

	jComment := jira.Comment{
		Body: body,
	}

	com, res, err := j.request(func() (interface{}, *jira.Response, error) {
		return j.client.Issue.AddComment(j.cfg.Context(), issue.ID, &jComment) //nolint:wrapcheck
	})
	if err != nil {
		log.Errorf("Error creating JIRA comment on issue %s. Error: %v", issue.Key, err)
		return jira.Comment{}, getErrorBody(res)
	}
	co, ok := com.(*jira.Comment)
	if !ok {
		log.Errorf("Create JIRA comment did not return comment! Got: %v", com)
		return jira.Comment{}, fmt.Errorf("create JIRA comment failed: expected *jira.Comment; got %T", com) //nolint:goerr113
	}
	return *co, nil
}

// UpdateComment updates a comment (identified by the `id` parameter) on a given
// JIRA with a new body from the fields of the given GitHub comment. It returns
// the updated comment.
func (j *realJIRAClient) UpdateComment(
	issue *jira.Issue,
	id string,
	comment *gogh.IssueComment,
	githubClient github.Client,
) (jira.Comment, error) {
	user, err := githubClient.GetUser(comment.User.GetLogin())
	if err != nil {
		return jira.Comment{}, fmt.Errorf("getting GitHub user: %w", err)
	}

	body := fmt.Sprintf("Comment [(ID %d)|%s]", comment.GetID(), comment.GetHTMLURL())
	body = fmt.Sprintf("%s from GitHub user [%s|%s]", body, user.GetLogin(), user.GetHTMLURL())
	if user.GetName() != "" {
		body = fmt.Sprintf("%s (%s)", body, user.GetName())
	}
	body = fmt.Sprintf(
		"%s at %s:\n\n%s",
		body,
		comment.CreatedAt.Format(commentDateFormat),
		comment.GetBody(),
	)

	if len(body) > maxBodyLength {
		body = body[:maxBodyLength]
	}

	// As it is, the JIRA API we're using doesn't have any way to update comments natively.
	// So, we have to build the request ourselves.
	request := struct {
		Body string `json:"body"`
	}{
		Body: body,
	}

	req, err := j.client.NewRequest(
		j.cfg.Context(),
		"PUT",
		fmt.Sprintf("rest/api/2/issue/%s/comment/%s", issue.Key, id),
		request,
	)
	if err != nil {
		log.Errorf("Error creating comment update request: %s", err)
		return jira.Comment{}, fmt.Errorf("creating comment update request: %w", err)
	}

	com, res, err := j.request(func() (interface{}, *jira.Response, error) {
		res, err := j.client.Do(req, nil)
		return nil, res, err //nolint:wrapcheck
	})
	if err != nil {
		log.Errorf("Error updating comment: %+v", err)
		return jira.Comment{}, getErrorBody(res)
	}
	co, ok := com.(*jira.Comment)
	if !ok {
		log.Errorf("Update JIRA comment did not return comment! Got: %v", com)
		return jira.Comment{}, fmt.Errorf("update JIRA comment failed: expected *jira.Comment; got %T", com) //nolint:goerr113
	}
	return *co, nil
}

// request executes a Jira request with exponential backoff, using the real
// client.
func (j *realJIRAClient) request(f func() (interface{}, *jira.Response, error)) (interface{}, *jira.Response, error) {
	ret, resp, err := synchttp.NewJiraRequest(f, j.cfg.GetTimeout())
	if err != nil {
		return ret, resp, fmt.Errorf("request error: %w", err)
	}

	return ret, resp, nil
}

// dryrunJIRAClient is an implementation of JIRAClient which performs all
// GET requests the same as the realJIRAClient, but does not perform any
// unsafe requests which may modify server data, instead printing out the
// actions it is asked to perform without making the request.
type dryrunJIRAClient struct {
	cfg    *config.Config
	client *jira.Client
}

// newlineReplaceRegex is a regex to match both "\r\n" and just "\n" newline styles,
// in order to allow us to escape both sequences cleanly in the output of a dry run.
var newlineReplaceRegex = regexp.MustCompile("\r?\n")

// truncate is a utility function to replace all the newlines in
// the string with the characters "\n", then truncate it to no
// more than 50 characters.
func truncate(s string, length int) string {
	if s == "" {
		return "empty"
	}

	s = newlineReplaceRegex.ReplaceAllString(s, "\\n")
	if len(s) <= length {
		return s
	}
	return fmt.Sprintf("%s...", s[0:length])
}

// ListIssues returns a list of JIRA issues on the configured project which
// have GitHub IDs in the provided list. `ids` should be a comma-separated
// list of GitHub IDs.
//
// This function is identical to that in realJIRAClient.
func (j *dryrunJIRAClient) ListIssues(ids []int) ([]jira.Issue, error) {
	jql := getJQLQuery(
		j.cfg.GetProjectKey(),
		j.cfg.GetFieldID(config.GitHubID),
		ids,
	)

	ji, res, err := j.request(func() (interface{}, *jira.Response, error) {
		// TODO(j-v2): Add query options
		return j.client.Issue.Search(j.cfg.Context(), jql, nil) //nolint:wrapcheck
	})
	if err != nil {
		log.Errorf("Error retrieving JIRA issues: %+v", err)
		return nil, getErrorBody(res)
	}
	jiraIssues, ok := ji.([]jira.Issue)
	if !ok {
		log.Errorf("Get JIRA issues did not return issues! Got: %v", ji)
		return nil, fmt.Errorf("get JIRA issues failed: expected []jira.Issue; got %T", ji) //nolint:goerr113
	}

	var issues []jira.Issue
	if len(ids) < maxJQLIssueLength {
		// The issues were already filtered by our JQL, so use as is
		issues = jiraIssues
	} else {
		// Filter only issues which have a defined GitHub ID in the list of IDs
		for _, v := range jiraIssues {
			if id, err := v.Fields.Unknowns.Int(j.cfg.GetFieldKey(config.GitHubID)); err == nil {
				for _, idOpt := range ids {
					if id == int64(idOpt) {
						issues = append(issues, v)
						break
					}
				}
			}
		}
	}

	return issues, nil
}

// GetIssue returns a single JIRA issue within the configured project
// according to the issue key (e.g. "PROJ-13").
//
// This function is identical to that in realJIRAClient.
func (j *dryrunJIRAClient) GetIssue(key string) (*jira.Issue, error) {
	i, res, err := j.request(func() (interface{}, *jira.Response, error) {
		// TODO(j-v2): Add query options
		return j.client.Issue.Get(j.cfg.Context(), key, nil) //nolint:wrapcheck
	})
	if err != nil {
		log.Errorf("Error retrieving JIRA issue: %+v", err)
		return nil, getErrorBody(res)
	}
	issue, ok := i.(*jira.Issue)
	if !ok {
		log.Errorf("Get JIRA issue did not return issue! Got %v", i)
		return nil, fmt.Errorf("get JIRA issue failed: expected *jira.Issue; got %T", i) //nolint:goerr113
	}

	return issue, nil
}

// CreateIssue prints out the fields that would be set on a new issue were
// it to be created according to the provided issue object. It returns the
// provided issue object as-is.
func (j *dryrunJIRAClient) CreateIssue(issue *jira.Issue) (jira.Issue, error) {
	fields := issue.Fields

	log.Info("")
	log.Info("Create new JIRA issue:")
	log.Infof("  Summary: %s", fields.Summary)
	log.Infof("  Description: %s", truncate(fields.Description, 50))
	log.Infof("  GitHub ID: %d", fields.Unknowns[j.cfg.GetFieldKey(config.GitHubID)])
	log.Infof("  GitHub Number: %d", fields.Unknowns[j.cfg.GetFieldKey(config.GitHubNumber)])
	log.Infof("  Labels: %s", fields.Unknowns[j.cfg.GetFieldKey(config.GitHubLabels)])
	log.Infof("  State: %s", fields.Unknowns[j.cfg.GetFieldKey(config.GitHubStatus)])
	log.Infof("  Reporter: %s", fields.Unknowns[j.cfg.GetFieldKey(config.GitHubReporter)])
	log.Info("")

	return *issue, nil
}

// UpdateIssue prints out the fields that would be set on a JIRA issue
// (identified by issue.Key) were it to be updated according to the issue
// object. It then returns the provided issue object as-is.
func (j *dryrunJIRAClient) UpdateIssue(issue *jira.Issue) (jira.Issue, error) {
	fields := issue.Fields

	log.Info("")
	log.Infof("Update JIRA issue %s:", issue.Key)
	log.Infof("  Summary: %s", fields.Summary)
	log.Infof("  Description: %s", truncate(fields.Description, 50))
	key := j.cfg.GetFieldKey(config.GitHubLabels)
	if labels, err := fields.Unknowns.String(key); err == nil {
		log.Infof("  Labels: %s", labels)
	}
	key = j.cfg.GetFieldKey(config.GitHubStatus)
	if state, err := fields.Unknowns.String(key); err == nil {
		log.Infof("  State: %s", state)
	}
	log.Info("")

	return *issue, nil
}

// CreateComment prints the body that would be set on a new comment if it were
// to be created according to the fields of the provided GitHub comment. It then
// returns a comment object containing the body that would be used.
func (j *dryrunJIRAClient) CreateComment(
	issue *jira.Issue,
	comment *gogh.IssueComment,
	githubClient github.Client,
) (jira.Comment, error) {
	user, err := githubClient.GetUser(comment.User.GetLogin())
	if err != nil {
		return jira.Comment{}, fmt.Errorf("getting GitHub user: %w", err)
	}

	body := fmt.Sprintf("Comment (ID %d) from GitHub user %s", comment.GetID(), user.GetLogin())
	if user.GetName() != "" {
		body = fmt.Sprintf("%s (%s)", body, user.GetName())
	}
	body = fmt.Sprintf(
		"%s at %s:\n\n%s",
		body,
		comment.CreatedAt.Format(commentDateFormat),
		comment.GetBody(),
	)

	log.Info("")
	log.Infof("Create comment on JIRA issue %s:", issue.Key)
	log.Infof("  GitHub ID: %d", comment.GetID())
	if user.GetName() != "" {
		log.Infof("  User: %s (%s)", user.GetLogin(), user.GetName())
	} else {
		log.Infof("  User: %s", user.GetLogin())
	}
	log.Infof("  Posted at: %s", comment.CreatedAt.Format(commentDateFormat))
	log.Infof("  Body: %s", truncate(comment.GetBody(), 100))
	log.Info("")

	return jira.Comment{
		Body: body,
	}, nil
}

// UpdateComment prints the body that would be set on a comment were it to be
// updated according to the provided GitHub comment. It then returns a comment
// object containing the body that would be used.
func (j *dryrunJIRAClient) UpdateComment(
	issue *jira.Issue,
	id string,
	comment *gogh.IssueComment,
	githubClient github.Client,
) (jira.Comment, error) {
	user, err := githubClient.GetUser(comment.User.GetLogin())
	if err != nil {
		return jira.Comment{}, fmt.Errorf("getting GitHub user: %w", err)
	}

	body := fmt.Sprintf("Comment (ID %d) from GitHub user %s", comment.GetID(), user.GetLogin())
	if user.GetName() != "" {
		body = fmt.Sprintf("%s (%s)", body, user.GetName())
	}
	body = fmt.Sprintf(
		"%s at %s:\n\n%s",
		body,
		comment.CreatedAt.Format(commentDateFormat),
		comment.GetBody(),
	)

	log.Info("")
	log.Infof("Update JIRA comment %s on issue %s:", id, issue.Key)
	log.Infof("  GitHub ID: %d", comment.GetID())
	if user.GetName() != "" {
		log.Infof("  User: %s (%s)", user.GetLogin(), user.GetName())
	} else {
		log.Infof("  User: %s", user.GetLogin())
	}
	log.Infof("  Posted at: %s", comment.CreatedAt.Format(commentDateFormat))
	log.Infof("  Body: %s", truncate(comment.GetBody(), 100))
	log.Info("")

	return jira.Comment{
		ID:   id,
		Body: body,
	}, nil
}

// request executes a Jira request with exponential backoff, using the dry-run
// client.
func (j *dryrunJIRAClient) request(f func() (interface{}, *jira.Response, error)) (interface{}, *jira.Response, error) {
	ret, resp, err := synchttp.NewJiraRequest(f, j.cfg.GetTimeout())
	if err != nil {
		return ret, resp, fmt.Errorf("request error: %w", err)
	}

	return ret, resp, nil
}
