// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"sync"
	"testing"
	"testing/synctest"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/loop"
	"github.com/jacob-delgado/workflow/internal/messaging"
	"github.com/jacob-delgado/workflow/internal/webserver"
	"github.com/jacob-delgado/workflow/internal/webserver/webtest"
)

// errStoreFull is a store that could not write.
var errStoreFull = errors.New("database or disk is full")

// unconfirmed is what every surface says of an announcement posted whose
// claim the store could not confirm; the web says it alone, without why.
const unconfirmed = "The store did not confirm it; it is not offered again."

// announceMemory is the store's claims on announcements, as a fake that keeps
// them as the store does: what it held to start with, each posted; what was
// claimed since, and of those what was confirmed. A claim is taken by the
// first to ask, given up only while it is a first claim never confirmed.
type announceMemory struct {
	mu         sync.Mutex
	held       []loop.Announced
	claimed    []loop.Announced
	recorded   []loop.Announced
	reads      int
	confirmErr error
}

// wire binds deps' announcement claims, and a post that goes, over m.
func (m *announceMemory) wire(deps webserver.Deps) webserver.Deps {
	deps.Messaging.Post = func(context.Context, string, string) error { return nil }

	return m.bind(deps)
}

// bind binds deps' announcement claims over m.
func (m *announceMemory) bind(deps webserver.Deps) webserver.Deps {
	deps.Store.Announced = m.announced
	deps.Store.ClaimAnnounce = m.claim
	deps.Store.ConfirmAnnounce = m.confirm
	deps.Store.ReleaseAnnounce = m.release

	return deps
}

// announced is every claim kept: those held to start with posted, and those
// taken since posted once confirmed.
func (m *announceMemory) announced(context.Context) ([]loop.AnnounceRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.reads++

	records := make([]loop.AnnounceRecord, 0, len(m.held)+len(m.claimed))
	for _, made := range m.held {
		records = append(records, loop.AnnounceRecord{Made: made, Generation: 1, Posted: true})
	}

	for _, made := range m.claimed {
		records = append(records, loop.AnnounceRecord{
			Made: made, Generation: 1, Posted: slices.Contains(m.recorded, made),
		})
	}

	return records, nil
}

// claim takes the first claim on made, or answers the one held.
func (m *announceMemory) claim(_ context.Context, made loop.Announced) (loop.Claim, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if slices.Contains(m.held, made) || slices.Contains(m.claimed, made) {
		return loop.Claim{Kind: loop.ClaimHeld, Generation: 1, Posted: !slices.Contains(m.claimed, made)}, nil
	}

	m.claimed = append(m.claimed, made)

	return loop.Claim{Kind: loop.ClaimTaken, Generation: 1}, nil
}

// confirm records made's post, answering confirmErr, which leaves it claimed
// and unconfirmed.
func (m *announceMemory) confirm(_ context.Context, made loop.Announced, _ int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.confirmErr != nil {
		return m.confirmErr
	}

	m.recorded = append(m.recorded, made)

	return nil
}

// release gives up the claim on made, never confirmed.
func (m *announceMemory) release(_ context.Context, made loop.Announced, _ int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !slices.Contains(m.recorded, made) {
		m.claimed = slices.DeleteFunc(m.claimed, func(claimed loop.Announced) bool { return claimed == made })
	}

	return nil
}

// readyAt42 is the filled pull request, #42, announced as ready for review.
func readyAt42() loop.Announced {
	return loop.Announced{Pull: 42, Moment: messaging.MomentReady}
}

func TestAnnounceConfirmsWhatItAnnounced(t *testing.T) {
	t.Parallel()

	// Arrange
	memory := &announceMemory{}
	handler := webtest.Serve(t, memory.wire(webtest.FilledDeps()), config.Default())

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
	deps := memory.wire(webtest.FilledDeps())
	posted := 0
	deps.Messaging.Post = func(context.Context, string, string) error {
		posted++

		return nil
	}

	// Act
	recorder := postAnnounce(t, webtest.Serve(t, deps, config.Default()), map[string]string{channelField: ""})

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
		want api.ReviewAnnounced
	}{
		"announced as ready": {held: []loop.Announced{readyAt42()}, want: api.ReviewAnnouncedPosted},
		"announced only once it had merged": {
			held: []loop.Announced{{Pull: 42, Moment: messaging.MomentMerged}}, want: api.ReviewAnnouncedNone,
		},
		"another pull request announced": {
			held: []loop.Announced{{Pull: 41, Moment: messaging.MomentReady}}, want: api.ReviewAnnouncedNone,
		},
		"nothing announced": {held: nil, want: api.ReviewAnnouncedNone},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			memory := &announceMemory{held: tt.held}
			handler := webtest.Serve(t, memory.wire(webtest.FilledDeps()), config.Default())

			// Act
			snap := webtest.FirstSnapshot(t, webtest.StreamOnce(t, handler, "/api/events").Body.String())

			// Assert
			if got := snap.Review.Announced; got != tt.want {
				t.Errorf("review.announced = %q, want %q", got, tt.want)
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
	deps := memory.wire(webtest.FilledDeps())
	deps.Forge.FindPullRequest = func(context.Context, string) (forge.PullRequest, bool, error) {
		return inState(forge.StateClosed, false), true, nil
	}

	// Act
	snap := webtest.FirstSnapshot(t, webtest.StreamOnce(t, webtest.Serve(t, deps, config.Default()),
		"/api/events").Body.String())

	// Assert
	if snap.Review.Announced != api.ReviewAnnouncedNone {
		t.Errorf("review.announced = %q for a pull request closed without merging, want none", snap.Review.Announced)
	}
}

func TestAnnounceRefusesAPullRequestClosedWithoutMergingAsNoneToAnnounce(t *testing.T) {
	t.Parallel()

	// Arrange
	// Remembered as announced before it was closed, it is still none to
	// announce: not one announced already.
	memory := &announceMemory{held: []loop.Announced{readyAt42()}}
	deps := memory.wire(webtest.FilledDeps())
	deps.Forge.FindPullRequest = func(context.Context, string) (forge.PullRequest, bool, error) {
		return inState(forge.StateClosed, false), true, nil
	}

	// Act
	recorder := postAnnounce(t, webtest.Serve(t, deps, config.Default()), map[string]string{channelField: ""})

	// Assert
	assertProblem(t, recorder, http.StatusConflict, "there is no pull request to announce")
}

func TestSnapshotSeesAnAnnouncementMadeHereAtOnce(t *testing.T) {
	t.Parallel()

	// Arrange
	memory := &announceMemory{}
	handler := webtest.Serve(t, memory.wire(webtest.FilledDeps()), config.Default())
	webtest.StreamOnce(t, handler, "/api/events")

	// Act
	postAnnounce(t, handler, map[string]string{channelField: ""})

	// Assert
	snap := webtest.FirstSnapshot(t, webtest.StreamOnce(t, handler, "/api/events").Body.String())
	if snap.Review.Announced != api.ReviewAnnouncedPosted {
		t.Errorf("review.announced = %q after an announcement, want posted", snap.Review.Announced)
	}
}

func TestSnapshotReadsNoStoreUnderADryRun(t *testing.T) {
	t.Parallel()

	// Arrange
	memory := &announceMemory{held: []loop.Announced{readyAt42()}}
	deps := memory.wire(webtest.FilledDeps())
	handler := webtest.ServeWith(t, deps, config.Default(), webserver.Info{Version: webtest.TestVersion, DryRun: true})

	// Act
	webtest.StreamOnce(t, handler, "/api/events")

	// Assert
	if memory.reads != 0 {
		t.Errorf("the store was read %d times under --dry-run, want none", memory.reads)
	}
}

func TestAPullRequestAnnouncedReadyStaysAnnouncedWhileACheckFails(t *testing.T) {
	t.Parallel()

	// Arrange
	memory := &announceMemory{held: []loop.Announced{readyAt42()}}
	deps := memory.wire(webtest.FilledDeps())
	deps.Forge.CheckStatus = func(context.Context, forge.PullRequest, string) (forge.CI, error) {
		return forge.CI{State: forge.CIFailed, Total: 1, Done: 1, Failed: 1}, nil
	}

	// Act
	snap := webtest.FirstSnapshot(t, webtest.StreamOnce(t, webtest.Serve(t, deps, config.Default()),
		"/api/events").Body.String())

	// Assert
	if snap.Review.Announced != api.ReviewAnnouncedPosted {
		t.Errorf("review.announced = %q for a pull request announced as ready whose check has failed since, "+
			"want posted", snap.Review.Announced)
	}
}

func TestAPostTheStoreCouldNotConfirmIsNeverOfferedAgain(t *testing.T) {
	t.Parallel()

	// Arrange
	// It went out, and the store could not confirm it: its claim, taken
	// before the post, still stands.
	memory := &announceMemory{confirmErr: errStoreFull}
	deps := memory.wire(webtest.FilledDeps())
	posted := 0
	deps.Messaging.Post = func(context.Context, string, string) error {
		posted++

		return nil
	}
	handler := webtest.Serve(t, deps, config.Default())
	postAnnounce(t, handler, map[string]string{channelField: ""})

	// Act
	snap := webtest.FirstSnapshot(t, webtest.StreamOnce(t, handler, "/api/events").Body.String())
	again := postAnnounce(t, handler, map[string]string{channelField: ""})

	// Assert
	if snap.Review.Announced != api.ReviewAnnouncedUnconfirmed {
		t.Errorf("review.announced = %q after a post the store could not confirm, want unconfirmed",
			snap.Review.Announced)
	}

	assertProblem(t, again, http.StatusConflict, "was not confirmed")

	if code := webtest.Decode[api.Problem](t, again).Code; code != api.ProblemCodeAnnouncedUnconfirmed {
		t.Errorf("code = %q, want announced_unconfirmed", code)
	}

	if posted != 1 {
		t.Errorf("posted %d times, want once: a post not confirmed may have gone out", posted)
	}
}

func TestAnnounceTheStoreCannotConfirmIsMadeAndNoted(t *testing.T) {
	t.Parallel()

	// Arrange
	memory := &announceMemory{confirmErr: errStoreFull}
	deps := memory.wire(webtest.FilledDeps())

	var noted []string

	deps.Unexpected = func(_ config.Config, err error) { noted = append(noted, err.Error()) }

	// Act
	recorder := postAnnounce(t, webtest.Serve(t, deps, config.Default()), map[string]string{channelField: ""})

	// Assert
	wantNoted := []string{"posted, but the store did not confirm it: " + errStoreFull.Error()}
	if recorder.Code != http.StatusOK || !slices.Equal(noted, wantNoted) {
		t.Errorf("status %d, noted %q; want the announcement made and the store's failure noted as %q",
			recorder.Code, noted, wantNoted)
	}
}

func TestAnnounceWarnsOnlyOfAPostTheStoreCannotConfirm(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		confirmErr  error
		wantWarning string
	}{
		"confirmed":     {confirmErr: nil, wantWarning: ""},
		"not confirmed": {confirmErr: errStoreFull, wantWarning: unconfirmed},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			deps := (&announceMemory{confirmErr: tt.confirmErr}).wire(webtest.FilledDeps())

			// Act
			recorder := postAnnounce(t, webtest.Serve(t, deps, config.Default()), map[string]string{channelField: ""})

			// Assert
			if recorder.Code != http.StatusOK {
				t.Fatalf("status = %d (%s), want 200: the announcement was posted", recorder.Code, recorder.Body)
			}

			if got := orEmpty(webtest.Decode[api.Announcement](t, recorder).Warning); got != tt.wantWarning {
				t.Errorf("warning = %q, want %q", got, tt.wantWarning)
			}
		})
	}
}

func TestAHeldAnnouncementTheStoreCannotConfirmWarnsSo(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		// Arrange
		world := newForgeWorld()
		deps := (&announceMemory{confirmErr: errStoreFull}).bind(world.deps())

		var noted []string

		deps.Unexpected = func(_ config.Config, err error) { noted = append(noted, err.Error()) }
		handler := heldServer(t, deps, config.Default())
		announceWhenGreen(t, handler, nil)
		world.turn(func(w *forgeWorld) { w.ci = forge.CIPassed })

		// Act
		advance(ciTick)

		// Assert
		held := heldAnnouncement(t, handler)
		if held == nil || held.State != api.QueuedAnnouncementStateAnnounced || orEmpty(held.Warning) != unconfirmed {
			t.Errorf("frame's held announcement = %+v, want it announced with the warning %q", held, unconfirmed)
		}

		wantNoted := []string{"posted, but the store did not confirm it: " + errStoreFull.Error()}
		if !slices.Equal(noted, wantNoted) {
			t.Errorf("noted %q, want the store's failure noted as %q", noted, wantNoted)
		}
	})
}

func TestTheNextHoldSaysNothingOfTheLastOnesWarning(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		change func(*forgeWorld)
		want   api.QueuedAnnouncementState
	}{
		"waiting for its CI": {
			change: func(w *forgeWorld) { w.ci = forge.CIRunning }, want: api.QueuedAnnouncementStateWaiting,
		},
		"dropped once it merges first": {
			change: func(w *forgeWorld) { w.state = forge.StateMerged }, want: api.QueuedAnnouncementStateDropped,
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			synctest.Test(t, func(t *testing.T) {
				// Arrange
				world := newForgeWorld()
				deps := (&announceMemory{confirmErr: errStoreFull}).bind(world.deps())
				handler := heldServer(t, deps, config.Default())
				cancelHeld(t, handler)
				announceWhenGreen(t, handler, nil)
				world.turn(func(w *forgeWorld) { w.ci = forge.CIPassed })
				// This read posts #42, and warns of it.
				advance(ciTick)
				world.turn(func(w *forgeWorld) { w.pull, w.ci = 43, forge.CIRunning })
				announceWhenGreen(t, handler, nil)
				world.turn(tt.change)

				// Act
				advance(ciTick)

				// Assert
				held := heldAnnouncement(t, handler)
				if held == nil {
					t.Fatalf("the frame holds no announcement, want #43 %s", tt.want)
				}

				if held.Pull != 43 || held.State != tt.want || held.Warning != nil {
					t.Errorf("frame's held announcement is #%d %s, warning %q; want #43 %s with no warning: "+
						"the store failed to confirm #42, not it", held.Pull, held.State, orEmpty(held.Warning), tt.want)
				}
			})
		})
	}
}

// unreadable is the store's read of what was announced, failing.
func unreadable(context.Context) ([]loop.AnnounceRecord, error) {
	return nil, errStoreFull
}

func TestAStoreThatCannotBeReadLeavesTheClaimToDecide(t *testing.T) {
	t.Parallel()

	// Arrange
	// What was announced cannot be read, but the claim on #42 is held.
	memory := &announceMemory{held: []loop.Announced{readyAt42()}}
	deps := memory.wire(webtest.FilledDeps())
	deps.Store.Announced = unreadable
	posted := 0
	deps.Messaging.Post = func(context.Context, string, string) error {
		posted++

		return nil
	}

	var noted []string

	deps.Unexpected = func(_ config.Config, err error) { noted = append(noted, err.Error()) }

	// Act
	recorder := postAnnounce(t, webtest.Serve(t, deps, config.Default()), map[string]string{channelField: ""})

	// Assert
	assertProblem(t, recorder, http.StatusConflict, "already announced")

	if posted != 0 || !slices.Contains(noted, errStoreFull.Error()) {
		t.Errorf("posted %d times, noted %q; want nothing posted and the store's failure noted", posted, noted)
	}
}

func TestAFrameOverAStoreThatCannotBeReadSaysItIsUnknown(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := (&announceMemory{held: []loop.Announced{readyAt42()}}).wire(webtest.FilledDeps())
	deps.Store.Announced = unreadable

	var noted []string

	deps.Unexpected = func(_ config.Config, err error) { noted = append(noted, err.Error()) }

	// Act
	snap := webtest.FirstSnapshot(t, webtest.StreamOnce(t, webtest.Serve(t, deps, config.Default()),
		"/api/events").Body.String())

	// Assert
	if snap.Review.Announced != api.ReviewAnnouncedUnknown || !slices.Contains(noted, errStoreFull.Error()) {
		t.Errorf("review.announced = %q, noted %q; want it unknown and the store's failure noted",
			snap.Review.Announced, noted)
	}
}
