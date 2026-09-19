// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"testing"

	"github.com/jacob-delgado/workflow/internal/forge"
)

func TestNoForgeTokenMessage(t *testing.T) {
	t.Parallel()

	installed := func(string) bool { return true }
	absent := func(string) bool { return false }

	cases := map[string]struct {
		available func(string) bool
		kind      forge.Kind
		host      string
		want      string
	}{
		"gh installed but signed out names the host": {
			available: installed, kind: forge.KindGitHub, host: "github.com",
			want: "gh is installed but not signed in to github.com — run `gh auth login`",
		},
		"gh absent lists the sources instead": {
			available: absent, kind: forge.KindGitHub, host: "github.com",
			want: "none — " + forge.Sources(forge.KindGitHub, "github.com"),
		},
		"a GitLab remote has no gh hint": {
			available: installed, kind: forge.KindGitLab, host: "gitlab.com",
			want: "none — " + forge.Sources(forge.KindGitLab, "gitlab.com"),
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			got := noForgeTokenMessage(tt.available, tt.kind, tt.host)

			// Assert
			if got != tt.want {
				t.Errorf("noForgeTokenMessage = %q, want %q", got, tt.want)
			}
		})
	}
}
