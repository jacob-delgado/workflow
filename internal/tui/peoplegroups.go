// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"slices"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/jacob-delgado/workflow/internal/loop"
)

// readRepoGroups reads the user groups this repository tags, for the overlay
// opened as opened.
func readRepoGroups(deps Deps, opened int) tea.Cmd {
	read, readWorkspace := deps.Store.RepoGroups, deps.Messaging.Workspace

	return func() tea.Msg {
		var groups []loop.SlackTarget

		err := inWorkspace(readWorkspace, func(workspace string) error {
			var err error

			groups, err = read(workspace)

			return err
		})

		return repoGroupsListed{opened: opened, groups: groups, err: err}
	}
}

// repoGroupsListed is the user groups the repository tags.
type repoGroupsListed struct {
	opened int
	groups []loop.SlackTarget
	err    error
}

var _ applier = repoGroupsListed{}

// apply checks them in the groups checklist.
func (msg repoGroupsListed) apply(m Model) (Model, tea.Cmd) {
	open, isOpen := beneath[peopleOverlay](m, msg.opened)
	if !isOpen {
		return m, nil
	}

	open.chosen, open.repoErr, open.repoReading = msg.groups, msg.err, false

	return m.withBeneath(open.withChoices()), nil
}

// withChoices is the checklist rebuilt from Slack's groups and the
// repository's, the cursor held in range.
func (p peopleOverlay) withChoices() peopleOverlay {
	choices := slices.Clone(p.groups.entries)

	for _, group := range p.chosen {
		if !slices.ContainsFunc(choices, func(listed loop.SlackTarget) bool { return listed.ID == group.ID }) {
			choices = append(choices, group)
		}
	}

	p.choices = pickList[loop.SlackTarget]{items: choices, selected: p.choices.selected}.moved(0)

	return p
}

// groupLines is the Groups tab.
func (p peopleOverlay) groupLines(kit renderKit, width, rows int) []string {
	var lines []string

	for _, err := range []error{p.groups.err, p.repoErr} {
		if err != nil {
			lines = append(lines, kit.failureBlock(err, width))
		}
	}

	if p.groups.reading {
		lines = append(lines, kit.marks.inFlight+" reading the user groups"+kit.marks.ellipsis)
	}

	if p.repoReading {
		lines = append(lines, kit.marks.inFlight+" reading the groups this repository tags"+kit.marks.ellipsis)
	}

	return append(lines, p.choices.rows(kit.marks, rows-len(lines), p.choiceRow)...)
}

// choiceRow is a group, checked when the repository tags it.
func (p peopleOverlay) choiceRow(group loop.SlackTarget) string {
	return checkbox(p.isChosen(group.ID)) + slackName(group, true)
}

// groupsEditable reports a checklist whose change can be saved: the
// repository's groups are read, so a save keeps the ones not shown changing.
func (p peopleOverlay) groupsEditable() bool {
	return !p.repoReading && p.repoErr == nil
}

// isChosen reports a group the repository tags.
func (p peopleOverlay) isChosen(id string) bool {
	return slices.ContainsFunc(p.chosen, func(group loop.SlackTarget) bool { return group.ID == id })
}

// handleGroupKey checks or unchecks a group, or reads the directory again.
func (p peopleOverlay) handleGroupKey(m Model, msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.toggleOption) && p.groupsEditable():
		return p.toggled(m)
	case key.Matches(msg, m.keys.refresh):
		return p.refreshed(m)
	default:
		return m, nil
	}
}

// toggled checks the group under the cursor, or unchecks it when it was,
// and saves the repository's groups at once, in the order Slack lists them.
func (p peopleOverlay) toggled(m Model) (Model, tea.Cmd) {
	group, ok := p.choices.chosen()
	if !ok {
		return m, nil
	}

	if p.isChosen(group.ID) {
		p.chosen = slices.DeleteFunc(slices.Clone(p.chosen), func(old loop.SlackTarget) bool { return old.ID == group.ID })
	} else {
		p.chosen = append(slices.Clone(p.chosen), group)
	}

	save, opened, readWorkspace := m.deps.Store.SetRepoGroups, p.opened, m.deps.Messaging.Workspace
	groups := slices.DeleteFunc(slices.Clone(p.choices.items), func(choice loop.SlackTarget) bool {
		return !p.isChosen(choice.ID)
	})

	return p.saving(m, func() tea.Msg {
		err := inWorkspace(readWorkspace, func(workspace string) error { return save(workspace, groups) })

		return repoGroupsSaved{opened: opened, err: err}
	})
}

// repoGroupsSaved is the repository's groups saved, or why they were not.
type repoGroupsSaved struct {
	opened int
	err    error
}

var _ applier = repoGroupsSaved{}

// apply says the groups were saved, or pins the refusal and reads them
// again, so the checklist shows what is kept rather than the change refused.
func (msg repoGroupsSaved) apply(m Model) (Model, tea.Cmd) {
	open, isOpen := beneath[peopleOverlay](m, msg.opened)
	if !isOpen {
		return m, nil
	}

	if msg.err != nil {
		open = open.failed(msg.err)
		open.repoReading = true

		return m.withBeneath(open), readRepoGroups(m.deps, open.opened)
	}

	open.send = sendState{}

	return m.withBeneath(open).noticed(m.marks.done + " saved the groups this repository tags"), nil
}

// refreshed drops the directory read so far and reads it again.
func (p peopleOverlay) refreshed(m Model) (Model, tea.Cmd) {
	refresh, readGroups, readMembers := m.deps.Messaging.RefreshDirectory, m.deps.Messaging.UserGroups,
		m.deps.Messaging.ChannelMembers
	channel, opened := p.channel, p.opened
	p.groups.reading, p.members.reading = true, true
	m.overlay = p

	return m, func() tea.Msg {
		// The directory reads are bound together, for a Slack user token, so
		// with one there are all three.
		refresh()

		again := directoryRefreshed{opened: opened, groups: directory{}, members: directory{}}

		again.groups.entries, again.groups.err = readGroups()
		again.members.entries, again.members.err = readMembers(channel)

		return again
	}
}

// directoryRefreshed is the directory read again after a refresh.
type directoryRefreshed struct {
	opened  int
	groups  directory
	members directory
}

var _ applier = directoryRefreshed{}

// apply shows what was read.
func (msg directoryRefreshed) apply(m Model) (Model, tea.Cmd) {
	open, isOpen := beneath[peopleOverlay](m, msg.opened)
	if !isOpen {
		return m, nil
	}

	open.groups, open.members = msg.groups, msg.members

	return m.withBeneath(open.withChoices()), nil
}

// directoryFor is the directory the owner picker chooses from: the default
// channel's members for a person, the user groups for a team.
func (p peopleOverlay) directoryFor(team bool) directory {
	if team {
		return p.groups
	}

	return p.members
}
