// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"cmp"
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
	preview.tagging.members, readMembers = m.readMembers(preview.channel, preview.opened)
	preview.tagging.groups, readGroups = m.readUserGroups(preview.opened)

	return preview, tea.Batch(m.readTags(preview.opened), readMembers, readGroups)
}

// readTags reads whom the announcement proposes to tag: the owners of the
// branch's changes against its base, and what the store kept about them, for
// the preview opened as opened.
func (m Model) readTags(opened int) tea.Cmd {
	owners := loop.OwnerSeams{
		ChangedPaths: m.deps.Git.ChangedPaths, CodeOwnersAt: m.deps.Git.CodeOwnersAt, Author: m.deps.Forge.Author,
	}
	store, base := m.deps.Store, m.branch.branch.BaseName()

	return func() tea.Msg {
		found, ownersErr := loop.OwnersOf(owners, base)
		links, linksErr := readKept(store.OwnerLinks)
		groups, groupsErr := readKept(store.RepoGroups)

		var proposal tagProposal
		if store.LastGroups != nil {
			proposal.last, proposal.lastChosen = store.LastGroups()
		}

		proposal.owners, proposal.links, proposal.repoGroups = found, links, groups

		return tagsRead{opened: opened, proposal: proposal, err: errors.Join(ownersErr, linksErr, groupsErr)}
	}
}

// readKept reads a list the store keeps, or nothing when there is no store,
// as under a dry run.
func readKept[T any](read func() ([]T, error)) ([]T, error) {
	if read == nil {
		return nil, nil
	}

	return read()
}

// tagsRead is whom the announcement proposes to tag, and why some of it could
// not be read.
type tagsRead struct {
	opened   int
	proposal tagProposal
	err      error
}

// apply fills the preview's tag section, when the preview is still open.
func (msg tagsRead) apply(m Model) (Model, tea.Cmd) {
	preview, open := beneath[messagingPreview](m, msg.opened)
	if !open {
		return m, nil
	}

	preview.tagging = preview.tagging.proposed(msg.proposal, msg.err)

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

	return s
}

// missingScope is the scope a directory read said the token lacks: tagging
// then waits until it is granted, and the post goes out untagged.
func (s tagSection) missingScope() string {
	return cmp.Or(s.members.missingScope(), s.groups.missingScope())
}

// interactive reports a section whose rows can be moved through and changed.
func (s tagSection) interactive() bool {
	return s.offered && !s.reading && s.missingScope() == ""
}

// lines draws the section below the destination.
func (s tagSection) lines(marks glyphs, sty styles, width int) []string {
	switch {
	case !s.offered:
		return nil
	case s.missingScope() != "":
		return []string{failedGlyph(sty, marks) + " tagging needs the " + s.missingScope() + " scope; this posts untagged"}
	case s.reading:
		return []string{marks.inFlight + " reading whom to tag" + marks.ellipsis}
	}

	lines := slices.Concat(s.ownerLines(marks), s.groupLines(marks))

	for _, err := range []error{s.readErr, s.members.err, s.groups.err, s.linkErr} {
		if err != nil && !errors.Is(err, messaging.ErrNoUserGroups) {
			lines = append(lines, failureBlock(sty, marks, err, width))
		}
	}

	return append(lines, "tags  "+s.summary())
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
			marks.checkbox(slices.Contains(s.checked, group.Slack.ID)) + slackName(group.Slack, true)
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
	if !s.offered {
		return ""
	}

	return ", tagging " + s.summary()
}

// postTags is whom a post tags, and whether it offered groups to choose, which
// makes the choice — even of none — one to remember.
type postTags struct {
	mentions     messaging.Mentions
	offersGroups bool
}

// postTags is whom the post tags: no one but for a ready-for-review
// announcement, which alone offers tags. A token lacking a scope tags no one,
// and so does a tag Slack could not read: tagging never holds a post back.
func (s tagSection) postTags() postTags {
	if !s.offered || s.missingScope() != "" {
		return postTags{}
	}

	mentions, err := s.tags.Mentions(s.checked)
	if err != nil {
		return postTags{}
	}

	return postTags{mentions: mentions, offersGroups: len(s.tags.Groups) > 0}
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

	return s
}

// handleTagKey answers the tag section's keys in the preview.
func (p messagingPreview) handleTagKey(m Model, msg tea.KeyPressMsg) (Model, tea.Cmd) {
	if !p.tagging.interactive() {
		return m, nil
	}

	switch {
	case key.Matches(msg, m.keys.down):
		p.tagging = p.tagging.moved(1)
	case key.Matches(msg, m.keys.up):
		p.tagging = p.tagging.moved(-1)
	case key.Matches(msg, m.keys.toggleOption):
		p.tagging = p.tagging.toggled()
	case key.Matches(msg, m.keys.linkToSlack):
		return p.pickLink(m)
	case key.Matches(msg, m.keys.notOnSlack):
		return p.markNotOnSlack(m)
	}

	m.overlay = p

	return m, nil
}

// pickLink opens the picker on the owner under the cursor: the channel's
// members for a person, the user groups for a team.
func (p messagingPreview) pickLink(m Model) (Model, tea.Cmd) {
	owner, onOwner := p.tagging.selectedOwner()
	if !onOwner || !p.tagging.canLink {
		return m, nil
	}

	m.overlay = newOwnerPicker(m, owner.Owner, owner.Team, p)

	return m, nil
}

// openedAs is the count of overlays opened when the preview opened.
func (p messagingPreview) openedAs() int {
	return p.opened
}

// directoryFor is the directory the owner picker chooses from: the channel's
// members for a person, the user groups for a team.
func (p messagingPreview) directoryFor(team bool) directory {
	if team {
		return p.tagging.groups
	}

	return p.tagging.members
}

// markNotOnSlack saves that the owner under the cursor is not on Slack.
func (p messagingPreview) markNotOnSlack(m Model) (Model, tea.Cmd) {
	owner, onOwner := p.tagging.selectedOwner()
	if !onOwner || !p.tagging.canLink {
		return m, nil
	}

	return m, m.saveLink(owner.Owner, nil, p.opened)
}
