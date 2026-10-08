// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package config_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/config"
)

// The host and login the base URLs below carry, none of which a report may
// quote.
const (
	jiraHost      = "jira.example.com"
	loginUser     = "alice"
	loginPassword = "sekret"
)

func TestALoadRefusesABaseURLWithALoginWithoutQuotingIt(t *testing.T) {
	t.Parallel()

	// Arrange
	body := `{"jira": {"base_url": "https://` + loginUser + ":" + loginPassword + "@" + jiraHost + `"}}`

	// Act
	_, err := config.Parse(strings.NewReader(body))

	// Assert
	if !errors.Is(err, config.ErrCredentialInBaseURL) {
		t.Errorf("Parse = %v, want ErrCredentialInBaseURL", err)
	}

	if strings.Contains(err.Error(), loginUser) || strings.Contains(err.Error(), loginPassword) {
		t.Errorf("Parse = %v, want a refusal that does not quote the login", err)
	}
}

func TestCheckBaseURLAcceptsOnlyAnAbsoluteWebURLWithNoLogin(t *testing.T) {
	t.Parallel()

	const login = loginUser + ":" + loginPassword + "@"

	cases := map[string]struct {
		raw  string
		want error
	}{
		"https":                 {raw: "https://" + jiraHost, want: nil},
		"http to this machine":  {raw: "http://127.0.0.1:8080/jira", want: nil},
		"http to localhost":     {raw: "http://localhost:8080", want: nil},
		"http to IPv6 loopback": {raw: "http://[::1]:8080", want: nil},
		// The token and every jira.headers value would cross the network in the
		// clear.
		"http to another machine": {raw: "http://" + jiraHost + "/jira", want: config.ErrInvalidBaseURL},
		"no scheme":               {raw: jiraHost + "/jira", want: config.ErrInvalidBaseURL},
		"another scheme":          {raw: "ftp://" + jiraHost, want: config.ErrInvalidBaseURL},
		"no host":                 {raw: "https:///jira", want: config.ErrInvalidBaseURL},
		"one url.Parse refuses":   {raw: "https://" + login + jiraHost + "/\x7f", want: config.ErrInvalidBaseURL},
		"a user and password":     {raw: "https://" + login + jiraHost, want: config.ErrCredentialInBaseURL},
		"a user alone":            {raw: "https://" + loginUser + "@" + jiraHost, want: config.ErrCredentialInBaseURL},
		// The login is refused before the scheme is read, so a URL wrong in both
		// ways names the login.
		"a login on another scheme": {raw: "ftp://" + login + jiraHost, want: config.ErrCredentialInBaseURL},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			err := config.CheckBaseURL(tt.raw)

			// Assert
			if !errors.Is(err, tt.want) {
				t.Fatalf("CheckBaseURL(%q) = %v, want %v", tt.raw, err, tt.want)
			}

			if err != nil && (strings.Contains(err.Error(), jiraHost) || strings.Contains(err.Error(), loginPassword)) {
				t.Errorf("CheckBaseURL(%q) = %q, want an error that does not quote the URL", tt.raw, err)
			}
		})
	}
}

func TestParseRefusesAnHTTPBaseURLOffThisMachine(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		raw  string
		want error
	}{
		"http to another machine": {raw: "http://" + jiraHost, want: config.ErrInvalidBaseURL},
		"http to this machine":    {raw: "http://127.0.0.1:8080", want: nil},
		"https":                   {raw: "https://" + jiraHost, want: nil},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			_, err := config.Parse(strings.NewReader(`{"jira": {"base_url": "` + tt.raw + `"}}`))

			// Assert
			if !errors.Is(err, tt.want) || (tt.want != nil && !errors.Is(err, config.ErrInvalid)) {
				t.Errorf("Parse of %q = %v, want %v", tt.raw, err, tt.want)
			}
		})
	}
}
