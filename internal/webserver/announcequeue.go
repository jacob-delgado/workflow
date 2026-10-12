// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"time"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/messaging"
)

// failedAtFormat stamps an announcement dropped for a failed CI with the time
// it was given up on, as the terminal does.
const failedAtFormat = "15:04"

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
// request's CI passes, answering 202 with it waiting; posts it at once when CI
// has passed already; and refuses one that is not ready for review, or whose
// pull request reports no CI to wait for. One posted at once is posted under
// ctx.
func (s *server) announceWhenGreen(ctx context.Context, post announcePost) api.AnnounceResponseObject {
	if post.moment != messaging.MomentReady {
		return api.Announce409ApplicationProblemPlusJSONResponse(problem(api.ProblemCodeConflict,
			"only a ready-for-review announcement waits for CI; this one announces a merge or a failed CI, "+
				"so announce it now"))
	}

	branch, ci, err := s.ciNow(ctx, post.pull)
	if err != nil {
		return s.announceFault(err)
	}

	switch ci.State {
	case forge.CIPassed:
		return s.announceNow(ctx, post)
	case forge.CIRunning:
		return s.holdAnswer(post, branch)
	case forge.CINone, forge.CIFailed:
	}

	return api.Announce409ApplicationProblemPlusJSONResponse(problem(api.ProblemCodeConflict,
		s.pullName(post.pull.Number)+" has no running CI to wait for; announce it now"))
}

// canWaitForCI reports whether an announcement at moment for pull can be held
// for its CI: it is ready for review and its CI is running now. A CI that
// cannot be read cannot be waited on.
func (s *server) canWaitForCI(ctx context.Context, moment messaging.Moment, pull forge.PullRequest) bool {
	if moment != messaging.MomentReady {
		return false
	}

	_, ci, err := s.ciNow(ctx, pull)

	return err == nil && ci.State == forge.CIRunning
}

// ciNow is the checked-out branch and where the CI of its pull request stands
// now, read fresh rather than from the stream's held answer: a hold decided on
// an answer an interval old could wait for a CI that has finished. No CI
// seam reads as no CI.
func (s *server) ciNow(ctx context.Context, pull forge.PullRequest) (gitrepo.Branch, forge.CI, error) {
	branch, err := s.readBranch(ctx)
	if err != nil {
		return gitrepo.Branch{}, forge.CI{}, err
	}

	if s.deps.Forge.CheckStatus == nil {
		return branch, forge.CI{}, nil
	}

	ci, err := s.deps.Forge.CheckStatus(ctx, pull, branch.Head)

	return branch, ci, err
}

// holdAnswer holds post, answering 202 with it waiting, or 409 while the
// one held before it is being posted.
func (s *server) holdAnswer(post announcePost, branch gitrepo.Branch) api.AnnounceResponseObject {
	queued, err := s.hold(post, branch)
	if err != nil {
		return api.Announce409ApplicationProblemPlusJSONResponse(problem(api.ProblemCodeConflict, err.Error()))
	}

	return api.Announce202JSONResponse(queued)
}

// hold keeps post until its pull request's CI passes, replacing any held
// before it, and sets the first of its reads of the CI; it refuses while the
// one before is being posted.
func (s *server) hold(post announcePost, branch gitrepo.Branch) (api.QueuedAnnouncement, error) {
	s.held.mu.Lock()
	defer s.held.mu.Unlock()

	if s.held.state == heldAnnouncing {
		return api.QueuedAnnouncement{}, errHeldAnnouncing
	}

	s.held.replace(post, branch.Name, heldWaiting)
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
// branch: posts it once its CI has passed, drops it with the reason once its
// CI has failed or its pull request is no longer the branch's open one, and
// otherwise keeps it waiting. Only one caller can take it to post, and nothing
// replaces or drops it while it posts, so it is never posted twice.
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
// read of branch, and the reason when it is dropped. The caller holds the
// lock.
func (s *server) verdictOn(branch gitrepo.Branch, read forgeRead) (string, heldVerdict) {
	number := s.held.post.pull.Number

	switch {
	case branch.Name != s.held.branch || !read.found || read.pull.Number != number:
		return s.pullName(number) + " is no longer this branch's " + s.noun(), heldDrop
	case read.pull.State == forge.StateMerged:
		return s.pullName(number) + " merged before its CI passed", heldDrop
	case !read.ciRead:
		return "", heldKeep
	case read.ci.State == forge.CIPassed:
		return "", heldPost
	case read.ci.State == forge.CIFailed:
		return "CI failed at " + s.now().Format(failedAtFormat), heldDrop
	default:
		return "", heldKeep
	}
}

// postHeld posts the announcement taken to post, and records how it went,
// with the warning when the store could not remember it: a moment announced
// meanwhile, from a page or a terminal, drops it unposted. It posts under the
// server's lifetime, whether a frame or the timer took it to post: no page
// asked for it, and it goes once, so none leaving may stop it halfway.
func (s *server) postHeld(post announcePost) {
	warning, err := s.deliver(s.worlds.lifetime, post)

	s.held.mu.Lock()
	defer s.held.mu.Unlock()

	switch {
	case errors.Is(err, errAnnouncedAlready):
		s.held.settle(heldDropped, s.announcedBefore(post.pull.Number).Detail)
	case err != nil:
		s.held.settle(heldDropped, s.fault(err).Detail)
	default:
		s.held.settle(heldAnnounced, "")
		s.held.warning = warning
	}
}

// dropHeld drops the announcement waiting for CI, unposted, and forgets what
// was said of the last one, reporting whether one was waiting; it refuses
// while one is being posted, which goes on and is reported as it ends.
func (s *server) dropHeld() (bool, error) {
	s.held.mu.Lock()
	defer s.held.mu.Unlock()

	if s.held.state == heldAnnouncing {
		return false, errHeldAnnouncing
	}

	waiting := s.held.state == heldWaiting

	s.held.replace(announcePost{}, "", heldNone)

	return waiting, nil
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
	waiting, err := s.dropHeld()
	if err == nil && !waiting {
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
	return api.QueuedAnnouncement{
		State: queuedState(h.state), Channel: h.post.delivery.Channel, Pull: h.post.pull.Number,
		Reason: optional(h.reason), Warning: optional(h.warning),
	}
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
