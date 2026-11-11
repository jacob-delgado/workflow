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

func TestAnAnnouncementIsClaimedOnce(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := standingStill().Deps()
	made := loop.Announced{Pull: 128, Moment: messaging.MomentReady}

	first, err := deps.Store.ClaimAnnounce(t.Context(), made)
	if err != nil || first.Kind != loop.ClaimTaken {
		t.Fatalf("the first claim = %+v, %v; want it taken", first, err)
	}

	// Act
	second, err := deps.Store.ClaimAnnounce(t.Context(), made)

	// Assert
	announced, readErr := deps.Store.Announced(t.Context())
	if err != nil || second.Kind != loop.ClaimHeld || readErr != nil || len(announced) != 1 || announced[0].Made != made {
		t.Errorf("a second claim = %+v, %v, the announcements %+v, %v; want it held and %+v kept once",
			second, err, announced, readErr, made)
	}
}

func TestAnAnnouncementWhosePostDidNotGoIsGivenUp(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := standingStill().Deps()
	made := loop.Announced{Pull: 128, Moment: messaging.MomentReady}
	_, _ = deps.Store.ClaimAnnounce(t.Context(), made)

	// Act
	err := deps.Store.ReleaseAnnounce(t.Context(), made, 1)

	// Assert
	announced, readErr := deps.Store.Announced(t.Context())
	if err != nil || readErr != nil || len(announced) != 0 {
		t.Errorf("the announcements after giving one up = %+v, %v, %v; want none", announced, err, readErr)
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
