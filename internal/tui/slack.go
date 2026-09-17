// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"slices"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/convention"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/slack"
)

// slackHelp is what the editor shows below a Slack message.
const slackHelp = "Edit the Slack message above this line. Slack's own markup works: <url|text>\n" +
	"is a link, and *bold* is bold."

// slackState is what has been posted to Slack this session. Nothing is kept
// between sessions: with no state file there is nowhere to keep it.
type slackState struct {
	// posted is the pull requests announced this session, by number.
	posted []int
	// sending is a post Slack has not answered yet, which is not offered
	// again until it does.
	sending bool
	pending queuedPost
	// dropped records a post given up on and why, so the reason outlives the
	// transient footer notice that first reports it.
	dropped string
	err     error
	author  string
}

// droppedTimeFormat stamps a dropped post with the time it was given up on.
const droppedTimeFormat = "15:04"

// queuedPost is a message waiting for one pull request's CI to pass. It names
// the pull request it was written for, because the one on screen can change
// while it waits, and a message is only ever sent for the one its writer saw.
type queuedPost struct {
	pull int
	text string
}

// waiting reports a post that has not been sent or given up on.
func (q queuedPost) waiting() bool {
	return q.text != ""
}

// quitGuard asks before quitting while a post is waiting for CI, which quitting
// would silently lose.
type quitGuard struct{}

var _ overlay = quitGuard{}

// view says what quitting would cost.
func (quitGuard) view(width, _ int) (string, string) {
	return "Quit", wrap("A post is waiting for CI and will be lost.", width)
}

// footer offers quitting anyway or staying.
func (quitGuard) footer(keys keyMap) []key.Binding {
	return []key.Binding{relabel(keys.confirm, "quit"), relabel(keys.closeOverlay, "stay")}
}

// handleKey answers a key while the guard is shown.
func (quitGuard) handleKey(m Model, msg tea.KeyMsg) (Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.confirm):
		return m, tea.Quit
	case key.Matches(msg, m.keys.closeOverlay):
		return m.closeOverlay(), nil
	default:
		return m, nil
	}
}

// loadAuthor is the command that asks the forge who opened the pull request.
func (m Model) loadAuthor() tea.Cmd {
	author := m.deps.Forge.Author
	if author == nil || m.slack.author != "" {
		return nil
	}

	return func() tea.Msg {
		name, err := author()

		return authorFound{name: name, err: err}
	}
}

// authorFound is who the forge credential belongs to.
type authorFound struct {
	name string
	err  error
}

// apply records the name. Not knowing it only makes the announcement say less.
func (msg authorFound) apply(m Model) (Model, tea.Cmd) {
	if msg.err == nil {
		m.slack.author = msg.name
	}

	return m, nil
}

// announcement is the message telling the channel the pull request is ready.
func (m Model) announcement() string {
	issueKey, _ := convention.IssueKey(m.branch.branch.Name)
	issue, _ := m.issues.find(issueKey)

	return slack.Announcement{
		Author: m.slack.author, PullRequestURL: m.review.pull.URL, PullRequestTitle: m.review.pull.Title,
		IssueKey: issueKey, IssueSummary: issue.Summary, IssueURL: m.browseURL(issueKey),
	}.Text()
}

// slackRail is where messages go and what has been posted.
func (m Model) slackRail(_ int) string {
	return m.cfg.Slack.Target() + "\n" + m.styles.label.Render(m.slackState())
}

// slackState says what has happened in Slack this session.
func (m Model) slackState() string {
	switch {
	case m.slack.sending:
		return m.marks.inFlight + " posting" + m.marks.ellipsis
	case m.slack.err != nil:
		return m.marks.failed + " " + m.slack.err.Error()
	case m.announced():
		return m.marks.done + " posted"
	case m.slack.pending.waiting():
		return m.marks.inFlight + " posts when CI passes"
	case m.slack.dropped != "":
		return m.marks.failed + " not posted: " + m.slack.dropped
	default:
		return m.marks.notStarted + " nothing posted"
	}
}

// slackDetail previews the announcement, or says what it needs first.
func (m Model) slackDetail(width int) string {
	if m.cfg.Slack.Mode() == config.SlackNone {
		return wrap("Slack is not set up.\n\nAdd slack.webhook_url (or slack.token and slack.channel) to\n~/"+
			config.FileName+". `workflow doctor --online` checks it.", width)
	}

	if !m.review.found {
		return wrap("Open a pull request first (4 Review); the message links to it.\n\n"+m.slackRail(0), width)
	}

	lines := []string{
		m.announcement(),
		"",
		m.styles.label.Render("to     ") + m.cfg.Slack.Target(),
		m.styles.label.Render("CI     ") + m.ciSummary(),
		m.styles.label.Render("state  ") + m.slackState(),
	}

	return wrap(strings.Join(lines, "\n"), width)
}

// announced reports that the pull request on screen was posted this session.
func (m Model) announced() bool {
	return m.review.found && slices.Contains(m.slack.posted, m.review.pull.Number)
}

// canPost reports a pull request to announce, a way to post it, and no post
// of it already made or on its way.
func (m Model) canPost() bool {
	return m.review.found && m.deps.Slack.Post != nil && m.cfg.Slack.Mode() != config.SlackNone &&
		!m.announced() && !m.slack.sending
}

// slackKeys offers composing the post.
func (m Model) slackKeys() []key.Binding {
	if !m.canPost() {
		return nil
	}

	return []key.Binding{m.keys.compose}
}

// handleSlackKey answers the Slack pane's own keys.
func (m Model) handleSlackKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	if !key.Matches(msg, m.keys.compose) || !m.canPost() {
		return m, nil
	}

	m.overlay = slackPreview{
		marks: m.marks, text: m.announcement(), target: m.cfg.Slack.Target(),
		noCI: m.review.checked && m.review.ci.State == forge.CINone,
	}

	return m, nil
}

// slackPreview is a Slack message about to be posted.
type slackPreview struct {
	marks   glyphs
	text    string
	target  string
	noCI    bool
	sending bool
	err     error
}

var _ overlay = slackPreview{}

// view shows the message as it will be posted, where, and how CI stands.
func (p slackPreview) view(width, _ int) (string, string) {
	lines := []string{wrap(p.text, width), "", "to  " + p.target}

	switch {
	case p.sending:
		lines = append(lines, "", "posting"+p.marks.ellipsis)
	case p.err != nil:
		lines = append(lines, "", p.marks.failed+" "+p.err.Error())
	}

	return "Post to Slack", strings.Join(lines, "\n")
}

// footer offers posting now or when CI passes, another edit, or leaving.
func (p slackPreview) footer(keys keyMap) []key.Binding {
	if p.sending {
		return []key.Binding{keys.interrupt}
	}

	buttons := []key.Binding{relabel(keys.confirm, "post now")}
	if !p.noCI {
		buttons = append(buttons, keys.postWhenGreen)
	}

	return append(buttons, keys.edit, relabel(keys.closeOverlay, "discard"))
}

// handleKey answers a key while the message is previewed.
func (p slackPreview) handleKey(m Model, msg tea.KeyMsg) (Model, tea.Cmd) {
	switch {
	case p.sending:
		return m, nil
	case key.Matches(msg, m.keys.closeOverlay):
		return m.closeOverlay(), nil
	case key.Matches(msg, m.keys.edit) && m.deps.Editor.Edit != nil:
		return m, m.deps.Editor.Edit(p.text, slackHelp, func(text string, err error) tea.Msg {
			return slackTextEdited{text: text, err: err}
		})
	case key.Matches(msg, m.keys.confirm):
		return p.post(m)
	case key.Matches(msg, m.keys.postWhenGreen):
		return p.postWhenGreen(m)
	default:
		return m, nil
	}
}

// post posts the message now.
func (p slackPreview) post(m Model) (Model, tea.Cmd) {
	if strings.TrimSpace(p.text) == "" {
		return m.closeOverlay().noticed("nothing to post: the message was empty"), nil
	}

	if m.dryRun {
		return m.closeOverlay().noticed("dry run: would post to " + p.target), nil
	}

	p.sending, p.err = true, nil
	m.overlay = p

	return m.sendToSlack(p.text)
}

// postWhenGreen posts once CI passes: now, if it already has.
func (p slackPreview) postWhenGreen(m Model) (Model, tea.Cmd) {
	if m.review.ci.State == forge.CIPassed {
		return p.post(m)
	}

	if m.dryRun {
		return m.closeOverlay().noticed("dry run: would post to " + p.target + " once CI passes"), nil
	}

	m = m.closeOverlay().noticed(m.marks.inFlight + " will post to " + p.target + " once CI passes")
	m.slack.pending, m.slack.err, m.slack.dropped = queuedPost{pull: m.review.pull.Number, text: p.text}, nil, ""

	return m.keepPolling(m.checkCI())
}

// sendToSlack posts text. It replaces any post waiting for CI: that one would
// otherwise follow it once CI passed, and the channel would read it twice.
func (m Model) sendToSlack(text string) (Model, tea.Cmd) {
	post, pull := m.deps.Slack.Post, m.review.pull.Number
	m.slack.sending, m.slack.pending, m.slack.dropped = true, queuedPost{}, ""

	return m, func() tea.Msg { return slackPosted{pull: pull, err: post(text)} }
}

// withoutQueuedPost gives up on a post waiting for CI, saying so, because the
// pull request it was written for is no longer the one on screen.
func (m Model) withoutQueuedPost() Model {
	if !m.slack.pending.waiting() {
		return m
	}

	dropped := m.slack.pending.pull
	m.slack.pending = queuedPost{}

	return m.noticed(m.marks.failed + " dropped the Slack post waiting for #" + strconv.Itoa(dropped) +
		", which is no longer this branch's pull request")
}

// postIfGreen posts the message waiting for CI once CI passes, and gives up on
// it, saying so, if CI fails or the pull request is another one.
func (m Model) postIfGreen() (Model, tea.Cmd) {
	if !m.slack.pending.waiting() {
		return m, nil
	}

	if m.slack.pending.pull != m.review.pull.Number {
		return m.withoutQueuedPost(), nil
	}

	switch m.review.ci.State {
	case forge.CIPassed:
		return m.sendToSlack(m.slack.pending.text)
	case forge.CIFailed:
		m.slack.pending = queuedPost{}
		m.slack.dropped = "CI failed at " + m.deps.now().Format(droppedTimeFormat)

		return m.noticed(m.marks.failed + " CI failed, so nothing was posted to Slack"), nil
	case forge.CINone, forge.CIRunning:
		return m, nil
	default:
		return m, nil
	}
}

// slackTextEdited is a Slack message back from the editor.
type slackTextEdited struct {
	text string
	err  error
}

// apply puts the message in the preview, if it is still open.
func (msg slackTextEdited) apply(m Model) (Model, tea.Cmd) {
	preview, open := m.overlay.(slackPreview)
	if !open {
		return m, nil
	}

	if msg.err != nil {
		preview.err = msg.err
	} else {
		preview.text = msg.text
	}

	m.overlay = preview

	return m, nil
}

// slackPosted reports how posting went, and for which pull request.
type slackPosted struct {
	pull int
	err  error
}

// apply records the post, or why it failed — in the preview if it is open, and
// in the pane if the post was one waiting for CI.
func (msg slackPosted) apply(m Model) (Model, tea.Cmd) {
	preview, open := m.overlay.(slackPreview)
	m.slack.sending = false

	if msg.err != nil {
		m.slack.err = msg.err

		if open {
			preview.sending, preview.err = false, msg.err
			m.overlay = preview
		}

		return m.noticed(m.failure(msg.err)), nil
	}

	m.slack.posted, m.slack.err = append(slices.Clone(m.slack.posted), msg.pull), nil

	if open {
		m = m.closeOverlay()
	}

	return m.noticed(m.marks.done + " posted to " + m.cfg.Slack.Target()), nil
}
