// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

import (
	"context"
	"net/http"
	"path/filepath"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/webserver"
	"github.com/jacob-delgado/workflow/internal/webserver/webtest"
)

// The titles the pull request opened here, #7, has as the forge's list has it
// and as the forge reads it by its number, so a frame tells which it shows;
// openedTitle, openableDeps' own, is the title its open answered.
const (
	openedTitle = "opened"
	listedTitle = "as the list has it"
	readTitle   = "as read by its number"
)

// listingWorld is a forge whose list of the branch's pull requests, and whose
// read of one by its number, a test sets, on a clock the test moves; each is
// behind a lock, since the server reads them as its frames ask.
type listingWorld struct {
	mu  sync.Mutex
	now time.Time
	// listed and listedFound are what the list finds for the branch, or
	// listErr how it fails.
	listed      forge.PullRequest
	listedFound bool
	listErr     error
	// read is what reading a pull request by its number answers, or readErr
	// how it fails; readAsked is each number it was asked for.
	read      forge.PullRequest
	readErr   error
	readAsked []int
}

// newListingWorld is a forge whose list finds no pull request for the branch,
// and which reads #7 by its number, open.
func newListingWorld() *listingWorld {
	return &listingWorld{
		now:  webtest.PaceStart(),
		read: forge.PullRequest{Number: 7, URL: webtest.PRURL, Title: readTitle, State: forge.StateOpen},
	}
}

// deps is openableDeps, which opens #7, reading the world's forge and clock.
func (w *listingWorld) deps() webserver.Deps {
	deps := openableDeps()
	deps.Clock = func() time.Time {
		w.mu.Lock()
		defer w.mu.Unlock()

		return w.now
	}
	deps.Forge.FindPullRequest = func(context.Context, string) (forge.PullRequest, bool, error) {
		w.mu.Lock()
		defer w.mu.Unlock()

		return w.listed, w.listedFound, w.listErr
	}
	deps.Forge.ReadPullRequest = func(_ context.Context, number int) (forge.PullRequest, error) {
		w.mu.Lock()
		defer w.mu.Unlock()

		w.readAsked = append(w.readAsked, number)

		return w.read, w.readErr
	}

	return deps
}

// later changes the world as change says, then moves its clock past the
// forge's interval, so the next frame asks the forge again.
func (w *listingWorld) later(change func(*listingWorld)) {
	w.mu.Lock()
	defer w.mu.Unlock()

	change(w)
	w.now = w.now.Add(time.Hour)
}

// asked is each number the forge was asked to read a pull request by.
func (w *listingWorld) asked() []int {
	w.mu.Lock()
	defer w.mu.Unlock()

	return slices.Clone(w.readAsked)
}

// openSeven opens #7 against handler.
func openSeven(t *testing.T, handler http.Handler) {
	t.Helper()

	opened := webtest.Send(t, handler, http.MethodPost, "/api/pull-request", openRequestBody)
	if opened.Code != http.StatusOK {
		t.Fatalf("opening the pull request: status = %d (%s), want 200", opened.Code, opened.Body.String())
	}
}

// nextFrame is the frame a stream opened now is sent first.
func nextFrame(t *testing.T, handler http.Handler) api.Snapshot {
	t.Helper()

	return webtest.FirstSnapshot(t, webtest.StreamOnce(t, handler, "/api/events").Body.String())
}

// shownTitle is the title of the pull request the frame shows, or "" for none.
func shownTitle(snap api.Snapshot) string {
	if !snap.Review.Found || snap.Review.Pull == nil {
		return ""
	}

	return snap.Review.Pull.Title
}

func TestAPullOpenedHereIsReadByItsNumberUntilTheForgesListIncludesIt(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		change    func(*listingWorld)
		wantTitle string
		wantAsked []int
	}{
		"the list finds none": {
			change:    func(*listingWorld) {},
			wantTitle: readTitle, wantAsked: []int{7},
		},
		"the list finds an older pull of the branch's name, merged": {
			change: func(w *listingWorld) {
				w.listed = forge.PullRequest{Number: 3, Title: listedTitle, State: forge.StateMerged}
				w.listedFound = true
			},
			wantTitle: readTitle, wantAsked: []int{7},
		},
		"the list cannot be read": {
			change:    func(w *listingWorld) { w.listErr = webtest.ErrSeam },
			wantTitle: readTitle, wantAsked: []int{7},
		},
		"the list includes it": {
			change: func(w *listingWorld) {
				w.listed = forge.PullRequest{Number: 7, Title: listedTitle, State: forge.StateOpen}
				w.listedFound = true
			},
			wantTitle: listedTitle, wantAsked: nil,
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			world := newListingWorld()
			handler := webtest.Serve(t, world.deps(), config.Default())
			openSeven(t, handler)
			world.later(tt.change)

			// Act
			frame := nextFrame(t, handler)

			// Assert
			if title := shownTitle(frame); title != tt.wantTitle {
				t.Errorf("the frame an interval after the open shows %q (%+v), want %q", title, frame.Review,
					tt.wantTitle)
			}

			if asked := world.asked(); !slices.Equal(asked, tt.wantAsked) {
				t.Errorf("the forge was asked to read the pull requests %v by number, want %v", asked, tt.wantAsked)
			}
		})
	}
}

func TestAPullOpenedHereStaysShownWhenItsReadByNumberFails(t *testing.T) {
	t.Parallel()

	// Arrange
	world := newListingWorld()
	handler := webtest.Serve(t, world.deps(), config.Default())
	openSeven(t, handler)
	world.later(func(w *listingWorld) { w.readErr = webtest.ErrSeam })

	// Act
	frame := nextFrame(t, handler)

	// Assert
	if title := shownTitle(frame); title != openedTitle {
		t.Errorf("the frame shows %q (%+v), want #7 as its open answered, %q", title, frame.Review, openedTitle)
	}

	if frame.Problems == nil || frame.Problems.Review == nil {
		t.Errorf("the frame's problems = %+v, want the failed read said beside the pull request", frame.Problems)
	}
}

func TestAPullOpenedHereStaysShownWithNoReadByNumber(t *testing.T) {
	t.Parallel()

	// Arrange
	world := newListingWorld()
	deps := world.deps()
	deps.Forge.ReadPullRequest = nil
	handler := webtest.Serve(t, deps, config.Default())
	openSeven(t, handler)
	world.later(func(*listingWorld) {})

	// Act
	frame := nextFrame(t, handler)

	// Assert
	if title := shownTitle(frame); title != openedTitle {
		t.Errorf("the frame shows %q (%+v), want #7 as its open answered, %q", title, frame.Review, openedTitle)
	}
}

func TestAPullOpenedHereIsNotReadByItsNumberOnceTheListIncludedIt(t *testing.T) {
	t.Parallel()

	// Arrange
	world := newListingWorld()
	handler := webtest.Serve(t, world.deps(), config.Default())
	openSeven(t, handler)
	world.later(func(w *listingWorld) {
		w.listed = forge.PullRequest{Number: 7, Title: listedTitle, State: forge.StateOpen}
		w.listedFound = true
	})
	nextFrame(t, handler)

	// The list finds none now, as it does once a pull request is closed.
	world.later(func(w *listingWorld) { w.listed, w.listedFound = forge.PullRequest{}, false })

	// Act
	frame := nextFrame(t, handler)

	// Assert
	if title := shownTitle(frame); title != "" {
		t.Errorf("the frame shows %q (%+v), want none, as the list says", title, frame.Review)
	}

	if asked := world.asked(); len(asked) != 0 {
		t.Errorf("the forge was asked to read the pull requests %v by number, want none once listed", asked)
	}
}

func TestAPullOpenedHereAndClosedOnTheForgeIsNoLongerShown(t *testing.T) {
	t.Parallel()

	// Arrange
	world := newListingWorld()
	handler := webtest.Serve(t, world.deps(), config.Default())
	openSeven(t, handler)
	world.later(func(w *listingWorld) { w.read.State = forge.StateClosed })

	// Act: a frame reads it closed
	closed := nextFrame(t, handler)

	// Assert: it shows none, as the list passes over a closed pull request
	if title := shownTitle(closed); title != "" {
		t.Errorf("the frame shows %q (%+v), want none for a pull request closed unmerged", title, closed.Review)
	}

	// Act: a frame an interval later
	world.later(func(*listingWorld) {})
	nextFrame(t, handler)

	// Assert: the closed pull request is not read again
	if asked := world.asked(); !slices.Equal(asked, []int{7}) {
		t.Errorf("the forge was asked to read the pull requests %v by number, want #7 once", asked)
	}
}

func TestSavedForgeSettingsForgetAPullOpenedHereUnderTheOldOnes(t *testing.T) {
	t.Parallel()

	// Arrange
	// #7 is a number on the forge the old settings point at; under the new
	// ones it may name another pull request, or none.
	world := newListingWorld()
	deps := world.deps()
	deps.UseForgeSettings = (&settingsRecorder{kind: forge.KindGitHub}).use
	cfg := config.Default()
	cfg.Path = filepath.Join(t.TempDir(), config.FileName)
	handler := webtest.ServeWith(t, deps, cfg, webserver.Info{Version: webtest.TestVersion, ForgeKind: forge.KindGitHub})
	openSeven(t, handler)

	next := cfg
	next.Forge = config.Forge{Token: "ghp-saved-in-settings", Host: savedHost}

	if saved := webtest.PutConfig(t, handler, webtest.Marshal(t, next)); saved.Code != http.StatusOK {
		t.Fatalf("the save answered %d, want 200: %s", saved.Code, saved.Body.String())
	}

	// Act
	frame := nextFrame(t, handler)

	// Assert
	if title := shownTitle(frame); title != "" {
		t.Errorf("the frame after the save shows %q (%+v), want none, as the list says", title, frame.Review)
	}

	if asked := world.asked(); len(asked) != 0 {
		t.Errorf("the forge was asked to read the pull requests %v by number, want none after the save", asked)
	}
}
