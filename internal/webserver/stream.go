// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver

import (
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"slices"
	"sync"
	"time"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/loop"
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

// assignedInterval is how long the tracker's answer to which of the branches'
// issues are yours serves every stream: an issue is reassigned or finished
// rarely, and each answer is a search of the tracker.
const assignedInterval = time.Minute

// assignedCache is which of the branches' issues the tracker last said are
// yours, held for every stream alike and asked again when the branches name
// other issues or assignedInterval has passed. An ask that fails keeps the last
// answer until the next is due, and an issue that answer never covered counts
// as yours, as every issue does with no answer yet, or no tracker to ask, so
// the list never waits on the tracker. Its lock is held across the ask, so
// streams that find one due together make one.
type assignedCache struct {
	mu     sync.Mutex
	held   bool
	keys   []jira.Key
	readAt time.Time
	// mine is the tracker's last answer, and answered the keys it was asked
	// about, sorted; mine is nil until the tracker has answered.
	mine     map[jira.Key]bool
	answered []jira.Key
}

// yours is which of keys are yours by the last answer: those it said are, and
// those it never covered; nil, counting every issue, before any answer. The
// caller holds the lock.
func (c *assignedCache) yours(keys []jira.Key) map[jira.Key]bool {
	if c.mine == nil {
		return nil
	}

	yours := maps.Clone(c.mine)

	for _, key := range keys {
		if _, covered := slices.BinarySearch(c.answered, key); !covered {
			yours[key] = true
		}
	}

	return yours
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
		case <-s.retired:
			// The directory was switched: the page reconnects in a second,
			// to the server for the directory switched to.
			_, _ = fmt.Fprint(w, "retry: 1000\n\n")

			flusher.Flush()

			return
		case <-time.After(interval):
		}
	}
}

// writeSnapshot writes one event-stream message carrying the snapshot and flushes
// it. It reports whether the write reached the client; a failed write means the
// connection is gone and the stream should stop.
func writeSnapshot(w http.ResponseWriter, flusher http.Flusher, eventID int, snap api.Snapshot) bool {
	// Trade-off TRADE-13: a snapshot always encodes.
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

		Tasks: s.snapshotTasks(),
		Here:  s.deps.Repositories.Here.Dir,
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

// snapshotBranches lists the branches named for one of your issues, the local
// ones and those only the remote has, marking the one checked out. These are
// the issues in flight; the branch, changes and review panels describe only
// the checked-out branch. It is empty outside a repository or when the local
// read fails, holds no branch only the remote has when the remote read fails,
// and marks none when checkedOut is empty, as it is when the branch is unknown.
func (s *server) snapshotBranches(checkedOut string) []api.TaskBranch {
	if s.deps.Branches == nil {
		return []api.TaskBranch{}
	}

	local, err := s.deps.Branches()
	if err != nil {
		return []api.TaskBranch{}
	}

	project := s.config().Jira.Project
	listing := branchListing{names: slices.Clone(local), remote: map[string]bool{}, links: s.issueLinks()}

	for _, name := range s.remoteBranches() {
		if !slices.Contains(local, name) {
			listing.names = append(listing.names, name)
			listing.remote[name] = true
		}
	}

	listing.mine = s.yourIssues(issueKeys(listing.names, listing.links, project))

	return taskBranchesDTO(listing, checkedOut, project)
}

// remoteBranches lists the remote's branches, or none outside a repository or
// when the read fails: the local ones are listed all the same.
func (s *server) remoteBranches() []string {
	if s.deps.RemoteBranches == nil {
		return nil
	}

	names, err := s.deps.RemoteBranches()
	if err != nil {
		return nil
	}

	return names
}

// issueLinks is every branch linked to an issue by hand, or none when there is
// no repository to ask.
func (s *server) issueLinks() map[string]string {
	if s.deps.IssueLinks == nil {
		return nil
	}

	return s.deps.IssueLinks()
}

// issueKeys is the issue each of names is for, by its link in links or its
// name, for those that are for one.
func issueKeys(names []string, links map[string]string, project string) []jira.Key {
	var keys []jira.Key

	for _, name := range names {
		if key, named := loop.NamedIssue(name, links, project); named {
			keys = append(keys, jira.Key(key.Key))
		}
	}

	return keys
}

// yourIssues is which of keys name your issues, from the assigned cache: nil,
// counting every issue, with no tracker to ask or none that has answered.
func (s *server) yourIssues(keys []jira.Key) map[jira.Key]bool {
	if s.deps.SearchLenient == nil {
		return nil
	}

	asked := slices.Compact(slices.Sorted(slices.Values(keys)))

	s.assigned.mu.Lock()
	defer s.assigned.mu.Unlock()

	now := s.now()
	if s.assigned.held && slices.Equal(s.assigned.keys, asked) && now.Sub(s.assigned.readAt) < assignedInterval {
		return s.assigned.yours(asked)
	}

	mine, err := loop.AssignedKeys(s.deps.SearchLenient, asked)
	s.assigned.held, s.assigned.keys, s.assigned.readAt = true, asked, now

	if err == nil {
		s.assigned.mine, s.assigned.answered = mine, asked
	}

	return s.assigned.yours(asked)
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
