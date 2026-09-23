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
	return api.Snapshot{
		Issues:    s.snapshotIssues(view),
		Branch:    s.snapshotBranch(),
		Changes:   s.snapshotChanges(),
		Review:    s.snapshotReview(),
		Messaging: messagingDTO(s.config(), s.author()),
		Branches:  s.snapshotBranches(),

		SuggestedScope: s.suggestedScope(),
	}
}

// snapshotBranches lists the local branches named for an issue, marking the one
// on HEAD. These are the issues in flight; the branch, changes and review panels
// describe only the checked-out branch. It is empty outside a repository or when
// the read fails.
func (s *server) snapshotBranches() []api.TaskBranch {
	if s.deps.Branches == nil {
		return taskBranchesDTO(nil, "", "")
	}

	names, err := s.deps.Branches()
	if err != nil {
		return taskBranchesDTO(nil, "", "")
	}

	return taskBranchesDTO(names, s.currentBranchName(), s.config().Jira.Project)
}

// currentBranchName is the checked-out branch's name, or "" outside a repository
// or when the read fails — used only to mark which task branch is on HEAD.
func (s *server) currentBranchName() string {
	if s.deps.Branch == nil {
		return ""
	}

	branch, err := s.deps.Branch()
	if err != nil {
		return ""
	}

	return branch.Name
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

// snapshotBranch is the current branch, or an empty one outside a repository or
// when the read fails.
func (s *server) snapshotBranch() api.Branch {
	if s.deps.Branch == nil {
		return branchDTO(gitrepo.Branch{})
	}

	branch, err := s.deps.Branch()
	if err != nil {
		return branchDTO(gitrepo.Branch{})
	}

	return branchDTO(branch)
}

// snapshotChanges is the working tree's changes, or none outside a repository or
// when the read fails.
func (s *server) snapshotChanges() api.ChangeList {
	if s.deps.Changes == nil {
		return changesDTO(nil)
	}

	changes, err := s.deps.Changes()
	if err != nil {
		return changesDTO(nil)
	}

	return changesDTO(changes)
}

// snapshotReview is the branch's pull request and CI, or an empty review when no
// pull can be found or a read fails.
func (s *server) snapshotReview() api.Review {
	if s.deps.Branch == nil || s.deps.FindPull == nil {
		return api.Review{Found: false}
	}

	branch, err := s.deps.Branch()
	if err != nil {
		return api.Review{Found: false}
	}

	pull, found, err := s.deps.FindPull(branch.Name)
	if err != nil {
		return api.Review{Found: false}
	}

	return s.review(pull, found, branch.Head)
}
