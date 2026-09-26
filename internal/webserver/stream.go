// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/jira"
)

// defaultStreamInterval is how often the event stream re-pushes a snapshot when
// Info names no interval. The upstreams have no change notification, so the
// server re-reads on this cadence and pushes the result; the browser never polls.
// The forge is asked on a slower cadence of its own: see forgeCache.
const defaultStreamInterval = 5 * time.Second

// defaultForgeInterval is how long the forge's answer serves every stream when
// timing.ci_interval names no interval: the terminal's own CI poll, often
// enough to see a check finish soon after it does, rarely enough that a page
// left open does not spend the forge's rate limit.
const defaultForgeInterval = 20 * time.Second

// forgeCache is the forge's part of a frame — the branch's pull request, its
// reviews and its CI — held for every stream alike, so the forge is asked at
// most once an interval however many pages are open, while the repository is
// read on every frame. It holds the answer for one branch at one head commit:
// a frame for another asks at once. A read that fails keeps the answer held
// for the same branch and head until the next read is due, and a CI read that
// fails keeps the CI held for the same pull request. Its lock is held across
// the read, so streams that find a read due together make one.
type forgeCache struct {
	mu     sync.Mutex
	held   bool
	key    forgeKey
	readAt time.Time
	review api.Review
}

// forgeKey is what a forge answer was read for: a branch, by name, at a head.
type forgeKey struct {
	branch string
	head   string
}

// drop forgets the held answer, so the next frame asks the forge: a write here
// that changes what the forge would say shows on the next frame rather than an
// interval later.
func (c *forgeCache) drop() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.held = false
}

// holdsPull reports whether the held answer is about the pull request of this
// number, so the CI held for it can stand in for a CI read that failed; the CI
// of another pull request, or of none, cannot. The caller holds the lock.
func (c *forgeCache) holdsPull(number int) bool {
	return c.review.Pull != nil && c.review.Pull.Number == number
}

// streamEvents serves the Server-Sent Events stream: a snapshot on connect, then
// another every interval, until the client disconnects and its request context is
// canceled. The loop runs on the connection's own goroutine, which the HTTP
// server provides, so this starts none of its own.
func (s *server) streamEvents(w http.ResponseWriter, request *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeProblem(w, api.Internal, "streaming is not supported")

		return
	}

	// An unknown view is refused before the upgrade: once the event-stream
	// headers are out, the only answer left is a stream of the wrong view.
	view := request.URL.Query().Get("view")
	if _, known := resolveJQL(s.config(), view); !known {
		writeProblem(w, api.NotFound, unknownView(view))

		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	interval := s.info.streamInterval()

	for eventID := 1; ; eventID++ {
		if !writeSnapshot(w, flusher, eventID, s.snapshot(view)) {
			return
		}

		select {
		case <-request.Context().Done():
			return
		case <-time.After(interval):
		}
	}
}

// writeSnapshot writes one event-stream message carrying the snapshot and flushes
// it. It reports whether the write reached the client; a failed write means the
// connection is gone and the stream should stop.
func writeSnapshot(w http.ResponseWriter, flusher http.Flusher, eventID int, snap api.Snapshot) bool {
	data, err := json.Marshal(snap)
	if err != nil {
		return false
	}

	_, err = fmt.Fprintf(w, "id: %d\nevent: snapshot\ndata: %s\n\n", eventID, data)
	if err != nil {
		return false
	}

	flusher.Flush()

	return true
}

// snapshot assembles the full read state the stream carries. A seam that is not
// configured, or that fails, yields an empty panel rather than failing the whole
// snapshot, so one unreachable upstream does not blank the cockpit; the forge's
// panel keeps its last answer instead (forgeReview).
func (s *server) snapshot(view string) api.Snapshot {
	branch, known := s.frameBranch()

	return api.Snapshot{
		Issues:    s.snapshotIssues(view),
		Branch:    branchDTO(branch),
		Changes:   s.snapshotChanges(),
		Review:    s.snapshotReview(branch, known),
		Messaging: s.readMessaging(),
		Branches:  s.snapshotBranches(branch.Name),

		CommitTypes:    s.commitConvention().Types(),
		SuggestedScope: s.suggestedScope(),
	}
}

// frameBranch is the checked-out branch, read once for the whole frame through
// readBranch so its branch, review and in-flight panels describe one branch
// even when a checkout lands mid-frame. It reports false, with an empty branch,
// when no repository is configured or the read fails.
func (s *server) frameBranch() (gitrepo.Branch, bool) {
	branch, err := s.readBranch()
	if err != nil || s.deps.Branch == nil {
		return gitrepo.Branch{}, false
	}

	return branch, true
}

// snapshotBranches lists the local branches named for an issue, marking the one
// checked out. These are the issues in flight; the branch, changes and review
// panels describe only the checked-out branch. It is empty outside a repository
// or when the read fails, and marks none when checkedOut is empty, as it is when
// the branch is unknown.
func (s *server) snapshotBranches(checkedOut string) []api.TaskBranch {
	if s.deps.Branches == nil {
		return taskBranchesDTO(nil, "", "")
	}

	names, err := s.deps.Branches()
	if err != nil {
		return taskBranchesDTO(nil, "", "")
	}

	return taskBranchesDTO(names, checkedOut, s.config().Jira.Project)
}

// snapshotIssues is the first page of the view's issues, or an empty page when
// the tracker is not configured, the search fails, or a configuration save has
// removed the view since the stream opened.
func (s *server) snapshotIssues(view string) api.IssuesPage {
	jql, known := resolveJQL(s.config(), view)
	if s.deps.Search == nil || !known {
		return issuesPageDTO(jira.SearchResult{}, 0)
	}

	result, err := s.deps.Search(jql, 0)
	if err != nil {
		return issuesPageDTO(jira.SearchResult{}, 0)
	}

	return issuesPageDTO(result, 0)
}

// snapshotChanges is the working tree's changes, or none outside a repository or
// when the read fails.
func (s *server) snapshotChanges() api.ChangeList {
	changes, err := s.readChanges()
	if err != nil {
		return changesDTO(nil)
	}

	return changesDTO(changes)
}

// snapshotReview is the frame's branch's pull request and CI as the forge last
// answered for it, or an empty review when the branch is not known, no pull
// can be found, or the forge has not answered for this branch at this head.
func (s *server) snapshotReview(branch gitrepo.Branch, known bool) api.Review {
	if !known {
		return api.Review{Found: false}
	}

	return s.forgeReview(branch)
}

// forgeReview is the branch's review from the forge cache, read again when the
// cache holds none for the branch at its head, or once the forge interval has
// passed since the last read.
func (s *server) forgeReview(branch gitrepo.Branch) api.Review {
	interval := s.forgeInterval()
	key := forgeKey{branch: branch.Name, head: branch.Head}

	s.forgeAnswer.mu.Lock()
	defer s.forgeAnswer.mu.Unlock()

	now := s.now()
	held := s.forgeAnswer.held && s.forgeAnswer.key == key

	if held && now.Sub(s.forgeAnswer.readAt) < interval {
		return s.forgeAnswer.review
	}

	read, err := s.readForge(branch)
	s.forgeAnswer.readAt = now

	if err != nil && held {
		return s.forgeAnswer.review
	}

	review := read.review
	if held && read.ciErr != nil && s.forgeAnswer.holdsPull(review.Pull.Number) {
		review.Ci = s.forgeAnswer.review.Ci
	}

	s.forgeAnswer.held, s.forgeAnswer.key, s.forgeAnswer.review = true, key, review

	return review
}

// forgeInterval is how long a forge answer serves the stream:
// timing.ci_interval in the configuration in effect, or defaultForgeInterval.
func (s *server) forgeInterval() time.Duration {
	interval := s.config().CIInterval()
	if interval <= 0 {
		return defaultForgeInterval
	}

	return interval
}

// now is the time by the clock the server was given.
func (s *server) now() time.Time {
	if s.deps.Clock == nil {
		return time.Now()
	}

	return s.deps.Clock()
}
