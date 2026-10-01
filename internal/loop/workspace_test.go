// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package loop_test

// Tags are kept per Slack workspace, so every surface asks which one the token
// is for before tagging: no Slack user token tags no one and says nothing, as
// a webhook does, and a workspace Slack will not name tags no one and says why.

import (
	"errors"
	"testing"

	"github.com/jacob-delgado/workflow/internal/loop"
	"github.com/jacob-delgado/workflow/internal/messaging"
)

func TestTagWorkspaceTellsNoTokenFromAnUnknownWorkspace(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		read          func() (string, error)
		wantWorkspace string
		want          error
	}{
		"the workspace Slack names": {
			read: func() (string, error) { return "T0ACME", nil }, wantWorkspace: "T0ACME", want: nil,
		},
		"no read, as with no Slack user token": {read: nil, want: messaging.ErrNoCredential},
		"no Slack user token": {
			read: func() (string, error) { return "", messaging.ErrNoCredential }, want: messaging.ErrNoCredential,
		},
		"a workspace Slack will not name": {
			read: func() (string, error) { return "", messaging.ErrNoWorkspace }, want: loop.ErrUnknownWorkspace,
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			workspace, err := loop.TagWorkspace(tt.read)

			// Assert
			if workspace != tt.wantWorkspace || !errors.Is(err, tt.want) || (tt.want == nil) != (err == nil) {
				t.Errorf("TagWorkspace = %q, %v; want %q, %v", workspace, err, tt.wantWorkspace, tt.want)
			}
		})
	}
}
