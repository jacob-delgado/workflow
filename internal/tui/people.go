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
)

// peopleTitle titles the People and groups overlay.
const peopleTitle = "People and groups"

// peopleTab is which half of People and groups is shown.
type peopleTab int

const (
	// tabPeople is whom each forge owner is on Slack.
	tabPeople peopleTab = iota
	// tabGroups is which user groups the repository tags.
	tabGroups
)

// person is a forge owner as People and groups lists them: decided, with what
// was decided, or one of this branch's owners not asked about yet.
type person struct {
	owner   string
	decided bool
	link    loop.OwnerLink
}

// team reports an owner that is a team (org/team, a GitLab group/subgroup),
// which links to a user group rather than a person.
func (p person) team() bool {
	return strings.Contains(p.owner, "/")
}

// peopleOverlay manages what the announcements tag: whom each forge owner is
// on Slack, and which user groups this repository tags. Its writes are saved
// at once, and a refusal stays pinned under its title until the next one.
type peopleOverlay struct {
	marks   glyphs
	styles  styles
	tab     peopleTab
	people  pickList[person]
	reading bool
	readErr error
	// channel is the channel a post goes to by default, whose members a
	// person is linked to.
	channel string
	members directory
	groups  directory
	// chosen is the repository's groups, each change saved at once, and
	// choices every group that can be chosen: Slack's, then any of the
	// repository's Slack no longer lists. Until the repository's are read,
	// and while they cannot be, nothing is changed, since a save replaces
	// the whole list.
	chosen      []loop.SlackTarget
	repoReading bool
	repoErr     error
	choices     pickList[loop.SlackTarget]
	send        sendState
	// opened is the count of overlays opened when this one opened, so a read
	// or save started for it lands in it alone.
	opened int
}

var (
	_ failable[peopleOverlay] = peopleOverlay{}
	_ steppable               = peopleOverlay{}
)

// managesPeople reports a store that keeps the associations, and can change
// them, as a dry run's cannot, and a Slack user token that can read the
// directory they are made from: the directory's seams may be bound where
// messaging posts some other way, and then only refuse.
func (m Model) managesPeople() bool {
	return m.cfg.Messaging.Mode() == config.MessagingUser && m.deps.Store.OwnerLinks != nil &&
		m.deps.Store.LinkOwner != nil && m.deps.Messaging.ChannelMembers != nil
}

// openPeople opens People and groups and starts reading everything it shows.
func (m Model) openPeople() (Model, tea.Cmd) {
	var readMembers, readGroups tea.Cmd

	m, opened := m.opening()
	people := peopleOverlay{
		marks: m.marks, styles: m.styles, reading: true, repoReading: true, channel: m.defaultChannel(),
		opened: opened,
	}
	people.members, readMembers = m.readMembers(people.channel, opened)
	people.groups, readGroups = m.readUserGroups(opened)
	m.overlay = people

	return m, tea.Batch(m.readPeople(opened), m.readRepoGroups(opened), readMembers, readGroups)
}

// readPeople reads who was decided on this forge host, and the owners of this
// branch's changes, who may not have been asked about yet, for the overlay
// opened as opened.
func (m Model) readPeople(opened int) tea.Cmd {
	owners := loop.OwnerSeams{
		ChangedPaths: m.deps.Git.ChangedPaths, CodeOwnersAt: m.deps.Git.CodeOwnersAt, Author: m.deps.Forge.Author,
	}
	links, base, readWorkspace := m.deps.Store.OwnerLinks, m.branch.branch.BaseName(), m.deps.Messaging.Workspace

	return func() tea.Msg {
		var decided []loop.OwnerLink

		linksErr := inWorkspace(readWorkspace, func(workspace string) error {
			var err error

			decided, err = links(workspace)

			return err
		})
		found, ownersErr := loop.OwnersOf(owners, base)

		return peopleListed{opened: opened, people: peopleFrom(decided, found), err: errors.Join(linksErr, ownersErr)}
	}
}

// peopleFrom is everyone decided, then each owner of the changes not decided.
func peopleFrom(decided []loop.OwnerLink, owners codeowners.Owners) []person {
	people := make([]person, 0, len(decided))
	for _, link := range decided {
		people = append(people, person{owner: link.Owner, decided: true, link: link})
	}

	for _, owner := range slices.Concat(owners.Users, owners.Teams) {
		if !slices.ContainsFunc(decided, func(link loop.OwnerLink) bool { return loop.SameOwner(link.Owner, owner) }) {
			people = append(people, person{owner: owner, decided: false, link: loop.OwnerLink{Owner: owner}})
		}
	}

	return people
}

// readRepoGroups reads the user groups this repository tags, for the overlay
// opened as opened.
func (m Model) readRepoGroups(opened int) tea.Cmd {
	read, readWorkspace := m.deps.Store.RepoGroups, m.deps.Messaging.Workspace

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

// peopleListed is everyone People lists, and why some could not be read.
type peopleListed struct {
	opened int
	people []person
	err    error
}

// apply lists them, the selection held on the owner it was on, or, where
// they are no longer listed, on the one after.
func (msg peopleListed) apply(m Model) (Model, tea.Cmd) {
	open, isOpen := beneath[peopleOverlay](m, msg.opened)
	if !isOpen {
		return m, nil
	}

	selected := open.people.selected
	if was, listed := open.people.chosen(); listed {
		if index := slices.IndexFunc(msg.people, func(row person) bool { return row.owner == was.owner }); index >= 0 {
			selected = index
		}
	}

	open.people = pickList[person]{items: msg.people, selected: selected}.moved(0)
	open.reading, open.readErr = false, msg.err

	return m.withBeneath(open), nil
}

// repoGroupsListed is the user groups the repository tags.
type repoGroupsListed struct {
	opened int
	groups []loop.SlackTarget
	err    error
}

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

// view draws the refusal pinned under the title, the tabs, and the tab shown.
func (p peopleOverlay) view(width, rows int) (string, string) {
	lines := pinnedOutcome(p.styles, p.marks, p.send, "saving", width)
	lines = append(lines, p.tabs(), "")

	body := map[peopleTab]func(int, int) []string{tabPeople: p.peopleLines, tabGroups: p.groupLines}[p.tab]

	return peopleTitle, strings.Join(append(lines, body(width, rows-len(lines))...), "\n")
}

// tabs names both tabs, the one shown marked.
func (p peopleOverlay) tabs() string {
	names := map[peopleTab]string{tabPeople: "People", tabGroups: "Groups"}
	shown := names[p.tab]
	names[p.tab] = p.marks.chosenOpen + shown + p.marks.chosenClose

	return names[tabPeople] + "  " + names[tabGroups]
}

// peopleLines is the People tab.
func (p peopleOverlay) peopleLines(width, rows int) []string {
	switch {
	case p.reading:
		return []string{p.marks.inFlight + " reading who was decided" + p.marks.ellipsis}
	case p.readErr != nil:
		return []string{failureBlock(p.styles, p.marks, p.readErr, width)}
	case len(p.people.items) == 0:
		return []string{"nobody decided yet, and no code owner of this branch's changes to ask about"}
	default:
		return p.people.rows(p.marks, rows, p.personRow)
	}
}

// personRow is an owner and what is known of them on Slack.
func (p peopleOverlay) personRow(row person) string {
	state := "? not asked yet"

	switch {
	case row.decided && row.link.OnSlack:
		state = strings.TrimSpace(p.marks.arrow) + " " + slackName(row.link.Slack, row.team())
	case row.decided:
		state = p.marks.unknown + " not on Slack"
	}

	return fmt.Sprintf("%-*s ", ownerColumn, row.owner) + state
}

// groupLines is the Groups tab.
func (p peopleOverlay) groupLines(width, rows int) []string {
	var lines []string

	for _, err := range []error{p.groups.err, p.repoErr} {
		if err != nil {
			lines = append(lines, failureBlock(p.styles, p.marks, err, width))
		}
	}

	if p.groups.reading {
		lines = append(lines, p.marks.inFlight+" reading the user groups"+p.marks.ellipsis)
	}

	if p.repoReading {
		lines = append(lines, p.marks.inFlight+" reading the groups this repository tags"+p.marks.ellipsis)
	}

	return append(lines, p.choices.rows(p.marks, rows-len(lines), p.choiceRow)...)
}

// choiceRow is a group, checked when the repository tags it.
func (p peopleOverlay) choiceRow(group loop.SlackTarget) string {
	return p.marks.checkbox(p.isChosen(group.ID)) + slackName(group, true)
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

// footer offers the shown tab's keys, and nothing while a write is out.
func (p peopleOverlay) footer(keys keyMap) []key.Binding {
	if p.send.sending {
		return []key.Binding{keys.interrupt}
	}

	if p.tab == tabGroups {
		var bindings []key.Binding
		if p.groupsEditable() {
			bindings = append(bindings, relabel(keys.toggleOption, "tag"))
		}

		return append(bindings,
			relabel(keys.refresh, "refresh directory"), relabel(keys.nextField, "people"), keys.closeOverlay)
	}

	return []key.Binding{
		relabel(keys.confirm, "change"), keys.notOnSlack, keys.forgetOwner,
		relabel(keys.nextField, "groups"), keys.closeOverlay,
	}
}

// handleKey answers the keys every tab shares, then the shown tab's own.
func (p peopleOverlay) handleKey(m Model, msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch {
	case p.send.sending:
		return m, nil
	case key.Matches(msg, m.keys.closeOverlay):
		return m.closeOverlay(), nil
	case key.Matches(msg, m.keys.nextField):
		p.tab = 1 - p.tab
	case key.Matches(msg, m.keys.down):
		return p.step(m, 1), nil
	case key.Matches(msg, m.keys.up):
		return p.step(m, -1), nil
	case p.tab == tabGroups:
		return p.handleGroupKey(m, msg)
	default:
		return p.handlePersonKey(m, msg)
	}

	m.overlay = p

	return m, nil
}

// step moves the shown tab's cursor by delta.
func (p peopleOverlay) step(m Model, delta int) Model {
	if p.tab == tabGroups {
		p.choices = p.choices.moved(delta)
	} else {
		p.people = p.people.moved(delta)
	}

	m.overlay = p

	return m
}

// handlePersonKey changes, marks or forgets the selected owner.
func (p peopleOverlay) handlePersonKey(m Model, msg tea.KeyPressMsg) (Model, tea.Cmd) {
	selected, ok := p.people.chosen()

	switch {
	case !ok:
		return m, nil
	case key.Matches(msg, m.keys.confirm):
		m.overlay = newOwnerPicker(m, selected.owner, selected.team(), p)

		return m, nil
	case key.Matches(msg, m.keys.notOnSlack):
		return p.saving(m, m.saveLink(selected.owner, nil, p.opened))
	case key.Matches(msg, m.keys.forgetOwner):
		forget, owner, opened, readWorkspace := m.deps.Store.ForgetOwner, selected.owner, p.opened,
			m.deps.Messaging.Workspace

		return p.saving(m, func() tea.Msg {
			err := inWorkspace(readWorkspace, func(workspace string) error { return forget(workspace, owner) })

			return peopleSaved{opened: opened, err: err}
		})
	default:
		return m, nil
	}
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

		return m.withBeneath(open), m.readRepoGroups(open.opened)
	}

	open.send = sendState{}

	return m.withBeneath(open).noticed(m.marks.done + " saved the groups this repository tags"), nil
}

// saving marks a write in flight and sends it.
func (p peopleOverlay) saving(m Model, write tea.Cmd) (Model, tea.Cmd) {
	p.send = starting()
	m.overlay = p

	return m, write
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

// apply shows what was read.
func (msg directoryRefreshed) apply(m Model) (Model, tea.Cmd) {
	open, isOpen := beneath[peopleOverlay](m, msg.opened)
	if !isOpen {
		return m, nil
	}

	open.groups, open.members = msg.groups, msg.members

	return m.withBeneath(open.withChoices()), nil
}

// peopleSaved is a write from People and groups done, or why it was refused.
type peopleSaved struct {
	opened int
	err    error
}

// apply pins a refusal, or reads the people again to show what changed.
func (msg peopleSaved) apply(m Model) (Model, tea.Cmd) {
	open, isOpen := beneath[peopleOverlay](m, msg.opened)
	if !isOpen {
		return m, nil
	}

	if msg.err != nil {
		return m.withBeneath(open.failed(msg.err)), nil
	}

	open.send = sendState{}

	return m.withBeneath(open), m.readPeople(open.opened)
}

// openedAs is the count of overlays opened when People and groups opened.
func (p peopleOverlay) openedAs() int {
	return p.opened
}

// directoryFor is the directory the owner picker chooses from: the default
// channel's members for a person, the user groups for a team.
func (p peopleOverlay) directoryFor(team bool) directory {
	if team {
		return p.groups
	}

	return p.members
}

// failed is the overlay kept open with the reason a write was refused.
func (p peopleOverlay) failed(err error) peopleOverlay {
	p.send = p.send.failed(err)

	return p
}
