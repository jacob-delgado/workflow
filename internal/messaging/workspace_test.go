// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package messaging_test

import (
	"errors"
	"net/http"
	"testing"

	"github.com/jacob-delgado/workflow/internal/messaging"
)

func TestWorkspaceIsTheTeamTheTokenIsFor(t *testing.T) {
	t.Parallel()

	// Arrange
	client := serve(t, func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte(okBody))
	})

	// Act
	workspace, err := client.Workspace(t.Context())

	// Assert
	if err != nil || workspace != "T00000000" {
		t.Errorf("Workspace() = %q, %v, want T00000000", workspace, err)
	}
}

func TestWorkspaceRefusesAnAnswerNamingNoTeam(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"no team ID":                   `{"ok":true,"user":"workflow"}`,
		"a team ID of the wrong shape": `{"ok":true,"user":"workflow","team_id":"T0 <!here>"}`,
	}

	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			client := serve(t, func(writer http.ResponseWriter, _ *http.Request) {
				_, _ = writer.Write([]byte(body))
			})

			// Act
			workspace, err := client.Workspace(t.Context())

			// Assert
			if !errors.Is(err, messaging.ErrNoWorkspace) || workspace != "" {
				t.Errorf("Workspace() = %q, %v, want ErrNoWorkspace", workspace, err)
			}
		})
	}
}
