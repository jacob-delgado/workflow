// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver

import (
	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/convention"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/jira"
)

// optional carries an empty string to the wire as an absent field rather than an
// empty one, for the properties the contract marks optional.
func optional(s string) *string {
	if s == "" {
		return nil
	}

	return &s
}

// issueDTO maps a tracker issue onto its wire shape.
func issueDTO(issue jira.Issue) api.Issue {
	return api.Issue{
		Key:            string(issue.Key),
		Summary:        issue.Summary,
		Status:         issue.Status,
		StatusCategory: api.StatusCategory(issue.StatusCategory),
		Type:           issue.Type,
		Priority:       optional(issue.Priority),
	}
}

// issuesPageDTO maps a page of search results, keeping the caller's start index.
func issuesPageDTO(result jira.SearchResult, startAt int) api.IssuesPage {
	issues := make([]api.Issue, 0, len(result.Issues))
	for _, issue := range result.Issues {
		issues = append(issues, issueDTO(issue))
	}

	return api.IssuesPage{Issues: issues, Total: result.Total, StartAt: startAt}
}

// issueDetailDTO maps an issue read in full, with its comments oldest first.
func issueDetailDTO(detail jira.IssueDetail) api.IssueDetail {
	comments := make([]api.Comment, 0, len(detail.Comments))
	for _, comment := range detail.Comments {
		comments = append(comments, api.Comment{
			Author:  comment.Author,
			Body:    comment.Body,
			Created: comment.Created,
		})
	}

	return api.IssueDetail{
		Key:            string(detail.Issue.Key),
		Summary:        detail.Issue.Summary,
		Status:         detail.Issue.Status,
		StatusCategory: api.StatusCategory(detail.Issue.StatusCategory),
		Type:           detail.Issue.Type,
		Priority:       optional(detail.Issue.Priority),
		Reporter:       detail.Reporter,
		Description:    detail.Description,
		Comments:       comments,
		CommentTotal:   detail.CommentTotal,
	}
}

// branchDTO maps the current branch and its commits.
func branchDTO(branch gitrepo.Branch) api.Branch {
	commits := make([]api.Commit, 0, len(branch.Commits))
	for _, commit := range branch.Commits {
		commits = append(commits, api.Commit{Hash: commit.Hash, Subject: commit.Subject})
	}

	return api.Branch{
		Name:     branch.Name,
		Detached: branch.Detached,
		Head:     branch.Head,
		Upstream: branch.Upstream,
		Ahead:    branch.Ahead,
		Behind:   branch.Behind,
		Base:     branch.Base,
		Commits:  commits,
	}
}

// taskBranchesDTO maps local branch names to the issues they are named for,
// keeping only the branches that name one and marking the checked-out branch.
// The slice is non-nil so the wire value is an empty array rather than null,
// matching the snapshot's other collections.
func taskBranchesDTO(names []string, current, project string) []api.TaskBranch {
	branches := make([]api.TaskBranch, 0, len(names))
	for _, name := range names {
		key, named := convention.IssueKey(name, project)
		if !named {
			continue
		}

		branches = append(branches, api.TaskBranch{
			Name:     name,
			IssueKey: key,
			Current:  name == current,
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
		Draft:            pull.Draft,
		Approvals:        pull.Approvals,
		ChangesRequested: pull.ChangesRequested,
		Mergeable:        mergeable(pull.Mergeable),
	}
}

// ciDTO maps a CI result and its checks.
func ciDTO(status forge.CI) api.CI {
	checks := make([]api.Check, 0, len(status.Checks))
	for _, check := range status.Checks {
		checks = append(checks, api.Check{Name: check.Name, State: ciState(check.State), URL: check.URL})
	}

	return api.CI{
		State:  ciState(status.State),
		Total:  status.Total,
		Done:   status.Done,
		Failed: status.Failed,
		Checks: checks,
	}
}

// ciState maps the forge's CI state onto its wire word. A map, not a switch, so
// there is no last-case arm gobco can never see; exhaustive keeps it complete.
func ciState(state forge.CIState) api.CIState {
	return map[forge.CIState]api.CIState{
		forge.CINone:    api.None,
		forge.CIRunning: api.Running,
		forge.CIPassed:  api.Passed,
		forge.CIFailed:  api.Failed,
	}[state]
}

// slackDTO maps the Slack destination: the channel, its alternates (an empty
// list rather than null on the wire), and who a post would come from.
func slackDTO(cfg config.Config, author string) api.Slack {
	channels := cfg.Messaging.ChannelChoices()
	if channels == nil {
		channels = []string{}
	}

	return api.Slack{Channel: cfg.Messaging.Channel, Channels: channels, Author: author}
}

// mergeable maps the forge's mergeability onto its wire word.
func mergeable(state forge.Mergeability) api.PullRequestMergeable {
	return map[forge.Mergeability]api.PullRequestMergeable{
		forge.MergeUnknown:   api.Unknown,
		forge.MergeClean:     api.Clean,
		forge.MergeConflicts: api.Conflicts,
	}[state]
}
