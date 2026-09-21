// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"errors"
	"strconv"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

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
	// generation rises each time a genuinely new review begins, so a poll left
	// over from an earlier one recognizes itself as stale and stops rather than
	// starting a fresh chain of its own.
	generation int
}

// ciCheckedFormat stamps the CI line with when it was last read.
const ciCheckedFormat = "15:04"

// reviewVocab is what a change is called on this forge: a pull request on
// GitHub, a merge request on GitLab, each with its own number sigil.
type reviewVocab struct {
	noun, sigil string
}

// forgeVocab is the vocabulary for a forge kind.
func forgeVocab(kind forge.Kind) reviewVocab {
	if kind == forge.KindGitLab {
		return reviewVocab{noun: "merge request", sigil: "!"}
	}

	return reviewVocab{noun: "pull request", sigil: "#"}
}

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

	if m.review.found && msg.found && msg.pull.Number == m.review.pull.Number {
		// The same pull request, found again: keep its CI and any poll already
		// running, so a refresh does not blank the display or start a second
		// polling chain beside the one already going.
		m.review.pull, m.review.err = msg.pull, msg.err
	} else {
		m.review = reviewState{
			pull: msg.pull, found: msg.found, loaded: true, err: msg.err,
			generation: m.review.generation + 1,
		}
	}

	if !m.review.found {
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

// apply records CI, posts a message that was waiting for it, rings the terminal
// if CI has just finished, and keeps asking while there is something to wait for.
func (msg ciChecked) apply(m Model) (Model, tea.Cmd) {
	if !m.review.found || msg.number != m.review.pull.Number {
		return m, nil
	}

	was := m.review.ci.State
	m.review.ci, m.review.ciErr, m.review.checked = msg.ci, msg.err, true
	m.review.checkedAt = m.deps.now()

	ring := m.ciFinishNotice(was, msg.ci.State)

	m, post := m.postIfGreen()

	return m.keepPolling(tea.Batch(post, ring))
}

// ciFinishNotice rings the terminal once when CI has just gone from running to a
// settled result, if the developer asked to be told and the interface can reach
// the terminal to ring it. It fires only on the change, not on later checks that
// find CI already finished.
func (m Model) ciFinishNotice(was, now forge.CIState) tea.Cmd {
	settled := now == forge.CIPassed || now == forge.CIFailed
	if !m.cfg.UI.Notify || m.deps.Notify == nil || was != forge.CIRunning || !settled {
		return nil
	}

	notify := m.deps.Notify

	return func() tea.Msg {
		notify()

		return nil
	}
}

// keepPolling schedules the next check while CI runs or a post waits, unless one
// is already scheduled or the last check failed. A failed check would only fail
// again at the same rate, so it stops until the next refresh rather than asking
// for as long as the program runs.
func (m Model) keepPolling(then tea.Cmd) (Model, tea.Cmd) {
	waiting := m.review.ci.State == forge.CIRunning || m.slack.pending.waiting()
	if !waiting || m.review.ciErr != nil || m.review.polling {
		return m, then
	}

	m.review.polling = true
	poll := ciPoll{number: m.review.pull.Number, generation: m.review.generation}

	return m, tea.Batch(then, tea.Tick(m.pollInterval(), func(time.Time) tea.Msg { return poll }))
}

// notifyPollInterval is how often CI is asked about when the developer only
// wants to be told it finished: a slower beat than the post-when-green path,
// because a notification the developer stepped away for is not in a hurry.
const notifyPollInterval = 3 * time.Minute

// pollInterval is how long to wait before asking about CI again. A post waiting
// on CI wants a prompt answer, and a configured interval is always honored; a
// bare notification, with no interval set, is content with a slower beat.
func (m Model) pollInterval() time.Duration {
	if m.cfg.UI.Notify && !m.slack.pending.waiting() && m.deps.CIInterval <= 0 {
		return notifyPollInterval
	}

	return m.deps.ciInterval()
}

// ciPoll is time to ask about CI again, for the pull request and the review it
// was scheduled in. A poll from a review since replaced is stale.
type ciPoll struct {
	number     int
	generation int
}

// apply asks again, unless it belongs to a review that has since been replaced,
// in which case it does nothing and its chain ends here.
func (msg ciPoll) apply(m Model) (Model, tea.Cmd) {
	if !m.review.found || msg.number != m.review.pull.Number || msg.generation != m.review.generation {
		return m, nil
	}

	m.review.polling = false

	return m, m.checkCI()
}

// ciGlyph is how CI stands, by shape.
func (m Model) ciGlyph() string {
	return map[forge.CIState]string{
		forge.CINone: m.marks.unknown, forge.CIRunning: m.marks.inFlight,
		forge.CIPassed: m.marks.done, forge.CIFailed: m.failedGlyph(),
	}[m.review.ci.State]
}

// ciSummary says how CI stands in words.
func (m Model) ciSummary() string {
	reported := m.review.ci

	switch {
	case m.review.ciErr != nil:
		return m.failedGlyph() + " " + m.review.ciErr.Error()
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
		return "on no feature branch"
	case !m.review.loaded:
		return "looking" + m.marks.ellipsis
	case m.review.err != nil:
		return m.failedGlyph() + " " + forgeReason(m.review.err)
	case !m.review.found:
		return "no " + m.vocab.noun + " yet"
	}

	return m.vocab.sigil + strconv.Itoa(m.review.pull.Number) + " " + m.review.pull.Title + "\n" + m.ciSummary()
}

// reviewDetail describes the pull request, or what opening one needs.
func (m Model) reviewDetail(width int) string {
	if m.outsideRepository() {
		return wrap(notInRepository, width)
	}

	if !m.review.found {
		lines := []string{m.reviewRail(0)}

		if m.review.err != nil {
			lines = append(lines, "", m.failureWithin(m.review.err, width))
		}

		if m.canOpenPullRequest() {
			lines = append(lines, "", "n opens one from this branch's commits and the repository's template.")
		}

		return wrap(strings.Join(lines, "\n"), width)
	}

	pull := m.review.pull
	lines := []string{
		m.styles.strong.Render(m.vocab.sigil+strconv.Itoa(pull.Number)) + " " + pull.Title,
		pull.URL,
		"",
		m.styles.label.Render("CI     ") + m.ciSummary(),
		m.styles.label.Render("review ") + m.reviewSummary(pull),
	}

	if pull.Draft {
		lines = append(lines, m.styles.label.Render("draft"))
	}

	return wrap(strings.Join(lines, "\n"), width)
}

// reviewSummary says how the review stands: how many approvals, whether changes
// are still asked for, and whether the branch can merge.
func (m Model) reviewSummary(pull forge.PullRequest) string {
	parts := []string{plural(pull.Approvals, "approval")}

	if pull.ChangesRequested {
		parts = append(parts, "changes requested")
	}

	if mergeable := mergeableLabel(pull.Mergeable); mergeable != "" {
		parts = append(parts, mergeable)
	}

	return strings.Join(parts, m.marks.separator)
}

// mergeableLabel names whether the branch can merge, or nothing while the forge
// has not worked it out.
func mergeableLabel(mergeable forge.Mergeability) string {
	switch mergeable {
	case forge.MergeClean:
		return "mergeable"
	case forge.MergeConflicts:
		return "conflicts"
	case forge.MergeUnknown:
		return ""
	}

	return ""
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

// reviewKeys offers opening a pull request, listing its checks, or checking
// again.
func (m Model) reviewKeys() []key.Binding {
	if m.outsideRepository() {
		return nil
	}

	var keys []key.Binding

	if m.canOpenPullRequest() {
		keys = append(keys, m.keys.newPullRequest)
	}

	if m.canOpenChecks() {
		keys = append(keys, m.keys.checks)
	}

	keys = append(keys, m.linkKeys(m.reviewPullURL())...)

	return append(keys, m.keys.refresh)
}

// reviewPullURL is the branch's open pull request URL, or empty when none is
// found.
func (m Model) reviewPullURL() string {
	if !m.review.found {
		return ""
	}

	return m.review.pull.URL
}

// handleReviewKey answers the Review pane's own keys.
func (m Model) handleReviewKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.newPullRequest) && m.canOpenPullRequest():
		return m.openPullRequestComposer()
	case key.Matches(msg, m.keys.checks) && m.canOpenChecks():
		return m.openChecks()
	case key.Matches(msg, m.keys.openLink):
		return m.openLink(m.reviewPullURL())
	case key.Matches(msg, m.keys.copyLink):
		return m.copyLink(m.reviewPullURL())
	case key.Matches(msg, m.keys.refresh):
		return m, tea.Batch(m.findPullRequest(), m.checkCI())
	default:
		return m, nil
	}
}
