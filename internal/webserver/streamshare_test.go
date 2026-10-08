// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

import (
	"net/http"
	"net/http/httptest"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/webserver"
)

const (
	// frameWait is how long a test waits for a frame that should not be held
	// up.
	frameWait = 2 * time.Second
	// pull42 is where the forge shows the pull request these tests find.
	pull42 = "https://x/42"
)

// sharedClock is a clock a test moves by hand, read behind a lock since the
// streams read it on their own goroutines.
type sharedClock struct {
	mu  sync.Mutex
	now time.Time
}

// read is the time now.
func (c *sharedClock) read() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.now
}

// pastTheForgeInterval moves the clock an hour on, past every interval a
// shared read is held for.
func (c *sharedClock) pastTheForgeInterval() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.now = c.now.Add(time.Hour)
}

// countedSearch is a tracker that counts its searches.
type countedSearch struct {
	mu    sync.Mutex
	asked int
}

// wire binds deps' search to the counter and its clock to clock.
func (c *countedSearch) wire(deps webserver.Deps, clock *sharedClock) webserver.Deps {
	search := deps.Jira.Search
	deps.Jira.Search = func(jql string, startAt int) (jira.SearchResult, error) {
		c.mu.Lock()
		c.asked++
		c.mu.Unlock()

		return search(jql, startAt)
	}
	deps.Clock = clock.read

	return deps
}

// searches is how many searches were made so far.
func (c *countedSearch) searches() int {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.asked
}

// newSharedClock is a clock at a fixed time.
func newSharedClock() *sharedClock {
	return &sharedClock{now: time.Date(2026, 10, 8, 9, 0, 0, 0, time.UTC)}
}

func TestTwoStreamsShareOneSearchAnInterval(t *testing.T) {
	t.Parallel()

	// Arrange
	clock, tracker := newSharedClock(), &countedSearch{}
	handler := serve(t, tracker.wire(filledDeps(), clock), config.Default())

	// Act: a frame of each of two streams
	first := firstSnapshot(t, streamOnce(t, handler, "/api/events").Body.String())
	second := firstSnapshot(t, streamOnce(t, handler, "/api/events").Body.String())

	// Assert: one search served both
	if got := tracker.searches(); got != 1 || first.Issues.Total != 1 || second.Issues.Total != 1 {
		t.Errorf("searched %d times for totals %d and %d, want once for both, each with the issue",
			got, first.Issues.Total, second.Issues.Total)
	}

	// Act: a frame once the interval has passed
	clock.pastTheForgeInterval()
	streamOnce(t, handler, "/api/events")

	// Assert: the tracker is searched again
	if got := tracker.searches(); got != 2 {
		t.Errorf("searched %d times, want a second search once the interval passed", got)
	}
}

func TestAStatusChangeMadeHereIsSearchedForOnTheNextFrame(t *testing.T) {
	t.Parallel()

	// Arrange
	var writes issueWrites

	clock, tracker := newSharedClock(), &countedSearch{}
	handler := serve(t, tracker.wire(formDeps(&writes), clock), config.Default())
	streamOnce(t, handler, "/api/events")

	if recorder := send(t, handler, http.MethodPost, statusChangesPath, noFieldsChange); recorder.Code != http.StatusOK {
		t.Fatalf("the status change answered %d: %s", recorder.Code, recorder.Body.String())
	}

	// Act
	streamOnce(t, handler, "/api/events")

	// Assert
	if got := tracker.searches(); got != 2 {
		t.Errorf("searched %d times, want the frame after the change to search again", got)
	}
}

// slowRead is a read over the network whose next call, once the test says
// so, waits until the test lets it go.
type slowRead struct {
	mu      sync.Mutex
	slow    bool
	started chan struct{}
	release chan struct{}
}

// newSlowRead is a read that answers at once.
func newSlowRead() *slowRead {
	return &slowRead{started: make(chan struct{}, 1), release: make(chan struct{})}
}

// call is the read's call: at once, or, when the read is slowed down, once the
// test lets it go.
func (r *slowRead) call() {
	r.mu.Lock()
	slow := r.slow
	r.slow = false
	r.mu.Unlock()

	if slow {
		r.started <- struct{}{}

		<-r.release
	}
}

// slowDown makes the next call wait until it is let go.
func (r *slowRead) slowDown() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.slow = true
}

// slowForge is a forge whose next pull request read, once the test says so,
// waits until the test lets it go, and then answers what the forge held as
// it began.
type slowForge struct {
	*slowRead

	mu   sync.Mutex
	pull bool
}

// newSlowForge is a forge that answers at once, finding no pull request.
func newSlowForge() *slowForge {
	return &slowForge{slowRead: newSlowRead()}
}

// wire binds deps' pull request read to the forge.
func (f *slowForge) wire(deps webserver.Deps) webserver.Deps {
	deps.Forge.FindPullRequest = func(string) (forge.PullRequest, bool, error) {
		f.mu.Lock()
		found := f.pull
		f.mu.Unlock()

		f.call()

		return forge.PullRequest{Number: 42, URL: pull42, Title: pullTitle, State: forge.StateOpen}, found, nil
	}

	return deps
}

// openThePull makes the forge find the pull request from now on.
func (f *slowForge) openThePull() {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.pull = true
}

// frameInFlight starts a frame of its own stream on handler, which ends once
// the slow read it waits on is let go, and waits for that read to start.
func frameInFlight(t *testing.T, handler http.Handler, read *slowRead) <-chan *httptest.ResponseRecorder {
	t.Helper()

	frame := make(chan *httptest.ResponseRecorder, 1)

	go func() { frame <- streamOnce(t, handler, "/api/events") }()

	select {
	case <-read.started:
	case <-time.After(frameWait):
		t.Fatal("the frame never made the slow read")
	}

	return frame
}

func TestASlowForgeReadHoldsUpNoOtherStream(t *testing.T) {
	t.Parallel()

	// Arrange
	clock, slow := newSharedClock(), newSlowForge()
	slow.openThePull()
	deps := slow.wire(filledDeps())
	deps.Clock = clock.read
	handler := serve(t, deps, config.Default())
	streamOnce(t, handler, "/api/events")
	clock.pastTheForgeInterval()
	slow.slowDown()
	reading := frameInFlight(t, handler, slow.slowRead)

	defer func() {
		close(slow.release)
		<-reading
	}()

	// Act
	other := make(chan *httptest.ResponseRecorder, 1)

	go func() { other <- streamOnce(t, handler, "/api/events") }()

	// Assert
	select {
	case recorder := <-other:
		if review := firstSnapshot(t, recorder.Body.String()).Review; !review.Found || review.Pull.Number != 42 {
			t.Errorf("the other stream's review = %+v, want the pull request held", review)
		}
	case <-time.After(frameWait):
		t.Error("the other stream's frame waited on the forge read under way")
	}
}

// failingAuthor binds deps' author read to read, on a forge that cannot say
// who the author is, counting each ask in asked.
func failingAuthor(deps webserver.Deps, read *slowRead, asked *atomic.Int32) webserver.Deps {
	deps.Forge.Author = func() (string, error) {
		asked.Add(1)
		read.call()

		return "", errSeam
	}

	return deps
}

func TestASlowAuthorReadHoldsUpNoOtherStream(t *testing.T) {
	t.Parallel()

	// Arrange
	// The forge could not say who the author is, so it is asked again once the
	// interval has passed, and that ask is slow.
	var asked atomic.Int32

	clock, author := newSharedClock(), newSlowRead()
	deps := failingAuthor(filledDeps(), author, &asked)
	deps.Clock = clock.read
	handler := serve(t, deps, config.Default())
	streamOnce(t, handler, "/api/events")
	clock.pastTheForgeInterval()
	author.slowDown()
	asking := frameInFlight(t, handler, author)

	defer func() {
		close(author.release)
		<-asking
	}()

	// Act
	other := make(chan *httptest.ResponseRecorder, 1)

	go func() { other <- streamOnce(t, handler, "/api/events") }()

	// Assert
	select {
	case <-other:
		if got := asked.Load(); got != 2 {
			t.Errorf("asked the forge who the author is %d times over two intervals, want once in each", got)
		}
	case <-time.After(frameWait):
		t.Error("the other stream's frame waited on the author read under way")
	}
}

// slowTracker binds deps' search for which issues are yours to read, which
// then answers as deps' own did.
func slowTracker(deps webserver.Deps, read *slowRead) webserver.Deps {
	answer := deps.Jira.SearchLenient
	deps.Jira.SearchLenient = func(jql string, startAt int) (jira.SearchResult, error) {
		read.call()

		return answer(jql, startAt)
	}

	return deps
}

func TestASlowAskForYourIssuesHoldsUpNoOtherStream(t *testing.T) {
	t.Parallel()

	// Arrange
	// The tracker said which issues are yours, and is asked again once the
	// minute has passed; that ask is slow.
	var asked atomic.Int32

	clock, tracker := newSharedClock(), newSlowRead()
	deps := slowTracker(scopedDeps([]jira.Key{yourIssue}, nil, &asked), tracker)
	deps.Clock = clock.read
	handler := serve(t, deps, scopedConfig())
	streamOnce(t, handler, "/api/events")
	clock.pastTheForgeInterval()
	tracker.slowDown()
	asking := frameInFlight(t, handler, tracker)

	defer func() {
		close(tracker.release)
		<-asking
	}()

	// Act
	other := make(chan *httptest.ResponseRecorder, 1)

	go func() { other <- streamOnce(t, handler, "/api/events") }()

	// Assert
	select {
	case recorder := <-other:
		want := []string{yourLocalBranch}
		if got := listed(firstSnapshot(t, recorder.Body.String()).Branches); !slices.Equal(got, want) {
			t.Errorf("the other stream's branches = %v, want the answer held, %v", got, want)
		}
	case <-time.After(frameWait):
		t.Error("the other stream's frame waited on the tracker's ask under way")
	}
}

func TestAPullRequestOpenedDuringAForgeReadShowsOnTheNextFrame(t *testing.T) {
	t.Parallel()

	// Arrange
	// The read under way began before the pull request was opened, so it
	// finds none; opening it here tells the cache the forge says more now.
	slow := newSlowForge()
	deps := slow.wire(openableDeps())
	deps.Forge.CreatePullRequest = func(forge.NewPullRequest) (forge.PullRequest, error) {
		slow.openThePull()

		return forge.PullRequest{Number: 42, URL: pull42, Title: pullTitle}, nil
	}
	handler := serve(t, deps, config.Default())

	slow.slowDown()
	reading := frameInFlight(t, handler, slow.slowRead)

	opened := make(chan int, 1)

	go func() { opened <- send(t, handler, http.MethodPost, "/api/pull-request", openRequestBody).Code }()

	select {
	case code := <-opened:
		if code != http.StatusOK {
			t.Fatalf("opening the pull request answered %d, want 200", code)
		}
	case <-time.After(frameWait):
		t.Fatal("opening the pull request waited on the forge read under way")
	}

	close(slow.release)
	<-reading

	// Act
	review := firstSnapshot(t, streamOnce(t, handler, "/api/events").Body.String()).Review

	// Assert
	if !review.Found || review.Pull.Number != 42 {
		t.Errorf("review = %+v, want the pull request opened during the read", review)
	}
}
