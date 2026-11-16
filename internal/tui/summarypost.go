// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"slices"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/loop"
	"github.com/jacob-delgado/workflow/internal/sanitize"
	"github.com/jacob-delgado/workflow/internal/tui/send"
)

// summaryPostHelp is what the editor shows below the Summary being edited.
const summaryPostHelp = "Edit the summary above this line. It is Markdown, posted as the service shows it."

// summaryPost is the Summary about to be posted, as messagingPreview is an
// announcement: the text, where it goes, and how the post stands.
type summaryPost struct {
	text string
	// fallback is where a post goes when no channel is chosen — a webhook's own
	// channel, or the note that none is set.
	fallback string
	// channel is the chosen bot channel, and channels the ones it can be cycled
	// through. Both are empty for a webhook, which carries its own channel.
	channel  string
	channels []string
	service  string
	// kind is the service's, which the length is measured for.
	kind config.MessagingKind
	send send.State
	// problem is why the editor would not take the Summary, which no post
	// was sent to find.
	problem error
	// opened is the round the preview opened in, which an edit and the post
	// carry back, so they land in no preview opened after.
	opened int
}

var (
	_ editable            = summaryPost{}
	_ sender[summaryPost] = summaryPost{}
)

// canPostSummary reports whether the Summary can be posted now: messaging is
// set up, and every source has answered for the period shown, so what is
// previewed is all there is.
func (m Model) canPostSummary() bool {
	return m.deps.Messaging.Post != nil && m.cfg.Messaging.Mode() != config.MessagingNone && m.summary.complete()
}

// previewSummaryPost opens the preview of the Summary shown, as Markdown, to
// the first channel there is a choice of. Every value in it is a source's to
// write, so it is neutralized before the terminal or the editor sees it.
func (m Model) previewSummaryPost() (Model, tea.Cmd) {
	m, opened := m.nextRound()
	m.overlay = summaryPost{
		opened:   opened,
		text:     sanitize.Text(m.summary.shown(m.deps.now()).Text(m.deps.now().Location())),
		fallback: m.cfg.Messaging.Target(), channel: defaultChannel(m.cfg), channels: m.cfg.Messaging.ChannelChoices(),
		service: m.cfg.Messaging.Service(), kind: m.cfg.Messaging.Kind,
	}

	return m, nil
}

// destination is where this post will go, as it is shown: the configuration
// names it, so it is drawn as text alone.
func (p summaryPost) destination() string {
	if p.channel != "" {
		return sanitize.Line(p.channel)
	}

	return sanitize.Line(p.fallback)
}

// view shows the Summary as it will be posted, where, and how long it is
// against what the service takes, its outcome pinned under the title so a
// long refusal is seen, not clipped.
func (p summaryPost) view(kit renderKit, width, _ int) (string, string) {
	lines := append(kit.pinnedSend(p.send, "posting", width), kit.pinnedProblem(p.problem, width)...)
	lines = append(lines, wrap(p.text, width), "",
		"to  "+p.destination()+", "+loop.SummaryLength(p.kind, p.text).String())

	return "Post to " + p.service, strings.Join(lines, "\n")
}

// footer offers posting, changing the channel where there is a choice,
// editing, or leaving.
func (p summaryPost) footer(keys keyMap) []key.Binding {
	if p.send.InFlight() {
		return []key.Binding{keys.interrupt}
	}

	buttons := []key.Binding{relabel(keys.confirm, "post")}
	if len(p.channels) > 1 {
		buttons = append(buttons, relabel(keys.cycleLeft, "change channel"))
	}

	return append(buttons, keys.edit, relabel(keys.closeOverlay, escDiscard))
}

// handleKey answers a key while the Summary is previewed.
func (p summaryPost) handleKey(m Model, msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch {
	case p.send.InFlight():
		return m, nil
	case key.Matches(msg, m.keys.closeOverlay):
		return m.closeOverlay(), nil
	case key.Matches(msg, m.keys.cycleRight):
		return p.cycleChannel(m, 1), nil
	case key.Matches(msg, m.keys.cycleLeft):
		return p.cycleChannel(m, -1), nil
	case key.Matches(msg, m.keys.edit) && m.deps.Editor.Edit != nil:
		return m, m.deps.Editor.Edit(p.text, summaryPostHelp, editedFor(p.opened))
	case key.Matches(msg, m.keys.confirm):
		return p.post(m)
	}

	return m, nil
}

// which names the summary's post preview.
func (summaryPost) which() overlayKind { return overlaySummaryPost }

// openedAs is the round the preview opened in.
func (p summaryPost) openedAs() int { return p.opened }

// sent is the Summary's post.
func (p summaryPost) sent() send.State { return p.send }

// withSend is the preview with its post as state.
func (p summaryPost) withSend(state send.State) summaryPost {
	p.send = state

	return p
}

// cycleChannel moves the destination to the next configured channel,
// wrapping.
func (p summaryPost) cycleChannel(m Model, step int) Model {
	if len(p.channels) > 1 {
		current := slices.Index(p.channels, p.channel)
		p.channel = p.channels[around(current, len(p.channels)).by(step)]
		m.overlay = p
	}

	return m
}

// post posts the Summary now, rendered for the service; a dry run says where
// it would go instead. handleKey takes no key while a post is out, so Sent
// always sends, and an editor's failure goes with it.
func (p summaryPost) post(m Model) (Model, tea.Cmd) {
	if strings.TrimSpace(p.text) == "" {
		return m.closeOverlay().noticedGuidance(loop.ErrEmptySummary), nil
	}

	if m.dryRun {
		return m.closeOverlay().noticed("dry run: would post to " + p.destination()), nil
	}

	p.send, _ = p.send.Sent()
	p.problem = nil
	m.overlay = p

	ctx, post, kind, channel, text := m.session, m.deps.Messaging.Post, m.cfg.Messaging.Kind, p.channel, p.text
	to, opened := p.destination(), p.opened

	return m, func() tea.Msg {
		return summaryPosted{opened: opened, to: to, err: loop.PostSummary(ctx, post, kind, channel, text)}
	}
}

// applyEdit puts the edited Summary back in the preview, clearing why an
// earlier edit failed, or keeps why the editor failed as its problem, in
// place of a refusal an earlier post had.
func (p summaryPost) applyEdit(m Model, text string, err error) (Model, tea.Cmd) {
	if err != nil {
		p.problem, p.send = err, p.send.Cleared()
	} else {
		p.text, p.problem = text, nil
	}

	return m.withAnswered(p), nil
}

// summaryPosted reports how posting the Summary went, and where it went, for
// the preview opened as opened.
type summaryPosted struct {
	opened int
	to     string
	err    error
}

var _ applier = summaryPosted{}

// apply closes the preview and says where the Summary went, or keeps it open
// with why it did not, which the footer says too. Either is said even for a
// preview no longer open, since the post went or failed all the same.
func (msg summaryPosted) apply(m Model) (Model, tea.Cmd) {
	m, _, done := answered[summaryPost](m, msg.opened, msg.err)
	if done {
		m = m.closeOverlay()
	}

	if msg.err != nil {
		return m.noticedFailure(msg.err), nil
	}

	return m.noticed(m.marks.done + " posted to " + msg.to), nil
}
