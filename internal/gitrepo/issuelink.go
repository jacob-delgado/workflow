// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package gitrepo

import (
	"context"
	"fmt"
	"strings"
)

// issueLinkKey is the variable, under a branch's own section of git's
// configuration, that names the issue a branch was linked to by hand: work
// begun outside workflow, on a branch whose name names no issue. Kept in git,
// it stays with the clone and goes when git deletes the branch.
const issueLinkKey = "workflow-issue"

// IssueLink is the issue branch was linked to, or empty when it was never
// linked — or when what git holds there is not text to show.
func (r Repository) IssueLink(ctx context.Context, branch string) string {
	link := r.storedLink(ctx, branch)
	if shownAsTheyAre(link) != nil {
		return ""
	}

	return link
}

// SetIssueLink links branch to the issue issueKey names.
func (r Repository) SetIssueLink(ctx context.Context, branch, issueKey string) error {
	_, err := r.run(ctx, gitProgram, "-C", r.dir, "config", linkVariable(branch), issueKey)
	if err != nil {
		return fmt.Errorf("linking %s to %s: %w", branch, issueKey, err)
	}

	return nil
}

// ClearIssueLink forgets the issue branch was linked to, if it was — even a
// value IssueLink will not show, since it is the one way to be rid of it.
func (r Repository) ClearIssueLink(ctx context.Context, branch string) error {
	if r.storedLink(ctx, branch) == "" {
		return nil
	}

	_, err := r.run(ctx, gitProgram, "-C", r.dir, "config", "--unset", linkVariable(branch))
	if err != nil {
		return fmt.Errorf("unlinking %s: %w", branch, err)
	}

	return nil
}

// IssueLinks is every branch linked to an issue by hand, by branch name, for a
// list of branches to read at once. A repository with none, or whose
// configuration cannot be read, has none; an entry that is not text to show is
// left out.
func (r Repository) IssueLinks(ctx context.Context) map[string]string {
	listed := optional(ctx, r.run, "-C", r.dir, "config", "--get-regexp", `^branch\..*\.`+issueLinkKey+`$`)
	links := map[string]string{}

	for line := range strings.SplitSeq(listed, "\n") {
		variable, link, found := strings.Cut(line, " ")
		branch, named := strings.CutSuffix(strings.TrimPrefix(variable, "branch."), "."+issueLinkKey)

		if found && named && shownAsTheyAre(branch, link) == nil {
			links[branch] = link
		}
	}

	return links
}

// storedLink is what git holds as branch's link, whether or not it can be shown.
func (r Repository) storedLink(ctx context.Context, branch string) string {
	return optional(ctx, r.run, "-C", r.dir, "config", "--get", linkVariable(branch))
}

// linkVariable is the configuration variable holding branch's link.
func linkVariable(branch string) string {
	return "branch." + branch + "." + issueLinkKey
}
