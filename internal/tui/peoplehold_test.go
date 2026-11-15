// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/tui"
)

// After a re-read of the people fails, what People lists may have changed
// since it was read, so it decides nothing about anyone until a read
// succeeds, and offers r to read them again.

// peopleRereadFailed is People with carla marked not on Slack and the read
// of everyone after it failed, the cursor still on the first owner listed.
func peopleRereadFailed(t *testing.T, refusing *world) tui.Model {
	t.Helper()

	opened := openPeople(t, refusing)
	refusing.mu.Lock()
	refusing.slack.linksErr = errDirectoryDown
	refusing.mu.Unlock()

	return typing(t, opened, "x")
}

func TestPeopleDecidesNothingAfterAFailedReread(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		keys  []string
		calls string
		shown string
	}{
		"enter opens no picker":   {keys: []string{keyEnter}, calls: "link-owner ", shown: "Choose whom"},
		"x marks nobody":          {keys: []string{"x"}, calls: "link-owner ", shown: "saving"},
		"d asks to forget no one": {keys: []string{"d", keyEnter}, calls: "forget-owner ", shown: "Forget a person"},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			refusing := taggingWorld()
			failed := peopleRereadFailed(t, refusing)
			before := len(refusing.asked(tt.calls))

			// Act
			view := typing(t, failed, tt.keys...).View().Content

			// Assert
			if calls := refusing.asked(tt.calls); len(calls) != before {
				t.Errorf("%s after a failed re-read sent %q", name, calls[before:])
			}

			requireScreen(t, view, "slack is down")
			refuseScreen(t, view, tt.shown)
		})
	}
}

func TestPeopleOffersToTryAgainAfterAFailedReread(t *testing.T) {
	t.Parallel()

	// Arrange
	refusing := taggingWorld()

	// Act
	footer := footerLine(peopleRereadFailed(t, refusing).View().Content)

	// Assert
	if !strings.Contains(footer, "r try again") || strings.Contains(footer, "change") {
		t.Errorf("the footer offers %q; want r to try again, and no change", footer)
	}
}

func TestTryingAgainReadsThePeopleAndFreesTheirDecisions(t *testing.T) {
	t.Parallel()

	// Arrange
	refusing := taggingWorld()
	failed := peopleRereadFailed(t, refusing)
	refusing.mu.Lock()
	refusing.slack.linksErr = nil
	refusing.mu.Unlock()
	marks := len(refusing.asked("link-owner "))

	// Act: try again
	retried := typing(t, failed, "r")

	// Assert: everyone is read again
	refuseScreen(t, retried.View().Content, "slack is down")

	// Act: mark the first owner not on Slack
	typing(t, retried, "x")

	// Assert: it is saved
	if calls := refusing.asked("link-owner "); len(calls) != marks+1 {
		t.Errorf("x once read again sent %q; want one more link saved", calls[marks:])
	}
}
