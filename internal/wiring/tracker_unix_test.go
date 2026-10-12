// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

//go:build unix

package wiring_test

// Every page of the Issues list shares the count of forge issues its first
// page led with, which a later page skips past in Jira's own rows. These hold
// a first page in the forge's list through a gh that never answers, which
// says it has begun through a named pipe, and only Unix makes one.

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/jira"
)

// A first page whose page left while the forge's issues were being listed
// counted none of them, and every later page, another page's too, then asked
// Jira from a row its list had already shown.
func TestAFirstPageThatLeftLeavesTheForgeRowsALaterPageSkips(t *testing.T) {
	// Arrange
	begun := installGHSilentAfter(t, theBugList)
	stand := &fakeJira{}
	tracker := bothTrackers(t, stand)

	_, err := tracker.Search(t.Context(), jira.AssignedToMe, 0)
	if err != nil {
		t.Fatalf("the first page, led by one forge issue: %v", err)
	}

	leaving, leave := context.WithCancel(t.Context())
	listed := make(chan struct{})

	go func() {
		defer close(listed)

		_, _ = tracker.Search(leaving, jira.AssignedToMe, 0)
	}()

	awaitBegun(t, begun)
	leave()

	select {
	case <-listed:
	case <-time.After(stopWait):
		t.Fatal("the first page of a page that left was still listing the forge's issues")
	}

	// Act
	// Two rows are listed, one of them the forge's, so Jira has given one.
	_, err = tracker.Search(t.Context(), jira.AssignedToMe, 2)

	// Assert
	asked := stand.requests()
	if err != nil || !strings.Contains(asked[len(asked)-1], "startAt=1") {
		t.Errorf("Search = %v; Jira was last asked %q, want from its second row", err, asked[len(asked)-1])
	}
}
