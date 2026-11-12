// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/httpx"
	"github.com/jacob-delgado/workflow/internal/messaging"
	"github.com/jacob-delgado/workflow/internal/webserver"
	"github.com/jacob-delgado/workflow/internal/webserver/webtest"
)

// heldServer serves deps under cfg, the held announcement read on ciTick, the
// CI interval, for a test on a fake clock.
func heldServer(t *testing.T, deps webserver.Deps, cfg config.Config) http.Handler {
	t.Helper()

	cfg.Timing.CIInterval = ciTick.String()

	return webtest.Serve(t, deps, cfg)
}

func TestAPostNowThatFailsKeepsTheHeldAnnouncement(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		// Arrange
		// #42's announcement waits for CI, and the service refuses the post
		// asked for now.
		world := newForgeWorld()
		handler := heldServer(t, world.deps(), config.Default())
		cancelHeld(t, handler)
		announceWhenGreen(t, handler, nil)
		world.turn(func(w *forgeWorld) { w.fail = fmt.Errorf("%w: 403", messaging.ErrRejected) })

		// Act: announce now
		refused := postAnnounce(t, handler, map[string]string{channelField: slackChannel})

		// Assert: refused, and the held one still waits
		if refused.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status = %d (%s), want 422 for a post the service refused", refused.Code, refused.Body.String())
		}

		if held := heldAnnouncement(t, handler); !sameHold(held, waitingOn(api.CIStateRunning)) {
			t.Fatalf("frame's held announcement = %+v, want #42 still waiting", held)
		}

		// Act: the service takes posts again, and CI passes
		world.turn(func(w *forgeWorld) { w.fail, w.ci = nil, forge.CIPassed })
		advance(ciTick)

		// Assert: the held one goes, once
		if held := heldAnnouncement(t, handler); held == nil || held.State != api.QueuedAnnouncementStateAnnounced ||
			len(world.posted()) != 1 {
			t.Errorf("frame's held announcement = %+v, posted %q; want it announced once", held, world.posted())
		}
	})
}

func TestStoppingADroppedAnnouncementChangesNothing(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		// Arrange
		// #42 merged before its CI passed, so its announcement was dropped.
		world := newForgeWorld()
		handler := heldServer(t, world.deps(), config.Default())
		announceWhenGreen(t, handler, nil)
		world.turn(func(w *forgeWorld) { w.state = forge.StateMerged })
		advance(ciTick)

		// Act
		recorder := webtest.Send(t, handler, http.MethodDelete, queuedPath, "")

		// Assert
		assertProblem(t, recorder, http.StatusConflict, "no announcement is waiting")

		dropped := api.QueuedAnnouncement{
			State: api.QueuedAnnouncementStateDropped, Channel: slackChannel, Pull: 42,
			Reason: new("#42 merged before its CI passed"),
		}
		if held := heldAnnouncement(t, handler); !sameHold(held, dropped) {
			t.Errorf("frame's held announcement = %+v, want it still dropped with its reason", held)
		}
	})
}

func TestOpeningAPullRequestNeverReplacesAnotherWaitingAnnouncement(t *testing.T) {
	t.Parallel()

	// Arrange
	// #42's announcement waits for CI when #43 opens.
	world := newForgeWorld()
	deps := openingDeps(world, false)
	world.gone = false
	handler := webtest.Serve(t, deps, webhookConfig())
	cancelHeld(t, handler)
	announceWhenGreen(t, handler, nil)
	world.turn(func(w *forgeWorld) { w.gone, w.pull = true, 43 })

	// Act
	answer := opened(t, handler)

	// Assert
	notArmed := "#42's announcement is still waiting, so #43's was not armed"
	if answer.Announcement != nil || orEmpty(answer.NotArmed) != notArmed {
		t.Errorf("announcement = %+v, not armed %q; want none armed, saying %q",
			answer.Announcement, orEmpty(answer.NotArmed), notArmed)
	}

	if held := heldAnnouncement(t, handler); held == nil || held.Pull != 42 ||
		held.State != api.QueuedAnnouncementStateWaiting {
		t.Errorf("frame's held announcement = %+v, want #42's still waiting", held)
	}
}

func TestHoldingAnotherPullRequestsAnnouncementReplacesTheOneWaitingAndSaysSo(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		// Arrange
		// #42's announcement waits for CI when the branch's pull request is
		// #43.
		world := newForgeWorld()
		handler := heldServer(t, world.deps(), config.Default())
		cancelHeld(t, handler)
		announceWhenGreen(t, handler, nil)
		world.turn(func(w *forgeWorld) { w.pull = 43 })

		// Act: hold #43's
		recorder := announceWhenGreen(t, handler, nil)

		// Assert: #43's is held, replacing #42's
		if recorder.Code != http.StatusAccepted {
			t.Fatalf("status = %d (%s), want 202", recorder.Code, recorder.Body.String())
		}

		held := webtest.Decode[api.QueuedAnnouncement](t, recorder)
		if held.Pull != 43 || held.ReplacedPull == nil || *held.ReplacedPull != 42 {
			t.Errorf("answer = %+v, want #43 held, replacing #42's", held)
		}

		// Act: the reads set for each come due, and #43's CI passes
		world.turn(func(w *forgeWorld) { w.ci = forge.CIPassed })
		advance(ciTick)

		// Assert: #43's goes, once
		if held := heldAnnouncement(t, handler); held == nil || held.Pull != 43 ||
			held.State != api.QueuedAnnouncementStateAnnounced || len(world.posted()) != 1 {
			t.Errorf("frame's held announcement = %+v, posted %q; want #43 announced once", held, world.posted())
		}
	})
}

func TestTwoServersOverOneStorePostAHeldAnnouncementOnce(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		// Arrange
		// Two servers, as two browsers' workflow --web, hold #42's
		// announcement over one store.
		world, memory := newForgeWorld(), &announceMemory{}
		deps := memory.bind(world.deps())
		first, second := heldServer(t, deps, config.Default()), heldServer(t, deps, config.Default())
		announceWhenGreen(t, first, nil)
		announceWhenGreen(t, second, nil)
		world.turn(func(w *forgeWorld) { w.ci = forge.CIPassed })

		// Act
		advance(ciTick)

		// Assert
		states := []api.QueuedAnnouncementState{heldAnnouncement(t, first).State, heldAnnouncement(t, second).State}
		if len(world.posted()) != 1 || !sameStates(states, api.QueuedAnnouncementStateAnnounced,
			api.QueuedAnnouncementStateElsewhere) {
			t.Errorf("posted %q, held announcements %v; want one post, one announced and one elsewhere",
				world.posted(), states)
		}
	})
}

// sameStates reports whether states are want, in either order.
func sameStates(states []api.QueuedAnnouncementState, want ...api.QueuedAnnouncementState) bool {
	return len(states) == 2 && len(want) == 2 &&
		(states[0] == want[0] && states[1] == want[1] || states[0] == want[1] && states[1] == want[0])
}

// heldFinds holds each find of a pull request, once holding is on, until the
// test releases it, saying as each begins.
type heldFinds struct {
	holding atomic.Bool
	began   chan struct{}
	release chan struct{}
}

// holdFindsIn is deps, each find of a pull request held by the heldFinds
// answered once its holding is on.
func holdFindsIn(deps webserver.Deps) (webserver.Deps, *heldFinds) {
	finds := &heldFinds{began: make(chan struct{}, 1), release: make(chan struct{})}
	find := deps.Forge.FindPullRequest
	deps.Forge.FindPullRequest = func(ctx context.Context, branch string) (forge.PullRequest, bool, error) {
		if finds.holding.Load() {
			finds.began <- struct{}{}

			<-finds.release
		}

		return find(ctx, branch)
	}

	return deps, finds
}

// awaitBegun waits for a held find to begin, failing the test after
// answerWait.
func (f *heldFinds) awaitBegun(t *testing.T) {
	t.Helper()

	select {
	case <-f.began:
	case <-time.After(answerWait):
		t.Fatal("no find began")
	}
}

func TestARetiredServersFrameNeverPostsItsHeldAnnouncement(t *testing.T) {
	t.Parallel()

	// Arrange
	// A frame of the server for api reads the forge while #42's announcement
	// waits; the directory is switched to web, and CI passes, before the read
	// answers.
	var reached []string

	world := newForgeWorld()
	deps := world.deps()
	deps.Repositories = repositoryDeps(favoritesKept()).Repositories
	deps, finds := holdFindsIn(reachable(deps, &reached))
	handler := webtest.Serve(t, deps, config.Default())
	announceWhenGreen(t, handler, nil)
	world.turn(func(*forgeWorld) {})
	finds.holding.Store(true)

	frame := make(chan *httptest.ResponseRecorder, 1)

	go func() { frame <- webtest.StreamOnce(t, handler, "/api/events") }()

	finds.awaitBegun(t)
	finds.holding.Store(false)

	if switched := webtest.Send(t, handler, http.MethodPut, reposPath+"/here", `{"dir":"`+webRoot+`"}`); switched.Code !=
		http.StatusOK {
		t.Fatalf("switching answered %d: %s", switched.Code, switched.Body.String())
	}

	world.turn(func(w *forgeWorld) { w.ci = forge.CIPassed })

	// Act
	close(finds.release)
	<-frame

	// Assert
	if posts := world.posted(); len(posts) != 0 {
		t.Errorf("posted %q, want nothing: the server held it for was retired", posts)
	}
}

func TestARetiredServersTimerNeverPostsItsHeldAnnouncement(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		// Arrange
		// #42's announcement waits on the server for api when the directory
		// is switched to web, and then CI passes.
		var reached []string

		world := newForgeWorld()
		deps := world.deps()
		deps.Repositories = repositoryDeps(favoritesKept()).Repositories
		handler := heldServer(t, reachable(deps, &reached), config.Default())
		announceWhenGreen(t, handler, nil)
		webtest.Send(t, handler, http.MethodPut, reposPath+"/here", `{"dir":"`+webRoot+`"}`)
		world.turn(func(w *forgeWorld) { w.ci = forge.CIPassed })

		// Act
		advance(ciTick)

		// Assert
		if posts := world.posted(); len(posts) != 0 {
			t.Errorf("posted %q, want nothing: the server held it for was retired", posts)
		}
	})
}

func TestAHeldAnnouncementWhoseCICannotBeReadForTenMinutesIsDropped(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		// Arrange
		// #42's CI cannot be read; a read fails, and another an hour later.
		world := newForgeWorld()
		handler := heldServer(t, world.deps(), config.Default())
		announceWhenGreen(t, handler, nil)
		world.turn(func(w *forgeWorld) { w.ciErr = forge.ErrUnreachable })
		advance(ciTick)
		world.turn(func(*forgeWorld) {})

		// Act
		advance(ciTick)

		// Assert
		reason := "nothing could be heard of #42 for 10m: the service could not be reached; " +
			"check the network, then try again"
		if held := heldAnnouncement(t, handler); held == nil || held.State != api.QueuedAnnouncementStateDropped ||
			orEmpty(held.Reason) != reason {
			t.Errorf("frame's held announcement = %+v, want it dropped because %q", held, reason)
		}
	})
}

func TestAHeldAnnouncementTheServiceRefusesIsDroppedWithItsReason(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		// Arrange
		// The post's claim is taken, and the service refuses the post.
		world, memory := newForgeWorld(), &announceMemory{}
		handler := heldServer(t, memory.bind(world.deps()), config.Default())
		announceWhenGreen(t, handler, nil)
		world.turn(func(w *forgeWorld) {
			w.ci, w.fail = forge.CIPassed, fmt.Errorf("%w: 404", messaging.ErrRejected)
		})

		// Act
		advance(ciTick)

		// Assert
		reason := "the messaging service refused the post; check the token, or that the webhook URL is current, " +
			"in Settings"
		if held := heldAnnouncement(t, handler); held == nil || held.State != api.QueuedAnnouncementStateDropped ||
			orEmpty(held.Reason) != reason {
			t.Errorf("frame's held announcement = %+v, want it dropped because %q", held, reason)
		}

		if records, _ := memory.announced(t.Context()); len(records) != 0 {
			t.Errorf("claims kept = %+v, want the refused post's claim given up", records)
		}
	})
}

func TestAPostNowTheServiceMayHaveTakenSaysSo(t *testing.T) {
	t.Parallel()

	// Arrange
	// The post's claim is taken, and the service cannot be reached.
	deps := (&announceMemory{}).wire(webtest.FilledDeps())
	deps.Messaging.Post = func(context.Context, string, string) error {
		return fmt.Errorf("%w: connection reset", messaging.ErrUnreachable)
	}
	handler := webtest.Serve(t, deps, config.Default())

	// Act
	recorder := postAnnounce(t, handler, map[string]string{channelField: ""})

	// Assert
	assertProblem(t, recorder, http.StatusBadGateway, mayHaveGoneOut)

	if again := postAnnounce(t, handler, map[string]string{channelField: ""}); again.Code != http.StatusConflict {
		t.Errorf("asking again answered %d, want 409: a post that may have gone out is not posted again", again.Code)
	}
}

// heldThenFailing is a messaging service whose first post waits until the
// test lets it go, saying as it starts, and then fails as asking to wait;
// every post counts.
type heldThenFailing struct {
	posts   atomic.Int32
	started chan struct{}
	release chan struct{}
}

// post counts the post, and the first waits to be let go, then fails.
func (p *heldThenFailing) post(context.Context, string, string) error {
	if p.posts.Add(1) == 1 {
		p.started <- struct{}{}

		<-p.release
	}

	return fmt.Errorf("%w: slow down", httpx.ErrRateLimited)
}

func TestAPostUnderWayAsItsServerIsRetiredIsTriedNoMore(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		// Arrange
		// #42's announcement is being posted when the directory is switched;
		// the service then asks the post to wait.
		var reached []string

		world := newForgeWorld()
		service := &heldThenFailing{started: make(chan struct{}, 1), release: make(chan struct{})}
		deps := world.deps()
		deps.Repositories = repositoryDeps(favoritesKept()).Repositories
		deps.Messaging.Post = service.post
		handler := heldServer(t, reachable(deps, &reached), config.Default())
		announceWhenGreen(t, handler, nil)
		world.turn(func(w *forgeWorld) { w.ci = forge.CIPassed })
		time.Sleep(ciTick)
		<-service.started
		webtest.Send(t, handler, http.MethodPut, reposPath+"/here", `{"dir":"`+webRoot+`"}`)

		// Act
		close(service.release)
		advance(10 * ciTick)

		// Assert
		if posts := service.posts.Load(); posts != 1 {
			t.Errorf("posted %d times, want once: a retired server tries it no more", posts)
		}
	})
}
