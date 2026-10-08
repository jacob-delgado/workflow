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

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/loop"
	"github.com/jacob-delgado/workflow/internal/messaging"
	"github.com/jacob-delgado/workflow/internal/sanitize"
)

// Refusals of an announcement that never went: one given up on is a failure,
// told with the service it was for.
var (
	errAnnouncementDropped = errors.New("dropped the announcement")
	errCIFailedUnannounced = errors.New("CI failed, so nothing was announced")
)

// messagingState is what has been announced, and how far the messaging pane's
// detail is scrolled. The announcements made this session are seeded at startup
// from the store's record of earlier ones, so a restart does not forget them.
type messagingState struct {
	// posted is the announcements made this session, each a pull request and the
	// moment it marked, so one pull request can be announced at each of its
	// moments — opened, then merged — without a moment being offered twice.
	posted []loop.Announced
	// send is a post the service has not answered yet, which is not offered
	// again until it does, or why the last one failed.
	send    sendState
	pending queuedPost
	// dropped records a post given up on and why, so the reason outlives the
	// transient footer notice that first reports it.
	dropped string
	author  string
	scroll  int
	// loading is a refresh begun and not yet answered.
	loading bool
}

// droppedTimeFormat stamps a dropped post with the time it was given up on.
const droppedTimeFormat = "15:04"

// queuedPost is a message waiting for one pull request's CI to pass. It names
// the pull request it was written for, because the one on screen can change
// while it waits, and a message is only ever sent for the one its writer saw.
type queuedPost struct {
	pull int
	post outgoingAnnouncement
}

// waiting reports a post that has not been sent or given up on.
func (q queuedPost) waiting() bool {
	return q.post.text != ""
}

// quitGuard asks before quitting while a post is waiting for CI, which quitting
// would silently lose.
type quitGuard struct{}

var _ overlay = quitGuard{}

// view says what quitting would cost.
func (quitGuard) view(_ renderKit, width, _ int) (string, string) {
	return "Quit", wrap("An announcement is waiting for CI and will be lost.", width)
}

// footer offers quitting anyway or staying.
func (quitGuard) footer(keys keyMap) []key.Binding {
	return []key.Binding{relabel(keys.confirm, "quit"), relabel(keys.closeOverlay, escStay)}
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

// loadAuthor is the command that asks the forge whose credential it holds:
// you, as the announcement and a forge issue's assignee name you.
func (m Model) loadAuthor() tea.Cmd {
	author := m.deps.Forge.Author
	if author == nil || m.messaging.author != "" {
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

var _ applier = authorFound{}

// apply records the name. Not knowing it only makes the announcement say less.
func (msg authorFound) apply(m Model) (Model, tea.Cmd) {
	if msg.err == nil {
		m.messaging.author = msg.name
	}

	return m, nil
}

// announcement is the message marking the pull request's current moment, told
// from what the interface already holds: the author it read, the pull request
// on screen, and the listed issue.
func (m Model) announcement(moment messaging.Moment) string {
	issueKey, _ := m.branch.issue(m.cfg.Jira.Project)
	issue, _ := m.issues.find(issueKey)

	return loop.Announcement(loop.AnnouncementFacts{
		Author: m.messaging.author, Pull: m.review.pull, IssueKey: issueKey,
		IssueSummary: issue.Summary, IssueURL: m.browseURL(issueKey), Moment: moment,
	}, m.cfg.Messaging, m.deps.Forge.Kind).Text()
}

// messagingRail is where messages go and what has been posted.
func (m Model) messagingRail(_ int) string {
	return sanitize.Line(m.cfg.Messaging.Target()) + "\n" + m.messagingState()
}

// messagingState says what has been announced this session.
func (m Model) messagingState() string {
	switch {
	case m.messaging.send.sending:
		return m.marks.inFlight + " announcing" + m.marks.ellipsis
	case m.messaging.send.err != nil:
		return m.kit().failureSummary(m.messaging.send.err)
	case m.announced():
		return m.marks.done + " announced"
	case m.messaging.pending.waiting():
		return m.marks.inFlight + " announces when CI passes"
	case m.messaging.dropped != "":
		return m.kit().failedGlyph() + " not announced: " + m.messaging.dropped
	default:
		return m.marks.notStarted + " nothing announced"
	}
}

// messagingDetail previews the announcement, or says what it needs first.
func (m Model) messagingDetail(width int) string {
	if m.cfg.Messaging.Mode() == config.MessagingNone {
		return wrap(m.marks.notStarted+" "+inFull(messaging.ErrNoCredential)+".\n\n`workflow doctor` checks "+
			config.FileName+"; `--online` also asks Slack about your user token.", width)
	}

	if !m.review.found {
		reviewPane := paneReview.label(m.cfg.Messaging.Service())

		return wrap("Open a "+m.vocab.noun+" first ("+reviewPane+"); the message links to it.\n\n"+
			m.messagingRail(0), width)
	}

	lines := []string{
		m.announcement(loop.AnnounceMoment(m.review.pull, m.review.ci)),
		"",
		m.styles.label.Render("to     ") + sanitize.Line(m.cfg.Messaging.Target()),
		m.styles.label.Render("CI     ") + m.ciSummary(),
		m.styles.label.Render("state  ") + m.messagingState(),
	}

	if m.messaging.send.err != nil {
		lines = append(lines, "", m.kit().failureBlock(m.messaging.send.err, width))
	}

	return wrap(strings.Join(lines, "\n"), width)
}

// announced reports that the pull request on screen was already posted at its
// current moment — a merge announced counts, an opening does not — this session
// or, from the store, an earlier one.
func (m Model) announced() bool {
	current := loop.Announced{Pull: m.review.pull.Number, Moment: loop.AnnounceMoment(m.review.pull, m.review.ci)}

	return m.review.found && slices.Contains(m.messaging.posted, current)
}

// refreshMessaging reads what was announced again, and the pull request and
// its CI the announcement is written from.
func (m Model) refreshMessaging() (Model, tea.Cmd) {
	read := m.loadAnnounces()
	m.messaging.loading = read != nil

	return m, tea.Batch(read, m.loadBranch())
}

// loadAnnounces reads what was announced in an earlier session from the store, so
// a pull request already posted opens as posted rather than being offered again.
func (m Model) loadAnnounces() tea.Cmd {
	if m.deps.Store.Announced == nil {
		return nil
	}

	read := m.deps.Store.Announced

	return func() tea.Msg {
		return announcesLoaded{made: read()}
	}
}

// announcesLoaded carries what the store remembers being posted.
type announcesLoaded struct {
	made []loop.Announced
}

var _ applier = announcesLoaded{}

// apply seeds the session's posted list from the store, so a restart does not
// forget what was announced and offer it again.
func (msg announcesLoaded) apply(m Model) (Model, tea.Cmd) {
	m.messaging.loading = false

	for _, remembered := range msg.made {
		if !slices.Contains(m.messaging.posted, remembered) {
			m.messaging.posted = append(m.messaging.posted, remembered)
		}
	}

	return m, nil
}

// canPost reports a pull request to announce, a way to post it, and no post
// of it already made or on its way.
func (m Model) canPost() bool {
	return m.review.found && m.deps.Messaging.Post != nil && m.cfg.Messaging.Mode() != config.MessagingNone &&
		!m.announced() && !m.messaging.send.sending
}

// messagingKeys offers composing the post, and managing whom posts tag.
func (m Model) messagingKeys() []key.Binding {
	var keys []key.Binding

	if m.canPost() {
		keys = append(keys, m.keys.compose)
	}

	if m.managesPeople() {
		keys = append(keys, m.keys.peopleAndGroups)
	}

	return append(keys, m.keys.refresh)
}

// handleMessagingKey answers the messaging pane's own keys.
func (m Model) handleMessagingKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.refresh):
		return m.refreshPane(paneMessaging)
	case key.Matches(msg, m.keys.peopleAndGroups) && m.managesPeople():
		return m.openPeople()
	case key.Matches(msg, m.keys.compose) && m.canPost():
		return m.previewAnnouncement()
	default:
		return m, nil
	}
}

// previewAnnouncement opens the preview of the post marking the pull
// request's current moment, and starts reading whom it tags.
func (m Model) previewAnnouncement() (Model, tea.Cmd) {
	channels, channel := m.cfg.Messaging.ChannelChoices(), m.defaultChannel()
	moment := loop.AnnounceMoment(m.review.pull, m.review.ci)
	m, opened := m.opening()

	preview, readTags := m.withTagging(messagingPreview{
		text: m.announcement(moment), fallback: m.cfg.Messaging.Target(),
		channel: channel, channels: channels, moment: moment, service: m.cfg.Messaging.Service(),
		noCI: m.review.checked && m.review.ci.State == forge.CINone, opened: opened,
	})
	m.overlay = preview

	return m, readTags
}

// defaultChannel is the channel a post goes to unless another is chosen: the
// first a Slack user token can post to, or none for a webhook, which carries
// its own.
func (m Model) defaultChannel() string {
	channels := m.cfg.Messaging.ChannelChoices()
	if len(channels) == 0 {
		return ""
	}

	return channels[0]
}

// withoutQueuedPost gives up on a post waiting for CI, saying so, because the
// pull request it was written for is no longer the one on screen.
func (m Model) withoutQueuedPost() Model {
	if !m.messaging.pending.waiting() {
		return m
	}

	dropped := m.messaging.pending.pull
	m.messaging.pending = queuedPost{}

	return m.noticedFailure(fmt.Errorf("%w to %s waiting for %s%d, which is no longer this branch's %s",
		errAnnouncementDropped, m.cfg.Messaging.Service(), m.vocab.sigil, dropped, m.vocab.noun))
}

// postIfGreen posts the message waiting for CI once CI passes, and gives up on
// it, saying so, if CI fails or the pull request is another one.
func (m Model) postIfGreen() (Model, tea.Cmd) {
	if !m.messaging.pending.waiting() {
		return m, nil
	}

	if m.messaging.pending.pull != m.review.pull.Number {
		return m.withoutQueuedPost(), nil
	}

	// A map, not a switch, so there is no last-case arm gobco can never see;
	// exhaustive keeps it complete.
	settle := map[forge.CIState]func() (Model, tea.Cmd){
		forge.CIPassed:  m.postQueued,
		forge.CIFailed:  m.dropQueued,
		forge.CINone:    m.keepQueued,
		forge.CIRunning: m.keepQueued,
	}[m.review.ci.State]

	return settle()
}

// postQueued sends the post that waited for CI.
func (m Model) postQueued() (Model, tea.Cmd) {
	return m.sendToMessaging(m.messaging.pending.post)
}

// dropQueued gives up on the post that waited for CI, which failed, and says
// so.
func (m Model) dropQueued() (Model, tea.Cmd) {
	m.messaging.pending = queuedPost{}
	m.messaging.dropped = "CI failed at " + m.deps.now().Format(droppedTimeFormat)

	return m.noticedFailure(fmt.Errorf("%w to %s", errCIFailedUnannounced, m.cfg.Messaging.Service())), nil
}

// keepQueued leaves the post waiting while CI has not finished.
func (m Model) keepQueued() (Model, tea.Cmd) {
	return m, nil
}

// messagingBehavior is the messaging pane's behavior.
func messagingBehavior() behavior {
	return behavior{
		rail: Model.messagingRail, detail: Model.messagingDetail, narrow: nil,
		keys: Model.messagingKeys, handle: Model.handleMessagingKey, pick: nil,
		refresh: Model.refreshMessaging, loading: func(m Model) bool { return m.messaging.loading },
		scroll: func(m *Model) *int { return &m.messaging.scroll }, readsBranch: true,
		answers: []string{"post", "people-and-groups", actionRefresh},
	}
}
