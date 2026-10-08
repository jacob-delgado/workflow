// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package wiring_test

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/loop"
	"github.com/jacob-delgado/workflow/internal/messaging"
	"github.com/jacob-delgado/workflow/internal/seams"
	"github.com/jacob-delgado/workflow/internal/store"
	"github.com/jacob-delgado/workflow/internal/wiring"
)

// brokenStore points the store at a directory that cannot be made, a file
// standing where it goes, and returns the session over it.
func brokenStore(t *testing.T) wiring.Workspace {
	t.Helper()

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_STATE_HOME", "")

	dir, err := store.DefaultDir()
	if err != nil {
		t.Fatalf("resolving the store directory: %v", err)
	}

	err = os.MkdirAll(filepath.Dir(dir), 0o700)
	if err != nil {
		t.Fatalf("making the store's parent: %v", err)
	}

	err = os.WriteFile(dir, nil, 0o600)
	if err != nil {
		t.Fatalf("putting a file where the store goes: %v", err)
	}

	return wiring.Workspace{Root: t.TempDir(), Remote: ""}
}

func TestEveryStoreWriteReportsAStoreThatCannotKeepIt(t *testing.T) {
	cases := map[string]func(deps seams.Store) error{
		"a scope":     func(deps seams.Store) error { return deps.RecordScope("api") },
		"an announce": func(deps seams.Store) error { return deps.RecordAnnounce(loop.Announced{Pull: 7}) },
		"an issue list": func(deps seams.Store) error {
			return deps.CacheIssues("assigned", []jira.Issue{{Key: "PROJ-9", Summary: "a"}})
		},
	}

	for name, write := range cases {
		t.Run(name, func(t *testing.T) {
			// Arrange
			where := brokenStore(t)
			cfg := config.Default()
			cfg.Jira.BaseURL = jiraAddress
			deps := wired(t, cfg, where, nil).Store

			// Act
			err := write(deps)

			// Assert
			if err == nil {
				t.Errorf("the write = nil, want the store's failure reported")
			}
		})
	}
}

func TestAnAnnouncementOfAMomentNoBuildKnowsIsNotReadBack(t *testing.T) {
	// Arrange
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", "")

	deps := wired(t, config.Default(), wiring.Workspace{Root: t.TempDir(), Remote: ""}, nil).Store
	known := loop.Announced{Pull: 7, Moment: messaging.MomentMerged}

	for _, made := range []loop.Announced{known, {Pull: 7, Moment: messaging.Moment(9)}} {
		err := deps.RecordAnnounce(made)
		if err != nil {
			t.Fatalf("recording %+v: %v", made, err)
		}
	}

	// Act
	announced := deps.Announced()

	// Assert
	if !slices.Equal(announced, []loop.Announced{known}) {
		t.Errorf("Announced = %+v, want only the moment this build knows", announced)
	}
}
