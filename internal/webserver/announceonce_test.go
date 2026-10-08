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

// postHeld is how long a slow post takes: long enough that a second
// announcement asked once the first has started arrives while it is under
// way, so a server that posts it twice is caught.
const postHeld = 100 * time.Millisecond

// answerWait is how long a test waits for an answer that must come while a
// post is held: a failsafe, never the pace of the test.
const answerWait = 5 * time.Second

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

// startedAgain reports whether another post has started.
func (p *heldPost) startedAgain() bool {
	select {
	case <-p.started:
		return true
	default:
		return false
	}
}

// slowPost is a messaging service whose posts each take postHeld, saying as
// the first starts, and noting a post that starts while another is under way.
type slowPost struct {
	mu         sync.Mutex
	texts      []string
	posting    int
	overlapped bool
	started    chan struct{}
}

// newSlowPost is a service none has posted to.
func newSlowPost() *slowPost {
	return &slowPost{started: make(chan struct{}, 1)}
}

// post records text and takes postHeld to send it.
func (p *slowPost) post(_, text string) error {
	p.mu.Lock()
	p.texts = append(p.texts, text)
	p.posting++
	p.overlapped = p.overlapped || p.posting > 1
	p.mu.Unlock()

	select {
	case p.started <- struct{}{}:
	default:
	}

	time.Sleep(postHeld)

	p.mu.Lock()
	p.posting--
	p.mu.Unlock()

	return nil
}

// sent is how many posts were made, and whether two were under way at once.
func (p *slowPost) sent() (int, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()

	return len(p.texts), p.overlapped
}

func TestTwoAnnouncementsAskedAtOnceArePostedOnce(t *testing.T) {
	t.Parallel()

	// Arrange
	// The second is asked once the first has started to post.
	memory := &announceMemory{}
	deps := memory.wire(filledDeps())
	service := newSlowPost()
	deps.Post = service.post
	handler := serve(t, deps, config.Default())
	answers := make(chan int, 2)
	announce := func() { answers <- postAnnounce(t, handler, map[string]string{channelField: ""}).Code }

	go announce()

	<-service.started

	// Act
	go announce()

	codes := []int{<-answers, <-answers}

	// Assert
	slices.Sort(codes)

	if !slices.Equal(codes, []int{http.StatusOK, http.StatusConflict}) {
		t.Errorf("answers %v, want one 200 and one 409", codes)
	}

	if posts, overlapped := service.sent(); posts != 1 || overlapped {
		t.Errorf("posted %d times, two at once: %v; want once: the channel reads an announcement once",
			posts, overlapped)
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

// whilePosting asks while a post is held open, waits for its answer, then
// lets every post go, and answers what the ask was answered and whether it
// started a post of its own. An ask that started a post, or waited on the one
// held, is not answered while it is held, which fails the test. The ask goes
// on its own goroutine, so it does not hold the test up.
func whilePosting(
	t *testing.T, service *heldPost, ask func() *httptest.ResponseRecorder,
) (*httptest.ResponseRecorder, bool) {
	t.Helper()

	defer close(service.release)

	answer := make(chan *httptest.ResponseRecorder, 1)

	go func() { answer <- ask() }()

	select {
	case recorder := <-answer:
		return recorder, service.startedAgain()
	case <-time.After(answerWait):
		t.Fatal("the ask was not answered while the post was held")

		return nil, false
	}
}

func TestAnnouncingNowWhileTheHeldAnnouncementIsPostedIsAConflict(t *testing.T) {
	t.Parallel()

	// Arrange
	handler, service, frame := postingHeld(t)

	// Act
	recorder, again := whilePosting(t, service, func() *httptest.ResponseRecorder {
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
	recorder, again := whilePosting(t, service, func() *httptest.ResponseRecorder {
		return announceWhenGreen(t, handler, nil)
	})

	<-frame

	// Assert
	assertProblem(t, recorder, http.StatusConflict, "being posted")

	if again {
		t.Error("a second post started, want the held one alone")
	}
}
