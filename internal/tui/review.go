// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"strconv"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/jacob-delgado/workflow/internal/forge"
)

// reviewState is the branch's pull request and its CI, as far as they have
// loaded, and how far the Review pane's detail is scrolled — which another pull
// request starts at the top.
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
	scroll    int
}

// beginReview replaces the review with next — another pull request, or none —
// and counts it begun, so a poll scheduled for the one replaced ends its chain.
// Every new review goes through here.
func (m Model) beginReview(next reviewState) Model {
	m.reviewsBegun++
	m.review = next

	return m
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
	return reviewVocab{noun: kind.Noun(), sigil: kind.Sigil()}
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
// checks its CI and who opened it. A find that fails keeps the pull request
// already found, beside the failure.
func (msg pullFound) apply(m Model) (Model, tea.Cmd) {
	if msg.branch != m.branch.branch.Name {
		return m, nil
	}

	if m.review.found && msg.err != nil {
		// A failed find says nothing about the pull request already found: keep
		// it and its poll, and read CI again for a head that may have moved.
		m.review.err = msg.err

		return m, m.checkCI()
	}

	if m.review.found && msg.found && msg.pull.Number == m.review.pull.Number {
		// The same pull request, found again: keep its CI and any poll already
		// running, so a refresh does not blank the display or start a second
		// polling chain beside the one already going.
		m.review.pull, m.review.err = msg.pull, nil
	} else {
		m = m.beginReview(reviewState{pull: msg.pull, found: msg.found, loaded: true, err: msg.err})
	}

	if !m.review.found {
		return m, nil
	}

	return m, tea.Batch(m.checkCI(), m.loadAuthor())
}

// checkCI is the command that asks how CI stands on the pull request.
func (m Model) checkCI() tea.Cmd {
	check, pull, head := m.deps.Forge.CheckStatus, m.review.pull, m.branch.branch.Head
	if check == nil || !m.review.found || pull.State != forge.StateOpen {
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
	waiting := m.review.ci.State == forge.CIRunning || m.messaging.pending.waiting()
	if !waiting || m.review.ciErr != nil || m.review.polling {
		return m, then
	}

	m.review.polling = true
	poll := ciPoll{review: m.reviewsBegun}

	return m, tea.Batch(then, m.deps.after(m.pollInterval(), func(time.Time) tea.Msg { return poll }))
}

// notifyPollInterval is how often CI is asked about when the developer only
// wants to be told it finished: a slower beat than the post-when-green path,
// because a notification the developer stepped away for is not in a hurry.
const notifyPollInterval = 3 * time.Minute

// pollInterval is how long to wait before asking about CI again. A post waiting
// on CI wants a prompt answer, and a configured interval is always honored; a
// bare notification, with no interval set, is content with a slower beat.
func (m Model) pollInterval() time.Duration {
	if m.cfg.UI.Notify && !m.messaging.pending.waiting() && m.deps.CIInterval <= 0 {
		return notifyPollInterval
	}

	return m.deps.ciInterval()
}

// ciPoll is time to ask about CI again, for the review it was scheduled in,
// counted among the reviews begun. A poll from a review since replaced is stale.
type ciPoll struct {
	review int
}

// apply asks again, unless it belongs to a review that has since been replaced,
// in which case it does nothing and its chain ends here.
func (msg ciPoll) apply(m Model) (Model, tea.Cmd) {
	if msg.review != m.reviewsBegun {
		return m, nil
	}

	m.review.polling = false

	return m, m.checkCI()
}

// ciGlyph is how the branch's CI stands, by shape.
func (m Model) ciGlyph() string {
	return m.ciStateGlyph(m.review.ci.State)
}

// ciSummary says how CI stands in words.
func (m Model) ciSummary() string {
	reported := m.review.ci

	switch {
	case m.review.ciErr != nil:
		return m.failureSummary(m.review.ciErr)
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
		return m.failureSummary(m.review.err)
	case !m.review.found:
		return "no " + m.vocab.noun + " yet"
	}

	line := m.vocab.sigil + strconv.Itoa(m.review.pull.Number) + " " + m.review.pull.Title
	if m.review.pull.State == forge.StateMerged {
		// A merged pull request has no live CI to poll, so the rail says it merged
		// rather than sitting forever on "checking".
		return line + "\n" + m.marks.done + " merged"
	}

	return line + "\n" + m.ciSummary()
}

// reviewDetail describes the pull request, or what opening one needs.
func (m Model) reviewDetail(width int) string {
	if m.outsideRepository() {
		return wrap(notInRepository, width)
	}

	if !m.review.found {
		lines := []string{m.reviewRail(0)}

		if m.review.err != nil {
			lines = append(lines, "", m.failureBlock(m.review.err, width))
		}

		if m.canOpenPullRequest() {
			lines = append(lines, "", "n opens one from this branch's commits and the repository's template.")
		}

		return wrap(strings.Join(lines, "\n"), width)
	}

	pull := m.review.pull

	if pull.State == forge.StateMerged {
		return wrap(m.mergedDetail(pull), width)
	}

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

	if m.review.ciErr != nil {
		lines = append(lines, "", m.failureBlock(m.review.ciErr, width))
	}

	if m.canEditPullRequest() {
		lines = append(lines, "", "e edits its title and description.")
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

// canOpenPullRequest reports a branch with commits and no open pull request —
// none found, or only a merged one, whose branch may carry commits worth a new
// one, as loop's refuseAnOpenPull allows. A forge failure offers nothing: n
// would open a composer whose push cannot land.
func (m Model) canOpenPullRequest() bool {
	return m.branch.onFeatureBranch() && len(m.branch.branch.Commits) > 0 && m.review.loaded &&
		m.review.err == nil && !m.hasOpenPullRequest() && m.deps.Forge.CreatePullRequest != nil
}

// hasOpenPullRequest reports a pull request found open on the branch. A find
// also returns a merged one, so found alone does not say so.
func (m Model) hasOpenPullRequest() bool {
	return m.review.found && m.review.pull.IsOpen()
}

// canEditPullRequest reports an open pull request whose title and body can be
// edited here; a merged one cannot be.
func (m Model) canEditPullRequest() bool {
	return m.hasOpenPullRequest() && m.deps.Forge.EditPullRequest != nil
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

	if m.canEditPullRequest() {
		keys = append(keys, m.keys.edit)
	}

	if m.canOpenChecks() {
		keys = append(keys, m.keys.checks)
	}

	if m.canRerun() {
		keys = append(keys, m.keys.rerun)
	}

	if m.canMerge() {
		keys = append(keys, m.keys.merge)
	}

	if m.canFinish() {
		keys = append(keys, m.keys.finish)
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
	case key.Matches(msg, m.keys.checks) && m.canOpenChecks():
		return m.openChecks()
	case key.Matches(msg, m.keys.rerun):
		return m.previewRerun()
	case key.Matches(msg, m.keys.merge):
		return m.startMerge()
	case key.Matches(msg, m.keys.finish):
		return m.startFinish()
	case key.Matches(msg, m.keys.refresh):
		return m, tea.Batch(m.findPullRequest(), m.checkCI())
	default:
		return m.handleReviewCompose(msg)
	}
}

// handleReviewCompose answers the keys that open a composer on the pull request:
// a new one, or an edit of the open one.
func (m Model) handleReviewCompose(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.newPullRequest) && m.canOpenPullRequest():
		return m.openPullRequestComposer()
	case key.Matches(msg, m.keys.edit) && m.canEditPullRequest():
		return m.openPullRequestEditor()
	default:
		return m.handleReviewLink(msg)
	}
}

// handleReviewLink answers the Review pane's open-and-copy-link keys.
func (m Model) handleReviewLink(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.openLink):
		return m.openLink(m.reviewPullURL())
	case key.Matches(msg, m.keys.copyLink):
		return m.copyLink(m.reviewPullURL())
	default:
		return m, nil
	}
}
