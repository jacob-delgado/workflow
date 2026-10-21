// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

import (
	"context"
	"maps"
	"net/http"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/webserver"
	"github.com/jacob-delgado/workflow/internal/webserver/webtest"
)

// heldReads is a shared read whose numbered calls each wait until the test
// lets that call go, so a test can land a write while a read is under way and
// then land the reads one at a time.
type heldReads struct {
	mu    sync.Mutex
	calls int
	held  map[int]chan struct{}
}

// newHeldReads is a read that holds the calls numbered, counting from one.
func newHeldReads(numbers ...int) *heldReads {
	held := map[int]chan struct{}{}
	for _, number := range numbers {
		held[number] = make(chan struct{})
	}

	return &heldReads{held: held}
}

// read is a call of the read: it waits when its number is held.
func (h *heldReads) read() {
	h.mu.Lock()
	h.calls++
	gate, held := h.held[h.calls]
	h.mu.Unlock()

	if held {
		<-gate
	}
}

// landInTurn lets each held call go in turn, waiting after each until
// everything it set going is blocked again.
func (h *heldReads) landInTurn() {
	for _, number := range slices.Sorted(maps.Keys(h.held)) {
		close(h.held[number])
		synctest.Wait()
	}
}

// twoStreams opens two streams on handler, one after the other, and moves
// the clock on a stream interval, so the next frame of each finds every
// shared read due and one of them starts it.
func twoStreams(t *testing.T, handler http.Handler) []*webtest.OpenedStream {
	t.Helper()

	first := webtest.OpenStream(t, handler, "/api/events")

	synctest.Wait()

	second := webtest.OpenStream(t, handler, "/api/events")

	synctest.Wait()
	advance(streamTick)

	return []*webtest.OpenedStream{first, second}
}

// lastFrames is the last frame each stream sent, once closed.
func lastFrames(t *testing.T, streams []*webtest.OpenedStream) []api.Snapshot {
	t.Helper()

	frames := make([]api.Snapshot, 0, len(streams))
	for _, stream := range streams {
		frames = append(frames, lastSnapshot(t, stream.Close()))
	}

	return frames
}

func TestAnEditDuringAForgeReadShowsOnEveryStreamsLastFrame(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		// Arrange
		// The second call is a frame's read under way as the edit lands, the
		// third the edit's own lookup of the pull request, and the fourth the
		// read the edit's drop asks for.
		reads := newHeldReads(2, 4)
		slow := newRetitlingForge()
		deps := slow.wire(webtest.FilledDeps())
		find := deps.Forge.FindPullRequest
		deps.Forge.FindPullRequest = func(ctx context.Context, branch string) (forge.PullRequest, bool, error) {
			pull, found, err := find(ctx, branch)

			reads.read()

			return pull, found, err
		}
		handler := webtest.ServeWith(t, deps, config.Default(),
			webserver.Info{Version: webtest.TestVersion, StreamInterval: streamTick})
		streams := twoStreams(t, handler)

		// Act
		edited := webtest.Send(t, handler, http.MethodPatch, "/api/pull-request", `{"title":"retitled","body":""}`)

		synctest.Wait()
		reads.landInTurn()

		// Assert
		if edited.Code != http.StatusOK {
			t.Fatalf("the edit answered %d: %s", edited.Code, edited.Body.String())
		}

		for index, frame := range lastFrames(t, streams) {
			if frame.Review.Pull == nil || frame.Review.Pull.Title != "retitled" {
				t.Errorf("stream %d's last frame shows the pull %+v, want #42 retitled", index, frame.Review.Pull)
			}
		}
	})
}

func TestAnAssignmentDuringAnAskShowsOnEveryStreamsLastFrame(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		// Arrange
		// The second ask is a frame's under way as the assignment lands, and
		// the third the one the assignment's drop asks for.
		var asked atomic.Int32

		reads := newHeldReads(2, 3)
		tracker := &assignableTracker{yours: []jira.Key{yourIssue}}
		deps := scopedDeps(nil, nil, &asked)
		deps.Git.RemoteBranches = nil
		deps.Jira.Assign = tracker.assign
		deps.Jira.SearchLenient = func(ctx context.Context, jql string, startAt int) (jira.SearchResult, error) {
			answer, err := tracker.search(ctx, jql, startAt)
			if strings.HasPrefix(jql, keyInYourSearch) {
				reads.read()
			}

			return answer, err
		}
		handler := webtest.ServeWith(t, deps, scopedConfig(),
			webserver.Info{Version: webtest.TestVersion, StreamInterval: streamTick})
		streams := twoStreams(t, handler)

		// Act
		assigned := webtest.Send(t, handler, http.MethodPut, "/api/issues/PROJ-2/assignee", `{"assignee":"me"}`)

		synctest.Wait()
		reads.landInTurn()

		// Assert
		if assigned.Code != http.StatusOK {
			t.Fatalf("the assignment answered %d: %s", assigned.Code, assigned.Body.String())
		}

		want := []string{yourLocalBranch, otherLocalBranch}
		for index, frame := range lastFrames(t, streams) {
			if got := listed(frame.Branches); !slices.Equal(got, want) {
				t.Errorf("stream %d's last frame lists %v, want %v once PROJ-2 is yours", index, got, want)
			}
		}
	})
}

// movingTracker is a tracker whose one issue, webtest.TestKey, moves to the
// status a change names, and whose search finds it where it stands.
type movingTracker struct {
	mu     sync.Mutex
	status string
}

// search finds the issue where it stands.
func (m *movingTracker) search(context.Context, string, int) (jira.SearchResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	return jira.SearchResult{Issues: []jira.Issue{{
		Key: webtest.TestKey, Summary: webtest.TestSummary, Status: m.status, StatusCategory: "indeterminate",
	}}, Total: 1}, nil
}

// transition moves the issue to where the change goes.
func (m *movingTracker) transition(_ context.Context, _ jira.Key, to jira.Transition, _ []jira.FieldValue) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.status = to.ToStatus

	return nil
}

func TestAStatusChangeDuringASearchShowsOnEveryStreamsLastFrame(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		// Arrange
		// The second search is a frame's under way as the change lands, and
		// the third the one the change's drop asks for.
		reads := newHeldReads(2, 3)
		tracker := &movingTracker{status: "In Progress"}
		deps := formDeps(&issueWrites{})
		deps.Jira.Transition = tracker.transition
		deps.Jira.Search = func(ctx context.Context, jql string, startAt int) (jira.SearchResult, error) {
			answer, err := tracker.search(ctx, jql, startAt)

			reads.read()

			return answer, err
		}
		handler := webtest.ServeWith(t, deps, config.Default(),
			webserver.Info{Version: webtest.TestVersion, StreamInterval: streamTick})
		streams := twoStreams(t, handler)

		// Act
		changed := webtest.Send(t, handler, http.MethodPost, statusChangesPath, noFieldsChange)

		synctest.Wait()
		reads.landInTurn()

		// Assert
		if changed.Code != http.StatusOK {
			t.Fatalf("the status change answered %d: %s", changed.Code, changed.Body.String())
		}

		for index, frame := range lastFrames(t, streams) {
			if issues := frame.Issues.Issues; len(issues) != 1 || issues[0].Status != blocked {
				t.Errorf("stream %d's last frame shows the issues %+v, want %s %s", index, issues, webtest.TestKey, blocked)
			}
		}
	})
}
