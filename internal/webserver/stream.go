// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"time"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/loop"
	"github.com/jacob-delgado/workflow/internal/progress"
	"github.com/jacob-delgado/workflow/internal/report"
	"github.com/jacob-delgado/workflow/internal/webserver/problem"
)

// defaultStreamInterval is how often the event stream re-pushes a snapshot when
// Info names no interval. The upstreams have no change notification, so the
// server re-reads on this cadence and pushes the result; the browser never polls.
// The forge is asked on a slower cadence of its own: see forgeCache.
const defaultStreamInterval = 5 * time.Second

// streamEvents serves the Server-Sent Events stream for the view the request
// names: it refuses a server that cannot stream and a view the configuration
// does not offer, sets the event-stream headers, sends the view's kept issues
// when the store holds them, and pushes the snapshots.
func (s *server) streamEvents(w http.ResponseWriter, request *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		problem.Write(w, api.ProblemCodeInternal, "streaming is not supported")

		return
	}

	// An unknown view is refused before the upgrade: once the event-stream
	// headers are out, the only answer left is a stream of the wrong view.
	view := request.URL.Query().Get("view")

	listed, known := jira.ViewNamed(s.config().Jira.Views, view)
	if !known {
		problem.Write(w, api.ProblemCodeNotFound, unknownView(view))

		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	out := &eventStream{w: w, flusher: flusher}
	if !s.seedFirst(request.Context(), out, listed.JQL) {
		return
	}

	s.pushSnapshots(request, out, view)
}

// pushSnapshots pushes a snapshot on connect, then another every interval,
// until the client disconnects and its request context is canceled, a write
// fails, or the directory is switched. The loop runs on the connection's own
// goroutine, which the HTTP server provides, so this starts none of its own.
func (s *server) pushSnapshots(request *http.Request, out *eventStream, view string) {
	interval := s.info.streamInterval()

	for sendEvent(out, snapshotEvent, s.snapshot(request.Context(), view)) {
		select {
		case <-request.Context().Done():
			return
		case <-s.retired:
			// The directory was switched: the page reconnects in a second,
			// to the server for the directory switched to.
			_, _ = fmt.Fprint(out.w, "retry: 1000\n\n")

			out.flusher.Flush()

			return
		case <-time.After(interval):
		}
	}
}

// streamEvent names a message's kind on the event stream.
type streamEvent string

// The stream's kinds of message: the view's kept issues, sent first when
// there are any, and the snapshots.
const (
	seedEvent     streamEvent = "seed"
	snapshotEvent streamEvent = "snapshot"
)

// eventStream is where a stream writes its messages, and how many it has
// sent, which numbers the next.
type eventStream struct {
	w       http.ResponseWriter
	flusher http.Flusher
	sent    int
}

// streamPayload is what a stream's message carries.
type streamPayload interface {
	api.Seed | api.Snapshot
}

// sendEvent writes one event-stream message of event carrying payload,
// numbered after the last sent, and flushes it. It reports whether the write
// reached the client; a failed write means the connection is gone and the
// stream should stop.
func sendEvent[P streamPayload](out *eventStream, event streamEvent, payload P) bool {
	// Trade-off TRADE-13: a stream's message always encodes.
	data, err := json.Marshal(payload)
	if err != nil {
		return false
	}

	out.sent++

	_, err = fmt.Fprintf(out.w, "id: %d\nevent: %s\ndata: %s\n\n", out.sent, event, data)
	if err != nil {
		return false
	}

	out.flusher.Flush()

	return true
}

// seedFirst sends the view's kept issues ahead of the first snapshot when the
// store holds them, reporting whether the stream should go on: false only
// for a seed that could not be written.
func (s *server) seedFirst(ctx context.Context, out *eventStream, jql string) bool {
	seed, kept := s.viewSeed(ctx, jql)

	return !kept || sendEvent(out, seedEvent, seed)
}

// snapshot assembles the full read state the stream carries. A seam that is not
// configured, or that fails, yields an empty panel rather than failing the whole
// snapshot, so one unreachable upstream does not blank the cockpit; the forge's
// panel keeps its last answer instead (forgeReview). A seam that fails says so
// beside its panel, in problems, so the empty panel is not taken for an answer.
func (s *server) snapshot(ctx context.Context, view string) api.Snapshot {
	branch, branchErr := s.frameBranch(ctx)
	read, reviewErr := s.snapshotReview(ctx, branch, branchErr)

	// A queued announcement waits on what the forge says; a read that failed
	// says nothing of the pull request, so it neither posts nor drops one.
	if branchErr == nil && reviewErr == nil {
		s.settleHeld(branch, read)
	}

	review := s.reviewDTO(branch, read)
	review.Announced = s.reviewAnnounced(ctx, read)
	issues, issuesErr := s.snapshotIssues(ctx, view)
	changes, changesErr := s.snapshotChanges(ctx)
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
		Messaging:          s.readMessaging(ctx),
		QueuedAnnouncement: s.heldStatus(),
		Run:                s.RunShown(),
		HooksUnmanaged:     s.HooksUnmanaged(ctx),
		UnstartedStages:    s.knownStages(progress.Work{IssueSelected: true}),

		CommitTypes:    commit.Types(),
		SubjectLimit:   commit.SubjectLimit(),
		SuggestedScope: s.suggestedScope(ctx),

		Tasks: s.Summary(ctx),
		Here:  s.deps.Repositories.Here.Dir,
	}
	frame.Stages = s.frameStages(branch, read, frame)
	frame.Branches = s.snapshotBranches(ctx, branch.Name, frame.Stages)

	return frame
}

// frameStages is the loop's stages for the frame's branch, derived by
// progress, as the terminal's spine and `workflow status` derive them, from
// what the frame read: the branch, its pull request and CI as the forge last
// answered, the files still to commit, whether the pull request was announced
// at its moment, and an announcement held for its CI. The issue stage is done
// once the branch names an issue and not started before. An issue picked but
// not yet branched for has no branch to read here; its work story draws the
// frame's UnstartedStages instead.
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
		AnnouncePending:    heldForCI(frame.QueuedAnnouncement),
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
func (s *server) frameBranch(ctx context.Context) (gitrepo.Branch, error) {
	if s.deps.Git.Branch == nil {
		return gitrepo.Branch{}, errNoBranch
	}

	branch, err := s.readBranch(ctx)
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
// base remote names — is a problem whose code says so, which the page shows as
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
// when the branch is unknown. The tracker is asked which issues are yours under
// ctx.
func (s *server) snapshotBranches(
	ctx context.Context, checkedOut string, checkedOutStages []api.Stage,
) []api.TaskBranch {
	if s.deps.Git.Branches == nil {
		return []api.TaskBranch{}
	}

	local, err := s.deps.Git.Branches(ctx)
	if err != nil {
		return []api.TaskBranch{}
	}

	project := s.config().Jira.Project
	listing := branchListing{
		names: slices.Clone(local), remote: map[string]bool{}, links: s.issueLinks(ctx),
		worktrees: s.otherWorktrees(ctx), home: s.deps.Repositories.Home,
		stages: branchStages{
			checkedOut: checkedOutStages,
			// A branch not checked out is known only to be named for an issue:
			// the working tree, the forge and the store are read for the
			// checked-out branch alone, so nothing after it reads as begun.
			elsewhere: s.knownStages(progress.Work{OnFeatureBranch: true, IssueNamed: true}),
		},
	}

	for _, name := range s.remoteBranches(ctx) {
		if !slices.Contains(local, name) {
			listing.names = append(listing.names, name)
			listing.remote[name] = true
		}
	}

	listing.mine = s.yourIssues(ctx, issueKeys(listing.names, listing.links, project))

	return taskBranchesDTO(listing, checkedOut, project)
}

// otherWorktrees is each other worktree, gone or not, by the branch it has
// checked out: none outside a repository or when the read fails, since a
// branch then reads as one to check out, as it did before.
func (s *server) otherWorktrees(ctx context.Context) map[string]gitrepo.Worktree {
	byBranch := map[string]gitrepo.Worktree{}
	if s.deps.Repositories.Worktrees == nil {
		return byBranch
	}

	worktrees, err := s.deps.Repositories.Worktrees(ctx)
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
func (s *server) remoteBranches(ctx context.Context) []string {
	if s.deps.Git.RemoteBranches == nil {
		return nil
	}

	names, err := s.deps.Git.RemoteBranches(ctx)
	if err != nil {
		return nil
	}

	return names
}

// issueLinks is every branch linked to an issue by hand, or none when there is
// no repository to ask.
func (s *server) issueLinks(ctx context.Context) map[string]string {
	if s.deps.Git.IssueLinks == nil {
		return nil
	}

	return s.deps.Git.IssueLinks(ctx)
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
// the stream opened. The search is made under ctx.
func (s *server) snapshotIssues(ctx context.Context, view string) (api.IssuesPage, error) {
	listed, known := jira.ViewNamed(s.config().Jira.Views, view)
	if s.deps.Jira.Search == nil || !known {
		return report.IssuesPage(jira.SearchResult{}, 0), nil
	}

	result, err := s.frameIssues(ctx, listed.JQL)
	if err != nil {
		return report.IssuesPage(jira.SearchResult{}, 0), err
	}

	return report.IssuesPage(result, 0), nil
}

// snapshotChanges is the working tree's changes, or none outside a repository or
// when the read fails, with its error.
func (s *server) snapshotChanges(ctx context.Context) (api.ChangeList, error) {
	changes, err := s.readChanges(ctx)
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
func (s *server) snapshotReview(ctx context.Context, branch gitrepo.Branch, branchErr error) (forgeRead, error) {
	if branchErr != nil {
		return forgeRead{}, branchErr
	}

	return s.forgeReview(ctx, branch)
}
