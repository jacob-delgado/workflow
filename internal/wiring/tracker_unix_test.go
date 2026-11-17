// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

//go:build unix

package wiring_test

// Every page of the Issues list shares the count of forge issues its first
// page led with, which a later page skips past in Jira's own rows. These hold
// a first page in the forge's list through a gh that never answers, or
// answers only once let go, which says it has begun through a named pipe,
// and only Unix makes one.

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
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

// twoBugsList is the forge's assigned-issue search with two issues.
const twoBugsList = `{"items":[` +
	`{"number":42,"html_url":"https://github.com/owner/repo/issues/42","title":"the bug"},` +
	`{"number":43,"html_url":"https://github.com/owner/repo/issues/43","title":"another"}]}`

// heldFirstGH is a stand-in gh whose first run writes to begun, waits for
// release to be written, then answers first; every later run answers later
// at once. Neither answer holds a single quote.
type heldFirstGH struct {
	begun, release string
}

// installHeldFirstGH puts a heldFirstGH on PATH.
func installHeldFirstGH(t *testing.T, first, later string) heldFirstGH {
	t.Helper()

	dir := t.TempDir()
	held := heldFirstGH{begun: filepath.Join(dir, "begun"), release: filepath.Join(dir, "release")}

	for _, pipe := range []string{held.begun, held.release} {
		err := syscall.Mkfifo(pipe, 0o600)
		if err != nil {
			t.Fatalf("making the pipe %s: %v", pipe, err)
		}
	}

	answer := "printf 'HTTP/1.1 200 OK\\r\\nContent-Type: application/json\\r\\n\\r\\n%s' '"
	write(t, filepath.Join(dir, "gh"), "#!/bin/sh\n"+
		"if [ ! -e '"+dir+"/held' ]; then : > '"+dir+"/held'\n"+
		"  printf x > '"+held.begun+"'\n"+
		"  cat '"+held.release+"' > /dev/null\n"+
		"  "+answer+first+"'\n"+
		"  exit 0\n"+
		"fi\n"+
		answer+later+"'\n", 0o755)
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	return held
}

// answer lets the held run answer.
func (g heldFirstGH) answer(t *testing.T) {
	t.Helper()

	err := os.WriteFile(g.release, []byte("x"), 0o600)
	if err != nil {
		t.Fatalf("letting gh answer: %v", err)
	}
}

// Two first pages are listed at once, and the one asked first is answered
// last: a later page skips the forge rows of the one asked last, which the
// list on screen shows.
func TestALaterPageSkipsTheForgeRowsOfTheFirstPageAskedLast(t *testing.T) {
	// Arrange
	standIn := installHeldFirstGH(t, theBugList, twoBugsList)
	stand := &fakeJira{}
	tracker := bothTrackers(t, stand)

	var askedFirst sync.WaitGroup

	askedFirst.Go(func() { _, _ = tracker.Search(t.Context(), jira.AssignedToMe, 0) })
	awaitBegun(t, standIn.begun)

	_, err := tracker.Search(t.Context(), jira.AssignedToMe, 0)
	if err != nil {
		t.Fatalf("the first page asked last, led by two forge issues: %v", err)
	}

	standIn.answer(t)
	askedFirst.Wait()

	// Act
	_, err = tracker.Search(t.Context(), jira.AssignedToMe, 50)

	// Assert
	asked := stand.requests()
	if err != nil || !strings.Contains(asked[len(asked)-1], "startAt=48") {
		t.Errorf("Search = %v; Jira was last asked %q, want from 48, past the two forge rows", err, asked[len(asked)-1])
	}
}
