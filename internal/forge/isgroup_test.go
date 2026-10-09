// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package forge_test

// CODEOWNERS spells a top-level GitLab group @acme, as it spells a user, so
// the forge tells which it is: a name GitLab knows no user by, and knows a
// group by, is a group.

import (
	"errors"
	"net/http"
	"testing"

	"github.com/jacob-delgado/workflow/internal/forge"
)

func TestIsGroupTellsAGitLabGroupFromAPerson(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		name string
		want bool
	}{
		"a name GitLab knows a group by": {name: "acme", want: true},
		"a name GitLab knows a user by":  {name: userAna, want: false},
		"a name GitLab knows neither by": {name: userGhost, want: false},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			client, _ := recordingForge(t, gitlabKnowing(map[string]string{userAna: "7"},
				map[string]string{"/groups/acme/members": `[]`}))

			// Act
			group, err := client.On(forge.KindGitLab).IsGroup(t.Context(), tt.name)

			// Assert
			if err != nil || group != tt.want {
				t.Errorf("IsGroup(%q) = %v, %v; want %v", tt.name, group, err, tt.want)
			}
		})
	}
}

func TestIsGroupSaysWhyItCannotTell(t *testing.T) {
	t.Parallel()

	failing := func(recorded) (int, string) { return http.StatusBadGateway, `{"message":"502 Bad Gateway"}` }
	noUser := gitlabKnowing(nil, nil)

	cases := map[string]func(recorded) (int, string){
		"GitLab failing to look the name up as a user": failing,
		"GitLab failing to read the group, once it knows no user by the name": func(asked recorded) (int, string) {
			if asked.path == "/groups/acme/members" {
				return failing(asked)
			}

			return noUser(asked)
		},
	}

	for name, answer := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			client, _ := recordingForge(t, answer)

			// Act
			group, err := client.On(forge.KindGitLab).IsGroup(t.Context(), "acme")

			// Assert
			if group || !errors.Is(err, forge.ErrUnexpectedStatus) {
				t.Errorf("IsGroup with GitLab failing = %v, %v; want no group and the failure", group, err)
			}
		})
	}
}

func TestIsGroupIsNotOfferedOnGitHub(t *testing.T) {
	t.Parallel()

	// Arrange
	client, seen := recordingForge(t, func(recorded) (int, string) { return http.StatusOK, "[]" })

	// Act
	_, err := client.On(forge.KindGitHub).IsGroup(t.Context(), "acme")

	// Assert
	if !errors.Is(err, forge.ErrNotSupported) || len(*seen) != 0 {
		t.Errorf("IsGroup = %v after %d requests, want ErrNotSupported and none", err, len(*seen))
	}
}
