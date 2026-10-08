// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"time"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/loop"
	"github.com/jacob-delgado/workflow/internal/progress"
)

// defaultStreamInterval is how often the event stream re-pushes a snapshot when
// Info names no interval. The upstreams have no change notification, so the
// server re-reads on this cadence and pushes the result; the browser never polls.
// The forge is asked on a slower cadence of its own: see forgeCache.
const defaultStreamInterval = 5 * time.Second

// streamEvents serves the Server-Sent Events stream: a snapshot on connect, then
// another every interval, until the client disconnects and its request context is
// canceled. The loop runs on the connection's own goroutine, which the HTTP
// server provides, so this starts none of its own.
func (s *server) streamEvents(w http.ResponseWriter, request *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeProblem(w, api.ProblemCodeInternal, "streaming is not supported")

		return
	}

	// An unknown view is refused before the upgrade: once the event-stream
	// headers are out, the only answer left is a stream of the wrong view.
	view := request.URL.Query().Get("view")
	if _, known := resolveJQL(s.config(), view); !known {
		writeProblem(w, api.ProblemCodeNotFound, unknownView(view))

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
// panel keeps its last answer instead (forgeReview). A seam that fails says so
// beside its panel, in problems, so the empty panel is not taken for an answer.
func (s *server) snapshot(view string) api.Snapshot {
	branch, branchErr := s.frameBranch()
	read, reviewErr := s.snapshotReview(branch, branchErr)

	// A queued announcement waits on what the forge says; a read that failed
	// says nothing of the pull request, so it neither posts nor drops one.
	if branchErr == nil && reviewErr == nil {
		s.settleHeld(branch, read)
	}

	review := s.reviewDTO(branch, read)
	review.Announced = s.reviewAnnounced(read)
	issues, issuesErr := s.snapshotIssues(view)
	changes, changesErr := s.snapshotChanges()
	commit := s.commitConvention()

	frame := api.Snapshot{
		Issues:  issues,
		Branch:  branchDTO(branch),
		Changes: changes,
		Review:  review,
		Problems: panelProblems(api.PanelProblems{
			Issues: panelProblem(issuesErr), Branch: panelProblem(branchErr),
			Changes: panelProblem(changesErr), Review: panelProblem(reviewErr),
		}),
		Messaging:          s.readMessaging(),
		QueuedAnnouncement: s.heldStatus(),
		Run:                s.runShown(),
		HooksUnmanaged:     s.hooksUnmanaged(),
		UnstartedStages:    s.knownStages(progress.Work{IssueSelected: true}),

		CommitTypes:    commit.Types(),
		SubjectLimit:   commit.SubjectLimit(),
		SuggestedScope: s.suggestedScope(),

		Tasks: s.snapshotTasks(),
		Here:  s.deps.Repositories.Here.Dir,
	}
	frame.Stages = s.frameStages(branch, read, frame)
	frame.Branches = s.snapshotBranches(branch.Name, frame.Stages)

	return frame
}

// frameStages is the loop's stages for the frame's branch, derived by
// progress, as the terminal's spine and `workflow status` derive them, from
// what the frame read: the branch, its pull request and CI as the forge last
// answered, the files still to commit, whether the pull request was announced
// at its moment, and an announcement held for its CI. The issue stage is done
// once the branch names an issue and not started before: an issue picked but
// not yet branched for has no branch to read here, and is one open in its
// work story, which draws the frame's UnstartedStages for it instead.
func (s *server) frameStages(branch gitrepo.Branch, read forgeRead, frame api.Snapshot) []api.Stage {
	_, named := loop.IssueOf(branch, s.config().Jira.Project)
	work := progress.Work{
		OnFeatureBranch:    loop.OnFeatureBranch(branch),
		IssueNamed:         named,
		Commits:            len(branch.Commits),
		UncommittedChanges: len(frame.Changes.Changes),
		PullRequest:        read.pullState(),
		CI:                 read.ciState(),
		ChangesRequested:   read.found && read.pull.ChangesRequested,
		Announced:          frame.Review.Announced,
		PostPending:        heldForCI(frame.QueuedAnnouncement),
	}

	return s.knownStages(work)
}

// knownStages is the loop's stages for work, the last named for the messaging
// service the configuration names.
func (s *server) knownStages(work progress.Work) []api.Stage {
	return stagesDTO(progress.Stages(work, s.config().Messaging.Service()))
}

// pullState is where the read's pull request stands for the loop: none, when
// the forge found none.
func (r forgeRead) pullState() progress.PullState {
	if !r.found {
		return progress.NoPullRequest
	}

	return progress.PullStateOf(r.pull.State)
}

// ciState is how the read's CI stands, a CI not read counting as none, as
// `workflow status` counts it.
func (r forgeRead) ciState() forge.CIState {
	if !r.ciRead {
		return forge.CINone
	}

	return r.ci.State
}

// heldForCI reports an announcement waiting for its CI, or being posted once
// it passed: not yet made, nor given up on.
func heldForCI(held *api.QueuedAnnouncement) bool {
	return held != nil &&
		(held.State == api.QueuedAnnouncementStateWaiting || held.State == api.QueuedAnnouncementStateAnnouncing)
}

// stagesDTO maps the loop's stages onto the wire, each by its step, name and
// state. Maps, so exhaustive keeps each complete.
func stagesDTO(stages []progress.Stage) []api.Stage {
	steps := map[progress.Step]api.StageStep{
		progress.StepIssue: api.StageStepIssue, progress.StepBranch: api.StageStepBranch,
		progress.StepCommits: api.StageStepCommits, progress.StepReview: api.StageStepReview,
		progress.StepAnnounce: api.StageStepAnnounce,
	}
	states := map[progress.State]api.StageState{
		progress.NotStarted: api.StageStateNotStarted, progress.InFlight: api.StageStateInFlight,
		progress.Done: api.StageStateDone, progress.Failed: api.StageStateFailed,
	}

	out := make([]api.Stage, 0, len(stages))
	for _, stage := range stages {
		out = append(out, api.Stage{Step: steps[stage.Step], Name: stage.Name, State: states[stage.State]})
	}

	return out
}

// frameBranch is the checked-out branch, read once for the whole frame through
// readBranch so its branch, review and in-flight panels describe one branch
// even when a checkout lands mid-frame. It answers an empty branch with
// errNoBranch when no repository is configured, and with the read's error when
// it fails.
func (s *server) frameBranch() (gitrepo.Branch, error) {
	if s.deps.Git.Branch == nil {
		return gitrepo.Branch{}, errNoBranch
	}

	branch, err := s.readBranch()
	if err != nil {
		return gitrepo.Branch{}, err
	}

	return branch, nil
}

// errNoBranch is the branch of a server with no repository configured: nothing
// to ask, so no panel's problem.
var errNoBranch = errors.New("no repository is configured")

// panelProblems is the problems of a frame, or nil when every panel read.
func panelProblems(problems api.PanelProblems) *api.PanelProblems {
	if problems == (api.PanelProblems{}) {
		return nil
	}

	return &problems
}

// panelProblem is the curated problem a panel shows for the error its read
// failed with, or nil when it read, or when there was nothing to ask: no
// repository. A service that is not set up — no credential, no forge the
// origin names — is a problem whose code says so, which the page shows as
// how to set it up rather than as a failure. It goes through faultProblem,
// as an answer's error does, so its detail never carries a host or a
// credential.
func panelProblem(err error) *api.Problem {
	if err == nil || nothingToAsk(err) {
		return nil
	}

	prob, _ := faultProblem(err)

	return &prob
}

// nothingToAsk reports an error that says a read had nothing to ask, rather
// than that something refused it.
func nothingToAsk(err error) bool {
	return errors.Is(err, errNoBranch) || errors.Is(err, gitrepo.ErrNotARepository)
}

// snapshotBranches lists the branches named for one of your issues, the local
// ones and those only the remote has, marking the one checked out, which has
// the frame's stages. These are the issues in flight; the branch, changes and
// review panels describe only the checked-out branch. It is empty outside a
// repository or when the local read fails, holds no branch only the remote has
// when the remote read fails, and marks none when checkedOut is empty, as it is
// when the branch is unknown.
func (s *server) snapshotBranches(checkedOut string, checkedOutStages []api.Stage) []api.TaskBranch {
	if s.deps.Git.Branches == nil {
		return []api.TaskBranch{}
	}

	local, err := s.deps.Git.Branches()
	if err != nil {
		return []api.TaskBranch{}
	}

	project := s.config().Jira.Project
	listing := branchListing{
		names: slices.Clone(local), remote: map[string]bool{}, links: s.issueLinks(),
		worktrees: s.otherWorktrees(), home: s.deps.Repositories.Home,
		stages: branchStages{
			checkedOut: checkedOutStages,
			// A branch not checked out is known only to be named for an issue:
			// the working tree, the forge and the store are read for the
			// checked-out branch alone, so nothing after it reads as begun.
			elsewhere: s.knownStages(progress.Work{OnFeatureBranch: true, IssueNamed: true}),
		},
	}

	for _, name := range s.remoteBranches() {
		if !slices.Contains(local, name) {
			listing.names = append(listing.names, name)
			listing.remote[name] = true
		}
	}

	listing.mine = s.yourIssues(issueKeys(listing.names, listing.links, project))

	return taskBranchesDTO(listing, checkedOut, project)
}

// otherWorktrees is each other worktree, gone or not, by the branch it has
// checked out: none outside a repository or when the read fails, since a
// branch then reads as one to check out, as it did before.
func (s *server) otherWorktrees() map[string]gitrepo.Worktree {
	byBranch := map[string]gitrepo.Worktree{}
	if s.deps.Repositories.Worktrees == nil {
		return byBranch
	}

	worktrees, err := s.deps.Repositories.Worktrees()
	if err != nil {
		return byBranch
	}

	for _, worktree := range worktrees {
		if worktree.Branch != "" && worktree.Dir != s.deps.Repositories.Here.Root {
			byBranch[worktree.Branch] = worktree
		}
	}

	return byBranch
}

// remoteBranches lists the remote's branches, or none outside a repository or
// when the read fails: the local ones are listed all the same.
func (s *server) remoteBranches() []string {
	if s.deps.Git.RemoteBranches == nil {
		return nil
	}

	names, err := s.deps.Git.RemoteBranches()
	if err != nil {
		return nil
	}

	return names
}

// issueLinks is every branch linked to an issue by hand, or none when there is
// no repository to ask.
func (s *server) issueLinks() map[string]string {
	if s.deps.Git.IssueLinks == nil {
		return nil
	}

	return s.deps.Git.IssueLinks()
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

// snapshotIssues is the first page of the view's issues from the issues
// cache, or an empty page when the tracker is not configured, the search
// fails — with its error — or a configuration save has removed the view since
// the stream opened.
func (s *server) snapshotIssues(view string) (api.IssuesPage, error) {
	jql, known := resolveJQL(s.config(), view)
	if s.deps.Jira.Search == nil || !known {
		return issuesPageDTO(jira.SearchResult{}, 0), nil
	}

	result, err := s.frameIssues(jql)
	if err != nil {
		return issuesPageDTO(jira.SearchResult{}, 0), err
	}

	return issuesPageDTO(result, 0), nil
}

// snapshotChanges is the working tree's changes, or none outside a repository or
// when the read fails, with its error.
func (s *server) snapshotChanges() (api.ChangeList, error) {
	changes, err := s.readChanges()
	if err != nil {
		return changesDTO(nil), err
	}

	return changesDTO(changes), nil
}

// snapshotReview is the frame's branch's pull request and CI as the forge last
// answered for it, or an empty review when the branch is not known, no pull
// can be found, or the forge has not answered for this branch at this head.
// Its error is why the forge did not answer, or why the branch it would be
// asked about could not be read.
func (s *server) snapshotReview(branch gitrepo.Branch, branchErr error) (forgeRead, error) {
	if branchErr != nil {
		return forgeRead{}, branchErr
	}

	return s.forgeReview(branch)
}

// forgeInterval is how long a forge answer serves the stream:
// timing.ci_interval in the configuration in effect, or config.DefaultCIInterval,
// the terminal's own CI poll.
func (s *server) forgeInterval() time.Duration {
	interval := s.config().CIInterval()
	if interval <= 0 {
		return config.DefaultCIInterval
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
