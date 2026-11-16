// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"errors"
	"slices"
	"strconv"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/jacob-delgado/workflow/internal/armed"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/loop"
	"github.com/jacob-delgado/workflow/internal/tui/latest"
)

// ciCheckedFormat stamps the CI line with when it was last read.
const ciCheckedFormat = "15:04"

// errNothingToRerun reports a failure the forge has no re-runnable job for.
var errNothingToRerun = errors.New("nothing to re-run: this failure has no job to restart")

// checkCI begins a read of CI, superseding any still out, and is the
// command that asks how CI stands on the pull request; with no forge to
// ask, or a pull request held that is not open, it begins nothing. Every
// caller holds a pull request found: a find's answer that found one, one
// opened here, and a poll, whose review a find finding none replaces.
func (m Model) checkCI() (Model, tea.Cmd) {
	ctx, check, pull, head := m.session, m.deps.Forge.CheckStatus, m.review.pull(), m.branch.checkedOut().Head
	if check == nil || pull.State != forge.StateOpen {
		return m, nil
	}

	read, round := m.review.ci.Begin()
	m.review.ci = read

	return m, func() tea.Msg {
		ci, err := check(ctx, pull, head)

		return ciChecked{round: round, ci: ci, err: err}
	}
}

// ciChecked carries how CI stands, to the read begun in round.
type ciChecked struct {
	round latest.Round
	ci    forge.CI
	err   error
}

var _ applier = ciChecked{}

// apply records CI, rings the terminal if CI has just finished, and keeps
// asking while it runs. The answer of a read a later one superseded, or one
// begun before a re-run was asked for or the pull request was replaced, is
// dropped: it would show CI as it stood before. A read that failed lists no
// check, which may have moved on since, but keeps how CI last stood, so the
// read that answers next rings for a change from it; the pane does not ask
// again after a failed read, leaving a post that waits to the shared hold's
// own reads.
func (msg ciChecked) apply(m Model) (Model, tea.Cmd) {
	now := m.deps.now()
	if msg.err == nil && m.review.awaitingRerun(msg.ci, now) {
		return m.heldForRerun(msg.round, now)
	}

	awaited := m.review.awaitedFinish()

	read, taken := m.review.ci.Answered(msg.round, msg.ci, msg.err)
	if !taken {
		return m, nil
	}

	m.review.ci, m.review.checkedAt = read, now

	if msg.err != nil {
		m.review.quiet = m.review.quiet.Unheard()

		return m, nil
	}

	m.review.reran, m.review.quiet = nil, m.review.quiet.Read(m.review.pull(), msg.ci, now)

	return m.keepPolling(ciFinishNotice(m.cfg, m.deps, awaited, msg.ci.State))
}

// heldForRerun ends round's read, made at now, without showing it: it still
// lists a run a re-run was asked for, so CI stays running, for a while, as
// the re-run told it: the forge has not started the re-run yet.
func (m Model) heldForRerun(round latest.Round, now time.Time) (Model, tea.Cmd) {
	read, taken := m.review.ci.Settled(round)
	if !taken {
		return m, nil
	}

	m.review.ci, m.review.checkedAt = read, now

	return m.keepPolling(nil)
}

// awaitedFinish reports CI the last read, or a re-run since, left still to
// come: running, or reporting no check yet for a pull request just opened. A
// read that then finds it settled is CI finishing.
func (s reviewState) awaitedFinish() bool {
	return s.checked() && s.ciToCome(s.checkedAt)
}

// checked reports CI read, or told by a re-run: whether or not the read
// failed, the pane no longer says it is reading.
func (s reviewState) checked() bool {
	stage := s.ci.Stage()

	return stage == latest.StageShown || stage == latest.StageFailed
}

// listed is CI as the pane lists it: as the latest read found it, or as a
// re-run told it, or after a failed read, how CI last stood and no check,
// since the checks may have moved on since.
func (s reviewState) listed() forge.CI {
	ci, _ := s.ci.Value()
	if s.ci.Stage() != latest.StageShown {
		return forge.CI{State: ci.State}
	}

	return ci
}

// ciFinishNotice rings the terminal once when CI awaited has just settled, if
// the developer asked to be told and the interface can reach the terminal to
// ring it. It fires only on the change, not on later checks that find CI
// already finished, nor on the first read, which finds CI as it already was.
func ciFinishNotice(cfg config.Config, deps Deps, awaited bool, now forge.CIState) tea.Cmd {
	settled := now == forge.CIPassed || now == forge.CIFailed
	if !cfg.UI.Notify || deps.Notify == nil || !awaited || !settled {
		return nil
	}

	notify := deps.Notify

	return func() tea.Msg {
		notify()

		return nil
	}
}

// keepPolling schedules the next check while CI is still to come, unless one
// is already scheduled. A failed check stops it: it would fail again at the
// same rate, so it stops until the next refresh rather than asking for as long
// as the program runs. An announcement waiting for CI reads it on a beat of
// its own.
func (m Model) keepPolling(then tea.Cmd) (Model, tea.Cmd) {
	polls, chain, scheduled := m.polls.scheduled()
	if !m.review.ciToCome(m.deps.now()) || !scheduled {
		return m, then
	}

	m.polls = polls
	poll := ciPoll{chain: chain}

	wait := pollInterval(m.cfg.UI, m.deps)

	return m, tea.Batch(then, m.deps.after(wait, func(time.Time) tea.Msg { return poll }))
}

// ciToCome reports CI still to come: running, or, read cleanly, reporting no
// check for a pull request this session opened within armed.NoCIPatience — a
// forge lists a new pull request's checks a moment after it opens, and an
// announcement armed as it opened waits that long for them too — or for one
// whose merge waits on its head's quiet to be offered.
func (s reviewState) ciToCome(now time.Time) bool {
	state := s.listed().State
	if state == forge.CIRunning {
		return true
	}

	if state != forge.CINone || s.ci.Stage() == latest.StageFailed {
		return false
	}

	justOpened := !s.opened.IsZero() && now.Sub(s.opened) < armed.NoCIPatience

	return justOpened || s.mergeAwaitsQuiet()
}

// mergeAwaitsQuiet reports a pull request ready to merge in every way but
// its head's quiet: no check reported on it, for less than armed.NoCIPatience
// of the reads heard.
func (s reviewState) mergeAwaitsQuiet() bool {
	return loop.CanMerge(s.pull(), s.listed()) && !s.quiet.CanMerge(s.pull(), s.listed())
}

// notifyPollInterval is how often CI is asked about when the developer only
// wants to be told it finished: a slower beat than an announcement waiting for
// CI reads it at, because a notification the developer stepped away for is
// not in a hurry.
const notifyPollInterval = 3 * time.Minute

// pollInterval is how long to wait before asking about CI again. A configured
// interval is always honored; a notification, asked for in ui, with no
// interval set, is content with a slower beat.
func pollInterval(ui config.UI, deps Deps) time.Duration {
	if ui.Notify && deps.CIInterval <= 0 {
		return notifyPollInterval
	}

	return deps.ciInterval()
}

// pollStage is where CI's poll chain stands: no poll waiting, or one
// scheduled on its timer.
type pollStage int

const (
	// pollIdle has no poll waiting: the next read that finds CI still to
	// come schedules one.
	pollIdle pollStage = iota
	// pollScheduled has a poll waiting on its timer, so a read answering
	// meanwhile, a refresh's, schedules no second one.
	pollScheduled
)

// pollChain is CI's poll chain: whether a poll waits, and the chain, which
// each review begins again, its polls carry back. A poll scheduled for a
// review since replaced ends its chain rather than polling beside the new
// one's. CI's round would not do: a read begun while a poll waits, a
// refresh's, would drop the poll before it let the chain go on.
type pollChain struct {
	stage pollStage
	chain int
}

// begun is the chain a new review polls on: nothing waiting, and a chain no
// poll scheduled before carries.
func (p pollChain) begun() pollChain {
	return pollChain{stage: pollIdle, chain: p.chain + 1}
}

// scheduled is the chain with a poll scheduled, the chain that poll
// carries, and true; while one already waits, it is unchanged and false.
func (p pollChain) scheduled() (pollChain, int, bool) {
	if p.stage == pollScheduled {
		return p, p.chain, false
	}

	return pollChain{stage: pollScheduled, chain: p.chain}, p.chain, true
}

// fired is the chain once the poll that carries chain fires: idle, and
// true. A poll of a chain since begun again changes nothing: false.
func (p pollChain) fired(chain int) (pollChain, bool) {
	if chain != p.chain {
		return p, false
	}

	return pollChain{stage: pollIdle, chain: p.chain}, true
}

// ciPoll is time to ask about CI again, on the poll chain it was scheduled
// in. A poll of a chain a new review has begun again since is stale.
type ciPoll struct {
	chain int
}

var _ applier = ciPoll{}

// apply asks again, unless it belongs to a review that has since been replaced,
// in which case it does nothing and its chain ends here.
func (msg ciPoll) apply(m Model) (Model, tea.Cmd) {
	polls, fired := m.polls.fired(msg.chain)
	if !fired {
		return m, nil
	}

	m.polls = polls

	return m.checkCI()
}

// ciGlyph is how the branch's CI stands, by shape.
func (s reviewState) ciGlyph(kit renderKit) string {
	return kit.ciGlyph(s.listed().State)
}

// ciSummary says how CI stands in words: that it is being read, how the
// latest read found it, or why that read failed.
func (s reviewState) ciSummary(kit renderKit) string {
	// Beside "CI", reading needs no mark of its own.
	reading := func() string { return "reading" + kit.marks.ellipsis }

	failed := func() string { return kit.failureSummary(s.ci.Err()) }

	return drawRead(s.ci, drawnRead[string]{
		reading: reading, shown: func() string { return s.ciShown(kit) }, failed: failed, failedOver: failed,
	})
}

// ciShown says how CI stands as the latest read found it: none reported, on
// a pull request whose merge waits on its head's quiet, as checks not
// reported yet.
func (s reviewState) ciShown(kit renderKit) string {
	reported := s.listed()

	switch {
	case reported.State == forge.CINone && s.mergeAwaitsQuiet():
		return s.ciGlyph(kit) + " no checks reported yet" + s.checkedAtSuffix(kit)
	case reported.State == forge.CINone:
		return s.ciGlyph(kit) + " no checks reported" + s.checkedAtSuffix(kit)
	}

	state := reported.State.Word()

	if reported.Total > 0 {
		state += " (" + strconv.Itoa(reported.Done) + " of " + strconv.Itoa(reported.Total) + " finished)"
	}

	return s.ciGlyph(kit) + " " + state + s.checkedAtSuffix(kit)
}

// ciBarWidth is how many columns the Review detail's bar of checks finished
// takes.
const ciBarWidth = 24

// ciBar is a bar of the share of CI's checks finished, on its way there, in
// the forge's hue and lined up under the CI line's words; nothing where the
// forge reports no checks one by one.
func (m Model) ciBar() []string {
	if m.review.listed().Total == 0 {
		return nil
	}

	return []string{strings.Repeat(" ", len("CI     ")) + m.kit().bar(m.motion.checks.shown, ciBarWidth, m.styles.forge)}
}

// checkedAtSuffix says when CI was last read, for a line that already says
// how it stands.
func (s reviewState) checkedAtSuffix(kit renderKit) string {
	if s.checkedAt.IsZero() {
		return ""
	}

	return kit.marks.separator + "checked " + s.checkedAt.Format(ciCheckedFormat)
}

// canRerun reports a failed pull request whose checks can be re-run. A pull
// request that has merged is left alone even if a stale CI read still reads as
// failed: there is nothing to re-run once it is in.
func (s reviewState) canRerun(deps Deps) bool {
	return s.found() && deps.Forge.Rerun != nil && s.ci.Stage() == latest.StageShown && loop.CanRerun(s.pull(), s.listed())
}

// previewRerun holds the re-run of the failed checks for a last look, naming the
// pull request it restarts CI on: a forge write, which goes only once confirmed.
func (m Model) previewRerun() (Model, tea.Cmd) {
	if !m.review.canRerun(m.deps) {
		return m, nil
	}

	pull, head := m.review.pull(), m.branch.checkedOut().Head
	m.overlay = lastLook{
		title: "Re-run checks",
		body:  "Re-run the failed checks on " + m.vocab.sigil + strconv.Itoa(pull.Number) + " " + pull.Title + "?",
		verb:  "re-run", doing: "re-running",
		proceed: func(m Model) (Model, tea.Cmd) { return m.rerunChecks(pull, head) },
	}

	return m, nil
}

// rerunChecks asks the forge to re-run the failed CI on the pull request the
// look named, the look open and in flight until the forge answers.
func (m Model) rerunChecks(pull forge.PullRequest, head string) (Model, tea.Cmd) {
	if m.dryRun {
		return m.closeOverlay().noticed("dry run: would re-run the failed checks"), nil
	}

	ctx, rerun, chain := m.session, m.deps.Forge.Rerun, m.polls.chain

	return m.sendingLook(func(round int) tea.Msg {
		reran, err := rerun(ctx, pull, head)

		return rerunRequested{round: round, chain: chain, pull: pull.Number, reran: reran, err: err}
	})
}

// rerunRequested is the outcome of asking the forge to re-run the failed
// checks of the pull request numbered pull, for the look that sent it in
// round, on the review whose poll chain was chain.
type rerunRequested struct {
	round, chain, pull int
	reran              bool
	err                error
}

var _ applier = rerunRequested{}

// apply returns the pane to "running" and restarts the poll once a re-run has
// started, or says nothing could be re-run — a failure the forge has no
// re-runnable job for, so the pane must not claim one — closing the look either
// way. A refusal stays pinned in the look, and says why in the notice. A
// review since replaced, by another pull request found or none, is left as
// it is, the re-run being of CI it no longer shows; the notice says which
// pull request's checks were re-run.
func (msg rerunRequested) apply(m Model) (Model, tea.Cmd) {
	// The look that asked is the one answered; nothing else is touched.
	m = m.answerLook(msg.round, msg.err)
	if msg.err != nil {
		return m.noticedFailureLedBy("re-run failed: ", msg.err), nil
	}

	if !msg.reran {
		return m.noticedFailure(errNothingToRerun), nil
	}

	if msg.chain != m.polls.chain {
		return m.noticed(m.marks.done + " re-ran the failed checks on " + m.vocab.sigil + strconv.Itoa(msg.pull)), nil
	}

	// Told, not read: a read begun before the re-run would list the failure
	// re-run, and is dropped.
	m.review.reran, m.review.reranAt = failedRuns(m.review.listed()), m.deps.now()
	m.review.ci = m.review.ci.Told(forge.CI{State: forge.CIRunning})

	// The re-run has only just started, so a check now would still read the old
	// failure; let the poll the set-to-running schedules read it once it moves.
	return m.keepPolling(nil)
}

// failedRuns are the ids of CI's failed checks that a re-run re-runs: the
// forge's own runs and jobs, whose logs it serves. A status reported by
// something else has no run to re-run, and fails on as it did.
func failedRuns(ci forge.CI) []string {
	var ids []string

	for _, check := range ci.Checks {
		if check.State == forge.CIFailed && check.LogAvailable {
			ids = append(ids, check.ID)
		}
	}

	return ids
}

// rerunPatience is how long a re-run is waited for while a read still lists a
// run it was asked for: the forge starts one within seconds, and a failed run
// it leaves alone — a job of a run still going, or one allowed to fail — would
// otherwise keep CI running for as long as the head stands.
const rerunPatience = time.Minute

// awaitingRerun reports a read of CI, made at now within rerunPatience of the
// re-run, that still lists, failed, a run the re-run was asked for: each run
// re-run is listed under an id of its own once the forge starts it, so the
// read is of the failure re-run, not of the re-run.
func (s reviewState) awaitingRerun(ci forge.CI, now time.Time) bool {
	if now.Sub(s.reranAt) >= rerunPatience {
		return false
	}

	return slices.ContainsFunc(ci.Checks, func(check forge.Check) bool {
		return check.State == forge.CIFailed && slices.Contains(s.reran, check.ID)
	})
}
