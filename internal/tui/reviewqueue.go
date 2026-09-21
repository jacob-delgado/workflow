// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"slices"
	"strconv"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/jacob-delgado/workflow/internal/forge"
)

// reviewQueueState is the pull requests on the forge that ask for your review —
// the queue the Reviews pane shows, as far as it has loaded.
type reviewQueueState struct {
	requests []forge.ReviewRequest
	loaded   bool
	err      error
	selected int
}

// reviewsLoaded carries the forge's answer about the review queue.
type reviewsLoaded struct {
	requests []forge.ReviewRequest
	err      error
}

var _ applier = reviewsLoaded{}

// apply records the queue oldest-first — the order it is worked through —
// keeping the selection on the same request across a refresh.
func (msg reviewsLoaded) apply(m Model) (Model, tea.Cmd) {
	previous, _ := m.reviewQueue.current()

	requests := slices.Clone(msg.requests)
	slices.SortStableFunc(requests, func(left, right forge.ReviewRequest) int {
		return left.OpenedAt.Compare(right.OpenedAt)
	})

	m.reviewQueue = reviewQueueState{requests: requests, loaded: true, err: msg.err}
	m.reviewQueue.selected = m.reviewQueue.indexOf(previous)

	// A refresh can return a shorter queue, leaving the scroll offset past the
	// end; re-clamp it so a click still lands on the row it appears to. Only when
	// this pane is the focused one, since the offset is shared with the others and
	// this load may arrive while another pane is being read.
	if m.focus == paneReviews {
		m.scroll, _ = window(m.reviewQueue.selected, len(m.reviewQueue.requests), m.detailRows())
	}

	return m, nil
}

// loadReviewQueue is the command that reads the review queue from the forge.
func (m Model) loadReviewQueue() tea.Cmd {
	list := m.deps.Forge.ReviewRequests
	if list == nil {
		return nil
	}

	return func() tea.Msg {
		requests, err := list()

		return reviewsLoaded{requests: requests, err: err}
	}
}

// current is the selected review request, if there is one.
func (s reviewQueueState) current() (forge.ReviewRequest, bool) {
	if s.selected < 0 || s.selected >= len(s.requests) {
		return forge.ReviewRequest{}, false
	}

	return s.requests[s.selected], true
}

// indexOf is the row of the request matching one already selected, clamped to
// the list, so a refresh keeps the cursor on the same pull request rather than
// on whatever now sits at its old row.
func (s reviewQueueState) indexOf(want forge.ReviewRequest) int {
	index := slices.IndexFunc(s.requests, func(candidate forge.ReviewRequest) bool {
		return candidate.URL == want.URL
	})

	return max(0, min(index, len(s.requests)-1))
}

// reviewQueueRail summarizes the queue: how many pull requests wait, or why the
// queue could not be read.
func (m Model) reviewQueueRail(_ int) string {
	switch {
	case m.deps.Forge.ReviewRequests == nil:
		return "no forge for reviews"
	case !m.reviewQueue.loaded:
		return "looking" + m.marks.ellipsis
	case m.reviewQueue.err != nil:
		return m.failedGlyph() + " " + forgeReason(m.reviewQueue.err)
	case len(m.reviewQueue.requests) == 0:
		return "none waiting on you"
	}

	return plural(len(m.reviewQueue.requests), "review request") + " waiting"
}

// reviewQueueDetail lists the queue, oldest-first, one request to a line.
func (m Model) reviewQueueDetail(width int) string {
	switch {
	case m.deps.Forge.ReviewRequests == nil:
		return wrap("This forge does not list the pull requests waiting on your review.", width)
	case !m.reviewQueue.loaded:
		return "looking" + m.marks.ellipsis
	case m.reviewQueue.err != nil:
		return m.failureWithin(m.reviewQueue.err, width)
	case len(m.reviewQueue.requests) == 0:
		return "No pull requests are waiting on your review."
	}

	return strings.Join(m.reviewRows(), "\n")
}

// reviewRows draws each queued request: how its CI stands, its number and
// title, then a faint tail of where it is, who wants it and how long it has
// waited. The forge client has already neutralized every value.
func (m Model) reviewRows() []string {
	now := m.deps.now()
	rows := make([]string, 0, len(m.reviewQueue.requests))

	for index, request := range m.reviewQueue.requests {
		head := m.marks.marker(index == m.reviewQueue.selected) + m.ciStateGlyph(request.CI) +
			" #" + strconv.Itoa(request.Number) + " " + request.Title

		rows = append(rows, head+"  "+m.styles.label.Render(m.reviewTail(request, now)))
	}

	return rows
}

// reviewTail is the faint metadata after a queued request's title: its
// repository, who wants the review, and how long it has waited.
func (m Model) reviewTail(request forge.ReviewRequest, now time.Time) string {
	tail := "by " + request.Author + m.marks.separator + age(now, request.OpenedAt)

	if request.Repository != "" {
		return request.Repository + m.marks.separator + tail
	}

	return tail
}

// ciStateGlyph is how a CI state looks, by shape, shared by the Review pane and
// the queue.
func (m Model) ciStateGlyph(state forge.CIState) string {
	return map[forge.CIState]string{
		forge.CINone: m.marks.unknown, forge.CIRunning: m.marks.inFlight,
		forge.CIPassed: m.marks.done, forge.CIFailed: m.failedGlyph(),
	}[state]
}

// reviewQueueKeys offers opening or copying the selected request, and refreshing
// while there is a forge to ask. Moving through the queue is a global affordance,
// shown in the help rather than the footer, as the other list panes have it.
func (m Model) reviewQueueKeys() []key.Binding {
	if m.deps.Forge.ReviewRequests == nil {
		return nil
	}

	keys := m.linkKeys(m.selectedReviewURL())

	return append(keys, m.keys.refresh)
}

// selectedReviewURL is the selected request's URL, or empty when none is.
func (m Model) selectedReviewURL() string {
	if request, ok := m.reviewQueue.current(); ok {
		return request.URL
	}

	return ""
}

// handleReviewQueueKey answers the Reviews pane's own keys.
func (m Model) handleReviewQueueKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.up, m.keys.down):
		return m.moveReviewSelection(msg), nil
	case key.Matches(msg, m.keys.openLink):
		return m.openLink(m.selectedReviewURL())
	case key.Matches(msg, m.keys.copyLink):
		return m.copyLink(m.selectedReviewURL())
	case key.Matches(msg, m.keys.refresh):
		return m, m.loadReviewQueue()
	}

	return m, nil
}

// moveReviewSelection moves the selection down or up, stopping at either end,
// and scrolls the detail so the selected request stays on screen.
func (m Model) moveReviewSelection(msg tea.KeyPressMsg) Model {
	last := len(m.reviewQueue.requests) - 1

	if key.Matches(msg, m.keys.down) {
		m.reviewQueue.selected = min(m.reviewQueue.selected+1, max(0, last))
	} else {
		m.reviewQueue.selected = max(0, m.reviewQueue.selected-1)
	}

	m.scroll, _ = window(m.reviewQueue.selected, len(m.reviewQueue.requests), m.detailRows())

	return m
}

// pickReview selects the request on a clicked line of the detail.
func (m Model) pickReview(line, _ int, inRail bool) (Model, tea.Cmd) {
	index := line + m.scroll
	if inRail || index < 0 || index >= len(m.reviewQueue.requests) {
		return m, nil
	}

	m.reviewQueue.selected = index

	return m, nil
}
