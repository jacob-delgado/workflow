// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"context"
	"slices"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/jacob-delgado/workflow/internal/loop"
	"github.com/jacob-delgado/workflow/internal/tui/latest"
	"github.com/jacob-delgado/workflow/internal/tui/send"
)

// readRepoGroups reads the user groups this repository tags, in the Slack
// workspace read under ctx, for the overlay opened as opened.
func readRepoGroups(ctx context.Context, deps Deps, opened int) tea.Cmd {
	read, readWorkspace := deps.Store.RepoGroups, deps.Messaging.Workspace

	return func() tea.Msg {
		var groups []loop.SlackTarget

		err := inWorkspace(ctx, readWorkspace, func(workspace string) error {
			var err error

			groups, err = read(ctx, workspace)

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
	open, isOpen := answerFor[peopleOverlay](m, msg.opened)
	if !isOpen {
		return m, nil
	}

	open.chosen, open.repoErr, open.repoReading = msg.groups, msg.err, false

	return m.withAnswered(open.withChoices()), nil
}

// withChoices is the checklist rebuilt from Slack's groups and the
// repository's, the cursor held in range.
func (p peopleOverlay) withChoices() peopleOverlay {
	choices := slices.Clone(p.groups.entries())

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

	for _, err := range []error{p.groups.err(), p.repoErr} {
		if err != nil {
			lines = append(lines, kit.failureBlock(err, width))
		}
	}

	if p.groups.reading() {
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

	ctx, save, opened, readWorkspace := m.session, m.deps.Store.SetRepoGroups, p.opened, m.deps.Messaging.Workspace
	groups := slices.DeleteFunc(slices.Clone(p.choices.items), func(choice loop.SlackTarget) bool {
		return !p.isChosen(choice.ID)
	})

	return p.saving(m, func() tea.Msg {
		err := inWorkspace(ctx, readWorkspace, func(workspace string) error { return save(ctx, workspace, groups) })

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
// Late, it changes nothing.
func (msg repoGroupsSaved) apply(m Model) (Model, tea.Cmd) {
	open, _ := answerFor[peopleOverlay](m, msg.opened)
	state, answer := open.send.Answered(msg.err)
	open.send = state

	// A map, not a switch, so there is no last-case arm gobco can never see:
	// no key closes People while its write is out, so none answers late.
	return map[send.Answer]func() (Model, tea.Cmd){
		send.AnswerDone: func() (Model, tea.Cmd) {
			return m.withAnswered(open).noticed(m.marks.done + " saved the groups this repository tags"), nil
		},
		send.AnswerRefused: func() (Model, tea.Cmd) {
			open.repoReading = true

			return m.withAnswered(open), readRepoGroups(m.session, m.deps, open.opened)
		},
		send.AnswerLate: func() (Model, tea.Cmd) { return m, nil },
	}[answer]()
}

// refreshed reads the directory again, superseding every read of it still on
// its way, and shows what it read until then.
func (p peopleOverlay) refreshed(m Model) (Model, tea.Cmd) {
	refreshDirectory, readGroups, readMembers := m.deps.Messaging.RefreshDirectory, m.deps.Messaging.UserGroups,
		m.deps.Messaging.ChannelMembers
	ctx, channel, opened := m.session, p.channel, p.opened

	var groupsRound, membersRound latest.Round

	p.groups, groupsRound = p.groups.begun()
	p.members, membersRound = p.members.begun()
	m.overlay = p

	return m, func() tea.Msg {
		// The directory reads are bound together, for a Slack user token, so
		// with one there are all three.
		refreshDirectory()

		groups, groupsErr := readGroups(ctx)
		members, membersErr := readMembers(ctx, channel)

		return directoryRefreshed{
			groups:  userGroupsRead{opened: opened, round: groupsRound, found: groups, err: groupsErr},
			members: membersRead{opened: opened, round: membersRound, found: members, err: membersErr},
		}
	}
}

// directoryRefreshed is the directory read again, after Slack's was asked
// for afresh: the user groups and the channel's members.
type directoryRefreshed struct {
	groups  userGroupsRead
	members membersRead
}

var _ applier = directoryRefreshed{}

// apply shows each read, unless a later refresh has begun since, and runs
// what either asks for next.
func (msg directoryRefreshed) apply(m Model) (Model, tea.Cmd) {
	m, membersNext := msg.members.apply(m)
	m, groupsNext := msg.groups.apply(m)

	return m, tea.Batch(membersNext, groupsNext)
}

// directoryFor is the directory the owner picker chooses from: the default
// channel's members for a person, the user groups for a team.
func (p peopleOverlay) directoryFor(team bool) directory {
	if team {
		return p.groups
	}

	return p.members
}
