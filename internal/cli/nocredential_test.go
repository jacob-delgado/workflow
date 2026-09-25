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
