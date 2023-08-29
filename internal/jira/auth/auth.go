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

package auth

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/dghubble/oauth1"

	"github.com/uwu-tools/gh-jira-issue-sync/internal/config"
	"github.com/uwu-tools/gh-jira-issue-sync/internal/options"
)

// NewJiraHTTPClient obtains an access token (either from configuration
// or from an OAuth handshake) and creates an HTTP client that uses the
// token, which can be used to configure a Jira client.
func NewJiraHTTPClient(cfg config.IConfig) (*http.Client, error) {
	ctx := context.Background()

	oauthConfig, err := oauthConfig(cfg)
	if err != nil {
		return nil, err
	}

	tok, ok := jiraTokenFromConfig(cfg)
	if !ok {
		tok, err = jiraTokenFromWeb(oauthConfig)
		if err != nil {
			return nil, err
		}
		cfg.SetJiraToken(tok)
	}

	return oauthConfig.Client(ctx, tok), nil
}

// oauthConfig parses a private key and consumer key from the
// configuration, and creates an OAuth configuration which can
// be used to begin a handshake.
func oauthConfig(cfg config.IConfig) (*oauth1.Config, error) {
	pvtKeyPath := cfg.GetConfigString(options.ConfigKeyJiraPrivateKeyPath)

	pvtKeyFile, err := os.Open(pvtKeyPath)
	if err != nil {
		return nil, fmt.Errorf("unable to open private key file for reading: %w", err)
	}

	pvtKey, err := io.ReadAll(pvtKeyFile)
	if err != nil {
		return nil, fmt.Errorf("unable to read contents of private key file: %w", err)
	}

	keyDERBlock, _ := pem.Decode(pvtKey)
	if keyDERBlock == nil {
		return nil, errPEMDecode
	}
	if keyDERBlock.Type != "PRIVATE KEY" && !strings.HasSuffix(keyDERBlock.Type, " PRIVATE KEY") {
		return nil, errUnexpectedKeyType(keyDERBlock.Type)
	}

	key, err := x509.ParsePKCS1PrivateKey(keyDERBlock.Bytes)
	if err != nil {
		return nil, fmt.Errorf("unable to parse PKCS1 private key: %w", err)
	}

	uri := cfg.GetConfigString(options.ConfigKeyJiraURI)

	return &oauth1.Config{
		ConsumerKey: cfg.GetConfigString(options.ConfigKeyJiraConsumerKey),
		CallbackURL: "oob",
		Endpoint: oauth1.Endpoint{
			RequestTokenURL: fmt.Sprintf("%splugins/servlet/oauth/request-token", uri),
			AuthorizeURL:    fmt.Sprintf("%splugins/servlet/oauth/authorize", uri),
			AccessTokenURL:  fmt.Sprintf("%splugins/servlet/oauth/access-token", uri),
		},
		Signer: &oauth1.RSASigner{
			PrivateKey: key,
		},
	}, nil
}

// jiraTokenFromConfig attempts to load an OAuth access token from the
// application configuration file. It returns the token (or null if not
// configured) and an "ok" bool to indicate whether the token is provided.
func jiraTokenFromConfig(cfg config.IConfig) (*oauth1.Token, bool) {
	token := cfg.GetConfigString(options.ConfigKeyJiraToken)
	if token == "" {
		return nil, false
	}

	secret := cfg.GetConfigString(options.ConfigKeyJiraSecret)
	if secret == "" {
		return nil, false
	}

	return &oauth1.Token{
		Token:       token,
		TokenSecret: secret,
	}, true
}

// jiraTokenFromWeb performs an OAuth handshake, obtaining a request and
// then an access token by authorizing with the Jira REST API.
func jiraTokenFromWeb(cfg *oauth1.Config) (*oauth1.Token, error) {
	requestToken, requestSecret, err := cfg.RequestToken()
	if err != nil {
		return nil, fmt.Errorf("unable to get request token: %w", err)
	}

	authURL, err := cfg.AuthorizationURL(requestToken)
	if err != nil {
		return nil, fmt.Errorf("unable to get authorize URL: %w", err)
	}

	fmt.Printf("Please go to the following URL in your browser:\n%v\n\n", authURL.String())
	fmt.Print("Authorization code: ")

	var code string
	_, err = fmt.Scan(&code)
	fmt.Println()
	if err != nil {
		return nil, fmt.Errorf("unable to read auth code: %w", err)
	}

	accessToken, accessSecret, err := cfg.AccessToken(requestToken, requestSecret, code)
	if err != nil {
		return nil, fmt.Errorf("unable to get access token: %w", err)
	}

	return oauth1.NewToken(accessToken, accessSecret), nil
}

// Errors

var errPEMDecode = errors.New("unable to decode private key PEM block")

func errUnexpectedKeyType(keyType string) error {
	return fmt.Errorf("unexpected private key DER block type: %s", keyType) //nolint:goerr113
}
