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

// readMembers starts reading a channel's members, for linking a user owner.
func (m Model) readMembers(channel string) (directory, tea.Cmd) {
	read := m.deps.Messaging.ChannelMembers

	return directory{reading: true}, func() tea.Msg {
		found, err := read(channel)

		return membersRead{channel: channel, found: directory{entries: found, err: err}}
	}
}

// readUserGroups starts reading the workspace's user groups, for linking a
// team owner, where Slack can list them.
func (m Model) readUserGroups() (directory, tea.Cmd) {
	read := m.deps.Messaging.UserGroups
	if read == nil {
		return directory{}, nil
	}

	return directory{reading: true}, func() tea.Msg {
		found, err := read()

		return userGroupsRead{found: directory{entries: found, err: err}}
	}
}

// membersRead is a channel's members, read for linking.
type membersRead struct {
	channel string
	found   directory
}

// apply hands the members to the preview, unless its channel has changed
// since they were asked for, or to People and groups.
func (msg membersRead) apply(m Model) (Model, tea.Cmd) {
	switch open := m.overlay.(type) {
	case messagingPreview:
		if open.channel == msg.channel {
			open.tagging.members = msg.found
			m.overlay = open
		}
	case peopleOverlay:
		open.members = msg.found
		m.overlay = open
	}

	return m, nil
}

// userGroupsRead is the workspace's user groups, read for linking a team.
type userGroupsRead struct {
	found directory
}

// apply hands the groups to the preview, or to People and groups.
func (msg userGroupsRead) apply(m Model) (Model, tea.Cmd) {
	switch open := m.overlay.(type) {
	case messagingPreview:
		open.tagging.groups = msg.found
		m.overlay = open
	case peopleOverlay:
		open.groups = msg.found
		m.overlay = open.withChoices()
	}

	return m, nil
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
