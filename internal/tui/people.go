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
// was decided, or one of this branch's owners not asked about yet; and
// whether they are a team (org/team, a GitLab group), which links to a user
// group rather than a person.
type person struct {
	owner   string
	team    bool
	decided bool
	link    loop.OwnerLink
}

// peopleOverlay manages what the announcements tag: whom each forge owner is
// on Slack, and which user groups this repository tags. Its writes are saved
// at once, and a refusal stays pinned under its title until the next one.
type peopleOverlay struct {
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
		reading: true, repoReading: true, channel: m.defaultChannel(),
		opened: opened,
	}
	people.members, readMembers = readChannelMembers(m.deps, people.channel, opened)
	people.groups, readGroups = readUserGroups(m.deps, opened)
	m.overlay = people

	return m, tea.Batch(m.branch.readPeople(m.deps, opened), readRepoGroups(m.deps, opened), readMembers, readGroups)
}

// readPeople reads who was decided on this forge host, and the owners of this
// branch's changes, who may not have been asked about yet, for the overlay
// opened as opened.
func (s branchState) readPeople(deps Deps, opened int) tea.Cmd {
	owners := taggedOwnerSeams(deps)
	links, base, readWorkspace := deps.Store.OwnerLinks, s.branch.BaseName(), deps.Messaging.Workspace

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
// A bare name decided as a person that the forge now knows as a group is
// listed as a team not asked about yet, so it can be linked to a user group.
func peopleFrom(decided []loop.OwnerLink, owners codeowners.Owners) []person {
	ownerTeam := func(owner string) bool {
		return slices.ContainsFunc(owners.Teams, func(team string) bool { return loop.SameOwner(team, owner) })
	}

	people := make([]person, 0, len(decided))
	for _, link := range decided {
		if link.Team || !ownerTeam(link.Owner) {
			people = append(people, person{owner: link.Owner, team: link.Team, decided: true, link: link})

			continue
		}

		asTeam := loop.OwnerLink{Owner: link.Owner, Team: true, OnSlack: false, Slack: loop.SlackTarget{}}
		people = append(people, person{owner: link.Owner, team: true, decided: false, link: asTeam})
	}

	for _, owner := range slices.Concat(owners.Users, owners.Teams) {
		if !slices.ContainsFunc(decided, func(link loop.OwnerLink) bool { return loop.SameOwner(link.Owner, owner) }) {
			team := ownerTeam(owner)
			link := loop.OwnerLink{Owner: owner, Team: team, OnSlack: false, Slack: loop.SlackTarget{}}
			people = append(people, person{owner: owner, team: team, decided: false, link: link})
		}
	}

	return people
}

// peopleListed is everyone People lists, and why some could not be read.
type peopleListed struct {
	opened int
	people []person
	err    error
}

var _ applier = peopleListed{}

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

// view draws the refusal pinned under the title, the tabs, and the tab shown.
func (p peopleOverlay) view(kit renderKit, width, rows int) (string, string) {
	lines := kit.pinnedOutcome(p.send, "saving", width)
	lines = append(lines, p.tabs(kit), "")

	body := map[peopleTab]func(renderKit, int, int) []string{tabPeople: p.peopleLines, tabGroups: p.groupLines}[p.tab]

	return peopleTitle, strings.Join(append(lines, body(kit, width, rows-len(lines))...), "\n")
}

// tabs names both tabs, the one shown marked.
func (p peopleOverlay) tabs(kit renderKit) string {
	names := map[peopleTab]string{tabPeople: "People", tabGroups: "Groups"}
	shown := names[p.tab]
	names[p.tab] = kit.marks.chosenOpen + shown + kit.marks.chosenClose

	return names[tabPeople] + "  " + names[tabGroups]
}

// peopleLines is the People tab.
func (p peopleOverlay) peopleLines(kit renderKit, width, rows int) []string {
	switch {
	case p.reading:
		return []string{kit.marks.inFlight + " reading who was decided" + kit.marks.ellipsis}
	case p.readErr != nil:
		return []string{kit.failureBlock(p.readErr, width)}
	case len(p.people.items) == 0:
		return []string{"nobody decided yet, and no code owner of this branch's changes to ask about"}
	default:
		return p.people.rows(kit.marks, rows, func(row person) string { return personRow(kit, row) })
	}
}

// personRow is an owner and what is known of them on Slack.
func personRow(kit renderKit, row person) string {
	state := "? not asked yet"

	switch {
	case row.decided && row.link.OnSlack:
		state = strings.TrimSpace(kit.marks.arrow) + " " + slackName(row.link.Slack, row.team)
	case row.decided:
		state = kit.marks.unknown + " not on Slack"
	}

	return fmt.Sprintf("%-*s ", ownerColumn, row.owner) + state
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
	if p.send.sending {
		return m, nil
	}

	if listed, answered := m.listKey(p, msg); answered {
		return listed, nil
	}

	switch {
	case key.Matches(msg, m.keys.nextField):
		p.tab = 1 - p.tab
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
		m.overlay = newOwnerPicker(m, selected.owner, selected.team, p)

		return m, nil
	case key.Matches(msg, m.keys.notOnSlack):
		return p.saving(m, saveLink(m.deps, decided(selected.owner, selected.team, nil), p.opened))
	case key.Matches(msg, m.keys.forgetOwner):
		return p.askToForget(m, selected.owner), nil
	default:
		return m, nil
	}
}

// askToForget holds forgetting owner for a last look, over People and groups,
// which esc goes back to: what is forgotten was decided once and is asked again
// rather than seen again.
func (p peopleOverlay) askToForget(m Model, owner string) Model {
	forget, opened, readWorkspace := m.deps.Store.ForgetOwner, p.opened, m.deps.Messaging.Workspace

	m.overlay = lastLook{
		title: "Forget a person", verb: "forget", leave: escBack, back: p,
		body: "Forget " + owner + "?\n\nThe next ready-for-review announcement asks whom they are on Slack again.",
		proceed: func(m Model) (Model, tea.Cmd) {
			return p.saving(m, func() tea.Msg {
				err := inWorkspace(readWorkspace, func(workspace string) error { return forget(workspace, owner) })

				return peopleSaved{opened: opened, err: err}
			})
		},
	}

	return m
}

// saving marks a write in flight and sends it.
func (p peopleOverlay) saving(m Model, write tea.Cmd) (Model, tea.Cmd) {
	p.send = starting()
	m.overlay = p

	return m, write
}

// peopleSaved is a write from People and groups done, or why it was refused.
type peopleSaved struct {
	opened int
	err    error
}

var _ applier = peopleSaved{}

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

	return m.withBeneath(open), m.branch.readPeople(m.deps, open.opened)
}

// openedAs is the count of overlays opened when People and groups opened.
func (p peopleOverlay) openedAs() int {
	return p.opened
}

// failed is the overlay kept open with the reason a write was refused.
func (p peopleOverlay) failed(err error) peopleOverlay {
	p.send = p.send.failed(err)

	return p
}
