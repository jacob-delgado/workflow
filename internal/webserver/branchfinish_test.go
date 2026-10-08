// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

import (
	"net/http"
	"slices"
	"testing"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
)

func TestTheBranchCarriesTheCommandsAFinishRuns(t *testing.T) {
	t.Parallel()

	// The finish's last look shows these before a force delete is confirmed,
	// so the page shows the server's own, never a copy of its own.
	cases := map[string]struct {
		branch gitrepo.Branch
		want   []string
	}{
		"a branch with a base": {
			branch: gitrepo.Branch{Name: yourLocalBranch, Base: "origin/main"},
			want:   gitrepo.FinishCommands("main", yourLocalBranch),
		},
		"a branch with no base": {branch: gitrepo.Branch{Name: yourLocalBranch}, want: nil},
		"a detached head":       {branch: gitrepo.Branch{Detached: true, Base: "origin/main"}, want: nil},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			deps := filledDeps()
			deps.Git.Branch = func() (gitrepo.Branch, error) { return tt.branch, nil }

			// Act
			answer := send(t, serve(t, deps, config.Default()), http.MethodGet, "/api/branch", "")

			// Assert
			got := decode[api.Branch](t, answer).FinishCommands
			if (got == nil) != (tt.want == nil) || (got != nil && !slices.Equal(*got, tt.want)) {
				t.Errorf("finish_commands = %v, want %q", got, tt.want)
			}
		})
	}
}
