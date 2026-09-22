// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"slices"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/slack"
)

// slackHelp is what the editor shows below a message being composed. The markup
// depends on the service, so the note stays general rather than naming one.
const slackHelp = "Edit the message above this line."

// slackState is what has been posted to Slack this session. Nothing is kept
// between sessions: with no state file there is nowhere to keep it.
type slackState struct {
	// posted is the announcements made this session, each a pull request and the
	// moment it marked, so one pull request can be announced at each of its
	// moments — opened, then merged — without a moment being offered twice.
	posted []postedMoment
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

// postedMoment is one announcement already made: a pull request, and the moment
// it marked.
type postedMoment struct {
	pull   int
	moment slack.Moment
}

// droppedTimeFormat stamps a dropped post with the time it was given up on.
const droppedTimeFormat = "15:04"

// queuedPost is a message waiting for one pull request's CI to pass. It names
// the pull request it was written for, because the one on screen can change
// while it waits, and a message is only ever sent for the one its writer saw.
type queuedPost struct {
	pull    int
	text    string
	channel string
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
func (quitGuard) handleKey(m Model, msg tea.KeyPressMsg) (Model, tea.Cmd) {
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

// announceMoment is the moment the pull request on screen is at: merged, its CI
// red, or — the common case — open and ready for review.
func (m Model) announceMoment() slack.Moment {
	switch {
	case m.review.pull.State == forge.StateMerged:
		return slack.MomentMerged
	case m.review.ci.State == forge.CIFailed:
		return slack.MomentCIRed
	default:
		return slack.MomentReady
	}
}

// announcement is the message marking the pull request's current moment.
func (m Model) announcement(moment slack.Moment) string {
	issueKey, _ := m.branchIssue()
	issue, _ := m.issues.find(issueKey)

	return slack.Announcement{
		Author: m.slack.author, PullRequestURL: m.review.pull.URL, PullRequestTitle: m.review.pull.Title,
		IssueKey: string(issueKey), IssueSummary: issue.Summary, IssueURL: m.browseURL(issueKey), Noun: m.vocab.noun,
		Moment: moment, Kind: m.cfg.Messaging.Kind, Template: m.cfg.Messaging.Announcement,
	}.Text()
}

// slackRail is where messages go and what has been posted.
func (m Model) slackRail(_ int) string {
	return m.cfg.Messaging.Target() + "\n" + m.slackState()
}

// slackState says what has happened in Slack this session.
func (m Model) slackState() string {
	switch {
	case m.slack.sending:
		return m.marks.inFlight + " posting" + m.marks.ellipsis
	case m.slack.err != nil:
		return m.failedGlyph() + " " + m.slack.err.Error()
	case m.announced():
		return m.marks.done + " posted"
	case m.slack.pending.waiting():
		return m.marks.inFlight + " posts when CI passes"
	case m.slack.dropped != "":
		return m.failedGlyph() + " not posted: " + m.slack.dropped
	default:
		return m.marks.notStarted + " nothing posted"
	}
}

// slackDetail previews the announcement, or says what it needs first.
func (m Model) slackDetail(width int) string {
	if m.cfg.Messaging.Mode() == config.MessagingNone {
		return wrap(m.cfg.Messaging.Service()+" is not set up.\n\nAdd messaging.webhook_url (or, for Slack, "+
			"messaging.token and\nmessaging.channel) to ~/"+config.FileName+
			". `workflow doctor --online` checks it.", width)
	}

	if !m.review.found {
		return wrap("Open a "+m.vocab.noun+" first ("+paneReview.label()+"); the message links to it.\n\n"+
			m.slackRail(0), width)
	}

	lines := []string{
		m.announcement(m.announceMoment()),
		"",
		m.styles.label.Render("to     ") + m.cfg.Messaging.Target(),
		m.styles.label.Render("CI     ") + m.ciSummary(),
		m.styles.label.Render("state  ") + m.slackState(),
	}

	return wrap(strings.Join(lines, "\n"), width)
}

// announced reports that the pull request on screen was already posted at its
// current moment this session — a merge announced counts, an opening does not.
func (m Model) announced() bool {
	return m.review.found &&
		slices.Contains(m.slack.posted, postedMoment{pull: m.review.pull.Number, moment: m.announceMoment()})
}

// canPost reports a pull request to announce, a way to post it, and no post
// of it already made or on its way.
func (m Model) canPost() bool {
	return m.review.found && m.deps.Slack.Post != nil && m.cfg.Messaging.Mode() != config.MessagingNone &&
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
func (m Model) handleSlackKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	if !key.Matches(msg, m.keys.compose) || !m.canPost() {
		return m, nil
	}

	channels := m.cfg.Messaging.ChannelChoices()

	channel := ""
	if len(channels) > 0 {
		channel = channels[0]
	}

	moment := m.announceMoment()

	m.overlay = slackPreview{
		marks: m.marks, styles: m.styles, text: m.announcement(moment), fallback: m.cfg.Messaging.Target(),
		channel: channel, channels: channels, moment: moment, service: m.cfg.Messaging.Service(),
		noCI: m.review.checked && m.review.ci.State == forge.CINone,
	}

	return m, nil
}

// slackPreview is a Slack message about to be posted.
type slackPreview struct {
	marks  glyphs
	styles styles
	text   string
	// fallback is where a post goes when no channel is chosen — a webhook's own
	// channel, or the note that none is set.
	fallback string
	// channel is the chosen bot channel, and channels the ones it can be cycled
	// through. Both are empty for a webhook, which carries its own channel.
	channel  string
	channels []string
	// moment is what this post marks, so the pane records the right one as posted
	// and offers "post when CI passes" only where waiting for CI makes sense.
	moment slack.Moment
	// service names the messaging service, for the preview title.
	service string
	noCI    bool
	send    sendState
}

// destination is where this post will go, as it is shown and as it is sent.
func (p slackPreview) destination() string {
	if p.channel != "" {
		return p.channel
	}

	return p.fallback
}

var _ editable = slackPreview{}

// view shows the message as it will be posted, where, and how CI stands, its
// outcome pinned under the title so a long refusal is seen, not clipped.
func (p slackPreview) view(width, _ int) (string, string) {
	lines := pinnedOutcome(p.styles, p.marks, p.send, "posting", width)
	lines = append(lines, wrap(p.text, width), "", "to  "+p.destination())

	return "Post to " + p.service, strings.Join(lines, "\n")
}

// footer offers posting now or when CI passes, changing the channel where there
// is a choice, another edit, or leaving.
func (p slackPreview) footer(keys keyMap) []key.Binding {
	if p.send.sending {
		return []key.Binding{keys.interrupt}
	}

	buttons := []key.Binding{relabel(keys.confirm, "post now")}
	if p.moment == slack.MomentReady && !p.noCI {
		// Only a "ready for review" post waits for CI. A merge or a red CI has
		// already happened; there is nothing to wait for.
		buttons = append(buttons, keys.postWhenGreen)
	}

	if len(p.channels) > 1 {
		buttons = append(buttons, relabel(keys.cycleLeft, "change channel"))
	}

	return append(buttons, keys.edit, relabel(keys.closeOverlay, "discard"))
}

// handleKey answers a key while the message is previewed.
func (p slackPreview) handleKey(m Model, msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch {
	case p.send.sending:
		return m, nil
	case key.Matches(msg, m.keys.closeOverlay):
		return m.closeOverlay(), nil
	case key.Matches(msg, m.keys.cycleRight):
		return p.cycleChannel(m, 1)
	case key.Matches(msg, m.keys.cycleLeft):
		return p.cycleChannel(m, -1)
	case key.Matches(msg, m.keys.edit) && m.deps.Editor.Edit != nil:
		return m, m.deps.Editor.Edit(p.text, slackHelp, func(text string, err error) tea.Msg {
			return textEdited{text: text, err: err}
		})
	case key.Matches(msg, m.keys.confirm):
		return p.post(m)
	case key.Matches(msg, m.keys.postWhenGreen):
		return p.postWhenGreen(m)
	default:
		return m, nil
	}
}

// cycleChannel moves the destination to the next configured channel, wrapping.
func (p slackPreview) cycleChannel(m Model, step int) (Model, tea.Cmd) {
	if len(p.channels) <= 1 {
		return m, nil
	}

	current := slices.Index(p.channels, p.channel)
	p.channel = p.channels[(current+step+len(p.channels))%len(p.channels)]
	m.overlay = p

	return m, nil
}

// post posts the message now.
func (p slackPreview) post(m Model) (Model, tea.Cmd) {
	if strings.TrimSpace(p.text) == "" {
		return m.closeOverlay().noticed("nothing to post: the message was empty"), nil
	}

	if m.dryRun {
		return m.closeOverlay().noticed("dry run: would post to " + p.destination()), nil
	}

	p.send = starting()
	m.overlay = p

	return m.sendToSlack(p.channel, p.text, p.moment)
}

// postWhenGreen posts once CI passes: now, if it already has. Only a "ready for
// review" post waits for CI; a merge or a red CI has already happened, so there
// is nothing to wait for and the key does nothing.
func (p slackPreview) postWhenGreen(m Model) (Model, tea.Cmd) {
	if p.moment != slack.MomentReady {
		return m, nil
	}

	if m.review.ci.State == forge.CIPassed {
		return p.post(m)
	}

	if m.dryRun {
		return m.closeOverlay().noticed("dry run: would post to " + p.destination() + " once CI passes"), nil
	}

	m = m.closeOverlay().noticed(m.marks.inFlight + " will post to " + p.destination() + " once CI passes")
	m.slack.pending, m.slack.err, m.slack.dropped = queuedPost{
		pull: m.review.pull.Number, text: p.text, channel: p.channel,
	}, nil, ""

	return m.keepPolling(m.checkCI())
}

// sendToSlack posts text marking moment. It replaces any post waiting for CI:
// that one would otherwise follow it once CI passed, and the channel would read
// it twice.
func (m Model) sendToSlack(channel, text string, moment slack.Moment) (Model, tea.Cmd) {
	post, pull := m.deps.Slack.Post, m.review.pull.Number
	m.slack.sending, m.slack.pending, m.slack.dropped = true, queuedPost{}, ""

	return m, func() tea.Msg { return slackPosted{pull: pull, moment: moment, err: post(channel, text)} }
}

// withoutQueuedPost gives up on a post waiting for CI, saying so, because the
// pull request it was written for is no longer the one on screen.
func (m Model) withoutQueuedPost() Model {
	if !m.slack.pending.waiting() {
		return m
	}

	dropped := m.slack.pending.pull
	m.slack.pending = queuedPost{}

	return m.noticed(m.failedGlyph() + " dropped the Slack post waiting for " + m.vocab.sigil + strconv.Itoa(dropped) +
		", which is no longer this branch's " + m.vocab.noun)
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
		// A queued post is always a "ready for review" one: it is what waits for CI.
		return m.sendToSlack(m.slack.pending.channel, m.slack.pending.text, slack.MomentReady)
	case forge.CIFailed:
		m.slack.pending = queuedPost{}
		m.slack.dropped = "CI failed at " + m.deps.now().Format(droppedTimeFormat)

		return m.noticed(m.failedGlyph() + " CI failed, so nothing was posted to Slack"), nil
	case forge.CINone, forge.CIRunning:
		return m, nil
	default:
		return m, nil
	}
}

// applyEdit puts the edited message back in the preview, or records why the
// editor failed.
func (p slackPreview) applyEdit(m Model, text string, err error) (Model, tea.Cmd) {
	if err != nil {
		p.send = p.send.failed(err)
	} else {
		p.text = text
	}

	m.overlay = p

	return m, nil
}

// slackPosted reports how posting went: for which pull request, and the moment
// it marked.
type slackPosted struct {
	pull   int
	moment slack.Moment
	err    error
}

// apply records the post, or why it failed — in the preview if it is open, and
// in the pane if the post was one waiting for CI.
func (msg slackPosted) apply(m Model) (Model, tea.Cmd) {
	preview, open := m.overlay.(slackPreview)
	m.slack.sending = false

	if msg.err != nil {
		m.slack.err = msg.err

		if open {
			preview.send = preview.send.failed(msg.err)
			m.overlay = preview
		}

		return m.noticed(m.failure(msg.err)), nil
	}

	m.slack.posted, m.slack.err = append(slices.Clone(m.slack.posted),
		postedMoment{pull: msg.pull, moment: msg.moment}), nil

	if open {
		m = m.closeOverlay()
	}

	return m.noticed(m.marks.done + " posted to " + m.cfg.Messaging.Target()), nil
}
