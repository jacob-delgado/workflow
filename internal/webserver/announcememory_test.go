// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

import (
	"errors"
	"net/http"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/loop"
	"github.com/jacob-delgado/workflow/internal/messaging"
	"github.com/jacob-delgado/workflow/internal/webserver"
)

// errStoreFull is a store that could not write.
var errStoreFull = errors.New("database or disk is full")

// notRemembered is what every surface says of an announcement posted but not
// remembered; the web says it alone, without why.
const notRemembered = "Posted, but not remembered: it may be offered again."

// announceMemory is the store's record of what was announced, as a fake: what
// it held to start with, and what was recorded since.
type announceMemory struct {
	mu       sync.Mutex
	held     []loop.Announced
	recorded []loop.Announced
	reads    int
}

// wire binds deps' announcement memory, and a post that goes, over m.
func (m *announceMemory) wire(deps webserver.Deps) webserver.Deps {
	deps.Messaging.Post = func(string, string) error { return nil }
	deps.Store.Announced = func() []loop.Announced {
		m.mu.Lock()
		defer m.mu.Unlock()

		m.reads++

		return slices.Concat(m.held, m.recorded)
	}
	deps.Store.RecordAnnounce = func(made loop.Announced) error {
		m.mu.Lock()
		defer m.mu.Unlock()

		m.recorded = append(m.recorded, made)

		return nil
	}

	return deps
}

// readyAt42 is the filled pull request, #42, announced as ready for review.
func readyAt42() loop.Announced {
	return loop.Announced{Pull: 42, Moment: messaging.MomentReady}
}

func TestAnnounceRecordsWhatItAnnounced(t *testing.T) {
	t.Parallel()

	// Arrange
	memory := &announceMemory{}
	handler := serve(t, memory.wire(filledDeps()), config.Default())

	// Act
	recorder := postAnnounce(t, handler, map[string]string{channelField: ""})

	// Assert
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", recorder.Code, recorder.Body)
	}

	if !slices.Equal(memory.recorded, []loop.Announced{readyAt42()}) {
		t.Errorf("recorded %v, want #42 as ready for review, so the terminal does not offer it again", memory.recorded)
	}
}

func TestAnnounceRefusesWhatWasAlreadyAnnounced(t *testing.T) {
	t.Parallel()

	// Arrange
	memory := &announceMemory{held: []loop.Announced{readyAt42()}}
	deps := memory.wire(filledDeps())
	posted := 0
	deps.Messaging.Post = func(string, string) error {
		posted++

		return nil
	}

	// Act
	recorder := postAnnounce(t, serve(t, deps, config.Default()), map[string]string{channelField: ""})

	// Assert
	assertProblem(t, recorder, http.StatusConflict, "already announced")

	if posted != 0 {
		t.Errorf("posted %d times, want none: the terminal does not announce a moment twice", posted)
	}
}

func TestSnapshotSaysWhetherThePullWasAnnouncedAtItsMoment(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		held []loop.Announced
		want bool
	}{
		"announced as ready": {held: []loop.Announced{readyAt42()}, want: true},
		"announced only once it had merged": {
			held: []loop.Announced{{Pull: 42, Moment: messaging.MomentMerged}}, want: false,
		},
		"another pull request announced": {
			held: []loop.Announced{{Pull: 41, Moment: messaging.MomentReady}}, want: false,
		},
		"nothing announced": {held: nil, want: false},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			memory := &announceMemory{held: tt.held}
			handler := serve(t, memory.wire(filledDeps()), config.Default())

			// Act
			snap := firstSnapshot(t, streamOnce(t, handler, "/api/events").Body.String())

			// Assert
			if got := snap.Review.Announced; got != tt.want {
				t.Errorf("review.announced = %t, want %t", got, tt.want)
			}
		})
	}
}

func TestSnapshotReadsAPullRequestClosedWithoutMergingAsNotAnnounced(t *testing.T) {
	t.Parallel()

	// Arrange
	// The store remembers pull request 42 announced as ready, before it was
	// closed without merging: the loop follows no pull request now, so the
	// review says so as the announce stage does.
	memory := &announceMemory{held: []loop.Announced{readyAt42()}}
	deps := memory.wire(filledDeps())
	deps.Forge.FindPullRequest = func(string) (forge.PullRequest, bool, error) {
		return inState(forge.StateClosed, false), true, nil
	}

	// Act
	snap := firstSnapshot(t, streamOnce(t, serve(t, deps, config.Default()), "/api/events").Body.String())

	// Assert
	if snap.Review.Announced {
		t.Error("review.announced reads a pull request closed without merging as announced")
	}
}

func TestAnnounceRefusesAPullRequestClosedWithoutMergingAsNoneToAnnounce(t *testing.T) {
	t.Parallel()

	// Arrange
	// Remembered as announced before it was closed, it is still none to
	// announce: not one announced already.
	memory := &announceMemory{held: []loop.Announced{readyAt42()}}
	deps := memory.wire(filledDeps())
	deps.Forge.FindPullRequest = func(string) (forge.PullRequest, bool, error) {
		return inState(forge.StateClosed, false), true, nil
	}

	// Act
	recorder := postAnnounce(t, serve(t, deps, config.Default()), map[string]string{channelField: ""})

	// Assert
	assertProblem(t, recorder, http.StatusConflict, "there is no pull request to announce")
}

func TestSnapshotSeesAnAnnouncementMadeHereAtOnce(t *testing.T) {
	t.Parallel()

	// Arrange
	memory := &announceMemory{}
	handler := serve(t, memory.wire(filledDeps()), config.Default())
	streamOnce(t, handler, "/api/events")

	// Act
	postAnnounce(t, handler, map[string]string{channelField: ""})

	// Assert
	snap := firstSnapshot(t, streamOnce(t, handler, "/api/events").Body.String())
	if !snap.Review.Announced {
		t.Error("the frame after an announcement does not say it was made")
	}
}

func TestSnapshotReadsNoStoreUnderADryRun(t *testing.T) {
	t.Parallel()

	// Arrange
	memory := &announceMemory{held: []loop.Announced{readyAt42()}}
	deps := memory.wire(filledDeps())
	handler := serveWith(t, deps, config.Default(), webserver.Info{Version: testVersion, DryRun: true})

	// Act
	streamOnce(t, handler, "/api/events")

	// Assert
	if memory.reads != 0 {
		t.Errorf("the store was read %d times under --dry-run, want none", memory.reads)
	}
}

func TestAnnouncedMomentFollowsTheCI(t *testing.T) {
	t.Parallel()

	// Arrange
	memory := &announceMemory{held: []loop.Announced{{Pull: 42, Moment: messaging.MomentCIRed}}}
	deps := memory.wire(filledDeps())
	deps.Forge.CheckStatus = func(forge.PullRequest, string) (forge.CI, error) {
		return forge.CI{State: forge.CIFailed, Total: 1, Done: 1, Failed: 1}, nil
	}

	// Act
	snap := firstSnapshot(t, streamOnce(t, serve(t, deps, config.Default()), "/api/events").Body.String())

	// Assert
	if !snap.Review.Announced {
		t.Error("a pull request announced with its CI red, still red, does not read as announced")
	}
}

func TestTheStoreIsReadAgainOnceTheForgeIntervalHasPassed(t *testing.T) {
	t.Parallel()

	// Arrange
	memory := &announceMemory{}
	deps := memory.wire(filledDeps())
	now := time.Date(2026, 10, 6, 9, 0, 0, 0, time.UTC)
	deps.Clock = func() time.Time { return now }
	handler := serve(t, deps, config.Default())

	streamOnce(t, handler, "/api/events")
	streamOnce(t, handler, "/api/events")

	// Act
	now = now.Add(time.Minute)

	streamOnce(t, handler, "/api/events")

	// Assert
	if memory.reads != 2 {
		t.Errorf("the store was read %d times, want once, then again once the interval passed", memory.reads)
	}
}

func TestAnnounceTheStoreCannotRememberIsMadeAndNoted(t *testing.T) {
	t.Parallel()

	// Arrange
	memory := &announceMemory{}
	deps := memory.wire(filledDeps())
	deps.Store.RecordAnnounce = func(loop.Announced) error { return errStoreFull }

	var noted []string

	deps.Unexpected = func(_ config.Config, err error) { noted = append(noted, err.Error()) }

	// Act
	recorder := postAnnounce(t, serve(t, deps, config.Default()), map[string]string{channelField: ""})

	// Assert
	wantNoted := []string{"posted, but not remembered: it may be offered again: " + errStoreFull.Error()}
	if recorder.Code != http.StatusOK || !slices.Equal(noted, wantNoted) {
		t.Errorf("status %d, noted %q; want the announcement made and the store's failure noted as %q",
			recorder.Code, noted, wantNoted)
	}
}

func TestAnnounceWarnsOnlyOfAPostTheStoreCannotRemember(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		recordErr   error
		wantWarning string
	}{
		"remembered":     {recordErr: nil, wantWarning: ""},
		"not remembered": {recordErr: errStoreFull, wantWarning: notRemembered},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			deps := (&announceMemory{}).wire(filledDeps())
			deps.Store.RecordAnnounce = func(loop.Announced) error { return tt.recordErr }

			// Act
			recorder := postAnnounce(t, serve(t, deps, config.Default()), map[string]string{channelField: ""})

			// Assert
			if recorder.Code != http.StatusOK {
				t.Fatalf("status = %d (%s), want 200: the announcement was posted", recorder.Code, recorder.Body)
			}

			if got := orEmpty(decode[api.Announcement](t, recorder).Warning); got != tt.wantWarning {
				t.Errorf("warning = %q, want %q", got, tt.wantWarning)
			}
		})
	}
}

func TestAHeldAnnouncementTheStoreCannotRememberWarnsSo(t *testing.T) {
	t.Parallel()

	// Arrange
	world := newForgeWorld()
	deps := world.deps()
	deps.Store.RecordAnnounce = func(loop.Announced) error { return errStoreFull }
	handler := serve(t, deps, config.Default())
	announceWhenGreen(t, handler, nil)
	world.turn(func(w *forgeWorld) { w.ci = forge.CIPassed })

	// Act
	held := heldAnnouncement(t, handler)

	// Assert
	if held == nil || held.State != api.QueuedAnnouncementStateAnnounced || orEmpty(held.Warning) != notRemembered {
		t.Errorf("frame's held announcement = %+v, want it announced with the warning %q", held, notRemembered)
	}
}

func TestTheNextHoldSaysNothingOfTheLastOnesWarning(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		ci   forge.CIState
		want api.QueuedAnnouncementState
	}{
		"waiting for its CI":        {ci: forge.CIRunning, want: api.QueuedAnnouncementStateWaiting},
		"dropped once its CI fails": {ci: forge.CIFailed, want: api.QueuedAnnouncementStateDropped},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			world := newForgeWorld()
			deps := world.deps()
			deps.Store.RecordAnnounce = func(loop.Announced) error { return errStoreFull }
			handler := serve(t, deps, config.Default())
			cancelHeld(t, handler)
			announceWhenGreen(t, handler, nil)
			world.turn(func(w *forgeWorld) { w.ci = forge.CIPassed })
			// A frame settles the hold: this one posts #42, and warns of it.
			heldAnnouncement(t, handler)
			world.turn(func(w *forgeWorld) { w.pull, w.ci = 43, forge.CIRunning })
			announceWhenGreen(t, handler, nil)
			world.turn(func(w *forgeWorld) { w.ci = tt.ci })

			// Act
			held := heldAnnouncement(t, handler)

			// Assert
			if held == nil {
				t.Fatalf("the frame holds no announcement, want #43 %s", tt.want)
			}

			if held.Pull != 43 || held.State != tt.want || held.Warning != nil {
				t.Errorf("frame's held announcement is #%d %s, warning %q; want #43 %s with no warning: "+
					"the store failed to remember #42, not it", held.Pull, held.State, orEmpty(held.Warning), tt.want)
			}
		})
	}
}
