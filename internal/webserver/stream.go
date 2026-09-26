// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/jira"
)

// defaultStreamInterval is how often the event stream re-pushes a snapshot when
// Info names no interval. The upstreams have no change notification, so the
// server re-reads on this cadence and pushes the result; the browser never polls.
const defaultStreamInterval = 5 * time.Second

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
// snapshot, so one unreachable upstream does not blank the cockpit.
func (s *server) snapshot(view string) api.Snapshot {
	branch, known := s.frameBranch()

	return api.Snapshot{
		Issues:    s.snapshotIssues(view),
		Branch:    branchDTO(branch),
		Changes:   s.snapshotChanges(),
		Review:    s.snapshotReview(branch, known),
		Messaging: s.readMessaging(),
		Branches:  s.snapshotBranches(branch.Name),

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

// snapshotReview is the frame's branch's pull request and CI, or an empty review
// when the branch is not known, no pull can be found, or a read fails.
func (s *server) snapshotReview(branch gitrepo.Branch, known bool) api.Review {
	if !known {
		return api.Review{Found: false}
	}

	review, err := s.reviewFor(branch)
	if err != nil {
		return api.Review{Found: false}
	}

	return review
}
