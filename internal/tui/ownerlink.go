// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"errors"
	"strings"
	"unicode/utf8"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/jacob-delgado/workflow/internal/loop"
	"github.com/jacob-delgado/workflow/internal/messaging"
	"github.com/jacob-delgado/workflow/internal/sanitize"
)

// directory is a read of Slack's directory — a channel's members, or the
// workspace's user groups — still on its way, or what it found, or why not.
type directory struct {
	entries []loop.SlackTarget
	err     error
	reading bool
}

// missingScope is the scope the read said the token lacks, or "" when it
// lacked none.
func (d directory) missingScope() string {
	if missing, found := errors.AsType[*messaging.MissingScopeError](d.err); found {
		return missing.Needed
	}

	return ""
}

// readChannelMembers starts reading a channel's members, for linking a user owner
// in the overlay opened as opened.
func readChannelMembers(deps Deps, channel string, opened int) (directory, tea.Cmd) {
	read := deps.Messaging.ChannelMembers

	return directory{reading: true}, func() tea.Msg {
		found, err := read(channel)

		return membersRead{opened: opened, channel: channel, found: directory{entries: found, err: err}}
	}
}

// readUserGroups starts reading the workspace's user groups, for linking a
// team owner, where Slack can list them.
func readUserGroups(deps Deps, opened int) (directory, tea.Cmd) {
	read := deps.Messaging.UserGroups
	if read == nil {
		return directory{}, nil
	}

	return directory{reading: true}, func() tea.Msg {
		found, err := read()

		return userGroupsRead{opened: opened, found: directory{entries: found, err: err}}
	}
}

// membersRead is a channel's members, read for linking.
type membersRead struct {
	opened  int
	channel string
	found   directory
}

var _ applier = membersRead{}

// apply hands the members to the overlay that asked for them: the preview,
// unless its channel has changed since, or People and groups, whose channel
// never does.
func (msg membersRead) apply(m Model) (Model, tea.Cmd) {
	if preview, open := beneath[messagingPreview](m, msg.opened); open && preview.channel == msg.channel {
		preview.tagging.members = msg.found

		return m.withBeneath(preview), nil
	}

	if people, open := beneath[peopleOverlay](m, msg.opened); open {
		people.members = msg.found

		return m.withBeneath(people), nil
	}

	return m, nil
}

// userGroupsRead is the workspace's user groups, read for linking a team.
type userGroupsRead struct {
	opened int
	found  directory
}

var _ applier = userGroupsRead{}

// apply hands the groups to the preview, or to People and groups.
func (msg userGroupsRead) apply(m Model) (Model, tea.Cmd) {
	if preview, open := beneath[messagingPreview](m, msg.opened); open {
		preview.tagging.groups = msg.found
		preview.tagging = preview.tagging.withGroupsReadable()

		return m.withBeneath(preview), nil
	}

	if people, open := beneath[peopleOverlay](m, msg.opened); open {
		people.groups = msg.found

		return m.withBeneath(people.withChoices()), nil
	}

	return m, nil
}

// linksOwners is an overlay the owner picker opens over, holding the
// directory the picker chooses from.
type linksOwners interface {
	overlay
	// openedAs is the count of overlays opened when it opened, which every
	// read and save it starts carries back.
	openedAs() int
	directoryFor(team bool) directory
}

// beneath is the open overlay a read or a save is for, when it is the T
// opened as opened: the one on top, or the one the owner picker was opened
// over, which a read must still reach, or esc would return to it as it was
// before the read. An answer meant for an overlay since closed finds none,
// even when another of its kind has opened since.
func beneath[T linksOwners](m Model, opened int) (T, bool) {
	target := m.overlay
	if picker, picking := m.overlay.(ownerPicker); picking {
		target = picker.back
	}

	open, isOpen := target.(T)
	if !isOpen || open.openedAs() != opened {
		var none T

		return none, false
	}

	return open, true
}

// withBeneath puts back the overlay beneath found, changed: under the owner
// picker when it is open, whose choices it brings up to date.
func (m Model) withBeneath(changed linksOwners) Model {
	if picker, picking := m.overlay.(ownerPicker); picking {
		m.overlay = picker.over(changed)

		return m
	}

	m.overlay = changed

	return m
}

// saveLink saves link, whom an owner is on Slack or that they are not on it,
// for the overlay opened as opened.
func saveLink(deps Deps, link loop.OwnerLink, opened int) tea.Cmd {
	save, readWorkspace := deps.Store.LinkOwner, deps.Messaging.Workspace

	return func() tea.Msg {
		err := inWorkspace(readWorkspace, func(workspace string) error { return save(workspace, link) })

		return ownerLinked{opened: opened, link: link, err: err}
	}
}

// decided is what was decided for owner, a team or a person: whom they are
// on Slack, or with no target that they are not on it.
func decided(owner string, team bool, target *loop.SlackTarget) loop.OwnerLink {
	link := loop.OwnerLink{Owner: owner, Team: team, OnSlack: false, Slack: loop.SlackTarget{}}
	if target != nil {
		link.OnSlack, link.Slack = true, *target
	}

	return link
}

// inWorkspace makes write in the Slack workspace the token is for, which every
// kept link is made under, or says why that cannot be read.
func inWorkspace(readWorkspace func() (string, error), write func(workspace string) error) error {
	workspace, err := loop.TagWorkspace(readWorkspace)
	if err != nil {
		return err
	}

	return write(workspace)
}

// ownerLinked is a link saved, or why it was not.
type ownerLinked struct {
	opened int
	link   loop.OwnerLink
	err    error
}

var _ applier = ownerLinked{}

// apply shows the link where it was made: in the preview's tags, or in
// People and groups.
func (msg ownerLinked) apply(m Model) (Model, tea.Cmd) {
	if preview, open := beneath[messagingPreview](m, msg.opened); open {
		preview.tagging = preview.tagging.relinked(msg.link, msg.err)

		return m.withBeneath(preview), nil
	}

	return peopleSaved{opened: msg.opened, err: msg.err}.apply(m)
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
	// arrows names the arrow keys that move the choice, in the interface's
	// glyphs.
	arrows string
	owner  string
	team   bool
	from   directory
	filter string
	list   pickList[linkChoice]
	back   linksOwners
}

var (
	_ overlay   = ownerPicker{}
	_ pasteable = ownerPicker{}
)

// newOwnerPicker opens the picker on owner, over the overlay it goes back to.
func newOwnerPicker(m Model, owner string, team bool, back linksOwners) ownerPicker {
	picker := ownerPicker{
		arrows: m.marks.upKey + "/" + m.marks.downKey, owner: owner, team: team, from: directory{}, back: back,
	}

	return picker.over(back)
}

// over is the picker opened over back, choosing from back's directory as it
// now stands, the filter and the choice kept.
func (p ownerPicker) over(back linksOwners) ownerPicker {
	selected := p.list.selected
	p.back, p.from = back, back.directoryFor(p.team)
	p = p.filtered(p.filter)
	p.list = pickList[linkChoice]{items: p.list.items, selected: selected}.moved(0)

	return p
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
func (p ownerPicker) view(kit renderKit, width, rows int) (string, string) {
	lines := []string{"filter  " + sanitize.Line(p.filter), ""}
	lines = append(lines, p.list.rows(kit.marks, rows-len(lines)-outcomeRows, p.choiceRow)...)

	switch {
	case p.from.reading:
		lines = append(lines, "", kit.marks.inFlight+" still reading Slack's directory"+kit.marks.ellipsis)
	case p.from.err != nil:
		lines = append(lines, "", kit.failureBlock(p.from.err, width))
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
		key.NewBinding(key.WithKeys("up", "down"), key.WithHelp(p.arrows, "select")),
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

// which names the owner picker.
func (ownerPicker) which() overlayKind { return overlayOwnerPicker }

// pasted types a paste into the filter, its lines joined by spaces, as typing
// it would.
func (p ownerPicker) pasted(m Model, paste tea.PasteMsg) (Model, tea.Cmd) {
	m.overlay = p.filtered(p.filter + oneLine(paste.Content))

	return m, nil
}

// choose saves the chosen link and goes back.
func (p ownerPicker) choose(m Model) (Model, tea.Cmd) {
	// "Not on Slack" is always a row, so one is always chosen.
	chosen, _ := p.list.chosen()
	m.overlay = p.back

	if chosen.notOnSlack {
		return m, saveLink(m.deps, decided(p.owner, p.team, nil), p.back.openedAs())
	}

	return m, saveLink(m.deps, decided(p.owner, p.team, &chosen.target), p.back.openedAs())
}
