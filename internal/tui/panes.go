// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"strconv"
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
)

// pane is one panel in the rail.
type pane int

const (
	paneIssues pane = iota
	paneBranch
	paneCommits
	paneReview
	paneMessaging
	paneReviews
	paneTasks
	paneSummary
	paneRepositories
)

// paneFresh is how long after a pane began loading a switch to it leaves it as
// it is: long enough that flicking between panes spends no requests, short
// enough that a pane come back to after a while is current.
const paneFresh = 30 * time.Second

// paneCount is untyped on purpose: typed as pane, the exhaustive linter would
// count it as a member and demand a case for it in every switch.
const paneCount = 9

// title names a pane. A lookup rather than a switch, because a switch over every
// pane leaves a final arm that can never be false. The messaging pane is named
// for the service in use — Slack, Teams, Discord or Webhook.
func (p pane) title(messaging string) string {
	titles := [paneCount]string{
		"Issues", "Branch", "Commits", "Review", messaging, "Reviews", "Tasks", "Summary", "Repositories",
	}

	return titles[p]
}

// label is the title with the number that jumps to it.
func (p pane) label(messaging string) string {
	return strconv.Itoa(int(p)+1) + " " + p.title(messaging)
}

// ring is a position among size places that wraps around at either end: the
// focused pane, a composer's field or commit type, the calendar's column.
type ring[T ~int] struct {
	at, size T
}

// around is the ring of size places standing at at.
func around[T ~int](at, size T) ring[T] {
	return ring[T]{at: at, size: size}
}

// next is the place after the ring's, the first after the last.
func (r ring[T]) next() T {
	return (r.at + 1) % r.size
}

// prev is the place before the ring's, the last before the first.
func (r ring[T]) prev() T {
	return (r.at + r.size - 1) % r.size
}

// paneNumbers are the digit keys that jump to each pane, "1" through the last,
// derived from paneCount so a new pane is reachable without a second edit.
func paneNumbers() []string {
	numbers := make([]string, paneCount)
	for index := range numbers {
		numbers[index] = strconv.Itoa(index + 1)
	}

	return numbers
}

// behavior is what one pane shows and does. Each pane is one entry in a table
// rather than a case in a switch every new pane would have to edit.
type behavior struct {
	// rail is the pane's content in the rail, in as many rows as fit.
	rail func(m Model, rows int) string
	// detail is everything the pane has to say in the detail pane, scrolled
	// and clipped by whatever draws it.
	detail func(m Model, width int) string
	// narrow is what the pane shows when the rail is gone and the detail is
	// the whole screen; nil means its detail.
	narrow func(m Model, rows int) string
	// keys is what the pane offers in the footer, right now.
	keys func(m Model) []key.Binding
	// handle answers a key the rest of the interface did not claim.
	handle func(m Model, msg tea.KeyPressMsg) (Model, tea.Cmd)
	// refresh loads the pane again, for r and for a switch to a stale pane.
	refresh func(m Model) (Model, tea.Cmd)
	// loading reports a refresh, or the pane's first read, begun and not yet
	// answered, which the pane's title marks in flight.
	loading func(m Model) bool
	// move steps the cursor of the pane's list delta rows, down for a positive
	// delta, stopping at either end; nil for a pane with no list, whose ends
	// are its detail's.
	move func(m Model, delta int) (Model, tea.Cmd)
	// pick selects the row of the pane's list drawn on a line; nil for a pane
	// with no list to pick from. inRail says which drawing was clicked.
	pick func(m Model, line, rows int, inRail bool) (Model, tea.Cmd)
	// scroll is where the pane keeps how far its detail is scrolled, on its own
	// state, so leaving the pane and coming back finds it where it was.
	scroll func(m *Model) *int
	// listInDetail marks a pane whose selectable list lives in the detail rather
	// than the rail, so the heavy focus border belongs on the detail, where the
	// cursor is, not on the rail's summary.
	listInDetail bool
	// readsBranch marks a pane whose refresh reads the branch again, which
	// goes on to find its pull request and read CI: one such refresh leaves
	// every pane it feeds fresh.
	readsBranch bool
	// answers are the key actions handle answers, beside the moving and
	// everywhere keys every pane takes. With those they are the pane's key
	// context, in which CheckKeys refuses two actions bound to one key. handle
	// still sees every key, so the list never turns one off.
	answers []string
}

// behaviorOf is a pane's behavior, each defined beside the pane it is.
func behaviorOf(target pane) behavior {
	return [paneCount]func() behavior{
		paneIssues: issuesBehavior, paneBranch: branchBehavior, paneCommits: commitsBehavior,
		paneReview: reviewBehavior, paneMessaging: messagingBehavior, paneReviews: reviewQueueBehavior,
		paneTasks: tasksBehavior, paneSummary: summaryBehavior, paneRepositories: repositoriesBehavior,
	}[target]()
}

// keyContext is the keys live on the pane: the moving and everywhere keys
// every pane takes, and the actions its handler answers. It is named for the
// pane in a conflict CheckKeys reports, the messaging pane for no particular
// service, since the check runs before one is chosen.
func (p pane) keyContext() keyContext {
	return keyContext{
		name:    "the " + p.title("messaging") + " pane",
		groups:  []int{groupMoving, groupEverywhere},
		actions: behaviorOf(p).answers,
	}
}

// commandless is a list's move that asks for nothing once it has moved, as a
// pane's move, which may.
func commandless(move func(Model, int) Model) func(Model, int) (Model, tea.Cmd) {
	return func(m Model, delta int) (Model, tea.Cmd) { return move(m, delta), nil }
}

// refreshPane loads target again, noting when, whatever its age, for it and
// for every pane its load also refreshed.
func (m Model) refreshPane(target pane) (Model, tea.Cmd) {
	for _, loaded := range alsoRefreshed(target) {
		m.refreshed[loaded] = m.deps.now()
	}

	return behaviorOf(target).refresh(m)
}

// alsoRefreshed is target and the panes a refresh of it loads with it: every
// pane fed by the branch read, when target reads the branch.
func alsoRefreshed(target pane) []pane {
	if !behaviorOf(target).readsBranch {
		return []pane{target}
	}

	var fed []pane

	for candidate := range pane(paneCount) {
		if behaviorOf(candidate).readsBranch {
			fed = append(fed, candidate)
		}
	}

	return fed
}

// switchTo moves focus to target, as a key or a click asks, loading it again
// when it is stale: it began loading more than paneFresh ago, and is not the
// Issues list holding further pages, which a reload of the first would drop.
func (m Model) switchTo(target pane) (Model, tea.Cmd) {
	if target == m.focus {
		return m, nil
	}

	m = m.focusOn(target)

	fresh := m.deps.now().Sub(m.refreshed[target]) < paneFresh
	if fresh || (target == paneIssues && m.issues.paged) {
		return m, nil
	}

	return m.refreshPane(target)
}

// offer is one of a pane's keys: the binding its footer names, whether it acts
// right now, what it does when it does, and, for a key that explains itself,
// why it cannot when it cannot. A pane lists its offers once, and its footer
// and its key handler both read that list, so a key the footer leaves out
// never acts.
type offer struct {
	binding key.Binding
	can     bool
	act     func() (Model, tea.Cmd)
	refusal error
}

// liveKeys is the bindings of the offers that act right now, in order, for a
// footer.
func liveKeys(offers []offer) []key.Binding {
	var keys []key.Binding

	for _, each := range offers {
		if each.can {
			keys = append(keys, each.binding)
		}
	}

	return keys
}

// answer acts on the first offer msg presses that acts right now, or, when
// none does, says why the first it presses that explains itself cannot. A key
// no offer takes does nothing.
func (m Model) answer(offers []offer, msg tea.KeyPressMsg) (Model, tea.Cmd) {
	for _, each := range offers {
		if each.can && key.Matches(msg, each.binding) {
			return each.act()
		}
	}

	for _, each := range offers {
		if each.refusal != nil && key.Matches(msg, each.binding) {
			return m.noticedGuidance(each.refusal), nil
		}
	}

	return m, nil
}
