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
	// loading is a refresh begun and not yet answered: the branch read, its
	// pull request looked for, and CI read for the one found.
	loading bool
}

// beginReview replaces the review with next — another pull request, or none —
// and counts it begun, so a poll scheduled for the one replaced ends its chain.
// Every new review goes through here.
func (m Model) beginReview(next reviewState) Model {
	m.reviewsBegun++
	// A refresh still waits for its answer, whichever pull request it finds.
	next.loading = m.review.loading
	m.review = next

	return m
}

// reviewVocab is what a change is called on this forge: a pull request on
// GitHub, a merge request on GitLab, each with its own number sigil.
type reviewVocab struct {
	noun, sigil string
}

// forgeVocab is the vocabulary for a forge kind.
func forgeVocab(kind forge.Kind) reviewVocab {
	return reviewVocab{noun: kind.Noun(), sigil: kind.Sigil()}
}

// refreshReview reads the branch again, which looks for its pull request, and
// the find reads CI for the one it finds: so a refresh picks up a branch
// switched in a shell, and never reads CI for a pull request since replaced.
func (m Model) refreshReview() (Model, tea.Cmd) {
	read := loadBranch(m.deps)
	m.review.loading = read != nil

	return m, read
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

var _ applier = pullFound{}

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

		return m.checkingCI(nil)
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
		m.review.loading = false

		return m, nil
	}

	return m.checkingCI(m.loadAuthor())
}

// rail is the pull request and its CI, in brief, or why there is none.
func (s reviewState) rail(kit renderKit, vocab reviewVocab, onFeatureBranch bool) string {
	switch {
	case !onFeatureBranch:
		return "on no feature branch"
	case !s.loaded:
		return kit.marks.reading()
	case s.err != nil:
		return kit.failureSummary(s.err)
	case !s.found:
		return "no " + vocab.noun + " yet"
	}

	line := vocab.sigil + strconv.Itoa(s.pull.Number) + " " + s.pull.Title
	if s.pull.State == forge.StateMerged {
		// A merged pull request has no live CI to poll, so the rail says it merged
		// rather than sitting forever on "checking".
		return line + "\n" + kit.marks.done + " merged"
	}

	return line + "\n" + s.ciSummary(kit)
}

// reviewDetail describes the pull request, or what opening one needs.
func (m Model) reviewDetail(width int) string {
	if m.branch.outsideRepository() {
		return m.kit().failureBlock(m.branch.err, width)
	}

	if !m.review.found {
		lines := []string{m.review.rail(m.kit(), m.vocab, m.branch.onFeatureBranch())}

		if m.review.err != nil {
			lines = append(lines, "", m.kit().failureBlock(m.review.err, width))
		}

		if m.canOpenPullRequest() {
			lines = append(lines, "", m.keys.newPullRequest.Help().Key+
				" opens one from this branch's commits and the repository's template.")
		}

		return wrap(strings.Join(lines, "\n"), width)
	}

	pull := m.review.pull

	if pull.State == forge.StateMerged {
		return wrap(m.mergedDetail(pull), width)
	}

	lines := []string{m.styles.strong.Render(m.vocab.sigil+strconv.Itoa(pull.Number)) + " " + pull.Title, pull.URL}
	if issue := m.reviewIssue(pull); issue != "" {
		lines = append(lines, issue)
	}

	lines = append(lines, "", m.styles.label.Render("CI     ")+m.review.ciSummary(m.kit()))
	lines = append(lines, m.review.failedChecks(m.kit())...)
	lines = append(lines, m.styles.label.Render("review ")+reviewSummary(m.marks, pull))

	if pull.Draft {
		lines = append(lines, m.styles.label.Render("draft"))
	}

	if m.review.ciErr != nil {
		lines = append(lines, "", m.kit().failureBlock(m.review.ciErr, width))
	}

	if m.canEditPullRequest() {
		lines = append(lines, "", m.keys.edit.Help().Key+" edits its title and description.")
	}

	return wrap(strings.Join(lines, "\n"), width)
}

// reviewSummary says how the review stands: how many approvals, whether changes
// are still asked for, and whether the branch can merge.
func reviewSummary(marks glyphs, pull forge.PullRequest) string {
	parts := []string{plural(pull.Approvals, "approval")}

	if pull.ChangesRequested {
		parts = append(parts, "changes requested")
	}

	if mergeable := mergeableLabel(pull.Mergeable); mergeable != "" {
		parts = append(parts, mergeable)
	}

	return strings.Join(parts, marks.separator)
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
// one — as loop's refuseAnOpenPull allows. A failed find offers it all the
// same, leaving the open to answer with the forge's reason, except for a
// missing forge token, which no open gets past.
func (m Model) canOpenPullRequest() bool {
	return m.branch.onFeatureBranch() && len(m.branch.branch.Commits) > 0 && m.review.loaded &&
		!errors.Is(m.review.err, forge.ErrNoToken) && !m.review.hasOpenPull() &&
		m.deps.Forge.CreatePullRequest != nil
}

// hasOpenPull reports a pull request found open on the branch. A find
// also returns a merged one, so found alone does not say so.
func (s reviewState) hasOpenPull() bool {
	return s.found && s.pull.IsOpen()
}

// canEditPullRequest reports an open pull request whose title and body can be
// edited here; a merged one cannot be.
func (m Model) canEditPullRequest() bool {
	return m.review.hasOpenPull() && m.deps.Forge.EditPullRequest != nil
}

// reviewKeys offers opening a pull request, listing its checks, or checking
// again.
func (m Model) reviewKeys() []key.Binding {
	if m.branch.outsideRepository() {
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

	keys = append(keys, m.linkKeys(m.review.pullURL())...)

	return append(keys, m.keys.refresh)
}

// pullURL is the branch's open pull request URL, or empty when none is
// found.
func (s reviewState) pullURL() string {
	if !s.found {
		return ""
	}

	return s.pull.URL
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
		return m.refreshPane(paneReview)
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
		return m.openLink(m.review.pullURL())
	case key.Matches(msg, m.keys.copyLink):
		return m.copyLink(m.review.pullURL())
	default:
		return m, nil
	}
}

// reviewBehavior is the Review pane's behavior.
func reviewBehavior() behavior {
	return behavior{
		rail: func(m Model, _ int) string {
			return m.review.rail(m.kit(), m.vocab, m.branch.onFeatureBranch())
		},
		detail: Model.reviewDetail, narrow: nil,
		keys: Model.reviewKeys, handle: Model.handleReviewKey, pick: nil,
		refresh: Model.refreshReview, loading: func(m Model) bool { return m.review.loading },
		scroll: func(m *Model) *int { return &m.review.scroll }, readsBranch: true,
		answers: []string{
			"open-pull-request", "edit", "checks", "rerun-checks", "merge", "finish-branch", actionOpenLink,
			actionCopyLink, actionRefresh,
		},
	}
}
