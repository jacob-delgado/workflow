// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jacob-delgado/workflow/internal/loop"
	"github.com/jacob-delgado/workflow/internal/messaging"
	"github.com/jacob-delgado/workflow/internal/sanitize"
)

// announceTagging is what an announcement's tags are read through: the code
// owners of the branch's changes, whom each is on Slack and the repository's
// user groups, kept between sessions in the Slack workspace Workspace names,
// and the scopes Grant lists the Slack token as granted, which one auth.test
// answers with the workspace. Workspace and Grant are bound together, with a
// Slack user token; a nil Workspace — no such token — or OwnerLinks — no
// store — tags no one.
type announceTagging struct {
	Owners       loop.OwnerSeams
	OwnerLinks   func(ctx context.Context, workspace string) ([]loop.OwnerLink, error)
	RepoGroups   func(ctx context.Context, workspace string) ([]loop.SlackTarget, error)
	LastGroups   func(ctx context.Context, workspace string) ([]string, bool)
	RecordGroups func(ctx context.Context, workspace string, ids []string) error
	Workspace    func(ctx context.Context) (string, error)
	Grant        func(ctx context.Context) (messaging.Grant, error)
}

// tagAnnouncement previews whom the announcement at moment tags — the
// linked owners and the groups that start checked, never asked — and who it
// leaves untagged, and answers the tags with what the post remembers: the
// groups chosen, when groups were offered. A token that lacks a scope tagging
// needs is told the scope, and the post goes out untagged. base is the branch
// the owners are read against. Slack is asked under ctx.
func tagAnnouncement(ctx context.Context, out output, seams announceSeams, moment messaging.Moment, base string) (
	messaging.Mentions, loop.AnnounceMemory,
) {
	memory := seams.Memory

	slack, tagging := seams.Tagging.identity(ctx, out, moment)
	if !tagging {
		return messaging.Mentions{}, memory
	}

	workspace := slack.workspace

	tags := seams.Tagging.propose(ctx, base, moment, workspace)
	if len(tags.Owners)+len(tags.Groups) == 0 {
		return messaging.Mentions{}, memory
	}

	if scope, missing := missingScope(slack.grant, tags); missing {
		fmt.Fprintf(out.notes, "Not tagging anyone: the Slack token lacks the %s scope; "+
			"add it to the Slack app, then run workflow slack login.\n", scope)

		return messaging.Mentions{}, memory
	}

	// The lines are read from the tags as the post sends them, so they name
	// as tagged no one the mentions leave out.
	tags, mentions := tags.Proposed()

	fmt.Fprintln(out.artifact, "tags "+taggedNames(tags))

	if untagged := untaggedOwners(tags.Owners); untagged != "" {
		fmt.Fprintln(out.artifact, "not tagged: "+untagged)
	}

	if record := seams.Tagging.RecordGroups; len(tags.Groups) > 0 && record != nil {
		memory.RecordGroups = func(ctx context.Context, ids []string) error { return record(ctx, workspace, ids) }
	}

	return mentions, memory
}

// slackIdentity is what auth.test says of the Slack user token: the
// workspace it is for, and the scopes it was granted.
type slackIdentity struct {
	workspace string
	grant     messaging.Grant
}

// identity is the Slack workspace an announcement at moment tags in and the
// scopes its token was granted, and whether it tags anyone at all: only one
// ready for review does, with a Slack user token and the store, and only in a
// workspace Slack names. The grant is read first, from the auth.test that
// answers the workspace too, so a grant that cannot be read is a workspace
// that cannot be: the post goes out untagged, and the notes say why. Slack is
// asked under ctx.
func (t announceTagging) identity(ctx context.Context, out output, moment messaging.Moment) (slackIdentity, bool) {
	if moment != messaging.MomentReady || t.Workspace == nil || t.OwnerLinks == nil {
		return slackIdentity{}, false
	}

	var grant messaging.Grant

	workspace, err := loop.TagWorkspace(ctx, func(ctx context.Context) (string, error) {
		granted, err := t.Grant(ctx)
		if err != nil {
			return "", err
		}

		grant = granted

		return t.Workspace(ctx)
	})
	if errors.Is(err, loop.ErrUnknownWorkspace) {
		fmt.Fprintf(out.notes, "Not tagging anyone: %v; posting untagged.\n", err)
	}

	return slackIdentity{workspace: workspace, grant: grant}, err == nil
}

// propose is whom an announcement at moment proposes to tag from what is kept
// in workspace, the owners those of the changes since base. The tags are a
// proposal, so a read that fails proposes fewer rather than holding the
// announcement back. What is kept is read under ctx.
func (t announceTagging) propose(
	ctx context.Context, base string, moment messaging.Moment, workspace string,
) loop.Tags {
	owners, _ := loop.OwnersOf(ctx, t.Owners, base)
	links, _ := t.OwnerLinks(ctx, workspace)

	var (
		repoGroups []loop.SlackTarget
		last       []string
		chosen     bool
	)

	if t.RepoGroups != nil {
		repoGroups, _ = t.RepoGroups(ctx, workspace)
	}

	if t.LastGroups != nil {
		last, chosen = t.LastGroups(ctx, workspace)
	}

	return loop.ProposeTags(owners, links, repoGroups, last, chosen, moment)
}

// missingScope is a scope the Slack token lacks to tag, as auth.test lists
// what it was granted: the channel's members need users:read and the channel
// scopes, and groups usergroups:read. It reads no directory, which in a large
// workspace is many pages, only to learn a scope; a token whose scopes Slack
// did not list is taken to lack none.
func missingScope(grant messaging.Grant, tags loop.Tags) (string, bool) {
	needed := []string{"users:read", "channels:read", "groups:read"}
	if len(tags.Groups) > 0 {
		needed = append(needed, "usergroups:read")
	}

	for _, scope := range needed {
		if grant.Lacks(scope) {
			return scope, true
		}
	}

	return "", false
}

// taggedNames names everyone the post tags, as Slack shows them: the linked
// people, then the checked groups; or no one.
func taggedNames(tags loop.Tags) string {
	var names []string

	for _, owner := range tags.Owners {
		if !owner.Team && owner.State == loop.OwnerLinked {
			names = append(names, "@"+sanitize.Line(owner.Slack.Label))
		}
	}

	for _, group := range tags.Groups {
		if group.Checked {
			names = append(names, "@"+sanitize.Line(group.Slack.Label))
		}
	}

	if len(names) == 0 {
		return "no one"
	}

	return strings.Join(names, " ")
}

// untaggedOwners names each owner the post leaves untagged, and why.
func untaggedOwners(owners []loop.OwnerTag) string {
	reasons := map[loop.OwnerState]string{
		loop.OwnerUnlinked:   " (not linked — associate in People and groups)",
		loop.OwnerNotOnSlack: " (not on Slack)",
		loop.OwnerLinked:     "",
	}

	var untagged []string

	for _, owner := range owners {
		if owner.State != loop.OwnerLinked {
			untagged = append(untagged, owner.Owner+reasons[owner.State])
		}
	}

	return strings.Join(untagged, ", ")
}
