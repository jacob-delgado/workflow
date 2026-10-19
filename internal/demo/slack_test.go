// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package demo_test

import (
	"errors"
	"testing"

	"github.com/jacob-delgado/workflow/internal/loop"
	"github.com/jacob-delgado/workflow/internal/messaging"
)

func TestAPostIsAcceptedOnlyOnAChannelTheDemosSlackHas(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		channel string
		want    error
	}{
		"the configured channel": {channel: "", want: nil},
		"#dev-workflow":          {channel: "#dev-workflow", want: nil},
		"#releases":              {channel: "#releases", want: nil},
		"#nowhere":               {channel: "#nowhere", want: messaging.ErrPostRefused},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			deps := standingStill().Deps()

			// Act
			err := deps.Messaging.Post(t.Context(), tt.channel, "ana.lopez opened #128")

			// Assert
			if !errors.Is(err, tt.want) {
				t.Errorf("posting to %q = %v, want %v", tt.channel, err, tt.want)
			}
		})
	}
}

func TestAnAnnouncementIsRemembered(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := standingStill().Deps()
	made := loop.Announced{Pull: 128, Moment: messaging.MomentReady}

	// Act
	err := deps.Store.RecordAnnounce(t.Context(), made)

	// Assert
	if announced := deps.Store.Announced(t.Context()); err != nil || len(announced) != 1 || announced[0] != made {
		t.Errorf("the announcements after recording one = %+v, %v; want %+v", announced, err, made)
	}
}

func TestTheCommitScopeUsedIsRemembered(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := standingStill().Deps()

	// Act
	err := deps.Store.RecordScope(t.Context(), "wiring")

	// Assert
	if scope, kept := deps.Store.LastScope(t.Context()); err != nil || !kept || scope != "wiring" {
		t.Errorf("the last scope after recording wiring = %q, %v, %v", scope, kept, err)
	}
}
