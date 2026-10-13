// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver

import (
	"cmp"
	"context"
	"errors"
	"strconv"
	"sync"
	"time"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/loop"
	"github.com/jacob-delgado/workflow/internal/messaging"
)

var (
	// errNothingHeld is a drop asked for with no announcement waiting for CI.
	errNothingHeld = errors.New("no announcement is waiting for CI; it was posted, dropped, or never held")
	// errHeldAnnouncing is a drop, a hold or a post asked for while the
	// announcement held for CI is being posted, which it would race.
	errHeldAnnouncing = errors.New("the announcement held for CI is being posted now; it goes once, " +
		"so ask again once it has")
)

// heldAnnouncement is the announcement held until a pull request's CI passes,
// as the terminal's w in the preview holds one while it is open; the server
// holds it while it runs. The server reads the CI for it on its own, on the
// forge's interval, so it goes whether or not a page is open, and each frame
// of the stream settles it too, so a page sees it go on the first frame with
// green CI. It lives in memory alone: a server that stops, or is switched to
// another directory, loses it unposted. round counts each hold and drop, so a
// read of the CI set for one before stops. Once taken to post it is neither
// replaced nor dropped until the post is done. Its lock is its own, since the
// stream, the timed reads of the CI and the writes all reach it.
type heldAnnouncement struct {
	mu    sync.Mutex
	post  announcePost
	round int
	// branch is what the announcement was written for, with the pull request
	// it announces: a branch that now has another pull request, or none, is
	// another moment.
	branch string
	// state is how it stands, reason why it was dropped, when it was, and
	// warning what is said of it once posted, when the store could not
	// remember it.
	state   heldState
	reason  string
	warning string
	// check is the timer of the next read of the CI for the announcement
	// waiting now; nil when none waits.
	check *time.Timer
	// watch is how long its CI has reported no check, and ci how it stood on
	// the last read, when ciRead says one answered.
	watch  loop.CIWatch
	ci     forge.CIState
	ciRead bool
}

// heldState is how the held announcement stands.
type heldState int

const (
	// heldNone is none held, or the last dropped by hand or replaced by a
	// post made now: a frame shows none.
	heldNone heldState = iota
	// heldWaiting waits for its CI to pass.
	heldWaiting
	// heldAnnouncing is being posted, once its CI passed.
	heldAnnouncing
	// heldAnnounced was posted.
	heldAnnounced
	// heldDropped was given up on, unposted, for its reason.
	heldDropped
)

// announceWhenGreen holds a ready-for-review announcement until its pull
// request's CI passes, answering 202 with it waiting — while CI runs, reports
// no check yet, has a check failed, which it waits out for a fix, or cannot be
// read; posts it at once when CI has passed already; and refuses one that is
// not ready for review. One posted at once is posted under ctx.
func (s *server) announceWhenGreen(ctx context.Context, post announcePost) api.AnnounceResponseObject {
	if post.delivery.Moment != messaging.MomentReady {
		return api.Announce409ApplicationProblemPlusJSONResponse(problem(api.ProblemCodeConflict,
			"only a ready-for-review announcement waits for CI; this one announces a merge, so announce it now"))
	}

	branch, ci, read := s.readCI(ctx, post.delivery.Pull)
	if read && ci.State == forge.CIPassed {
		return s.announceNow(ctx, post)
	}

	return s.holdAnswer(post, cmp.Or(branch.Name, post.branch), ciReading{ci: ci, read: read})
}

// canWaitForCI reports whether an announcement at moment can be held for its
// pull request's CI, read as ci when read: it is ready for review and its CI
// has not passed.
func canWaitForCI(moment messaging.Moment, ci forge.CI, read bool) bool {
	return moment == messaging.MomentReady && (!read || ci.State != forge.CIPassed)
}

// readCI is the checked-out branch and where the CI of its pull request stands
// now, read fresh rather than from the stream's held answer: a hold decided on
// an answer an interval old could wait for a CI that has finished. It reports
// false when the branch or the CI could not be read, or there is no way to
// read CI.
func (s *server) readCI(ctx context.Context, pull forge.PullRequest) (gitrepo.Branch, forge.CI, bool) {
	branch, err := s.readBranch(ctx)
	if err != nil || s.deps.Forge.CheckStatus == nil {
		return branch, forge.CI{}, false
	}

	ci, err := s.deps.Forge.CheckStatus(ctx, pull, branch.Head)

	return branch, ci, err == nil
}

// ciReading is a read of a pull request's CI, and whether it answered.
type ciReading struct {
	ci   forge.CI
	read bool
}

// holdAnswer holds post, written for branch and its CI as read, answering
// 202 with it waiting, or 409 while the one held before it is being posted.
func (s *server) holdAnswer(post announcePost, branch string, reading ciReading) api.AnnounceResponseObject {
	queued, err := s.hold(post, branch, reading)
	if err != nil {
		return api.Announce409ApplicationProblemPlusJSONResponse(problem(api.ProblemCodeConflict, err.Error()))
	}

	return api.Announce202JSONResponse(queued)
}

// hold keeps post, written for branch, until its pull request's CI passes,
// replacing any held before it, its patience with no check counted from now
// and its CI as read, and sets the first of its reads of the CI; it refuses
// while the one before is being posted.
func (s *server) hold(post announcePost, branch string, reading ciReading) (api.QueuedAnnouncement, error) {
	s.held.mu.Lock()
	defer s.held.mu.Unlock()

	if s.held.state == heldAnnouncing {
		return api.QueuedAnnouncement{}, errHeldAnnouncing
	}

	s.held.replace(post, branch, heldWaiting)
	s.held.watch = loop.WatchFrom(s.now())

	if reading.read {
		s.held.watched(reading.ci, s.now())
	}

	s.held.check = s.checkHeldAfter(s.held.round)

	return s.held.queued(), nil
}

// checkHeldAfter reads the CI for the announcement held in round once the
// forge interval has passed, settling it, and goes on doing so each interval
// while it waits — whether or not a page is open. It reads through the
// stream's forge cache, so with a page open it asks the forge no more often
// than the page alone would. The read runs on the timer's own goroutine, as
// the server's shutdown runs on context.AfterFunc's, and it stops once the
// announcement is settled, dropped or replaced, or the server is retired by a
// switch.
func (s *server) checkHeldAfter(round int) *time.Timer {
	return time.AfterFunc(s.forgeInterval(), func() { s.checkHeld(round) })
}

// checkHeld reads the CI for the announcement held in round, settles it, and
// sets the next read while it still waits. No page asked for the read, so it
// is made under the server's lifetime.
func (s *server) checkHeld(round int) {
	select {
	case <-s.retired:
		return
	default:
	}

	s.settleOnRead(s.worlds.lifetime)

	s.held.mu.Lock()
	defer s.held.mu.Unlock()

	if s.held.round == round && s.held.state == heldWaiting {
		// A gate that refused it may have set a read already: one goes.
		s.held.stopWatching()
		s.held.check = s.checkHeldAfter(round)
	}
}

// settleOnRead settles the held announcement against what the branch and the
// forge, asked under ctx, say now. A read that failed says nothing of the pull
// request, so it neither posts the announcement nor drops it.
func (s *server) settleOnRead(ctx context.Context) {
	branch, err := s.frameBranch(ctx)
	if err != nil {
		return
	}

	read, err := s.forgeReview(ctx, branch)
	if err == nil {
		s.settleHeld(branch, read)
	}
}

// settleHeld settles the held announcement against what the forge read of
// branch, by verdictOn: posts it, drops it with the reason, or keeps it
// waiting. One it posts goes as the pull request and the head that read
// passed on, which a fix pushed since it was held has moved, so the gate
// reads the CI that let it go. Only one caller can take it to post, and
// nothing replaces or drops it while it posts, so it is never posted twice.
func (s *server) settleHeld(branch gitrepo.Branch, read forgeRead) {
	s.held.mu.Lock()

	if s.held.state != heldWaiting {
		s.held.mu.Unlock()

		return
	}

	reason, verdict := s.verdictOn(branch, read)

	switch verdict {
	case heldKeep:
		s.held.mu.Unlock()
	case heldDrop:
		s.held.settle(heldDropped, reason)
		s.held.mu.Unlock()
	case heldPost:
		s.held.post.delivery.Pull, s.held.post.delivery.Head = read.pull, branch.Head
		post := s.held.post
		s.held.settle(heldAnnouncing, "")
		s.held.mu.Unlock()

		s.postHeld(post)
	}
}

// heldVerdict is what becomes of the held announcement on a read of its CI:
// kept waiting, dropped, or posted.
type heldVerdict int

const (
	heldKeep heldVerdict = iota
	heldDrop
	heldPost
)

// verdictOn is what becomes of the held announcement given what the forge
// read of branch, and the reason when it is dropped: posted once its CI has
// passed, kept waiting through a failed run for a fix, and dropped once its
// pull request is no longer the branch's open one, or no check was reported
// within loop.NoCIPatience. The caller holds the lock.
func (s *server) verdictOn(branch gitrepo.Branch, read forgeRead) (string, heldVerdict) {
	number := s.held.post.delivery.Pull.Number

	err := loop.StillArmed(number, read.pull, read.found)
	if branch.Name != s.held.branch {
		err = loop.ErrPullChanged
	}

	switch {
	case errors.Is(err, loop.ErrMergedFirst):
		return s.pullName(number) + " merged before its CI passed", heldDrop
	case err != nil:
		return s.pullName(number) + " is no longer this branch's " + s.noun(), heldDrop
	case !read.ciRead:
		return "", heldKeep
	}

	return s.verdictOnCI(number, s.held.watched(read.ci, s.now()))
}

// verdictOnCI is what becomes of the held announcement for pull request
// number, standing as armed on a read of its CI. The caller holds the lock.
func (s *server) verdictOnCI(number int, armed loop.Armed) (string, heldVerdict) {
	switch armed {
	case loop.ArmedDue:
		return "", heldPost
	case loop.ArmedNoCI:
		patience := strconv.Itoa(int(loop.NoCIPatience.Minutes())) + "m"

		return "no checks reported on " + s.pullName(number) + " within " + patience + "; announce it by hand", heldDrop
	case loop.ArmedWaiting, loop.ArmedAwaitingChecks, loop.ArmedAwaitingFix:
	}

	return "", heldKeep
}

// postHeld posts the announcement taken to post, and records how it went,
// with the warning when the store could not remember it: a moment announced
// meanwhile, from a page or a terminal, drops it unposted, and one the gate
// refused — a check failed since the read that let it go, or the CI could not
// be read — goes back to waiting. It posts under the
// server's lifetime, whether a frame or the timer took it to post: no page
// asked for it, and it goes once, so none leaving may stop it halfway.
func (s *server) postHeld(post announcePost) {
	warning, err := s.deliver(s.worlds.lifetime, post)

	s.held.mu.Lock()
	defer s.held.mu.Unlock()

	switch {
	case errors.Is(err, errAnnouncedAlready):
		s.held.settle(heldDropped, s.announcedBefore(post.delivery.Pull.Number).Detail)
	case errors.Is(err, loop.ErrCIFailed) || errors.Is(err, loop.ErrCIUnread):
		s.held.waitAgain(err)
		s.held.check = s.checkHeldAfter(s.held.round)
	case err != nil:
		s.held.settle(heldDropped, s.fault(err).Detail)
	default:
		s.held.settle(heldAnnounced, "")
		s.held.warning = warning
	}
}

// droppedHold is the announcement dropHeld took from waiting, to put back
// should what replaced it not go: whether one waited, and as it stood.
type droppedHold struct {
	waiting bool
	post    announcePost
	branch  string
	watch   loop.CIWatch
	ci      forge.CIState
	ciRead  bool
	round   int
}

// dropHeld drops the announcement waiting for CI, unposted, and forgets what
// was said of the last one, answering it as it stood; it refuses while one is
// being posted, which goes on and is reported as it ends.
func (s *server) dropHeld() (droppedHold, error) {
	s.held.mu.Lock()
	defer s.held.mu.Unlock()

	if s.held.state == heldAnnouncing {
		return droppedHold{}, errHeldAnnouncing
	}

	dropped := droppedHold{
		waiting: s.held.state == heldWaiting, post: s.held.post, branch: s.held.branch, watch: s.held.watch,
		ci: s.held.ci, ciRead: s.held.ciRead,
	}

	s.held.replace(announcePost{}, "", heldNone)
	dropped.round = s.held.round

	return dropped, nil
}

// restoreHeld puts back the announcement dropped waiting, as it stood, the
// post made in its place having been refused by the gate, unless another
// was held or dropped since.
func (s *server) restoreHeld(dropped droppedHold) {
	s.held.mu.Lock()
	defer s.held.mu.Unlock()

	if !dropped.waiting || s.held.round != dropped.round {
		return
	}

	s.held.replace(dropped.post, dropped.branch, heldWaiting)
	s.held.watch, s.held.ci, s.held.ciRead = dropped.watch, dropped.ci, dropped.ciRead
	s.held.check = s.checkHeldAfter(s.held.round)
}

// heldStatus is how the held announcement stands, for a frame: nil when none
// was held, or the last was dropped by hand or replaced by a post.
func (s *server) heldStatus() *api.QueuedAnnouncement {
	s.held.mu.Lock()
	defer s.held.mu.Unlock()

	if s.held.state == heldNone {
		return nil
	}

	shown := s.held.queued()

	return &shown
}

// CancelQueuedAnnouncement drops the announcement waiting for CI, unposted.
func (s *server) CancelQueuedAnnouncement(
	_ context.Context, _ api.CancelQueuedAnnouncementRequestObject,
) (api.CancelQueuedAnnouncementResponseObject, error) {
	dropped, err := s.dropHeld()
	if err == nil && !dropped.waiting {
		err = errNothingHeld
	}

	if err != nil {
		return api.CancelQueuedAnnouncement409ApplicationProblemPlusJSONResponse(
			problem(api.ProblemCodeConflict, err.Error())), nil
	}

	return api.CancelQueuedAnnouncement204Response{}, nil
}

// replace holds post, written for branch, at state in place of the one
// before, with nothing said of it yet: it stops the reads of the CI for the
// one before and counts the round. The caller holds the lock.
func (h *heldAnnouncement) replace(post announcePost, branch string, state heldState) {
	h.stopWatching()
	h.round++
	h.post, h.branch, h.state, h.reason, h.warning = post, branch, state, "", ""
	h.ci, h.ciRead = forge.CINone, false
}

// watched is how the held announcement stands on ci, read at readAt, noting
// the read. The caller holds the lock.
func (h *heldAnnouncement) watched(ci forge.CI, readAt time.Time) loop.Armed {
	var armed loop.Armed

	h.watch, armed = h.watch.Read(ci, readAt)
	h.ci, h.ciRead = ci.State, true

	return armed
}

// waitAgain puts the announcement taken to post back to waiting, the gate
// having refused it with err: a failed check is its CI as last read. The
// caller holds the lock.
func (h *heldAnnouncement) waitAgain(err error) {
	h.state = heldWaiting

	if errors.Is(err, loop.ErrCIFailed) {
		h.ci, h.ciRead = forge.CIFailed, true
	}
}

// settle moves the held announcement to state, with the reason when it was
// dropped, and stops its reads of the CI. The caller holds the lock.
func (h *heldAnnouncement) settle(state heldState, reason string) {
	h.stopWatching()
	h.state, h.reason = state, reason
}

// queued is how the held announcement stands, on the wire. The caller holds
// the lock.
func (h *heldAnnouncement) queued() api.QueuedAnnouncement {
	queued := api.QueuedAnnouncement{
		State: queuedState(h.state), Channel: h.post.delivery.Channel, Pull: h.post.delivery.Pull.Number,
		Reason: optional(h.reason), Warning: optional(h.warning),
	}

	if h.ciRead {
		queued.Ci = new(ciState(h.ci))
	}

	return queued
}

// stopWatching stops the next read of the CI for the announcement waiting,
// if one is set. The caller holds the lock.
func (h *heldAnnouncement) stopWatching() {
	if h.check != nil {
		h.check.Stop()
		h.check = nil
	}
}

// pullName names a pull request by its number, in the forge's own sigil.
func (s *server) pullName(number int) string {
	return s.forgeKindNow().Sigil() + strconv.Itoa(number)
}
