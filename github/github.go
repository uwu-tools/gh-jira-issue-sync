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

package github

import (
	"context"
	"fmt"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/google/go-github/v48/github"
	"golang.org/x/oauth2"

	"github.com/uwu-tools/gh-jira-issue-sync/config"
)

// Client is a wrapper around the GitHub API Client library we
// use. It allows us to swap in other implementations, such as a dry run
// clients, or mock clients for testing.
type Client interface {
	ListIssues() ([]*github.Issue, error)
	ListComments(issue *github.Issue) ([]*github.IssueComment, error)
	GetUser(login string) (github.User, error)
	GetRateLimits() (github.RateLimits, error)
}

// realGHClient is a standard GitHub clients, that actually makes all of the
// requests against the GitHub REST API. It is the canonical implementation
// of GitHubClient.
type realGHClient struct {
	cfg    config.Config
	client github.Client
}

// ListIssues returns the list of GitHub issues since the last run of the tool.
func (g *realGHClient) ListIssues() ([]*github.Issue, error) {
	log := g.cfg.GetLogger()

	ctx := context.Background()

	user, repo := g.cfg.GetRepo()

	// Set it so that it will run the loop once, and it'll be updated in the loop.
	pages := 1
	var issues []*github.Issue

	for page := 1; page <= pages; page++ {
		is, res, err := g.request(func() (interface{}, *github.Response, error) {
			return g.client.Issues.ListByRepo(ctx, user, repo, &github.IssueListByRepoOptions{ //nolint:wrapcheck
				Since:     g.cfg.GetSinceParam(),
				State:     "all",
				Sort:      "created",
				Direction: "asc",
				ListOptions: github.ListOptions{
					Page:    page,
					PerPage: 100,
				},
			})
		})
		if err != nil {
			return nil, err
		}
		issuePointers, ok := is.([]*github.Issue)
		if !ok {
			log.Errorf("Get GitHub issues did not return issues! Got: %v", is)
			return nil, fmt.Errorf("get GitHub issues failed: expected []*github.Issue; got %T", is) //nolint:goerr113
		}

		var issuePage []*github.Issue
		for _, v := range issuePointers {
			// If PullRequestLinks is not nil, it's a Pull Request
			if v.PullRequestLinks == nil {
				issuePage = append(issuePage, v)
			}
		}

		pages = res.LastPage
		issues = append(issues, issuePage...)
	}

	log.Debug("Collected all GitHub issues")

	return issues, nil
}

// ListComments returns the list of all comments on a GitHub issue in
// ascending order of creation.
func (g *realGHClient) ListComments(issue *github.Issue) ([]*github.IssueComment, error) {
	log := g.cfg.GetLogger()

	ctx := context.Background()
	user, repo := g.cfg.GetRepo()
	c, _, err := g.request(
		func() (interface{}, *github.Response, error) {
			return g.client.Issues.ListComments( //nolint:wrapcheck
				ctx,
				user,
				repo,
				issue.GetNumber(),
				&github.IssueListCommentsOptions{
					Sort:      github.String("created"),
					Direction: github.String("asc"),
				},
			)
		},
	)
	if err != nil {
		log.Errorf("Error retrieving GitHub comments for issue #%d. Error: %v.", issue.GetNumber(), err)
		return nil, err
	}
	comments, ok := c.([]*github.IssueComment)
	if !ok {
		log.Errorf("Get GitHub comments did not return comments! Got: %v", c)
		return nil, fmt.Errorf("get GitHub comments failed: expected []*github.IssueComment; got %T", c) //nolint:goerr113
	}

	return comments, nil
}

// GetUser returns a GitHub user from its login.
func (g *realGHClient) GetUser(login string) (github.User, error) {
	log := g.cfg.GetLogger()

	u, _, err := g.request(func() (interface{}, *github.Response, error) {
		return g.client.Users.Get(context.Background(), login) //nolint:wrapcheck
	})
	if err != nil {
		log.Errorf("Error retrieving GitHub user %s. Error: %v", login, err)
	}

	user, ok := u.(*github.User)
	if !ok {
		log.Errorf("Get GitHub user did not return user! Got: %v", u)
		return github.User{}, fmt.Errorf("get GitHub user failed: expected *github.User; got %T", u) //nolint:goerr113
	}

	return *user, nil
}

// GetRateLimits returns the current rate limits on the GitHub API. This is a
// simple and lightweight request that can also be used simply for testing the API.
func (g *realGHClient) GetRateLimits() (github.RateLimits, error) {
	log := g.cfg.GetLogger()

	ctx := context.Background()

	rl, _, err := g.request(func() (interface{}, *github.Response, error) {
		return g.client.RateLimits(ctx) //nolint:wrapcheck
	})
	if err != nil {
		log.Errorf("Error connecting to GitHub; check your token. Error: %v", err)
		return github.RateLimits{}, err
	}
	rate, ok := rl.(*github.RateLimits)
	if !ok {
		log.Errorf("Get GitHub rate limits did not return rate limits! Got: %v", rl)
		return github.RateLimits{},
			fmt.Errorf( //nolint:goerr113
				"get GitHub rate limits failed: expected *github.RateLimits; got %T",
				rl,
			)
	}

	return *rate, nil
}

const RetryBackoffRoundRatio = time.Millisecond / time.Nanosecond

// request takes an API function from the GitHub library
// and calls it with exponential backoff. If the function succeeds, it
// returns the expected value and the GitHub API response, as well as a nil
// error. If it continues to fail until a maximum time is reached, it returns
// a nil result as well as the returned HTTP response and a timeout error.
func (g *realGHClient) request(f func() (interface{}, *github.Response, error)) (interface{}, *github.Response, error) {
	log := g.cfg.GetLogger()

	var ret interface{}
	var res *github.Response

	op := func() error {
		var err error
		ret, res, err = f()
		return err
	}

	b := backoff.NewExponentialBackOff()
	b.MaxElapsedTime = g.cfg.GetTimeout()

	_ = backoff.RetryNotify(op, b, func(err error, duration time.Duration) { //nolint:errcheck
		// Round to a whole number of milliseconds
		duration /= RetryBackoffRoundRatio // Convert nanoseconds to milliseconds
		duration *= RetryBackoffRoundRatio // Convert back so it appears correct

		log.Errorf("Error performing operation; retrying in %v: %v", duration, err)
	})

	return ret, res, nil
}

// New creates a GitHubClient and returns it; which
// implementation it uses depends on the configuration of this
// run. For example, a dry-run clients may be created which does
// not make any requests that would change anything on the server,
// but instead simply prints out the actions that it's asked to take.
func New(cfg *config.Config) (Client, error) {
	log := cfg.GetLogger()

	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: cfg.GetConfigString("github-token")},
	)
	tc := oauth2.NewClient(ctx, ts)

	client := github.NewClient(tc)

	ret := &realGHClient{
		cfg:    *cfg,
		client: *client,
	}

	// Make a request so we can check that we can connect fine.
	_, err := ret.GetRateLimits()
	if err != nil {
		return nil, fmt.Errorf("getting GitHub rate limits: %w", err)
	}
	log.Debug("Successfully connected to GitHub.")

	return ret, nil
}
