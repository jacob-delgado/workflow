// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/jacob-delgado/workflow/internal/loop"
	"github.com/jacob-delgado/workflow/internal/messaging"
	"github.com/jacob-delgado/workflow/internal/sanitize"
	"github.com/jacob-delgado/workflow/internal/tui/latest"
)

// directory is what the read of Slack's directory begun last — a channel's
// members, or the workspace's user groups — found, or why not, and whether
// a read of it is still on its way.
type directory struct {
	read latest.Read[[]loop.SlackTarget]
}

// entries is who the directory lists: what its latest read that succeeded
// found, kept across a failure since, or nobody.
func (d directory) entries() []loop.SlackTarget {
	entries, _ := d.read.Value()

	return entries
}

// err is why the latest read of the directory to answer failed, or nil.
func (d directory) err() error {
	return d.read.Err()
}

// reading reports a read of the directory still on its way.
func (d directory) reading() bool {
	return d.read.Out()
}

// missingScope is the scope the read said the token lacks, or "" when it
// lacked none.
func (d directory) missingScope() string {
	return missingScopeOf(d.err())
}

// missingScopeOf is the scope err says the Slack token lacks, or "" when it
// says none.
func missingScopeOf(err error) string {
	if missing, found := errors.AsType[*messaging.MissingScopeError](err); found {
		return missing.Needed
	}

	return ""
}

// begun is the directory with a read of it begun, superseding any still on
// its way, and the round the read answers in.
func (d directory) begun() (directory, latest.Round) {
	var round latest.Round

	d.read, round = d.read.Begin()

	return d, round
}

// cleared is the directory never read, for a channel that changed: a read
// of the channel before it, still on its way, lands in it no more.
func (d directory) cleared() directory {
	d.read = d.read.Cleared()

	return d
}

// answered is the directory with found, or why it could not be read, as the
// read begun in round answered; a read a later one superseded leaves it as
// it was.
func (d directory) answered(round latest.Round, found []loop.SlackTarget, err error) directory {
	d.read, _ = d.read.Answered(round, found, err)

	return d
}

// readingMembers is the directory reading a channel's members under ctx, for
// linking a user owner in the overlay opened as opened, and the command that
// reads them. It supersedes any read of the directory still on its way.
func (d directory) readingMembers(ctx context.Context, deps Deps, channel string, opened int) (directory, tea.Cmd) {
	read := deps.Messaging.ChannelMembers

	d, round := d.begun()

	return d, func() tea.Msg {
		found, err := read(ctx, channel)

		return membersRead{opened: opened, round: round, found: found, err: err}
	}
}

// readingGroups is the directory reading the workspace's user groups under
// ctx, for linking a team owner, where Slack can list them, and the command
// that reads them. It supersedes any read of the directory still on its way.
func (d directory) readingGroups(ctx context.Context, deps Deps, opened int) (directory, tea.Cmd) {
	read := deps.Messaging.UserGroups
	if read == nil {
		return d, nil
	}

	d, round := d.begun()

	return d, func() tea.Msg {
		found, err := read(ctx)

		return userGroupsRead{opened: opened, round: round, found: found, err: err}
	}
}

// membersRead is a channel's members, read for linking, as the read begun in
// round found them.
type membersRead struct {
	opened int
	round  latest.Round
	found  []loop.SlackTarget
	err    error
}

var _ applier = membersRead{}

// apply hands the members to the overlay that asked for them, the preview or
// People and groups, unless a later read of them has begun since: the
// preview's, of a channel chosen since, or chosen again; People's, a refresh.
func (msg membersRead) apply(m Model) (Model, tea.Cmd) {
	if preview, open := answerFor[messagingPreview](m, msg.opened); open {
		preview.tagging.members = preview.tagging.members.answered(msg.round, msg.found, msg.err)

		return m.withAnswered(preview), nil
	}

	if people, open := answerFor[peopleOverlay](m, msg.opened); open {
		people.members = people.members.answered(msg.round, msg.found, msg.err)

		return m.withAnswered(people), nil
	}

	return m, nil
}

// userGroupsRead is the workspace's user groups, read for linking a team, as
// the read begun in round found them.
type userGroupsRead struct {
	opened int
	round  latest.Round
	found  []loop.SlackTarget
	err    error
}

var _ applier = userGroupsRead{}

// apply hands the groups to the preview, or to People and groups, unless a
// later read of them, a refresh, has begun since.
func (msg userGroupsRead) apply(m Model) (Model, tea.Cmd) {
	if preview, open := answerFor[messagingPreview](m, msg.opened); open {
		preview.tagging.groups = preview.tagging.groups.answered(msg.round, msg.found, msg.err)
		preview.tagging = preview.tagging.withGroupsReadable()

		return m.withAnswered(preview), nil
	}

	if people, open := answerFor[peopleOverlay](m, msg.opened); open {
		people.groups = people.groups.answered(msg.round, msg.found, msg.err)

		return m.withAnswered(people.withChoices()), nil
	}

	return m, nil
}

// linksOwners is an overlay the owner picker opens over, holding the
// directory the picker chooses from.
type linksOwners interface {
	opened
	directoryFor(team bool) directory
}

// savingLink saves link, counted on its way until it answers, for the
// overlay opened as opened.
func (m Model) savingLink(link loop.OwnerLink, opened int) (Model, tea.Cmd) {
	m.paneWrites.links++

	return m, saveLink(m.session, m.deps, link, opened)
}

// saveLink saves link under ctx, whom an owner is on Slack or that they are
// not on it, for the overlay opened as opened.
func saveLink(ctx context.Context, deps Deps, link loop.OwnerLink, opened int) tea.Cmd {
	save, readWorkspace := deps.Store.LinkOwner, deps.Messaging.Workspace

	return func() tea.Msg {
		err := inWorkspace(ctx, readWorkspace, func(workspace string) error { return save(ctx, workspace, link) })

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

// inWorkspace makes write in the Slack workspace the token is for, read under
// ctx, which every kept link is made under, or says why that cannot be read.
func inWorkspace(
	ctx context.Context, readWorkspace func(ctx context.Context) (string, error), write func(workspace string) error,
) error {
	workspace, err := loop.TagWorkspace(ctx, readWorkspace)
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
	m.paneWrites.links--

	if preview, open := answerFor[messagingPreview](m, msg.opened); open {
		preview.tagging = preview.tagging.relinked(msg.link, msg.err)

		return m.withAnswered(preview), nil
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
	choices := make([]linkChoice, 0, len(p.from.entries())+1)

	for _, target := range p.from.entries() {
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
	case p.from.reading():
		lines = append(lines, "", kit.marks.inFlight+" still reading Slack's directory"+kit.marks.ellipsis)
	case p.from.err() != nil:
		lines = append(lines, "", kit.failureBlock(p.from.err(), width))
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

// choose saves the chosen link and goes back: to People and groups marked
// saving, as its own saves mark it, so no other write of it goes beside this
// one.
func (p ownerPicker) choose(m Model) (Model, tea.Cmd) {
	// "Not on Slack" is always a row, so one is always chosen.
	chosen, _ := p.list.chosen()

	var target *loop.SlackTarget
	if !chosen.notOnSlack {
		target = &chosen.target
	}

	linking, save := m.savingLink(decided(p.owner, p.team, target), p.back.openedAs())

	if people, fromPeople := p.back.(peopleOverlay); fromPeople {
		return people.saving(linking, save)
	}

	linking.overlay = p.back

	return linking, save
}
