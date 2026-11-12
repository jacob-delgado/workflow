// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/jacob-delgado/workflow/internal/armed"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/loop"
	"github.com/jacob-delgado/workflow/internal/messaging"
	"github.com/jacob-delgado/workflow/internal/progress"
	"github.com/jacob-delgado/workflow/internal/sanitize"
)

// claimedFormat is when a post whose claim was never confirmed began, as the
// rail says it.
const claimedFormat = "Jan 2 15:04"

// messagingState is what has been announced, the announcement waiting for CI,
// and how far the messaging pane's detail is scrolled.
type messagingState struct {
	// claims is every announcement the store holds a claim on, by any
	// session, each a pull request and the moment it marked, so one pull
	// request can be announced at each of its moments — opened, then merged —
	// without a moment being offered twice. It is replaced at each read of the
	// store, never added to, so what the store forgot is offered again.
	claims []loop.AnnounceRecord
	// send is a post the service has not answered yet, which is not offered
	// again until it does, or why the last one failed.
	send sendState
	// hold is the announcement armed to go once its pull request's CI passes,
	// holdTags whom its preview chose to tag, and tick the read of it
	// scheduled last.
	hold     armed.Hold
	holdTags postTags
	tick     int
	author   string
	scroll   int
	// loading is a refresh begun and not yet answered.
	loading bool
}

// loadAuthor is the command that asks the forge whose credential it holds:
// you, as the announcement and a forge issue's assignee name you.
func (s messagingState) loadAuthor(ctx context.Context, deps Deps) tea.Cmd {
	author := deps.Forge.Author
	if author == nil || s.author != "" {
		return nil
	}

	return func() tea.Msg {
		name, err := author(ctx)

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
		IssueSummary: issue.Summary, IssueURL: browseURL(m.deps, issueKey), Moment: moment,
	}, m.cfg.Messaging, m.deps.Forge.Kind).Text()
}

// messagingRail is where messages go and how the announcement stands.
func (m Model) messagingRail() string {
	return sanitize.Line(m.cfg.Messaging.Target()) + "\n" + m.announceStanding()
}

// announceStanding says how the announcement of the pull request on screen
// stands: on its way, failed, announced or begun and never confirmed, by the
// store's claims; else how the announcement held for CI stands.
func (m Model) announceStanding() string {
	kit, sent := m.kit(), m.messaging.send

	switch {
	case m.messaging.onItsWay():
		return kit.marks.inFlight + " announcing" + kit.marks.ellipsis
	case m.messaging.hold.Stage().Waiting() && m.onScreen(m.messaging.hold.Pull()):
		return m.waitingLine()
	case sent.err != nil:
		return kit.failureSummary(sent.err)
	}

	// A map, not a switch, so there is no last-case arm gobco can never see.
	return map[loop.Standing]func() string{
		loop.StandingPosted:      func() string { return kit.marks.done + " announced" },
		loop.StandingUnconfirmed: m.unconfirmedLine,
		loop.StandingNone:        m.holdLine,
	}[loop.StandingOf(m.messaging.claims, m.review.pull, m.review.found)]()
}

// unconfirmedLine says that the post of the pull request on screen began and
// was never confirmed, so it may have gone out: only workflow announce,
// asking first, posts it again.
func (m Model) unconfirmedLine() string {
	record, _ := loop.RecordOf(m.messaging.claims,
		loop.Announced{Pull: m.review.pull.Number, Moment: loop.AnnounceMoment(m.review.pull)})

	began := record.ClaimedAt.In(m.deps.now().Location()).Format(claimedFormat)

	return m.kit().failedGlyph() + " a post began at " + began + " and was not confirmed; workflow announce posts it again"
}

// holdLine says how the announcement held for CI stands: waiting, on what,
// whichever pull request it is for; and, for the pull request on screen,
// posted, posted elsewhere or given up on. Any other stands as nothing
// announced.
func (m Model) holdLine() string {
	hold, kit := m.messaging.hold, m.kit()
	nothing := func() string { return kit.marks.notStarted + " nothing announced" }

	if hold.Stage().Waiting() {
		return m.waitingLine()
	}

	if !m.onScreen(hold.Pull()) {
		return nothing()
	}

	// A map, not a switch, so there is no arm for the stages handled above.
	return map[armed.Stage]func() string{
		armed.StagePosted:    func() string { return kit.marks.done + " announced" },
		armed.StageElsewhere: func() string { return kit.marks.done + " announced elsewhere" },
		armed.StageDropped: func() string {
			return kit.failedGlyph() + " not announced: " + m.droppedWords(hold)
		},
		armed.StageIdle: nothing, armed.StageStopped: nothing, armed.StageListing: nothing,
		armed.StageAwaitingChecks: nothing, armed.StageRunning: nothing, armed.StageAwaitingFix: nothing,
		armed.StageUnheard: nothing, armed.StageDue: nothing,
	}[hold.Stage()]()
}

// waitingLine says that the held announcement waits for CI, naming its pull
// request where it is not the one on screen, and what it waits on: the forge
// to list it, checks to be reported, a fix for one that failed, or a read or
// post to try again; nothing more while CI runs.
func (m Model) waitingLine() string {
	hold, kit := m.messaging.hold, m.kit()

	which := ""
	if !m.onScreen(hold.Pull()) {
		which = " " + m.pullName(hold.Pull())
	}

	none := func() string { return "" }
	// A map, not a switch, so there is no arm for the stages that do not wait.
	waitingOn := map[armed.Stage]func() string{
		armed.StageListing: func() string {
			return kit.marks.separator + "waiting for the forge to list " + m.pullName(hold.Pull())
		},
		armed.StageAwaitingChecks: func() string { return kit.marks.separator + "no checks reported yet" },
		armed.StageAwaitingFix:    func() string { return kit.marks.separator + "a check failed; waiting on a fix" },
		armed.StageUnheard: func() string {
			return kit.marks.separator + briefly(hold.Reason()) + "; trying again"
		},
		armed.StageRunning: none, armed.StageIdle: none, armed.StageDue: none, armed.StagePosted: none,
		armed.StageElsewhere: none, armed.StageDropped: none, armed.StageStopped: none,
	}[hold.Stage()]()

	return kit.marks.inFlight + " announces" + which + " when CI passes" + waitingOn
}

// messagingDetail previews the announcement, or says what it needs first: a
// pull request the loop follows, which one closed without merging is not.
func (m Model) messagingDetail(width int) string {
	if m.cfg.Messaging.Mode() == config.MessagingNone {
		return wrap(m.marks.notStarted+" "+inFull(messaging.ErrNoCredential)+".\n\n`workflow doctor` checks "+
			config.FileName+"; `--online` also asks Slack about your user token.", width)
	}

	if m.review.pullState() == progress.NoPullRequest {
		reviewPane := paneReview.label(m.cfg.Messaging.Service())

		return wrap("Open a "+m.vocab.noun+" first ("+reviewPane+"); the message links to it.\n\n"+
			m.messagingRail(), width)
	}

	lines := []string{
		m.announcement(loop.AnnounceMoment(m.review.pull)),
		"",
		m.styles.label.Render("to     ") + sanitize.Line(m.cfg.Messaging.Target()),
		m.styles.label.Render("CI     ") + m.review.ciSummary(m.kit()),
		m.styles.label.Render("state  ") + m.announceStanding(),
	}

	if m.messaging.send.err != nil {
		lines = append(lines, "", m.kit().failureBlock(m.messaging.send.err, width))
	}

	return wrap(strings.Join(lines, "\n"), width)
}

// announced reports that the pull request on screen was claimed at its
// current moment — a merge announced counts, an opening does not — by any
// session, as the store last said, by the rule every surface reads it by:
// posted, or begun and never confirmed, which may have gone out. What it
// holds was read already and is in memory, so nothing is asked and no
// context could stop it.
func (s messagingState) announced(review reviewState) bool {
	return loop.StandingOf(s.claims, review.pull, review.found) != loop.StandingNone
}

// onScreen reports that the pull request numbered number is the one on
// screen: the branch's, found.
func (m Model) onScreen(number int) bool {
	return m.review.found && m.review.pull.Number == number
}

// claimed reports that made was claimed, as the store last said.
func (s messagingState) claimed(made loop.Announced) bool {
	_, known := loop.RecordOf(s.claims, made)

	return known
}

// loadAnnounces reads what every session announced from the store, so a pull
// request already posted is not offered again. The store is read under ctx.
func loadAnnounces(ctx context.Context, deps Deps) tea.Cmd {
	if deps.Store.Announced == nil {
		return nil
	}

	read := deps.Store.Announced

	return func() tea.Msg {
		made, err := read(ctx)

		return announcesLoaded{made: made, err: err}
	}
}

// announcesLoaded carries what the store holds a claim on, or why it could
// not be read.
type announcesLoaded struct {
	made []loop.AnnounceRecord
	err  error
}

var _ applier = announcesLoaded{}

// apply replaces what the session knows was announced with what the store
// says, so what it forgot is offered again. A store that could not be read
// says so, and what was read before stands: the claim each post takes still
// keeps one from going twice.
func (msg announcesLoaded) apply(m Model) (Model, tea.Cmd) {
	m.messaging.loading = false

	if msg.err != nil {
		return m.noticedFailure(fmt.Errorf("%w: %w", errAnnouncedUnread, msg.err)), nil
	}

	m.messaging.claims = msg.made

	return m, nil
}

// errAnnouncedUnread is a store whose record of what was announced could
// not be read.
var errAnnouncedUnread = errors.New("could not read what was announced")

// canPost reports a pull request to announce, a way to post it, and no post
// of it already made or on its way.
func (m Model) canPost() bool {
	return m.review.pullState() != progress.NoPullRequest && m.deps.Messaging.Post != nil &&
		m.cfg.Messaging.Mode() != config.MessagingNone && !m.messaging.announced(m.review) &&
		!m.messaging.onItsWay()
}

// onItsWay reports a post the service has not answered yet: one asked for
// now, or the announcement that waited for CI.
func (s messagingState) onItsWay() bool {
	return s.send.sending || s.hold.Stage() == armed.StageDue
}

// messagingKeys offers composing the post, and managing whom posts tag.
// While an announcement waits for CI, stopping it is offered, and, for the
// pull request on screen, composing follows as announce now instead, since
// the post it opens goes in the waiting one's place.
func (m Model) messagingKeys() []key.Binding {
	var keys []key.Binding

	hold := m.messaging.hold
	waiting := hold.Stage().Waiting()
	instead := waiting && m.onScreen(hold.Pull())

	if m.canPost() && !instead {
		keys = append(keys, m.keys.compose)
	}

	if managesPeople(m.cfg, m.deps) {
		keys = append(keys, m.keys.peopleAndGroups)
	}

	if waiting {
		keys = append(keys, m.keys.stopWaiting)
	}

	if instead && m.canPost() {
		keys = append(keys, relabel(m.keys.compose, "announce now instead"))
	}

	return append(keys, m.keys.refresh)
}

// handleMessagingKey answers the messaging pane's own keys.
func (m Model) handleMessagingKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.refresh):
		return m.refreshPane(paneMessaging)
	case key.Matches(msg, m.keys.peopleAndGroups) && managesPeople(m.cfg, m.deps):
		return m.openPeople()
	case key.Matches(msg, m.keys.compose) && m.canPost():
		return m.previewAnnouncement()
	case key.Matches(msg, m.keys.stopWaiting):
		return m.stopWaiting()
	default:
		return m, nil
	}
}

// previewAnnouncement opens the preview of the post marking the pull
// request's current moment, and starts reading whom it tags.
func (m Model) previewAnnouncement() (Model, tea.Cmd) {
	channels, channel := m.cfg.Messaging.ChannelChoices(), defaultChannel(m.cfg)
	moment := loop.AnnounceMoment(m.review.pull)
	m, opened := m.opening()

	preview, readTags := m.withTagging(messagingPreview{
		text: m.announcement(moment), fallback: m.cfg.Messaging.Target(),
		channel: channel, channels: channels, moment: moment, service: m.cfg.Messaging.Service(),
		opened: opened,
	})
	m.overlay = preview

	return m, readTags
}

// defaultChannel is the channel a post goes to unless another is chosen: the
// first a Slack user token can post to, or none for a webhook, which carries
// its own.
func defaultChannel(cfg config.Config) string {
	channels := cfg.Messaging.ChannelChoices()
	if len(channels) == 0 {
		return ""
	}

	return channels[0]
}

// messagingBehavior is the messaging pane's behavior.
func messagingBehavior() behavior {
	return behavior{
		rail:   func(m Model, _ int) string { return m.messagingRail() },
		detail: Model.messagingDetail, narrow: nil,
		keys: Model.messagingKeys, handle: Model.handleMessagingKey, pick: nil,
		// refresh reads what was announced again, the pull request and its CI
		// the announcement is written from, and the one held for CI.
		refresh: func(m Model) (Model, tea.Cmd) {
			read := loadAnnounces(m.session, m.deps)
			m.messaging.loading = read != nil

			return m, tea.Batch(read, loadBranch(m.session, m.deps), m.rereadHold())
		},
		loading: func(m Model) bool { return m.messaging.loading },
		scroll:  func(m *Model) *int { return &m.messaging.scroll }, readsBranch: true,
		answers: []string{"post", "people-and-groups", "stop-waiting", actionRefresh},
	}
}
