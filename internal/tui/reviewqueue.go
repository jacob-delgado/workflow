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
// the queue the Reviews pane shows, as far as it has loaded, and how far that
// pane's detail is scrolled.
type reviewQueueState struct {
	// all is the queue as the forge answered it; requests is what the pane
	// lists of it, narrowed to the picked facets and in order.
	all      []forge.ReviewRequest
	order    reviewOrder
	facets   []forge.ReviewFacet
	requests []forge.ReviewRequest
	loaded   bool
	err      error
	selected int
	scroll   int
	// loading is a refresh begun and not yet answered.
	loading bool
}

// reviewsLoaded carries the forge's answer about the review queue.
type reviewsLoaded struct {
	requests []forge.ReviewRequest
	err      error
}

var _ applier = reviewsLoaded{}

// apply records the queue in the order and under the filter chosen, keeping
// the selection on the same request across a refresh.
func (msg reviewsLoaded) apply(m Model) (Model, tea.Cmd) {
	previous, _ := m.reviewQueue.current()

	m.reviewQueue = reviewQueueState{
		all: msg.requests, order: m.reviewQueue.order, facets: m.reviewQueue.facets, loaded: true, err: msg.err,
	}
	m.reviewQueue = m.reviewQueue.listed(previous, m.detailRows())

	return m, nil
}

// listed is the queue listed again, narrowed and in its order, with the
// selection held on previous where it is still listed.
func (s reviewQueueState) listed(previous forge.ReviewRequest, rows int) reviewQueueState {
	s.requests = s.order.sorted(filteredReviews(s.facets, s.all))
	s.selected = s.indexOf(previous)

	return s.following(rows)
}

// refreshReviewQueue reads the review queue again.
func (m Model) refreshReviewQueue() (Model, tea.Cmd) {
	read := m.loadReviewQueue()
	m.reviewQueue.loading = read != nil

	return m, read
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

// following is the queue scrolled so its selection shows in rows lines.
func (s reviewQueueState) following(rows int) reviewQueueState {
	lines := s.lineRequests()
	s.scroll, _ = window(slices.Index(lines, s.selected), len(lines), rows)

	return s
}

// lineRequests is which request each line of the queue's listing draws: -1
// for the line naming its order and filters, the blank under it, a
// repository's heading, and what says no request matches the filters.
func (s reviewQueueState) lineRequests() []int {
	var lines []int
	if s.headed() {
		lines = append(lines, -1, -1)
	}

	if len(s.requests) == 0 {
		return append(lines, -1)
	}

	for index, request := range s.requests {
		if s.order == orderRepository && (index == 0 || s.requests[index-1].Repository != request.Repository) {
			lines = append(lines, -1)
		}

		lines = append(lines, index)
	}

	return lines
}

// headed reports whether the listing opens with a line naming its order and
// filters: whenever either is not the usual.
func (s reviewQueueState) headed() bool {
	return s.order != orderOldest || len(s.facets) > 0
}

// heading names the order the queue is in and the filters narrowing it.
func (s reviewQueueState) heading(marks glyphs) string {
	if len(s.facets) == 0 {
		return s.order.title()
	}

	return s.order.title() + marks.separator + facetsLine(s.facets)
}

// current is the selected review request, if there is one.
func (s reviewQueueState) current() (forge.ReviewRequest, bool) {
	if s.selected >= len(s.requests) {
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
		return m.marks.reading()
	case m.reviewQueue.err != nil:
		return m.failureSummary(m.reviewQueue.err)
	case len(m.reviewQueue.all) == 0:
		return "none waiting on you"
	}

	return strconv.Itoa(len(m.reviewQueue.all)) + " waiting"
}

// reviewQueueDetail lists the queue, oldest-first, one request to a line.
func (m Model) reviewQueueDetail(width int) string {
	switch {
	case m.deps.Forge.ReviewRequests == nil:
		return wrap("This forge does not list the "+m.vocab.noun+"s waiting on your review.", width)
	case !m.reviewQueue.loaded:
		return m.marks.reading()
	case m.reviewQueue.err != nil:
		return m.failureBlock(m.reviewQueue.err, width)
	case len(m.reviewQueue.all) == 0:
		return "No " + m.vocab.noun + "s are waiting on your review."
	}

	return strings.Join(m.reviewLines(), "\n")
}

// reviewLines draws the queue line by line, as lineRequests maps them: the
// order it is in, when it is not the usual, and each repository's heading when
// grouped by repository.
func (m Model) reviewLines() []string {
	rows, lines := m.reviewRows(), m.reviewQueue.lineRequests()
	drawn := make([]string, 0, len(lines))

	for line, index := range lines {
		drawn = append(drawn, m.reviewLine(rows, lines, line, index))
	}

	return drawn
}

// reviewLine is the line numbered line: a request's row; the order and
// filters, and the blank under them, which are the only lines lineRequests
// opens with that draw no request; what says nothing matches the filters; or
// the heading of the repository the next row is in.
func (m Model) reviewLine(rows []string, lines []int, line, index int) string {
	switch {
	case index >= 0:
		return rows[index]
	case line == 0:
		return m.styles.label.Render(m.reviewQueue.heading(m.marks))
	case line == 1:
		return ""
	case len(rows) == 0:
		return "No review request matches the filters."
	default:
		return m.styles.strong.Render(m.reviewQueue.repositoryAfter(lines[line+1]))
	}
}

// reviewRows draws each queued request: how its CI stands, its number and
// title, then a faint tail of where it is, who wants it and how long it has
// waited. The forge client has already neutralized every value.
func (m Model) reviewRows() []string {
	now := m.deps.now()
	rows := make([]string, 0, len(m.reviewQueue.requests))

	for index, request := range m.reviewQueue.requests {
		head := m.marks.marker(index == m.reviewQueue.selected) + m.ciStateGlyph(request.CI) +
			" " + m.vocab.sigil + strconv.Itoa(request.Number) + " " + request.Title

		rows = append(rows, head+"  "+m.styles.label.Render(m.reviewTail(request, now)))
	}

	return rows
}

// reviewTail is the faint metadata after a queued request's title: its
// repository, who wants the review, how long it has waited, and whether it is
// a draft, as the Review pane labels the branch's own.
func (m Model) reviewTail(request forge.ReviewRequest, now time.Time) string {
	tail := "by " + request.Author + m.marks.separator + age(now, request.OpenedAt)
	if request.Draft {
		tail += m.marks.separator + "draft"
	}

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
	if m.reviewQueue.sortable() {
		keys = append(keys, m.keys.sortReviews)
	}

	if m.reviewQueue.narrowable() {
		keys = append(keys, m.keys.filterReviews)
	}

	return append(keys, m.keys.refresh)
}

// sortable reports a queue read back, which an order chosen now holds for.
func (s reviewQueueState) sortable() bool {
	return s.loaded && s.err == nil
}

// narrowable reports a queue read back holding requests to narrow, even when
// the facets already picked leave none of them listed.
func (s reviewQueueState) narrowable() bool {
	return s.sortable() && len(s.all) > 0
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
	case key.Matches(msg, m.keys.sortReviews) && m.reviewQueue.sortable():
		previous, _ := m.reviewQueue.current()
		m.reviewQueue.order = m.reviewQueue.order.next()
		m.reviewQueue = m.reviewQueue.listed(previous, m.detailRows())

		return m, nil
	case key.Matches(msg, m.keys.filterReviews) && m.reviewQueue.narrowable():
		return m.openFacetPicker()
	case key.Matches(msg, m.keys.up, m.keys.down):
		return m.moveReviewBy(m.keys.stepOf(msg)), nil
	case key.Matches(msg, m.keys.openLink):
		return m.openLink(m.selectedReviewURL())
	case key.Matches(msg, m.keys.copyLink):
		return m.copyLink(m.selectedReviewURL())
	case key.Matches(msg, m.keys.refresh):
		return m.refreshPane(paneReviews)
	}

	return m, nil
}

// moveReviewBy moves the selection delta requests down, or up for a negative
// delta, stopping at either end, and scrolls the detail so the selected
// request stays on screen.
func (m Model) moveReviewBy(delta int) Model {
	m.reviewQueue.selected = max(0, min(m.reviewQueue.selected+delta, len(m.reviewQueue.requests)-1))

	m.reviewQueue = m.reviewQueue.following(m.detailRows())

	return m
}

// pickReview selects the request on a clicked line of the detail.
func (m Model) pickReview(line, _ int, inRail bool) (Model, tea.Cmd) {
	clicked, drawn := m.detailLineAt(line)
	lines := m.reviewQueue.lineRequests()

	if inRail || !drawn || clicked >= len(lines) || lines[clicked] < 0 {
		return m, nil
	}

	m.reviewQueue.selected = lines[clicked]

	return m, nil
}

// reviewOrder is the order the Reviews pane lists the queue in.
type reviewOrder int

const (
	orderOldest reviewOrder = iota
	orderNewest
	orderRepository
	// orderCount is how many orders there are to cycle through.
	orderCount
)

// next is the order after this one, round to the first.
func (o reviewOrder) next() reviewOrder {
	return (o + 1) % orderCount
}

// title says the order, above a queue listed in it.
func (o reviewOrder) title() string {
	return [orderCount]string{"oldest first", "newest first", "by repository"}[o]
}

// sorted is the queue in this order.
func (o reviewOrder) sorted(requests []forge.ReviewRequest) []forge.ReviewRequest {
	return [orderCount]func([]forge.ReviewRequest) []forge.ReviewRequest{
		forge.OldestFirst, forge.NewestFirst, forge.ByRepository,
	}[o](requests)
}

// repositoryAfter is the repository of the request a heading heads: the one
// on the line below it, since lineRequests heads a repository only above its
// first request.
func (s reviewQueueState) repositoryAfter(index int) string {
	if name := s.requests[index].Repository; name != "" {
		return name
	}

	return "no repository"
}
