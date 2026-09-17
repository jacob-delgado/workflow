// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/jacob-delgado/workflow/internal/forge"
)

// reviewState is the branch's pull request and its CI, as far as they have
// loaded.
type reviewState struct {
	pull      forge.PullRequest
	found     bool
	loaded    bool
	err       error
	ci        forge.CI
	checked   bool
	checkedAt time.Time
	ciErr     error
	polling   bool
}

// ciCheckedFormat stamps the CI line with when it was last read.
const ciCheckedFormat = "15:04"

// findPullRequest is the command that looks for the branch's open pull request.
func (m Model) findPullRequest() tea.Cmd {
	find, branch := m.deps.Forge.FindPullRequest, m.branch.branch.Name
	if find == nil || !m.branch.onFeatureBranch() {
		return nil
	}

	return func() tea.Msg {
		pull, found, err := find(branch)

		return pullFound{branch: branch, pull: pull, found: found, err: err}
	}
}

// pullFound carries what the forge said about the branch's pull request.
type pullFound struct {
	branch string
	pull   forge.PullRequest
	found  bool
	err    error
}

// apply records the pull request, unless the branch has changed since, and
// checks its CI and who opened it.
func (msg pullFound) apply(m Model) (Model, tea.Cmd) {
	if msg.branch != m.branch.branch.Name {
		return m, nil
	}

	m.review = reviewState{pull: msg.pull, found: msg.found, loaded: true, err: msg.err}

	if !msg.found {
		return m, nil
	}

	return m, tea.Batch(m.checkCI(), m.loadAuthor())
}

// checkCI is the command that asks how CI stands on the pull request.
func (m Model) checkCI() tea.Cmd {
	check, pull, head := m.deps.Forge.CheckStatus, m.review.pull, m.branch.branch.Head
	if check == nil || !m.review.found {
		return nil
	}

	return func() tea.Msg {
		ci, err := check(pull, head)

		return ciChecked{number: pull.Number, ci: ci, err: err}
	}
}

// ciChecked carries how CI stands.
type ciChecked struct {
	number int
	ci     forge.CI
	err    error
}

// apply records CI, posts a message that was waiting for it, and keeps asking
// while there is something to wait for.
func (msg ciChecked) apply(m Model) (Model, tea.Cmd) {
	if !m.review.found || msg.number != m.review.pull.Number {
		return m, nil
	}

	m.review.ci, m.review.ciErr, m.review.checked = msg.ci, msg.err, true
	m.review.checkedAt = m.deps.now()

	m, post := m.postIfGreen()

	return m.keepPolling(post)
}

// keepPolling schedules the next check while CI runs or a post waits, unless
// one is already scheduled.
func (m Model) keepPolling(then tea.Cmd) (Model, tea.Cmd) {
	waiting := m.review.ci.State == forge.CIRunning || m.slack.pending.waiting()
	if !waiting || m.review.polling {
		return m, then
	}

	m.review.polling = true

	return m, tea.Batch(then, tea.Tick(m.deps.ciInterval(), func(time.Time) tea.Msg { return ciPoll{} }))
}

// ciPoll is time to ask about CI again.
type ciPoll struct{}

// apply asks again.
func (ciPoll) apply(m Model) (Model, tea.Cmd) {
	m.review.polling = false

	return m, m.checkCI()
}

// ciGlyph is how CI stands, by shape.
func (m Model) ciGlyph() string {
	return map[forge.CIState]string{
		forge.CINone: m.marks.unknown, forge.CIRunning: m.marks.inFlight,
		forge.CIPassed: m.marks.done, forge.CIFailed: m.marks.failed,
	}[m.review.ci.State]
}

// ciSummary says how CI stands in words.
func (m Model) ciSummary() string {
	reported := m.review.ci

	switch {
	case m.review.ciErr != nil:
		return m.marks.failed + " " + m.review.ciErr.Error()
	case !m.review.checked:
		return "checking" + m.marks.ellipsis
	case reported.State == forge.CINone:
		return m.ciGlyph() + " no checks reported" + m.checkedAtSuffix()
	}

	state := map[forge.CIState]string{
		forge.CINone: "", forge.CIRunning: "running", forge.CIPassed: "passed", forge.CIFailed: "failed",
	}[reported.State]

	if reported.Total > 0 {
		state += " (" + strconv.Itoa(reported.Done) + " of " + strconv.Itoa(reported.Total) + " finished)"
	}

	return m.ciGlyph() + " " + state + m.checkedAtSuffix()
}

// checkedAtSuffix says when CI was last read, for a line that already says how
// it stands.
func (m Model) checkedAtSuffix() string {
	if m.review.checkedAt.IsZero() {
		return ""
	}

	return m.marks.separator + "checked " + m.review.checkedAt.Format(ciCheckedFormat)
}

// reviewRail is the pull request and its CI, in brief.
func (m Model) reviewRail(_ int) string {
	switch {
	case !m.branch.onFeatureBranch():
		return m.styles.label.Render("on no feature branch")
	case !m.review.loaded:
		return "looking" + m.marks.ellipsis
	case m.review.err != nil:
		return m.marks.failed + " " + forgeReason(m.review.err)
	case !m.review.found:
		return "no pull request yet"
	}

	return "#" + strconv.Itoa(m.review.pull.Number) + " " + m.review.pull.Title + "\n" + m.ciSummary()
}

// reviewDetail describes the pull request, or what opening one needs.
func (m Model) reviewDetail(width int) string {
	if m.outsideRepository() {
		return wrap(notInRepository, width)
	}

	if !m.review.found {
		lines := []string{m.reviewRail(0)}

		if m.review.err != nil {
			lines = append(lines, "", m.failure(m.review.err))
		}

		if m.canOpenPullRequest() {
			lines = append(lines, "", "n opens one from this branch's commits and the repository's template.")
		}

		return wrap(strings.Join(lines, "\n"), width)
	}

	pull := m.review.pull
	lines := []string{
		m.styles.strong.Render("#"+strconv.Itoa(pull.Number)) + " " + pull.Title,
		pull.URL,
		"",
		m.styles.label.Render("CI     ") + m.ciSummary(),
	}

	if pull.Draft {
		lines = append(lines, m.styles.label.Render("draft"))
	}

	return wrap(strings.Join(lines, "\n"), width)
}

// canOpenPullRequest reports a branch with commits and no pull request yet. A
// forge failure offers nothing: n would open a composer whose push cannot land.
func (m Model) canOpenPullRequest() bool {
	return m.branch.onFeatureBranch() && len(m.branch.branch.Commits) > 0 && m.review.loaded &&
		m.review.err == nil && !m.review.found && m.deps.Forge.CreatePullRequest != nil
}

// forgeReason names why the forge could not be reached, by cause.
func forgeReason(err error) string {
	switch {
	case errors.Is(err, forge.ErrNoToken):
		return "no forge token"
	case errors.Is(err, forge.ErrNotARemote), errors.Is(err, forge.ErrUnknownForge),
		errors.Is(err, forge.ErrKindNeedsHost):
		return "origin is not GitHub or GitLab"
	case errors.Is(err, forge.ErrUnreachable):
		return "could not reach the forge"
	default:
		return "the forge did not answer"
	}
}

// reviewKeys offers opening a pull request, or checking again.
func (m Model) reviewKeys() []key.Binding {
	if m.outsideRepository() {
		return nil
	}

	if m.canOpenPullRequest() {
		return []key.Binding{m.keys.newPullRequest, m.keys.refresh}
	}

	return []key.Binding{m.keys.refresh}
}

// handleReviewKey answers the Review pane's own keys.
func (m Model) handleReviewKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.newPullRequest) && m.canOpenPullRequest():
		return m.openPullRequestComposer()
	case key.Matches(msg, m.keys.refresh):
		return m, tea.Batch(m.findPullRequest(), m.checkCI())
	default:
		return m, nil
	}
}
