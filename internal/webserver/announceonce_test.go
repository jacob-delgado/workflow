// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

import (
	"net/http"
	"net/http/httptest"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/forge"
)

// secondPostWait is how long a test gives a second announcement to reach the
// post a first one holds open, so a server that posts it twice is caught.
const secondPostWait = 200 * time.Millisecond

// heldPost is a messaging service whose posts wait until the test lets them
// go, saying as each one starts.
type heldPost struct {
	mu      sync.Mutex
	texts   []string
	started chan struct{}
	release chan struct{}
}

// newHeldPost is a service holding every post until release is closed.
func newHeldPost() *heldPost {
	return &heldPost{started: make(chan struct{}, 2), release: make(chan struct{})}
}

// post records text, says it started, and waits to be let go.
func (p *heldPost) post(_, text string) error {
	p.mu.Lock()
	p.texts = append(p.texts, text)
	p.mu.Unlock()

	p.started <- struct{}{}

	<-p.release

	return nil
}

// posted is every text posted so far.
func (p *heldPost) posted() []string {
	p.mu.Lock()
	defer p.mu.Unlock()

	return slices.Clone(p.texts)
}

// awaitStart waits for a post to start, failing the test after a second.
func (p *heldPost) awaitStart(t *testing.T) {
	t.Helper()

	select {
	case <-p.started:
	case <-time.After(time.Second):
		t.Fatal("no post started")
	}
}

// startedAgain reports whether another post starts within secondPostWait.
func (p *heldPost) startedAgain() bool {
	select {
	case <-p.started:
		return true
	case <-time.After(secondPostWait):
		return false
	}
}

func TestTwoAnnouncementsAskedAtOnceArePostedOnce(t *testing.T) {
	t.Parallel()

	// Arrange
	memory := &announceMemory{}
	deps := memory.wire(filledDeps())
	service := newHeldPost()
	deps.Post = service.post
	handler := serve(t, deps, config.Default())
	answers := make(chan int, 2)
	announce := func() { answers <- postAnnounce(t, handler, map[string]string{channelField: ""}).Code }

	go announce()

	service.awaitStart(t)

	// Act
	go announce()

	again := service.startedAgain()
	close(service.release)

	codes := []int{<-answers, <-answers}

	// Assert
	slices.Sort(codes)

	if again || !slices.Equal(codes, []int{http.StatusOK, http.StatusConflict}) {
		t.Errorf("a second post started: %v; answers %v, want one 200 and one 409", again, codes)
	}

	if posts := service.posted(); len(posts) != 1 {
		t.Errorf("posted %d times, want once: the channel reads an announcement once", len(posts))
	}
}

// postingHeld is a server whose held announcement has been taken to post, its
// post held open by service, and the frame that took it, which ends once the
// post is let go.
func postingHeld(t *testing.T) (http.Handler, *heldPost, <-chan *httptest.ResponseRecorder) {
	t.Helper()

	world := newForgeWorld()
	deps := world.deps()
	service := newHeldPost()
	deps.Post = service.post
	handler := serve(t, deps, config.Default())

	if recorder := announceWhenGreen(t, handler, nil); recorder.Code != http.StatusAccepted {
		t.Fatalf("holding the announcement answered %d: %s", recorder.Code, recorder.Body.String())
	}

	world.turn(func(w *forgeWorld) { w.ci = forge.CIPassed })

	frame := make(chan *httptest.ResponseRecorder, 1)

	go func() { frame <- streamOnce(t, handler, "/api/events") }()

	service.awaitStart(t)

	return handler, service, frame
}

// whilePosting asks while a post is held open, then lets every post go, and
// answers what the ask was answered and whether it started a post of its own.
// The ask goes on its own goroutine, so one that waits on a post it started
// does not hold the test up.
func whilePosting(service *heldPost, ask func() *httptest.ResponseRecorder) (*httptest.ResponseRecorder, bool) {
	answer := make(chan *httptest.ResponseRecorder, 1)

	go func() { answer <- ask() }()

	again := service.startedAgain()
	close(service.release)

	return <-answer, again
}

func TestAnnouncingNowWhileTheHeldAnnouncementIsPostedIsAConflict(t *testing.T) {
	t.Parallel()

	// Arrange
	handler, service, frame := postingHeld(t)

	// Act
	recorder, again := whilePosting(service, func() *httptest.ResponseRecorder {
		return postAnnounce(t, handler, map[string]string{channelField: slackChannel})
	})

	<-frame

	// Assert
	assertProblem(t, recorder, http.StatusConflict, "being posted")

	if again || len(service.posted()) != 1 {
		t.Errorf("a second post started: %v, and %d posted; want the held one alone", again, len(service.posted()))
	}

	if held := heldAnnouncement(t, handler); held == nil || held.State != api.QueuedAnnouncementStateAnnounced {
		t.Errorf("held announcement = %+v, want it announced", held)
	}
}

func TestDroppingTheHeldAnnouncementWhileItIsPostedIsAConflict(t *testing.T) {
	t.Parallel()

	// Arrange
	handler, service, frame := postingHeld(t)

	// Act
	recorder := send(t, handler, http.MethodDelete, queuedPath, "")

	close(service.release)
	<-frame

	// Assert
	assertProblem(t, recorder, http.StatusConflict, "being posted")

	if held := heldAnnouncement(t, handler); held == nil || held.State != api.QueuedAnnouncementStateAnnounced {
		t.Errorf("held announcement = %+v, want it announced, as it was", held)
	}
}

func TestHoldingAnotherWhileTheHeldAnnouncementIsPostedIsAConflict(t *testing.T) {
	t.Parallel()

	// Arrange
	handler, service, frame := postingHeld(t)

	// Act
	recorder, again := whilePosting(service, func() *httptest.ResponseRecorder {
		return announceWhenGreen(t, handler, nil)
	})

	<-frame

	// Assert
	assertProblem(t, recorder, http.StatusConflict, "being posted")

	if again {
		t.Error("a second post started, want the held one alone")
	}
}
