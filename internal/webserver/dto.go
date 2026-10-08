// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver

import (
	"slices"
	"time"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/convention"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/loop"
	"github.com/jacob-delgado/workflow/internal/workdirs"
)

// optional carries an empty string to the wire as an absent field rather than an
// empty one, for the properties the contract marks optional.
func optional(s string) *string {
	if s == "" {
		return nil
	}

	return &s
}

// optionalTime is moment, or nil for the zero time, which a read gives for a
// time there is none of — a date a task does not have, one a tracker or a
// forge did not give or could not be read: the wire leaves such a time out,
// never sending the zero time.
func optionalTime(moment time.Time) *time.Time {
	if moment.IsZero() {
		return nil
	}

	return &moment
}

// issueDTO maps a tracker issue onto its wire shape.
func issueDTO(issue jira.Issue) api.Issue {
	return api.Issue{
		Key:            string(issue.Key),
		Tracker:        trackerOf(issue.Key),
		Summary:        issue.Summary,
		Status:         issue.Status,
		StatusCategory: api.StatusCategory(issue.StatusCategory),
		Type:           issue.Type,
		Priority:       optional(issue.Priority),
	}
}

// trackerOf is where the issue a key names lives, by the key's shape: a
// number is the forge's, anything else Jira's.
func trackerOf(issueKey jira.Key) api.IssueTracker {
	ref, known := convention.RefOf(string(issueKey))
	if known && ref.Tracker == convention.TrackerForge {
		return api.IssueTrackerForge
	}

	return api.IssueTrackerJira
}

// issuesPageDTO maps a page of search results, keeping the caller's start index.
func issuesPageDTO(result jira.SearchResult, startAt int) api.IssuesPage {
	issues := make([]api.Issue, 0, len(result.Issues))
	for _, issue := range result.Issues {
		issues = append(issues, issueDTO(issue))
	}

	unavailable := result.Unavailable
	if unavailable == nil {
		unavailable = []string{}
	}

	return api.IssuesPage{Issues: issues, Total: result.Total, StartAt: startAt, Unavailable: unavailable}
}

// commentDTO maps a comment on an issue.
func commentDTO(comment jira.Comment) api.Comment {
	return api.Comment{Author: comment.Author, Body: comment.Body, Created: optionalTime(comment.Created)}
}

// issueDetailDTO maps an issue read in full, with its comments oldest first and
// link, the issue's page in the tracker ("" when there is none to give).
func issueDetailDTO(detail jira.IssueDetail, link string) api.IssueDetail {
	comments := make([]api.Comment, 0, len(detail.Comments))
	for _, comment := range detail.Comments {
		comments = append(comments, commentDTO(comment))
	}

	return api.IssueDetail{
		Key:            string(detail.Issue.Key),
		Tracker:        trackerOf(detail.Issue.Key),
		Summary:        detail.Issue.Summary,
		Status:         detail.Issue.Status,
		StatusCategory: api.StatusCategory(detail.Issue.StatusCategory),
		Type:           detail.Issue.Type,
		Priority:       optional(detail.Issue.Priority),
		Reporter:       detail.Reporter,
		Assignee:       optional(detail.Assignee),
		Description:    detail.Description,
		Comments:       comments,
		CommentTotal:   detail.CommentTotal,
		URL:            link,
	}
}

// branchDTO maps the current branch and its commits.
func branchDTO(branch gitrepo.Branch) api.Branch {
	unpushed := branch.Unpushed()

	commits := make([]api.Commit, 0, len(branch.Commits))
	for _, commit := range branch.Commits {
		commits = append(commits, api.Commit{
			Hash: commit.Hash, Subject: commit.Subject, Unpushed: slices.Contains(unpushed, commit),
		})
	}

	return api.Branch{
		IssueLink:        branch.IssueLink,
		IssueLinkTracker: linkTracker(branch.IssueLink),
		FinishCommands:   finishCommands(branch),
		Name:             branch.Name,
		Detached:         branch.Detached,
		Head:             branch.Head,
		Upstream:         branch.Upstream,
		PushRemote:       branch.PushRemote,
		Ahead:            branch.Ahead,
		Behind:           branch.Behind,
		Base:             branch.Base,
		Commits:          commits,
	}
}

// finishCommands are the commands finishing branch runs, or nil for one with
// no branch or no base to finish onto.
func finishCommands(branch gitrepo.Branch) *[]string {
	if branch.Detached || branch.Name == "" || branch.BaseName() == "" {
		return nil
	}

	commands := gitrepo.FinishCommands(branch.BaseName(), branch.Name)

	return &commands
}

// linkTracker is where the issue a branch was linked to lives, or nil for a
// branch never linked.
func linkTracker(link string) *api.IssueTracker {
	if link == "" {
		return nil
	}

	tracker := trackerOf(jira.Key(link))

	return &tracker
}

// branchListing is the branches a frame lists: each name, the local ones
// first; which of them only the remote has; and which issues are yours — nil
// counting every issue, as when there is no tracker to ask.
type branchListing struct {
	names  []string
	remote map[string]bool
	mine   map[jira.Key]bool
	// links are the branches linked to an issue by hand, by name.
	links map[string]string
	// worktrees are the other worktrees, by the branch each has checked
	// out, and home what their directories are written from.
	worktrees map[string]gitrepo.Worktree
	home      string
}

// taskBranchesDTO maps branch names to the issues they are named for, keeping
// only the branches that name one of yours and the checked-out branch, whoever
// its issue is assigned to — the work story reads the issue being worked on
// from it — marking the checked-out branch, and saying of each whether only
// the remote has it and which other worktree has it checked out — false and
// empty too, as the client's schema defaults them, so a frame reads back as it
// was sent. The slice is non-nil so the wire value is an empty
// array rather than null, matching the snapshot's other collections.
func taskBranchesDTO(listing branchListing, current, project string) []api.TaskBranch {
	branches := make([]api.TaskBranch, 0, len(listing.names))
	for _, name := range listing.names {
		key, named := loop.NamedIssue(name, listing.links, project)
		if !named || (name != current && listing.mine != nil && !listing.mine[jira.Key(key.Key)]) {
			continue
		}

		remote, worktree := listing.remote[name], listing.worktrees[name]

		shown := ""
		if worktree.Dir != "" {
			shown = workdirs.Shown(worktree.Dir, listing.home)
		}

		branches = append(branches, api.TaskBranch{
			Name:            name,
			IssueKey:        key.Key,
			Current:         name == current,
			Remote:          &remote,
			Worktree:        &worktree.Dir,
			WorktreeShown:   &shown,
			WorktreeMissing: &worktree.Missing,
		})
	}

	return branches
}

// changesDTO maps the working tree's changes, each with a display-ready kind.
func changesDTO(changes []gitrepo.Change) api.ChangeList {
	out := make([]api.Change, 0, len(changes))
	for _, change := range changes {
		out = append(out, api.Change{
			Path:         change.Path,
			OriginalPath: optional(change.OriginalPath),
			Kind:         api.ChangeKind(change.Kind()),
			Staged:       change.IsStaged(),
			HasUnstaged:  change.HasUnstaged(),
			Conflicted:   change.Conflicted(),
		})
	}

	return api.ChangeList{Changes: out}
}

// pullDTO maps a pull request.
func pullDTO(pull forge.PullRequest) api.PullRequest {
	return api.PullRequest{
		Number:           pull.Number,
		URL:              pull.URL,
		Title:            pull.Title,
		State:            pullState(pull.State),
		Draft:            pull.Draft,
		Approvals:        pull.Approvals,
		ChangesRequested: pull.ChangesRequested,
		Mergeable:        mergeable(pull.Mergeable),
	}
}

// pullState maps where a pull request stands onto its wire word. A map, as
// ciState is, so exhaustive keeps it complete.
func pullState(state forge.PullState) api.PullRequestState {
	return map[forge.PullState]api.PullRequestState{
		forge.StateOpen:   api.PullRequestStateOpen,
		forge.StateMerged: api.PullRequestStateMerged,
		forge.StateClosed: api.PullRequestStateClosed,
	}[state]
}

// reviewQueueDTO maps the review queue onto the wire.
func reviewQueueDTO(requests []forge.ReviewRequest) api.ReviewQueue {
	return api.ReviewQueue{Available: true, Requests: ReviewRequests(requests)}
}

// ReviewRequests maps the requests waiting on your review onto the wire, an
// empty list rather than null when none waits: the requests GET /api/reviews
// answers, and what `workflow reviews --json` prints.
func ReviewRequests(requests []forge.ReviewRequest) []api.ReviewRequest {
	queue := make([]api.ReviewRequest, 0, len(requests))
	for _, request := range requests {
		queue = append(queue, api.ReviewRequest{
			Number: request.Number, URL: request.URL, Title: request.Title, Author: request.Author,
			Repository: request.Repository, Draft: request.Draft, Ci: ciState(request.CI),
			OpenedAt: optionalTime(request.OpenedAt),
		})
	}

	return queue
}

// noReviewQueue is the answer where there is no forge to ask: not available,
// and empty.
func noReviewQueue() api.ReviewQueue {
	return api.ReviewQueue{Available: false, Requests: []api.ReviewRequest{}}
}

// ciDTO maps a CI result and its checks.
func ciDTO(status forge.CI) api.CI {
	checks := make([]api.Check, 0, len(status.Checks))
	for _, check := range status.Checks {
		checks = append(checks, api.Check{
			Name: check.Name, State: ciState(check.State), URL: check.URL,
			ID: optional(check.ID), Stage: optional(check.Stage), Reason: optional(check.Reason),
			LogAvailable: optionalTrue(check.LogAvailable),
		})
	}

	return api.CI{
		State:  ciState(status.State),
		Total:  status.Total,
		Done:   status.Done,
		Failed: status.Failed,
		Checks: checks,
	}
}

// ciState is the forge's CI state as the API writes it, in the forge's own
// word for it.
func ciState(state forge.CIState) api.CIState {
	return api.CIState(state.Word())
}

// queuedState maps how a held announcement stands onto its wire word. A map,
// not a switch, so there is no last-case arm gobco can never see; exhaustive
// keeps it complete. heldNone has no word, since a frame then shows no held
// announcement at all.
func queuedState(state heldState) api.QueuedAnnouncementState {
	return map[heldState]api.QueuedAnnouncementState{
		heldNone:       "",
		heldWaiting:    api.QueuedAnnouncementStateWaiting,
		heldAnnouncing: api.QueuedAnnouncementStateAnnouncing,
		heldAnnounced:  api.QueuedAnnouncementStateAnnounced,
		heldDropped:    api.QueuedAnnouncementStateDropped,
	}[state]
}

// messagingDTO maps the messaging destination: the service in use, the channel,
// its alternates (an empty list rather than null on the wire), and who a post
// would come from.
func messagingDTO(cfg config.Config, author string) api.MessagingDestination {
	channels := cfg.Messaging.ChannelChoices()
	if channels == nil {
		channels = []string{}
	}

	return api.MessagingDestination{
		Kind:       messagingKind(cfg.Messaging.Kind),
		Service:    cfg.Messaging.Service(),
		Configured: cfg.Messaging.Mode() != config.MessagingNone,
		Channel:    cfg.Messaging.Channel,
		Channels:   channels,
		Author:     author,
	}
}

// messagingKind maps the service a messaging block posts to onto its wire
// word. A map, as ciState is, so exhaustive keeps it complete; the empty kind
// is read as Slack, as the configuration reads it.
func messagingKind(kind config.MessagingKind) api.MessagingDestinationKind {
	return map[config.MessagingKind]api.MessagingDestinationKind{
		"":                 api.MessagingDestinationKindSlack,
		config.KindSlack:   api.MessagingDestinationKindSlack,
		config.KindTeams:   api.MessagingDestinationKindTeams,
		config.KindDiscord: api.MessagingDestinationKindDiscord,
		config.KindWebhook: api.MessagingDestinationKindWebhook,
	}[kind]
}

// mergeable maps the forge's mergeability onto its wire word.
func mergeable(state forge.Mergeability) api.PullRequestMergeable {
	return map[forge.Mergeability]api.PullRequestMergeable{
		forge.MergeUnknown:   api.PullRequestMergeableUnknown,
		forge.MergeClean:     api.PullRequestMergeableClean,
		forge.MergeConflicts: api.PullRequestMergeableConflicts,
	}[state]
}

// linkedIssue is the issue branch and its pull request are for, wherever it
// was named, with its page; nil when none is. Its key and page are worked out
// here, never read from the tracker, since the stream reads the review often.
func (s *server) linkedIssue(branch gitrepo.Branch, pull forge.PullRequest) *api.LinkedIssue {
	ref, origin, found := loop.BranchIssue(loop.IssueSource{
		Branch: branch.Name, Link: branch.IssueLink, Pull: &pull, Project: s.config().Jira.Project,
	})
	if !found {
		return nil
	}

	origins := map[loop.IssueOrigin]api.LinkedIssueOrigin{
		loop.OriginLink: api.LinkedIssueOriginByHand, loop.OriginBranch: api.LinkedIssueOriginBranchName,
		loop.OriginPull: api.LinkedIssueOriginPullRequest,
	}

	return &api.LinkedIssue{
		Key: ref.Key, Tracker: trackerOf(jira.Key(ref.Key)), URL: s.browseURL(jira.Key(ref.Key)), Origin: origins[origin],
	}
}

// optionalTrue is a flag the wire leaves out when it is false.
func optionalTrue(flag bool) *bool {
	if !flag {
		return nil
	}

	return &flag
}

// statusChangesDTO maps the status changes a tracker offers onto the wire,
// each with the fields it needs; never nil, so it goes to the page as [].
func statusChangesDTO(moves []jira.Transition) []api.StatusChange {
	changes := make([]api.StatusChange, 0, len(moves))
	for _, move := range moves {
		fields := make([]api.StatusChangeField, 0, len(move.Fields))
		for _, field := range move.Fields {
			fields = append(fields, statusChangeFieldDTO(field))
		}

		changes = append(changes, api.StatusChange{
			ID: move.ID, Name: move.Name, ToStatus: move.ToStatus,
			ToStatusCategory: api.StatusCategory(move.ToStatusCategory), Fields: fields,
		})
	}

	return changes
}

// statusChangeFieldDTO maps a field a status change needs, with the values it
// allows. A map, not a switch, as ciState's is.
func statusChangeFieldDTO(field jira.Field) api.StatusChangeField {
	options := make([]api.FieldOption, 0, len(field.Options))
	for _, option := range field.Options {
		options = append(options, api.FieldOption{ID: option.ID, Name: option.Name})
	}

	kind := map[jira.FieldKind]api.StatusChangeFieldKind{
		jira.FieldUnsupported: api.StatusChangeFieldKindOnlyJira,
		jira.FieldOption:      api.StatusChangeFieldKindOption,
		jira.FieldOptionList:  api.StatusChangeFieldKindOptionList,
		jira.FieldText:        api.StatusChangeFieldKindText,
		jira.FieldUser:        api.StatusChangeFieldKindUser,
		jira.FieldDate:        api.StatusChangeFieldKindDate,
	}[field.Kind]

	return api.StatusChangeField{ID: field.ID, Name: field.Name, Kind: kind, Options: options}
}
