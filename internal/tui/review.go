// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/jacob-delgado/workflow/internal/armed"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/loop"
	"github.com/jacob-delgado/workflow/internal/tui/latest"
)

// reviewState is the branch's pull request, as the latest find of it found
// it, its CI, as far as it has loaded, and how far the Review pane's detail is
// scrolled — which another pull request starts at the top.
type reviewState struct {
	find      latest.Read[pullFind]
	ci        latest.Read[forge.CI]
	checkedAt time.Time
	// quiet is how long the pull request's head has reported no check, as
	// the reads heard count it: a merge under no CI waits on it.
	quiet   armed.HeadWatch
	polling bool
	// opened is when this session opened the pull request; zero for one it
	// found.
	opened time.Time
	// reran are the ids of the failed runs a re-run was asked for, until a
	// read shows the re-run begun; reranAt is when it was asked for.
	reran   []string
	reranAt time.Time
	scroll  int
}

// pullFind is what a find of the branch's pull request found: the pull
// request, if one was, and, for one merged, loop.RefuseMadeAfter's refusal of
// finishing its branch: the commits made on it after it merged, or why they
// could not be read.
type pullFind struct {
	pull      forge.PullRequest
	found     bool
	madeAfter error
}

// beginReview replaces the review with one of the pull request find holds —
// another, or none — and counts it begun, so a poll scheduled for the one
// replaced ends its chain. Every new review goes through here.
func (m Model) beginReview(find latest.Read[pullFind]) Model {
	m.reviewsBegun++
	m.review = reviewState{find: find, ci: m.review.ci.Cleared()}

	return m
}

// pull is the pull request held: the latest find's, or the one told since,
// kept across a find that failed. A find that found none holds none.
func (s reviewState) pull() forge.PullRequest {
	held, _ := s.find.Value()

	return held.pull
}

// found reports a pull request held.
func (s reviewState) found() bool {
	held, _ := s.find.Value()

	return held.found
}

// madeAfter is loop.RefuseMadeAfter's refusal of finishing the branch of the
// merged pull request held, or nil.
func (s reviewState) madeAfter() error {
	held, _ := s.find.Value()

	return held.madeAfter
}

// looked reports a find answered, or a pull request told: what the pane says
// of the pull request is no longer that it is reading.
func (s reviewState) looked() bool {
	stage := s.find.Stage()

	return stage == latest.StageShown || stage == latest.StageFailed
}

// reviewRefreshing reports a read the Review pane's mark turns for: the
// branch's, the find's or CI's, out over what the pane already shows.
func (m Model) reviewRefreshing() bool {
	return m.branch.read.Refreshing() || m.review.find.Refreshing() || m.review.ci.Refreshing()
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

// findingPull begins a find of the branch's open pull request, superseding
// any still out, and is the command that looks for it; on no feature branch,
// or with no forge to ask, it begins nothing.
func (m Model) findingPull() (Model, tea.Cmd) {
	if m.deps.Forge.FindPullRequest == nil || !m.branch.onFeatureBranch() {
		return m, nil
	}

	find, round := m.review.find.Begin()
	m.review.find = find

	return m, m.branch.findPull(m.session, m.deps, round)
}

// findPull is the command that looks for the branch's open pull request, in
// round, and for a merged one whose branch loop.RefuseFinish finds nothing in
// the way of, reads what loop.RefuseMadeAfter says of the commits made on it
// since.
func (s branchState) findPull(ctx context.Context, deps Deps, round latest.Round) tea.Cmd {
	find, madeAfter, branch := deps.Forge.FindPullRequest, deps.Git.MadeAfter, s.checkedOut()

	return func() tea.Msg {
		pull, found, err := find(ctx, branch.Name)

		var refused error
		if loop.RefuseFinish(pull, found, branch) == nil {
			refused = loop.RefuseMadeAfter(ctx, madeAfter, pull)
		}

		return pullFound{round: round, find: pullFind{pull: pull, found: found, madeAfter: refused}, err: err}
	}
}

// pullFound carries what the forge said about the branch's pull request, to
// the find begun in round.
type pullFound struct {
	round latest.Round
	find  pullFind
	err   error
}

var _ applier = pullFound{}

// apply records the pull request, and checks its CI and who opened it. The
// answer of a find a later one superseded is dropped, as is one begun before
// the branch changed, or before a pull request was opened or edited here:
// it would show the pull request as it stood before, or blank the review and
// drop the announcement armed as one opened. A find that fails keeps the
// pull request already found, beside the failure.
func (msg pullFound) apply(m Model) (Model, tea.Cmd) {
	held, previous := m.review.found(), m.review.pull()

	find, taken := m.review.find.Answered(msg.round, msg.find, msg.err)
	if !taken {
		return m, nil
	}

	// A failed find says nothing about the pull request already found: keep
	// it, its CI and its poll, and read CI again for a head that may have
	// moved. The same pull request found again keeps them too, so a refresh
	// neither blanks the display nor starts a second poll beside the one
	// going.
	if held && (msg.err != nil || sameOpen(previous, msg.find)) {
		m.review.find = find
	} else {
		m = m.beginReview(find)
	}

	if !m.review.found() {
		return m, nil
	}

	m, check := m.checkCI()
	if msg.err != nil {
		return m, check
	}

	return m, tea.Batch(check, m.messaging.loadAuthor(m.session, m.deps))
}

// edited is the review told the pull request held as edited here: its new
// title and description.
func (s reviewState) edited(pull forge.PullRequest) reviewState {
	held, _ := s.find.Value()
	held.pull.Title, held.pull.Body = pull.Title, pull.Body
	s.find = s.find.Told(held)

	return s
}

// sameOpen reports a find of the pull request held, found again.
func sameOpen(held forge.PullRequest, find pullFind) bool {
	return find.found && find.pull.Number == held.Number
}

// rail is the pull request and its CI, in brief, or why there is none; after
// a failed find, the failure, then the pull request still held.
func (s reviewState) rail(kit renderKit, vocab reviewVocab, onFeatureBranch bool) string {
	if !onFeatureBranch {
		return "on no feature branch"
	}

	failed := func() string { return kit.failureSummary(s.find.Err()) }

	return drawRead(kit, s.find, drawnRead{
		shown:  func() string { return s.pulled(kit, vocab) },
		failed: failed,
		failedOver: func() string {
			if !s.found() {
				return failed()
			}

			// A read of CI that failed as the find did is said once.
			rows := []string{failed(), s.pullLine(vocab)}
			if standing := s.standing(kit); standing != failed() {
				rows = append(rows, standing)
			}

			return strings.Join(rows, "\n")
		},
	})
}

// pulled is the pull request held and how it stands, in brief, or that
// there is none yet.
func (s reviewState) pulled(kit renderKit, vocab reviewVocab) string {
	if !s.found() {
		return "no " + vocab.noun + " yet"
	}

	return s.pullLine(vocab) + "\n" + s.standing(kit)
}

// pullLine is the pull request held by number and title.
func (s reviewState) pullLine(vocab reviewVocab) string {
	pull := s.pull()

	return vocab.sigil + strconv.Itoa(pull.Number) + " " + pull.Title
}

// standing is where the pull request held stands: merged, merging, or its CI
// in brief.
func (s reviewState) standing(kit renderKit) string {
	// A pull request merged, or merging, has no live CI to poll, so the rail
	// says where it is rather than sitting forever on "checking".
	if s.pull().State == forge.StateMerged {
		return kit.marks.done + " merged"
	}

	if s.pull().State == forge.StateMerging {
		return kit.marks.inFlight + " merging"
	}

	return s.ciSummary(kit)
}

// reviewDetail describes the pull request, or what opening one needs.
func (m Model) reviewDetail(width int) string {
	if m.branch.outsideRepository() {
		return m.kit().failureBlock(m.branch.read.Err(), width)
	}

	if !m.review.found() {
		return m.unfoundDetail(width)
	}

	pull := m.review.pull()

	if done, ok := m.doneWithDetail(pull); ok {
		return wrap(done, width)
	}

	lines := []string{m.styles.strong.Render(m.vocab.sigil+strconv.Itoa(pull.Number)) + " " + pull.Title, pull.URL}
	if issue := m.reviewIssue(pull); issue != "" {
		lines = append(lines, issue)
	}

	lines = append(lines, "", m.styles.label.Render("CI     ")+m.review.ciSummary(m.kit()))
	lines = append(lines, m.ciBar()...)
	lines = append(lines, m.review.failedChecks(m.kit())...)
	lines = append(lines, m.styles.label.Render("review ")+reviewSummary(m.marks, pull))

	if pull.Draft {
		lines = append(lines, m.styles.label.Render("draft"))
	}

	// A failed read of CI, or a failed find of the pull request held, is
	// said beneath what it kept.
	lines = append(lines, m.kit().failureBlocks(width, m.review.ci.Err(), m.review.find.Err())...)

	if m.review.canEditPull(m.deps) {
		lines = append(lines, "", m.keys.edit.Help().Key+" edits its title and description.")
	}

	return wrap(strings.Join(lines, "\n"), width)
}

// unfoundDetail says there is no pull request yet, or why none was found,
// and what opening one needs.
func (m Model) unfoundDetail(width int) string {
	lines := []string{m.review.rail(m.kit(), m.vocab, m.branch.onFeatureBranch())}

	if m.review.find.Err() != nil {
		lines = append(lines, "", m.kit().failureBlock(m.review.find.Err(), width))
	}

	if m.review.canOpenPull(m.deps, m.branch) {
		lines = append(lines, "", m.keys.newPullRequest.Help().Key+
			" opens one from this branch's commits and the repository's template.")
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
// has not worked it out. A map, not a switch, so there is no return after the
// last case that no state reaches; exhaustive keeps it complete.
func mergeableLabel(mergeable forge.Mergeability) string {
	return map[forge.Mergeability]string{
		forge.MergeClean: "mergeable", forge.MergeConflicts: "conflicts", forge.MergeUnknown: "",
	}[mergeable]
}

// canOpenPull reports a branch with commits and no open or merging pull
// request — none found, or only a merged one, whose branch may carry commits
// worth a new one — as loop's refuseAnOpenPull allows. A failed find offers it all the
// same, leaving the open to answer with the forge's reason, except for a
// missing forge token, which no open gets past.
func (s reviewState) canOpenPull(deps Deps, branch branchState) bool {
	return branch.onFeatureBranch() && len(branch.checkedOut().Commits) > 0 && s.looked() &&
		!errors.Is(s.find.Err(), forge.ErrNoToken) && !s.hasOpenPull() && !s.merging() &&
		deps.Forge.CreatePullRequest != nil
}

// hasOpenPull reports a pull request found open on the branch. A find
// also returns a merged one, so found alone does not say so.
func (s reviewState) hasOpenPull() bool {
	return s.found() && s.pull().IsOpen()
}

// merging reports a pull request found that the forge is merging.
func (s reviewState) merging() bool {
	return s.found() && s.pull().State == forge.StateMerging
}

// canEditPull reports an open pull request whose title and body can be
// edited here; a merged one cannot be.
func (s reviewState) canEditPull(deps Deps) bool {
	return s.hasOpenPull() && deps.Forge.EditPullRequest != nil
}

// reviewKeys are the Review pane's keys that can go now: open or edit a pull
// request, list, re-run or merge on its checks, finish its merged branch,
// open or copy its link, and read it again.
func (m Model) reviewKeys() []key.Binding {
	if m.branch.outsideRepository() {
		return nil
	}

	var keys []key.Binding

	if m.review.canOpenPull(m.deps, m.branch) {
		keys = append(keys, m.keys.newPullRequest)
	}

	if m.review.canEditPull(m.deps) {
		keys = append(keys, m.keys.edit)
	}

	if m.review.canOpenChecks(m.deps) {
		keys = append(keys, m.keys.checks)
	}

	if m.review.canRerun(m.deps) {
		keys = append(keys, m.keys.rerun)
	}

	if m.review.canMerge(m.deps) {
		keys = append(keys, m.keys.merge)
	}

	if m.review.canFinish(m.deps, m.branch) {
		keys = append(keys, m.keys.finish)
	}

	keys = append(keys, m.linkKeys(m.review.pullURL())...)

	return append(keys, m.keys.refresh)
}

// pullURL is the branch's open pull request URL, or empty when none is
// found.
func (s reviewState) pullURL() string {
	if !s.found() {
		return ""
	}

	return s.pull().URL
}

// handleReviewKey answers the Review pane's own keys.
func (m Model) handleReviewKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.checks) && m.review.canOpenChecks(m.deps):
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
	case key.Matches(msg, m.keys.newPullRequest) && m.review.canOpenPull(m.deps, m.branch):
		return m.openPullRequestComposer()
	case key.Matches(msg, m.keys.edit) && m.review.canEditPull(m.deps):
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
		// refresh reads the branch again, which looks for its pull request, and
		// the find reads CI for the one it finds: so a refresh picks up a branch
		// switched in a shell, and never reads CI for a pull request since replaced.
		refresh: func(m Model) (Model, tea.Cmd) { return m.readingBranch() },
		loading: Model.reviewRefreshing,
		scroll:  func(m *Model) *int { return &m.review.scroll }, readsBranch: true,
		answers: []string{
			"open-pull-request", "edit", "checks", actionRerunChecks, "merge", "finish-branch", actionOpenLink,
			actionCopyLink, actionRefresh,
		},
	}
}
