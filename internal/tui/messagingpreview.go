// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"errors"
	"slices"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/loop"
	"github.com/jacob-delgado/workflow/internal/messaging"
)

// messagingHelp is what the editor shows below a message being composed. The markup
// depends on the service, so the note stays general rather than naming one.
const messagingHelp = "Edit the message above this line."

// errEmptyMessage refuses an announcement with nothing in it: guidance, not a
// failure.
var errEmptyMessage = errors.New("nothing to announce: the message was empty")

// messagingPreview is an announcement about to be sent.
type messagingPreview struct {
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
	// and offers "announce when CI passes" only where waiting for CI makes sense.
	moment messaging.Moment
	// service names the messaging service, for the preview title.
	service string
	noCI    bool
	send    sendState
}

// destination is where this post will go, as it is shown and as it is sent.
func (p messagingPreview) destination() string {
	if p.channel != "" {
		return p.channel
	}

	return p.fallback
}

var (
	_ editable                   = messagingPreview{}
	_ failable[messagingPreview] = messagingPreview{}
)

// view shows the message as it will be posted, where, and how CI stands, its
// outcome pinned under the title so a long refusal is seen, not clipped.
func (p messagingPreview) view(width, _ int) (string, string) {
	lines := pinnedOutcome(p.styles, p.marks, p.send, "announcing", width)
	lines = append(lines, wrap(p.text, width), "", "to  "+p.destination())

	return "Announce to " + p.service, strings.Join(lines, "\n")
}

// footer offers announcing now or when CI passes, changing the channel where
// there is a choice, another edit, or leaving.
func (p messagingPreview) footer(keys keyMap) []key.Binding {
	if p.send.sending {
		return []key.Binding{keys.interrupt}
	}

	buttons := []key.Binding{relabel(keys.confirm, "announce now")}
	if p.waitsForCI() {
		buttons = append(buttons, relabel(keys.postWhenGreen, "when CI passes"))
	}

	if len(p.channels) > 1 {
		buttons = append(buttons, relabel(keys.cycleLeft, "change channel"))
	}

	return append(buttons, keys.edit, relabel(keys.closeOverlay, "discard"))
}

// handleKey answers a key while the message is previewed.
func (p messagingPreview) handleKey(m Model, msg tea.KeyPressMsg) (Model, tea.Cmd) {
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
		return m, m.deps.Editor.Edit(p.text, messagingHelp, func(text string, err error) tea.Msg {
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
func (p messagingPreview) cycleChannel(m Model, step int) (Model, tea.Cmd) {
	if len(p.channels) <= 1 {
		return m, nil
	}

	current := slices.Index(p.channels, p.channel)
	p.channel = p.channels[(current+step+len(p.channels))%len(p.channels)]
	m.overlay = p

	return m, nil
}

// post posts the message now.
func (p messagingPreview) post(m Model) (Model, tea.Cmd) {
	if strings.TrimSpace(p.text) == "" {
		return m.closeOverlay().noticedGuidance(errEmptyMessage), nil
	}

	if m.dryRun {
		return m.closeOverlay().noticed("dry run: would announce to " + p.destination()), nil
	}

	p.send = starting()
	m.overlay = p

	return m.sendToMessaging(p.channel, p.text, p.moment)
}

// waitsForCI reports whether this post can wait for CI to pass. Only a "ready
// for review" announcement can: a merge or a red CI has already happened, and a
// pull request that reports no CI has none to wait for.
func (p messagingPreview) waitsForCI() bool {
	return p.moment == messaging.MomentReady && !p.noCI
}

// postWhenGreen posts once CI passes: now, if it already has. Where the post
// cannot wait for CI, the preview does not offer the key, and it does nothing.
func (p messagingPreview) postWhenGreen(m Model) (Model, tea.Cmd) {
	if !p.waitsForCI() {
		return m, nil
	}

	if m.review.ci.State == forge.CIPassed {
		return p.post(m)
	}

	if m.dryRun {
		return m.closeOverlay().noticed("dry run: would announce to " + p.destination() + " once CI passes"), nil
	}

	m = m.closeOverlay().noticed(m.marks.inFlight + " will announce to " + p.destination() + " once CI passes")
	m.messaging.pending, m.messaging.send.err, m.messaging.dropped = queuedPost{
		pull: m.review.pull.Number, text: p.text, channel: p.channel,
	}, nil, ""

	return m.keepPolling(m.checkCI())
}

// sendToMessaging posts text marking moment, and once it has gone out records
// it in the store, the way every surface delivers an announcement. It replaces
// any post waiting for CI: that one would otherwise follow it once CI passed,
// and the channel would read it twice.
func (m Model) sendToMessaging(channel, text string, moment messaging.Moment) (Model, tea.Cmd) {
	post, memory := m.deps.Messaging.Post, loop.AnnounceMemory{Record: m.deps.Store.RecordAnnounce}
	made := loop.Announced{Pull: m.review.pull.Number, Moment: moment}
	delivery := loop.Delivery{Channel: channel, Text: text, Made: made}
	m.messaging.send, m.messaging.pending, m.messaging.dropped = starting(), queuedPost{}, ""

	return m, func() tea.Msg { return messagingPosted{made: made, err: loop.Deliver(post, memory, delivery)} }
}

// applyEdit puts the edited message back in the preview, or records why the
// editor failed.
func (p messagingPreview) applyEdit(m Model, text string, err error) (Model, tea.Cmd) {
	if err != nil {
		p.send = p.send.failed(err)
	} else {
		p.text = text
	}

	m.overlay = p

	return m, nil
}

// messagingPosted reports how posting went, and the announcement it made: a pull
// request and the moment it marked.
type messagingPosted struct {
	made loop.Announced
	err  error
}

// apply records the post, or why it failed — in the preview if it is open, and
// in the pane if the post was one waiting for CI.
func (msg messagingPosted) apply(m Model) (Model, tea.Cmd) {
	if msg.err != nil {
		m.messaging.send = m.messaging.send.failed(msg.err)

		return keepOpenWith[messagingPreview](m, msg.err).noticedFailure(msg.err), nil
	}

	m.messaging.posted, m.messaging.send = append(slices.Clone(m.messaging.posted), msg.made), sendState{}

	if _, open := m.overlay.(messagingPreview); open {
		m = m.closeOverlay()
	}

	return m.noticed(m.marks.done + " announced to " + m.cfg.Messaging.Target()), nil
}

// failed is the preview kept open with the reason the post failed.
func (p messagingPreview) failed(err error) messagingPreview {
	p.send = p.send.failed(err)

	return p
}
