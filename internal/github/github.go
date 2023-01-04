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

	gogh "github.com/google/go-github/v48/github"
	"golang.org/x/oauth2"
	"sigs.k8s.io/release-sdk/github"

	"github.com/uwu-tools/gh-jira-issue-sync/internal/config"
	synchttp "github.com/uwu-tools/gh-jira-issue-sync/internal/http"
	"github.com/uwu-tools/gh-jira-issue-sync/internal/options"
)

// Client is a wrapper around the GitHub API Client library we
// use. It allows us to swap in other implementations, such as a dry run
// clients, or mock clients for testing.
type Client interface {
	ListIssues() ([]*gogh.Issue, error)
	ListComments(issue *gogh.Issue) ([]*gogh.IssueComment, error)
	GetUser(login string) (gogh.User, error)
	GetRateLimits() (gogh.RateLimits, error)
}

// realGHClient is a standard GitHub clients, that actually makes all of the
// requests against the GitHub REST API. It is the canonical implementation
// of GitHubClient.
type realGHClient struct {
	cfg          *config.Config
	client       *github.GitHub
	githubClient *gogh.Client
}

const (
	itemsPerPage  = 100
	sortOption    = "created"
	sortDirection = "asc"
)

// ListIssues returns the list of GitHub issues since the last run of the tool.
func (g *realGHClient) ListIssues() ([]*gogh.Issue, error) {
	log := g.cfg.GetLogger()

	owner, repo := g.cfg.GetRepo()

	var issues []*gogh.Issue

	// TODO(github): Should issue state be configurable?
	issueState := github.IssueStateAll

	// TODO(github): Consider if any of these options need to be exposed.
	_ = &gogh.IssueListByRepoOptions{
		Since:     g.cfg.GetSinceParam(),
		State:     string(issueState),
		Sort:      sortOption,
		Direction: sortDirection,
		ListOptions: gogh.ListOptions{
			PerPage: itemsPerPage,
		},
	}

	is, err := g.client.ListIssues(owner, repo, issueState)
	if err != nil {
		return nil, fmt.Errorf("listing GitHub issues: %w", err)
	}

	for _, v := range is {
		// If PullRequestLinks is not nil, it's a Pull Request
		if v.PullRequestLinks == nil {
			issues = append(issues, v)
		}
	}

	log.Debug("Collected all GitHub issues")
	return issues, nil
}

// ListComments returns the list of all comments on a GitHub issue in
// ascending order of creation.
func (g *realGHClient) ListComments(issue *gogh.Issue) ([]*gogh.IssueComment, error) {
	log := g.cfg.GetLogger()

	ctx := context.Background()
	user, repo := g.cfg.GetRepo()
	c, _, err := g.request(
		func() (interface{}, *gogh.Response, error) {
			return g.githubClient.Issues.ListComments( //nolint:wrapcheck
				ctx,
				user,
				repo,
				issue.GetNumber(),
				&gogh.IssueListCommentsOptions{
					Sort:      gogh.String(sortOption),
					Direction: gogh.String(sortDirection),
				},
			)
		},
	)
	if err != nil {
		log.Errorf("Error retrieving GitHub comments for issue #%d. Error: %v.", issue.GetNumber(), err)
		return nil, err
	}
	comments, ok := c.([]*gogh.IssueComment)
	if !ok {
		log.Errorf("Get GitHub comments did not return comments! Got: %v", c)
		return nil, fmt.Errorf("get GitHub comments failed: expected []*github.IssueComment; got %T", c) //nolint:goerr113
	}

	return comments, nil
}

// GetUser returns a GitHub user from its login.
func (g *realGHClient) GetUser(login string) (gogh.User, error) {
	log := g.cfg.GetLogger()

	u, _, err := g.request(func() (interface{}, *gogh.Response, error) {
		return g.githubClient.Users.Get(context.Background(), login) //nolint:wrapcheck
	})
	if err != nil {
		log.Errorf("Error retrieving GitHub user %s. Error: %v", login, err)
	}

	user, ok := u.(*gogh.User)
	if !ok {
		log.Errorf("Get GitHub user did not return user! Got: %v", u)
		return gogh.User{}, fmt.Errorf("get GitHub user failed: expected *github.User; got %T", u) //nolint:goerr113
	}

	return *user, nil
}

// GetRateLimits returns the current rate limits on the GitHub API. This is a
// simple and lightweight request that can also be used simply for testing the API.
func (g *realGHClient) GetRateLimits() (gogh.RateLimits, error) {
	log := g.cfg.GetLogger()

	ctx := context.Background()

	rl, _, err := g.request(func() (interface{}, *gogh.Response, error) {
		return g.githubClient.RateLimits(ctx) //nolint:wrapcheck
	})
	if err != nil {
		log.Errorf("Error connecting to GitHub; check your token. Error: %v", err)
		return gogh.RateLimits{}, err
	}
	rate, ok := rl.(*gogh.RateLimits)
	if !ok {
		log.Errorf("Get GitHub rate limits did not return rate limits! Got: %v", rl)
		return gogh.RateLimits{},
			fmt.Errorf( //nolint:goerr113
				"get GitHub rate limits failed: expected *github.RateLimits; got %T",
				rl,
			)
	}

	return *rate, nil
}

// request takes an API function from the GitHub library
// and calls it with exponential backoff. If the function succeeds, it
// returns the expected value and the GitHub API response, as well as a nil
// error. If it continues to fail until a maximum time is reached, it returns
// a nil result as well as the returned HTTP response and a timeout error.
func (g *realGHClient) request(f func() (interface{}, *gogh.Response, error)) (interface{}, *gogh.Response, error) {
	ret, resp, err := synchttp.NewGitHubRequest(f, g.cfg.GetLogger(), g.cfg.GetTimeout())
	if err != nil {
		return ret, resp, fmt.Errorf("request error: %w", err)
	}

	return ret, resp, nil
}

// New creates a GitHubClient and returns it; which
// implementation it uses depends on the configuration of this
// run. For example, a dry-run clients may be created which does
// not make any requests that would change anything on the server,
// but instead simply prints out the actions that it's asked to take.
func New(cfg *config.Config) (Client, error) {
	log := cfg.GetLogger()

	token := cfg.GetConfigString(options.ConfigKeyGitHubToken)
	client, err := github.NewWithToken(token)
	if err != nil {
		return nil, fmt.Errorf("creating sync client: %w", err)
	}

	client.SetOptions(
		&github.Options{
			ItemsPerPage: itemsPerPage,
		},
	)

	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{
			AccessToken: token,
		},
	)
	tc := oauth2.NewClient(ctx, ts)

	githubClient := gogh.NewClient(tc)

	ret := &realGHClient{
		cfg:          cfg,
		client:       client,
		githubClient: githubClient,
	}

	// Make a request so we can check that we can connect fine.
	_, err = ret.GetRateLimits()
	if err != nil {
		return nil, fmt.Errorf("getting GitHub rate limits: %w", err)
	}
	log.Debug("Successfully connected to GitHub.")

	return ret, nil
}
