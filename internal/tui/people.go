// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/jacob-delgado/workflow/internal/codeowners"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/loop"
	"github.com/jacob-delgado/workflow/internal/sanitize"
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
	// channel is the default channel, whose members a person is linked to.
	channel string
	members directory
	groups  directory
	// chosen is the repository's groups as they are being edited, and
	// choices every group that can be chosen: Slack's, then any of the
	// repository's Slack no longer lists.
	chosen  []loop.SlackTarget
	repoErr error
	choices pickList[loop.SlackTarget]
	send    sendState
}

var (
	_ failable[peopleOverlay] = peopleOverlay{}
	_ steppable               = peopleOverlay{}
)

// managesPeople reports a store that keeps the associations and a Slack user
// token that can read the directory they are made from. The directory seams
// are bound whatever the settings, so the mode says whether there is a token.
func (m Model) managesPeople() bool {
	return m.deps.Store.OwnerLinks != nil && m.deps.Messaging.ChannelMembers != nil &&
		m.cfg.Messaging.Mode() == config.MessagingUser
}

// openPeople opens People and groups and starts reading everything it shows.
func (m Model) openPeople() (Model, tea.Cmd) {
	var readMembers, readGroups tea.Cmd

	people := peopleOverlay{marks: m.marks, styles: m.styles, reading: true, channel: m.cfg.Messaging.Channel}
	people.members, readMembers = m.readMembers(people.channel)
	people.groups, readGroups = m.readUserGroups()
	m.overlay = people

	return m, tea.Batch(m.readPeople(), m.readRepoGroups(), readMembers, readGroups)
}

// readPeople reads who was decided on this forge host, and the owners of this
// branch's changes, who may not have been asked about yet.
func (m Model) readPeople() tea.Cmd {
	owners := loop.OwnerSeams{
		ChangedPaths: m.deps.Git.ChangedPaths, CodeOwnersAt: m.deps.Git.CodeOwnersAt, Author: m.deps.Forge.Author,
	}
	links, base := m.deps.Store.OwnerLinks, m.branch.branch.BaseName()

	return func() tea.Msg {
		decided, linksErr := links()
		found, ownersErr := loop.OwnersOf(owners, base)

		return peopleListed{people: peopleFrom(decided, found), err: errors.Join(linksErr, ownersErr)}
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

// readRepoGroups reads the user groups this repository tags.
func (m Model) readRepoGroups() tea.Cmd {
	read := m.deps.Store.RepoGroups

	return func() tea.Msg {
		groups, err := read()

		return repoGroupsListed{groups: groups, err: err}
	}
}

// peopleListed is everyone People lists, and why some could not be read.
type peopleListed struct {
	people []person
	err    error
}

// apply lists them, the selection held where it was.
func (msg peopleListed) apply(m Model) (Model, tea.Cmd) {
	if open, isOpen := m.overlay.(peopleOverlay); isOpen {
		open.people = pickList[person]{items: msg.people, selected: open.people.selected}.moved(0)
		open.reading, open.readErr = false, msg.err
		m.overlay = open
	}

	return m, nil
}

// repoGroupsListed is the user groups the repository tags.
type repoGroupsListed struct {
	groups []loop.SlackTarget
	err    error
}

// apply checks them in the groups checklist.
func (msg repoGroupsListed) apply(m Model) (Model, tea.Cmd) {
	if open, isOpen := m.overlay.(peopleOverlay); isOpen {
		open.chosen, open.repoErr = msg.groups, msg.err
		m.overlay = open.withChoices()
	}

	return m, nil
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

	return append(lines, p.choices.rows(p.marks, rows-len(lines), p.choiceRow)...)
}

// choiceRow is a group, checked when the repository tags it.
func (p peopleOverlay) choiceRow(group loop.SlackTarget) string {
	return p.marks.checkbox(p.isChosen(group.ID)) + slackName(group, true)
}

// isChosen reports a group the repository tags, as edited so far.
func (p peopleOverlay) isChosen(id string) bool {
	return slices.ContainsFunc(p.chosen, func(group loop.SlackTarget) bool { return group.ID == id })
}

// footer offers the shown tab's keys, and nothing while a write is out.
func (p peopleOverlay) footer(keys keyMap) []key.Binding {
	if p.send.sending {
		return []key.Binding{keys.interrupt}
	}

	if p.tab == tabGroups {
		return []key.Binding{
			relabel(keys.toggleOption, "select"), relabel(keys.confirm, "save"),
			relabel(keys.refresh, "refresh directory"), relabel(keys.nextField, "people"), keys.closeOverlay,
		}
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
		from := map[bool]directory{false: p.members, true: p.groups}[selected.team()]
		m.overlay = newOwnerPicker(m, selected.owner, selected.team(), from, p)

		return m, nil
	case key.Matches(msg, m.keys.notOnSlack):
		return p.saving(m, m.saveLink(selected.owner, nil))
	case key.Matches(msg, m.keys.forgetOwner):
		forget, owner := m.deps.Store.ForgetOwner, selected.owner

		return p.saving(m, func() tea.Msg { return peopleSaved{err: forget(owner)} })
	default:
		return m, nil
	}
}

// handleGroupKey checks a group, saves the checklist, or reads the directory
// again.
func (p peopleOverlay) handleGroupKey(m Model, msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.toggleOption):
		m.overlay = p.toggled()

		return m, nil
	case key.Matches(msg, m.keys.confirm):
		save := m.deps.Store.SetRepoGroups
		groups := slices.DeleteFunc(slices.Clone(p.choices.items), func(group loop.SlackTarget) bool {
			return !p.isChosen(group.ID)
		})

		return p.saving(m, func() tea.Msg {
			return peopleSaved{saved: "saved the groups this repository tags", err: save(groups)}
		})
	case key.Matches(msg, m.keys.refresh):
		return p.refreshed(m)
	default:
		return m, nil
	}
}

// toggled is the checklist with the group under the cursor checked, or
// unchecked when it was.
func (p peopleOverlay) toggled() peopleOverlay {
	group, ok := p.choices.chosen()
	if !ok {
		return p
	}

	if p.isChosen(group.ID) {
		p.chosen = slices.DeleteFunc(slices.Clone(p.chosen), func(old loop.SlackTarget) bool { return old.ID == group.ID })
	} else {
		p.chosen = append(slices.Clone(p.chosen), group)
	}

	return p
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
	channel := p.channel
	p.groups.reading, p.members.reading = true, true
	m.overlay = p

	return m, func() tea.Msg {
		// The directory reads are bound together, for a Slack user token, so
		// with one there are all three.
		refresh()

		var again directoryRefreshed

		again.groups.entries, again.groups.err = readGroups()
		again.members.entries, again.members.err = readMembers(channel)

		return again
	}
}

// directoryRefreshed is the directory read again after a refresh.
type directoryRefreshed struct {
	groups  directory
	members directory
}

// apply shows what was read.
func (msg directoryRefreshed) apply(m Model) (Model, tea.Cmd) {
	if open, isOpen := m.overlay.(peopleOverlay); isOpen {
		open.groups, open.members = msg.groups, msg.members
		m.overlay = open.withChoices()
	}

	return m, nil
}

// peopleSaved is a write from People and groups done, or why it was refused.
type peopleSaved struct {
	saved string
	err   error
}

// apply pins a refusal, or reads the people again to show what changed.
func (msg peopleSaved) apply(m Model) (Model, tea.Cmd) {
	open, isOpen := m.overlay.(peopleOverlay)
	if !isOpen {
		return m, nil
	}

	if msg.err != nil {
		return keepOpenWith[peopleOverlay](m, msg.err), nil
	}

	open.send = sendState{}
	m.overlay = open

	if msg.saved != "" {
		m = m.noticed(m.marks.done + " " + msg.saved)
	}

	return m, m.readPeople()
}

// failed is the overlay kept open with the reason a write was refused.
func (p peopleOverlay) failed(err error) peopleOverlay {
	p.send = p.send.failed(err)

	return p
}

// saveLink saves whom owner is on Slack, or with no target that they are not
// on it.
func (m Model) saveLink(owner string, target *loop.SlackTarget) tea.Cmd {
	save, link := m.deps.Store.LinkOwner, loop.OwnerLink{Owner: owner, OnSlack: false, Slack: loop.SlackTarget{}}
	if target != nil {
		link.OnSlack, link.Slack = true, *target
	}

	return func() tea.Msg {
		return ownerLinked{link: link, err: save(owner, target)}
	}
}

// ownerLinked is a link saved, or why it was not.
type ownerLinked struct {
	link loop.OwnerLink
	err  error
}

// apply shows the link where it was made: in the preview's tags, or in
// People and groups.
func (msg ownerLinked) apply(m Model) (Model, tea.Cmd) {
	switch open := m.overlay.(type) {
	case messagingPreview:
		open.tagging = open.tagging.relinked(msg.link, msg.err)
		m.overlay = open
	case peopleOverlay:
		return peopleSaved{saved: "", err: msg.err}.apply(m)
	}

	return m, nil
}

// slackName is how a Slack user or group is shown: a group with its @.
func slackName(target loop.SlackTarget, group bool) string {
	if group {
		return "@" + sanitize.Line(target.Label)
	}

	return sanitize.Line(target.Label)
}

// linkChoice is a row of the owner picker: someone on Slack, or "not on
// Slack".
type linkChoice struct {
	target     loop.SlackTarget
	notOnSlack bool
}

// ownerPicker chooses whom an owner is on Slack, from a directory narrowed by
// what is typed, and goes back to the overlay it was opened from.
type ownerPicker struct {
	marks  glyphs
	styles styles
	owner  string
	team   bool
	from   directory
	filter string
	list   pickList[linkChoice]
	back   overlay
}

var _ overlay = ownerPicker{}

// newOwnerPicker opens the picker on owner, choosing from from.
func newOwnerPicker(m Model, owner string, team bool, from directory, back overlay) ownerPicker {
	picker := ownerPicker{marks: m.marks, styles: m.styles, owner: owner, team: team, from: from, back: back}

	return picker.filtered("")
}

// filtered is the picker narrowed to the entries whose name holds filter,
// with "not on Slack" always last.
func (p ownerPicker) filtered(filter string) ownerPicker {
	needle := strings.ToLower(filter)
	choices := make([]linkChoice, 0, len(p.from.entries)+1)

	for _, target := range p.from.entries {
		if strings.Contains(strings.ToLower(target.Label), needle) {
			choices = append(choices, linkChoice{target: target, notOnSlack: false})
		}
	}

	p.filter = filter
	p.list = pickList[linkChoice]{items: append(choices, linkChoice{target: loop.SlackTarget{}, notOnSlack: true})}

	return p
}

// view draws the filter, then the choices in as many rows as fit, then how
// the directory read went.
func (p ownerPicker) view(width, rows int) (string, string) {
	lines := []string{"filter  " + sanitize.Line(p.filter), ""}
	lines = append(lines, p.list.rows(p.marks, rows-len(lines)-outcomeRows, p.choiceRow)...)

	switch {
	case p.from.reading:
		lines = append(lines, "", p.marks.inFlight+" still reading Slack's directory"+p.marks.ellipsis)
	case p.from.err != nil:
		lines = append(lines, "", failureBlock(p.styles, p.marks, p.from.err, width))
	}

	return "Link " + p.owner + " to Slack", strings.Join(lines, "\n")
}

// choiceRow names a choice.
func (p ownerPicker) choiceRow(choice linkChoice) string {
	if choice.notOnSlack {
		return "Not on Slack"
	}

	return slackName(choice.target, p.team)
}

// footer offers choosing and going back; every other key types the filter.
func (p ownerPicker) footer(_ keyMap) []key.Binding {
	return []key.Binding{
		key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "link")),
		key.NewBinding(key.WithKeys("up", "down"), key.WithHelp(p.marks.upKey+"/"+p.marks.downKey, "select")),
		key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back")),
	}
}

// handleKey types the filter, moves the choice, links the chosen one or goes
// back. Like the Issues filter, it reads enter and esc themselves, so a
// printable key ui.keys moved onto either still types.
func (p ownerPicker) handleKey(m Model, msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch msg.Code {
	case tea.KeyEscape:
		m.overlay = p.back

		return m, nil
	case tea.KeyEnter:
		return p.choose(m)
	case tea.KeyDown:
		p.list = p.list.moved(1)
	case tea.KeyUp:
		p.list = p.list.moved(-1)
	case tea.KeyBackspace:
		_, size := utf8.DecodeLastRuneInString(p.filter)
		p = p.filtered(p.filter[:len(p.filter)-size])
	default:
		p = p.filtered(p.filter + typedText(msg))
	}

	m.overlay = p

	return m, nil
}

// choose saves the chosen link and goes back.
func (p ownerPicker) choose(m Model) (Model, tea.Cmd) {
	// "Not on Slack" is always a row, so one is always chosen.
	chosen, _ := p.list.chosen()
	m.overlay = p.back

	if chosen.notOnSlack {
		return m, m.saveLink(p.owner, nil)
	}

	return m, m.saveLink(p.owner, &chosen.target)
}
