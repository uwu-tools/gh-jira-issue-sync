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

package http

import (
	"fmt"
	"time"

	jira "github.com/andygrunwald/go-jira/v2/cloud"
	"github.com/cenkalti/backoff/v4"
	"github.com/google/go-github/v48/github"
	"github.com/sirupsen/logrus"
)

const retryBackoffRoundRatio = time.Millisecond / time.Nanosecond

// NewGitHubRequest takes an API function from the GitHub library
// and calls it with exponential backoff. If the function succeeds, it
// returns the expected value and the GitHub API response, as well as a nil
// error. If it continues to fail until a maximum time is reached, it returns
// a nil result as well as the returned HTTP response and a timeout error.
func NewGitHubRequest(
	f func() (interface{}, *github.Response, error),
	log logrus.Entry, //nolint:gocritic
	timeout time.Duration,
) (interface{}, *github.Response, error) {
	var ret interface{}
	var res *github.Response

	op := func() error {
		var err error
		ret, res, err = f()
		return err
	}

	backoffErr := retryNotify(op, log, timeout)
	if backoffErr != nil {
		return ret, res, errBackoff(backoffErr)
	}

	return ret, res, nil
}

// NewJiraRequest takes an API function from the JIRA library and calls it with
// exponential backoff. If the function succeeds, it returns the expected value
// and the JIRA API response, as well as a nil error. If it continues to fail
// until a maximum time is reached, it returns a nil result as well as the
// returned HTTP response and a timeout error.
func NewJiraRequest(
	f func() (interface{}, *jira.Response, error),
	log logrus.Entry, //nolint:gocritic
	timeout time.Duration,
) (interface{}, *jira.Response, error) {
	var ret interface{}
	var res *jira.Response

	op := func() error {
		var err error
		ret, res, err = f()
		return err
	}

	backoffErr := retryNotify(op, log, timeout)
	if backoffErr != nil {
		return ret, res, errBackoff(backoffErr)
	}

	return ret, res, nil
}

func retryNotify(
	op backoff.Operation,
	log logrus.Entry, //nolint:gocritic
	timeout time.Duration,
) error {
	b := backoff.NewExponentialBackOff()
	b.MaxElapsedTime = timeout

	err := backoff.RetryNotify(
		op,
		b,
		func(err error, duration time.Duration) {
			// Round to a whole number of milliseconds
			duration /= retryBackoffRoundRatio // Convert nanoseconds to milliseconds
			duration *= retryBackoffRoundRatio // Convert back so it appears correct

			log.Errorf("Error performing operation; retrying in %v: %v", duration, err)
		},
	)
	if err != nil {
		return fmt.Errorf("retry notify: %w", err)
	}

	return nil
}

func errBackoff(e error) error {
	return fmt.Errorf("backoff error: %w", e)
}
