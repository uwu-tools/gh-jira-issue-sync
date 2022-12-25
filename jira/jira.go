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
	"time"

	"github.com/andygrunwald/go-jira"
	"github.com/cenkalti/backoff/v4"
	gh "github.com/google/go-github/v48/github"

	"github.com/uwu-tools/gh-jira-issue-sync/auth"
	"github.com/uwu-tools/gh-jira-issue-sync/config"
	"github.com/uwu-tools/gh-jira-issue-sync/github"
)

// commentDateFormat is the format used in the headers of JIRA comments.
const commentDateFormat = "15:04 PM, January 2 2006"

// maxJQLIssueLength is the maximum number of GitHub issues we can
// use before we need to stop using JQL and filter issues ourself.
const maxJQLIssueLength = 100

// getErrorBody reads the HTTP response body of a JIRA API response,
// logs it as an error, and returns an error object with the contents
// of the body. If an error occurs during reading, that error is
// instead printed and returned. This function closes the body for
// further reading.
func getErrorBody(cfg config.Config, res *jira.Response) error {
	log := cfg.GetLogger()
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
	GetIssue(key string) (jira.Issue, error)
	CreateIssue(issue jira.Issue) (jira.Issue, error)
	UpdateIssue(issue jira.Issue) (jira.Issue, error)
	CreateComment(issue jira.Issue, comment gh.IssueComment, githubClient github.Client) (jira.Comment, error)
	UpdateComment(issue jira.Issue, id string, comment gh.IssueComment, githubClient github.Client) (jira.Comment, error)
}

// New creates a new Client and configures it with
// the config object provided. The type of clients created depends
// on the configuration; currently, it creates either a standard
// clients, or a dry-run clients.
func New(cfg *config.Config) (Client, error) {
	log := cfg.GetLogger()

	var j Client
	var tp http.Client
	var err error

	if !cfg.IsBasicAuth() {
		oauth, err := auth.NewJiraHTTPClient(*cfg)
		if err != nil {
			log.Errorf("Error getting OAuth config: %+v", err)
			return nil, fmt.Errorf("initializing Jira client: %w", err)
		}

		tp = *oauth
	} else {
		basicAuth := jira.BasicAuthTransport{
			Username: strings.TrimSpace(cfg.GetConfigString("jira-user")),
			Password: strings.TrimSpace(cfg.GetConfigString("jira-pass")),
		}

		tp.Transport = &basicAuth
	}

	client, err := jira.NewClient(&tp, strings.TrimSpace(cfg.GetConfigString("jira-uri")))
	if err != nil {
		log.Errorf("Error initializing JIRA clients; check your base URI. Error: %+v", err)
		return nil, fmt.Errorf("initializing Jira client: %w", err)
	}

	log.Debug("JIRA clients initialized")

	err = cfg.LoadJIRAConfig(*client)
	if err != nil {
		return nil, fmt.Errorf("loading Jira configuration: %w", err)
	}

	if cfg.IsDryRun() {
		j = dryrunJIRAClient{
			cfg:    *cfg,
			client: *client,
		}
	} else {
		j = realJIRAClient{
			cfg:    *cfg,
			client: *client,
		}
	}

	return j, nil
}

// realJIRAClient is a standard JIRA clients, which actually makes
// of the requests against the JIRA REST API. It is the canonical
// implementation of JIRAClient.
type realJIRAClient struct {
	cfg    config.Config
	client jira.Client
}

// ListIssues returns a list of JIRA issues on the configured project which
// have GitHub IDs in the provided list. `ids` should be a comma-separated
// list of GitHub IDs.
func (j realJIRAClient) ListIssues(ids []int) ([]jira.Issue, error) {
	log := j.cfg.GetLogger()

	idStrs := make([]string, len(ids))
	for i, v := range ids {
		idStrs[i] = fmt.Sprint(v)
	}

	// If the list of IDs is too long, we get a 414 Request-URI Too Large, so in that case,
	// we'll need to do the filtering ourselves.
	// TODO: Re-enable manual filtering, if required
	/*
		if len(ids) < maxJQLIssueLength {
			jql = fmt.Sprintf(
				// TODO(jira): Fix "The operator 'in' is not supported by the 'cf[#####]' field."
				"project='%s' AND cf[%s] in (%s)",
				j.cfg.GetProjectKey(),
				j.cfg.GetFieldID(config.GitHubID),
				strings.Join(idStrs, ","),
			)
		} else {
			jql = fmt.Sprintf("project='%s'", j.cfg.GetProjectKey())
		}
	*/
	jql := fmt.Sprintf("project='%s'", j.cfg.GetProjectKey())
	log.Debugf("JQL query used: %s", jql)

	// TODO(backoff): Considering restoring backoff logic here
	jiraIssues, res, err := j.client.Issue.Search(jql, nil)
	if err != nil {
		log.Errorf("Error retrieving JIRA issues: %+v", err)
		return nil, getErrorBody(j.cfg, res)
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
func (j realJIRAClient) GetIssue(key string) (jira.Issue, error) {
	log := j.cfg.GetLogger()

	i, res, err := j.request(func() (interface{}, *jira.Response, error) {
		return j.client.Issue.Get(key, nil) //nolint:wrapcheck
	})
	if err != nil {
		log.Errorf("Error retrieving JIRA issue: %+v", err)
		return jira.Issue{}, getErrorBody(j.cfg, res)
	}
	issue, ok := i.(*jira.Issue)
	if !ok {
		log.Errorf("Get JIRA issue did not return issue! Got %v", i)
		return jira.Issue{}, fmt.Errorf("get JIRA issue failed: expected *jira.Issue; got %T", i) //nolint:goerr113
	}

	return *issue, nil
}

// CreateIssue creates a new JIRA issue according to the fields provided in
// the provided issue object. It returns the created issue, with all the
// fields provided (including e.g. ID and Key).
func (j realJIRAClient) CreateIssue(issue jira.Issue) (jira.Issue, error) {
	log := j.cfg.GetLogger()

	i, res, err := j.request(func() (interface{}, *jira.Response, error) {
		return j.client.Issue.Create(&issue) //nolint:wrapcheck
	})
	if err != nil {
		log.Errorf("Error creating JIRA issue: %+v", err)
		return jira.Issue{}, getErrorBody(j.cfg, res)
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
func (j realJIRAClient) UpdateIssue(issue jira.Issue) (jira.Issue, error) {
	log := j.cfg.GetLogger()

	i, res, err := j.request(func() (interface{}, *jira.Response, error) {
		return j.client.Issue.Update(&issue) //nolint:wrapcheck
	})
	if err != nil {
		log.Errorf("Error updating JIRA issue %s: %v", issue.Key, err)
		return jira.Issue{}, getErrorBody(j.cfg, res)
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
func (j realJIRAClient) CreateComment(issue jira.Issue, comment gh.IssueComment, githubClient github.Client) (jira.Comment, error) {
	log := j.cfg.GetLogger()

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
		return j.client.Issue.AddComment(issue.ID, &jComment) //nolint:wrapcheck
	})
	if err != nil {
		log.Errorf("Error creating JIRA comment on issue %s. Error: %v", issue.Key, err)
		return jira.Comment{}, getErrorBody(j.cfg, res)
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
func (j realJIRAClient) UpdateComment(issue jira.Issue, id string, comment gh.IssueComment, githubClient github.Client) (jira.Comment, error) {
	log := j.cfg.GetLogger()

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

	req, err := j.client.NewRequest("PUT", fmt.Sprintf("rest/api/2/issue/%s/comment/%s", issue.Key, id), request)
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
		return jira.Comment{}, getErrorBody(j.cfg, res)
	}
	co, ok := com.(*jira.Comment)
	if !ok {
		log.Errorf("Update JIRA comment did not return comment! Got: %v", com)
		return jira.Comment{}, fmt.Errorf("update JIRA comment failed: expected *jira.Comment; got %T", com) //nolint:goerr113
	}
	return *co, nil
}

// request takes an API function from the JIRA library
// and calls it with exponential backoff. If the function succeeds, it
// returns the expected value and the JIRA API response, as well as a nil
// error. If it continues to fail until a maximum time is reached, it returns
// a nil result as well as the returned HTTP response and a timeout error.
func (j realJIRAClient) request(f func() (interface{}, *jira.Response, error)) (interface{}, *jira.Response, error) {
	log := j.cfg.GetLogger()

	var ret interface{}
	var res *jira.Response

	op := func() error {
		var err error
		ret, res, err = f()
		return err
	}

	b := backoff.NewExponentialBackOff()
	b.MaxElapsedTime = j.cfg.GetTimeout()

	backoffErr := backoff.RetryNotify(op, b, func(err error, duration time.Duration) {
		// Round to a whole number of milliseconds
		duration /= github.RetryBackoffRoundRatio // Convert nanoseconds to milliseconds
		duration *= github.RetryBackoffRoundRatio // Convert back so it appears correct

		log.Errorf("Error performing operation; retrying in %v: %v", duration, err)
	})

	return ret, res, fmt.Errorf("backoff error: %w", backoffErr)
}

// dryrunJIRAClient is an implementation of JIRAClient which performs all
// GET requests the same as the realJIRAClient, but does not perform any
// unsafe requests which may modify server data, instead printing out the
// actions it is asked to perform without making the request.
type dryrunJIRAClient struct {
	cfg    config.Config
	client jira.Client
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
func (j dryrunJIRAClient) ListIssues(ids []int) ([]jira.Issue, error) {
	log := j.cfg.GetLogger()

	idStrs := make([]string, len(ids))
	for i, v := range ids {
		idStrs[i] = fmt.Sprint(v)
	}

	var jql string
	// If the list of IDs is too long, we get a 414 Request-URI Too Large, so in that case,
	// we'll need to do the filtering ourselves.
	if len(ids) < maxJQLIssueLength {
		jql = fmt.Sprintf("project='%s' AND cf[%s] in (%s)",
			j.cfg.GetProjectKey(), j.cfg.GetFieldID(config.GitHubID), strings.Join(idStrs, ","))
	} else {
		jql = fmt.Sprintf("project='%s'", j.cfg.GetProjectKey())
	}

	ji, res, err := j.request(func() (interface{}, *jira.Response, error) {
		return j.client.Issue.Search(jql, nil) //nolint:wrapcheck
	})
	if err != nil {
		log.Errorf("Error retrieving JIRA issues: %+v", err)
		return nil, getErrorBody(j.cfg, res)
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
func (j dryrunJIRAClient) GetIssue(key string) (jira.Issue, error) {
	log := j.cfg.GetLogger()

	i, res, err := j.request(func() (interface{}, *jira.Response, error) {
		return j.client.Issue.Get(key, nil) //nolint:wrapcheck
	})
	if err != nil {
		log.Errorf("Error retrieving JIRA issue: %+v", err)
		return jira.Issue{}, getErrorBody(j.cfg, res)
	}
	issue, ok := i.(*jira.Issue)
	if !ok {
		log.Errorf("Get JIRA issue did not return issue! Got %v", i)
		return jira.Issue{}, fmt.Errorf("get JIRA issue failed: expected *jira.Issue; got %T", i) //nolint:goerr113
	}

	return *issue, nil
}

// CreateIssue prints out the fields that would be set on a new issue were
// it to be created according to the provided issue object. It returns the
// provided issue object as-is.
func (j dryrunJIRAClient) CreateIssue(issue jira.Issue) (jira.Issue, error) {
	log := j.cfg.GetLogger()

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

	return issue, nil
}

// UpdateIssue prints out the fields that would be set on a JIRA issue
// (identified by issue.Key) were it to be updated according to the issue
// object. It then returns the provided issue object as-is.
func (j dryrunJIRAClient) UpdateIssue(issue jira.Issue) (jira.Issue, error) {
	log := j.cfg.GetLogger()

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

	return issue, nil
}

// CreateComment prints the body that would be set on a new comment if it were
// to be created according to the fields of the provided GitHub comment. It then
// returns a comment object containing the body that would be used.
func (j dryrunJIRAClient) CreateComment(issue jira.Issue, comment gh.IssueComment, githubClient github.Client) (jira.Comment, error) {
	log := j.cfg.GetLogger()

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
func (j dryrunJIRAClient) UpdateComment(issue jira.Issue, id string, comment gh.IssueComment, githubClient github.Client) (jira.Comment, error) {
	log := j.cfg.GetLogger()

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

// request takes an API function from the JIRA library
// and calls it with exponential backoff. If the function succeeds, it
// returns the expected value and the JIRA API response, as well as a nil
// error. If it continues to fail until a maximum time is reached, it returns
// a nil result as well as the returned HTTP response and a timeout error.
//
// This function is identical to that in realJIRAClient.
func (j dryrunJIRAClient) request(f func() (interface{}, *jira.Response, error)) (interface{}, *jira.Response, error) {
	log := j.cfg.GetLogger()

	var ret interface{}
	var res *jira.Response

	op := func() error {
		var err error
		ret, res, err = f()
		return err
	}

	b := backoff.NewExponentialBackOff()
	b.MaxElapsedTime = j.cfg.GetTimeout()

	backoffErr := backoff.RetryNotify(op, b, func(err error, duration time.Duration) {
		// Round to a whole number of milliseconds
		duration /= github.RetryBackoffRoundRatio // Convert nanoseconds to milliseconds
		duration *= github.RetryBackoffRoundRatio // Convert back so it appears correct

		log.Errorf("Error performing operation; retrying in %v: %v", duration, err)
	})

	return ret, res, fmt.Errorf("backoff error: %w", backoffErr)
}
