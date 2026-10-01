// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli_test

import (
	"strings"
	"testing"
)

// A token_env that gives nothing is a configured source, so the error names
// the variable rather than saying no token is configured.
func TestAJiraTokenVariableThatGivesNothingIsNamedNotCalledUnconfigured(t *testing.T) {
	// Arrange
	t.Setenv(emptyTokenVariable, "")

	repo := repoForBranch(t, "https://jira.example.net")
	writeFile(t, repo, `{"jira": {"base_url": "https://jira.example.net", "token_env": "`+emptyTokenVariable+`"}}`)

	// Act
	_, err := run(t, repo, "branch", "PROJ-7", "--yes")

	// Assert
	if err == nil || strings.Contains(err.Error(), "is configured") || !strings.Contains(err.Error(), emptyTokenVariable) {
		t.Errorf("branch = %v, want the error to name %s and not call the token unconfigured", err, emptyTokenVariable)
	}
}

// A Slack user token not logged in yet is named with the command that logs it
// in, rather than called a token that is not configured. The file keeps the
// token's credentials, so no keychain is read.
func TestASlackUserTokenNotLoggedInNamesTheLogin(t *testing.T) {
	// Arrange
	fakeGh(t, ghResponses{pulls: openPull("Add login")})

	repo := githubRepo(t, "fix/PROJ-2-thing")
	writeFile(t, repo, `{"forge":{"cli":true,"kind":"github","host":"github.com"},`+
		`"messaging":{"client_id":"1234.5678","client_secret":"client-secret-9999","channel":"#dev"}}`)

	// Act
	_, err := run(t, repo, "announce", "--yes")

	// Assert
	if err == nil || !strings.Contains(err.Error(), "workflow slack login") {
		t.Errorf("announce = %v, want the error to name workflow slack login", err)
	}
}
