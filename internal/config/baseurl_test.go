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

func TestProblemsRefusesABaseURLWithALoginWithoutQuotingIt(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := config.Config{Jira: config.Jira{BaseURL: "https://" + loginUser + ":" + loginPassword + "@" + jiraHost}}

	// Act
	got := cfg.Problems()

	// Assert
	named := 0

	for _, problem := range got {
		if strings.Contains(problem, "jira.base_url") {
			named++
		}

		if strings.Contains(problem, loginUser) || strings.Contains(problem, loginPassword) {
			t.Errorf("Problems() = %q, want no problem quoting the login", got)
		}
	}

	if named != 1 {
		t.Errorf("Problems() = %q, want jira.base_url named once", got)
	}
}

func TestProblemsAcceptsAnAbsoluteBaseURL(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := config.Config{Jira: config.Jira{BaseURL: "https://" + jiraHost + "/jira"}}

	// Act
	got := cfg.Problems()

	// Assert
	if len(got) != 0 {
		t.Errorf("Problems() = %q, want none", got)
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
		"http, under a path":    {raw: "http://" + jiraHost + "/jira", want: nil},
		"no scheme":             {raw: jiraHost + "/jira", want: config.ErrInvalidBaseURL},
		"another scheme":        {raw: "ftp://" + jiraHost, want: config.ErrInvalidBaseURL},
		"no host":               {raw: "https:///jira", want: config.ErrInvalidBaseURL},
		"one url.Parse refuses": {raw: "https://" + login + jiraHost + "/\x7f", want: config.ErrInvalidBaseURL},
		"a user and password":   {raw: "https://" + login + jiraHost, want: config.ErrCredentialInBaseURL},
		"a user alone":          {raw: "https://" + loginUser + "@" + jiraHost, want: config.ErrCredentialInBaseURL},
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
