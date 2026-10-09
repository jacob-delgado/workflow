// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/seams"
	"github.com/jacob-delgado/workflow/internal/setup"
	"github.com/jacob-delgado/workflow/internal/tui"
)

// errNoJiraAddress is the search refusing before it asks Jira anything, as
// with no file to read Jira's address from.
var errNoJiraAddress = errors.New("jira.baseUrl is not set")

func TestTheFirstRunIsOfferedOverIssuesAnEarlierSessionCached(t *testing.T) {
	t.Parallel()

	// Arrange
	// No file applies, so the search fails; the store still holds the list an
	// earlier session read, drawn with its first issue selected.
	working := reposWorld()
	working.cachedIssues = working.issues
	deps := working.deps()
	deps.Jira.Search = func(string, int) (jira.SearchResult, error) { return jira.SearchResult{}, errNoJiraAddress }
	deps.Settings.Setup = seams.Setup{
		Offer: func() setup.Offer { return setup.Offer{} },
		Check: func(config.Jira) (string, error) { return "", nil },
		Write: func(setup.Request) (setup.Written, error) { return setup.Written{}, nil },
	}
	model := sized(t, tui.New(config.Default(), config.ErrNotFound, deps), 120, 40)

	// Act
	view := drain(t, model, model.Init()).View().Content

	// Assert
	requireScreen(t, view, issueKey)

	if footer := footerLine(view); !strings.Contains(footer, "enter set up") {
		t.Errorf("the footer = %q, want it to offer enter to set up", footer)
	}
}
