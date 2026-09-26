// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

import (
	"slices"
	"testing"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/convention"
)

func TestSnapshotCarriesTheEffectiveCommitTypes(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		configured []string
		want       []string
	}{
		"the built-in types, in order, with none configured": {
			configured: nil, want: convention.DefaultCommitConvention().Types(),
		},
		"the team's own, trimmed, when configured": {
			configured: []string{" wip ", "deps"}, want: []string{"wip", "deps"},
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			cfg := config.Default()
			cfg.Commit.Types = tt.configured

			// Act
			recorder := streamOnce(t, serve(t, filledDeps(), cfg), "/api/events")

			// Assert
			if got := firstSnapshot(t, recorder.Body.String()).CommitTypes; !slices.Equal(got, tt.want) {
				t.Errorf("commit types = %q, want %q", got, tt.want)
			}
		})
	}
}
