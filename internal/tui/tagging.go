// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/jacob-delgado/workflow/internal/codeowners"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/loop"
	"github.com/jacob-delgado/workflow/internal/messaging"
	"github.com/jacob-delgado/workflow/internal/seams"
)

// ownerColumn is how wide an owner's name is padded, so their states line up.
const ownerColumn = 20

// tagProposal is what the tags are proposed from: the changes' owners, what
// was decided for them, the repository's groups, and the last choice.
type tagProposal struct {
	owners     codeowners.Owners
	links      []loop.OwnerLink
	repoGroups []loop.SlackTarget
	last       []string
	lastChosen bool
}

// tags is who the proposal tags.
func (p tagProposal) tags() loop.Tags {
	return loop.ProposeTags(p.owners, p.links, p.repoGroups, p.last, p.lastChosen, messaging.MomentReady)
}

// relinked is the proposal with link in place of what was decided for its
// owner before.
func (p tagProposal) relinked(link loop.OwnerLink) tagProposal {
	sameOwner := func(old loop.OwnerLink) bool { return loop.SameOwner(old.Owner, link.Owner) }
	p.links = append(slices.DeleteFunc(slices.Clone(p.links), sameOwner), link)

	return p
}

// tagSection is whom a ready-for-review announcement tags: the code owners of
// its changes and the user groups it offers, which of those are checked, and
// the row the cursor is on — the owners first, then the groups.
type tagSection struct {
	offered  bool
	canLink  bool
	reading  bool
	proposal tagProposal
	tags     loop.Tags
	checked  []string
	readErr  error
	linkErr  error
	members  directory
	groups   directory
	cursor   int
	// workspace is the Slack workspace the tags were read under, which a
	// post's choice of groups is kept under too, and workspaceErr why it
	// could not be read, when nobody is tagged.
	workspace    string
	workspaceErr error
}

// withTagging is the preview with a tag section and the reads that fill it,
// for a ready-for-review announcement posted with a Slack user token: only
// that one tags anyone, and only that token can read the directory.
func (m Model) withTagging(preview messagingPreview) (messagingPreview, tea.Cmd) {
	if preview.moment != messaging.MomentReady || m.cfg.Messaging.Mode() != config.MessagingUser ||
		m.deps.Messaging.ChannelMembers == nil {
		return preview, nil
	}

	var readMembers, readGroups tea.Cmd

	preview.tagging = tagSection{offered: true, canLink: m.deps.Store.LinkOwner != nil, reading: true}
	preview.tagging.members, readMembers = readChannelMembers(m.deps, preview.channel, preview.opened)
	preview.tagging.groups, readGroups = readUserGroups(m.deps, preview.opened)

	return preview, tea.Batch(m.branch.readTags(m.deps, preview.opened), readMembers, readGroups)
}

// readTags reads whom the announcement proposes to tag: the owners of the
// branch's changes against its base, and what the store kept about them in
// the Slack workspace the token is for, for the preview opened as opened.
func (s branchState) readTags(deps Deps, opened int) tea.Cmd {
	owners := taggedOwnerSeams(deps)
	store, base, readWorkspace := deps.Store, s.branch.BaseName(), deps.Messaging.Workspace

	return func() tea.Msg {
		found, ownersErr := loop.OwnersOf(owners, base)

		workspace, err := loop.TagWorkspace(readWorkspace)
		if err != nil {
			return tagsRead{opened: opened, proposal: tagProposal{owners: found}, err: ownersErr, workspaceErr: err}
		}

		proposal, keptErr := readProposal(store, workspace)
		proposal.owners = found

		return tagsRead{opened: opened, proposal: proposal, err: errors.Join(ownersErr, keptErr), workspace: workspace}
	}
}

// taggedOwnerSeams are what the owners a post tags are read through: the
// forge tells a bare name that is a group from a person, since a group is
// tagged through its user group.
func taggedOwnerSeams(deps Deps) loop.OwnerSeams {
	return loop.OwnerSeams{
		ChangedPaths: deps.Git.ChangedPaths, CodeOwnersAt: deps.Git.CodeOwnersAt, Author: deps.Forge.Author,
		IsGroup: deps.Forge.IsGroup,
	}
}

// readProposal is what the store kept in workspace that tags are proposed
// from: what was decided for owners, the repository's groups, and the last
// choice.
func readProposal(store seams.Store, workspace string) (tagProposal, error) {
	var proposal tagProposal

	links, linksErr := readKept(store.OwnerLinks, workspace)
	groups, groupsErr := readKept(store.RepoGroups, workspace)

	if store.LastGroups != nil {
		proposal.last, proposal.lastChosen = store.LastGroups(workspace)
	}

	proposal.links, proposal.repoGroups = links, groups

	return proposal, errors.Join(linksErr, groupsErr)
}

// readKept reads a list the store keeps in workspace, or nothing when there
// is no store, as under a dry run.
func readKept[T any](read func(workspace string) ([]T, error), workspace string) ([]T, error) {
	if read == nil {
		return nil, nil
	}

	return read(workspace)
}

// tagsRead is whom the announcement proposes to tag, and why some of it could
// not be read: workspaceErr when the Slack workspace could not be, which
// leaves nobody to tag.
type tagsRead struct {
	opened       int
	proposal     tagProposal
	err          error
	workspace    string
	workspaceErr error
}

var _ applier = tagsRead{}

// apply fills the preview's tag section, when the preview is still open.
func (msg tagsRead) apply(m Model) (Model, tea.Cmd) {
	preview, open := beneath[messagingPreview](m, msg.opened)
	if !open {
		return m, nil
	}

	preview.tagging = preview.tagging.proposed(msg.proposal, msg.err)
	preview.tagging.workspace, preview.tagging.workspaceErr = msg.workspace, msg.workspaceErr

	return m.withBeneath(preview), nil
}

// proposed is the section once its proposal is read, with the groups the
// proposal checks checked.
func (s tagSection) proposed(proposal tagProposal, err error) tagSection {
	s.reading, s.readErr, s.proposal, s.tags = false, err, proposal, proposal.tags()
	s.checked = nil

	for _, group := range s.tags.Groups {
		if group.Checked {
			s.checked = append(s.checked, group.Slack.ID)
		}
	}

	return s.withGroupsReadable()
}

// withGroupsReadable is the section offering no group when the token cannot
// read the user groups: the people it can tag are tagged, and the groups wait
// until the scope is granted.
func (s tagSection) withGroupsReadable() tagSection {
	if s.groups.missingScope() == "" {
		return s
	}

	s.tags.Groups, s.checked = nil, nil

	return s.moved(0)
}

// missingScope is the scope the read of the channel's members said the token
// lacks: tagging then waits until it is granted, and the post goes out
// untagged.
func (s tagSection) missingScope() string {
	return s.members.missingScope()
}

// shown reports a section offered whose directory could be asked at all: a
// directory with no credential to read it with means no tagging, said
// nowhere, as for a webhook.
func (s tagSection) shown() bool {
	noCredential := errors.Is(s.members.err, messaging.ErrNoCredential) ||
		errors.Is(s.groups.err, messaging.ErrNoCredential) || errors.Is(s.workspaceErr, messaging.ErrNoCredential)

	return s.offered && !noCredential
}

// tags reports a section that tags anyone: one shown, whose Slack workspace
// was read and whose token has the scope tagging needs.
func (s tagSection) tagsAnyone() bool {
	return s.shown() && s.workspaceErr == nil && s.missingScope() == ""
}

// interactive reports a section whose rows can be moved through and changed.
func (s tagSection) interactive() bool {
	return s.tagsAnyone() && !s.reading
}

// lines draws the section below the destination.
func (s tagSection) lines(kit renderKit, width int) []string {
	switch {
	case !s.shown():
		return nil
	case s.missingScope() != "":
		return []string{kit.failedGlyph() + " tagging needs the " + s.missingScope() + " scope; this posts untagged"}
	case s.workspaceErr != nil:
		return []string{kit.failureBlock(s.workspaceErr, width), "this posts untagged"}
	case s.reading:
		return []string{kit.marks.inFlight + " reading whom to tag" + kit.marks.ellipsis}
	}

	lines := slices.Concat(s.ownerLines(kit.marks), s.groupLines(kit.marks), s.failureLines(kit, width))

	return append(lines, "tags  "+s.summary())
}

// failureLines say what could not be read or saved, and the scope groups
// need when the token lacks it.
func (s tagSection) failureLines(kit renderKit, width int) []string {
	var lines []string

	groupsErr := s.groups.err
	if scope := s.groups.missingScope(); scope != "" {
		groupsErr = nil

		lines = append(lines, kit.failedGlyph()+" tagging groups needs the "+scope+" scope")
	}

	for _, err := range []error{s.readErr, s.members.err, groupsErr, s.linkErr} {
		if err != nil && !errors.Is(err, messaging.ErrNoUserGroups) {
			lines = append(lines, kit.failureBlock(err, width))
		}
	}

	return lines
}

// ownerLines are the code owners, each with what is known of them on Slack.
func (s tagSection) ownerLines(marks glyphs) []string {
	if len(s.tags.Owners) == 0 {
		return nil
	}

	lines := []string{"Code owners"}

	for index, owner := range s.tags.Owners {
		state := map[loop.OwnerState]string{
			loop.OwnerUnlinked:   "? not linked",
			loop.OwnerLinked:     strings.TrimSpace(marks.arrow) + " " + slackName(owner.Slack, owner.Team),
			loop.OwnerNotOnSlack: marks.unknown + " not on Slack",
		}[owner.State]
		lines = append(lines, marks.marker(index == s.cursor)+fmt.Sprintf("%-*s ", ownerColumn, owner.Owner)+state)
	}

	return lines
}

// groupLines are the groups offered, each checked when it is tagged.
func (s tagSection) groupLines(marks glyphs) []string {
	if len(s.tags.Groups) == 0 {
		return nil
	}

	lines := []string{"Groups"}

	for index, group := range s.tags.Groups {
		row := marks.marker(len(s.tags.Owners)+index == s.cursor) +
			checkbox(slices.Contains(s.checked, group.Slack.ID)) + slackName(group.Slack, true)
		if group.FromOwners {
			row += "   owns changed paths"
		}

		lines = append(lines, row)
	}

	return lines
}

// summary names everyone the post tags, or nobody.
func (s tagSection) summary() string {
	var names []string

	for _, owner := range s.tags.Owners {
		if !owner.Team && owner.State == loop.OwnerLinked {
			names = append(names, "@"+slackName(owner.Slack, false))
		}
	}

	for _, group := range s.tags.Groups {
		if slices.Contains(s.checked, group.Slack.ID) {
			names = append(names, slackName(group.Slack, true))
		}
	}

	if len(names) == 0 {
		return "nobody"
	}

	return strings.Join(names, " ")
}

// dryRunNote says whom a dry run's post would have tagged.
func (s tagSection) dryRunNote() string {
	if !s.shown() {
		return ""
	}

	return ", tagging " + s.summary()
}

// postTags is whom a post tags, and whether it offered groups to choose, which
// makes the choice — even of none — one to remember in the Slack workspace
// they were offered in.
type postTags struct {
	mentions     messaging.Mentions
	offersGroups bool
	workspace    string
}

// postTags is whom the post tags: no one but for a ready-for-review
// announcement, which alone offers tags. A token lacking a scope tags no one,
// nor does one whose workspace Slack would not name, and so does a tag Slack
// could not read: tagging never holds a post back.
func (s tagSection) postTags() postTags {
	if !s.tagsAnyone() {
		return postTags{}
	}

	mentions, err := s.tags.Mentions(s.checked)
	if err != nil {
		return postTags{}
	}

	return postTags{mentions: mentions, offersGroups: len(s.tags.Groups) > 0, workspace: s.workspace}
}

// keys are the tag section's keys where the cursor is: linking an owner, or
// tagging a group.
func (s tagSection) keys(keys keyMap) []key.Binding {
	if !s.interactive() {
		return nil
	}

	var bindings []key.Binding

	if _, onOwner := s.selectedOwner(); onOwner && s.canLink {
		bindings = append(bindings, keys.linkToSlack, keys.notOnSlack)
	}

	if s.cursor >= len(s.tags.Owners) {
		bindings = append(bindings, relabel(keys.toggleOption, "tag"))
	}

	return append(bindings, keys.up, keys.down)
}

// selectedOwner is the owner under the cursor, if the cursor is on one.
func (s tagSection) selectedOwner() (loop.OwnerTag, bool) {
	if s.cursor >= len(s.tags.Owners) {
		return loop.OwnerTag{}, false
	}

	return s.tags.Owners[s.cursor], true
}

// moved is the section with the cursor moved by step, held to its rows.
func (s tagSection) moved(step int) tagSection {
	s.cursor = max(0, min(s.cursor+step, len(s.tags.Owners)+len(s.tags.Groups)-1))

	return s
}

// toggled is the section with the group under the cursor tagged, or untagged
// when it was; on an owner, or with no row at all, it changes nothing.
func (s tagSection) toggled() tagSection {
	index := s.cursor - len(s.tags.Owners)
	if index < 0 || index >= len(s.tags.Groups) {
		return s
	}

	id := s.tags.Groups[index].Slack.ID
	if slices.Contains(s.checked, id) {
		s.checked = slices.DeleteFunc(slices.Clone(s.checked), func(checked string) bool { return checked == id })
	} else {
		s.checked = append(slices.Clone(s.checked), id)
	}

	return s
}

// relinked is the section once a link was saved, or with why it was not. A
// group the link newly offers starts as proposed; the others keep their
// check.
func (s tagSection) relinked(link loop.OwnerLink, err error) tagSection {
	s.linkErr = err
	if err != nil {
		return s
	}

	before := s.tags.Groups
	s.proposal = s.proposal.relinked(link)
	s.tags = s.proposal.tags()

	var checked []string

	for _, group := range s.tags.Groups {
		offered := slices.ContainsFunc(before, func(old loop.GroupTag) bool { return old.Slack.ID == group.Slack.ID })
		if offered && slices.Contains(s.checked, group.Slack.ID) || !offered && group.Checked {
			checked = append(checked, group.Slack.ID)
		}
	}

	s.checked = checked

	return s.withGroupsReadable()
}
