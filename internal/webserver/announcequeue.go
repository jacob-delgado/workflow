// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver

import (
	"context"
	"errors"
	"slices"
	"strconv"
	"sync"
	"time"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/loop"
	"github.com/jacob-delgado/workflow/internal/messaging"
)

// failedAtFormat stamps an announcement dropped for a failed CI with the time
// it was given up on, as the terminal does.
const failedAtFormat = "15:04"

// errNothingHeld is a drop asked for with no announcement waiting for CI.
var errNothingHeld = errors.New("no announcement is waiting for CI; it was posted, dropped, or never held")

// heldAnnouncement is the announcement held until a pull request's CI passes,
// as the terminal's w in the preview holds one while it is open; the server
// holds it while it runs. The server reads the CI for it on its own, on the
// forge's interval, so it goes whether or not a page is open, and each frame
// of the stream settles it too, so a page sees it go on the first frame with
// green CI. It lives in memory alone: a server that stops, or is switched to
// another directory, loses it unposted. round counts each hold and drop, so a
// post that finishes after the announcement was replaced or dropped leaves
// the newer state be. Its lock is its own, since the stream, the timed reads
// of the CI and the writes all reach it.
type heldAnnouncement struct {
	mu    sync.Mutex
	post  announcePost
	round int
	// branch and pull are what the announcement was written for: a branch
	// that now has another pull request, or none, is another moment.
	branch string
	shown  api.QueuedAnnouncement
	// check is the timer of the next read of the CI for the announcement
	// waiting now; nil when none waits.
	check *time.Timer
}

// announceWhenGreen holds a ready-for-review announcement until its pull
// request's CI passes, answering 202 with it waiting; posts it at once when CI
// has passed already; and refuses one that is not ready for review, or whose
// pull request reports no CI to wait for.
func (s *server) announceWhenGreen(post announcePost) api.AnnounceResponseObject {
	if post.moment != messaging.MomentReady {
		return api.Announce409ApplicationProblemPlusJSONResponse(problem(api.Conflict,
			"only a ready-for-review announcement waits for CI; this one announces a merge or a failed CI, "+
				"so announce it now"))
	}

	branch, ci, err := s.ciNow(post.pull)
	if err != nil {
		return s.announceFault(err)
	}

	switch ci.State {
	case forge.CIPassed:
		return s.announceNow(post)
	case forge.CIRunning:
		return api.Announce202JSONResponse(s.hold(post, branch))
	case forge.CINone, forge.CIFailed:
	}

	return api.Announce409ApplicationProblemPlusJSONResponse(problem(api.Conflict,
		s.pullName(post.pull.Number)+" has no running CI to wait for; announce it now"))
}

// canWaitForCI reports whether an announcement at moment for pull can be held
// for its CI: it is ready for review and its CI is running now. A CI that
// cannot be read cannot be waited on.
func (s *server) canWaitForCI(moment messaging.Moment, pull forge.PullRequest) bool {
	if moment != messaging.MomentReady {
		return false
	}

	_, ci, err := s.ciNow(pull)

	return err == nil && ci.State == forge.CIRunning
}

// ciNow is the checked-out branch and where the CI of its pull request stands
// now, read fresh rather than from the stream's held answer: a hold decided on
// an answer an interval old could wait for a CI that has finished. No CI
// seam reads as no CI.
func (s *server) ciNow(pull forge.PullRequest) (gitrepo.Branch, forge.CI, error) {
	branch, err := s.readBranch()
	if err != nil {
		return gitrepo.Branch{}, forge.CI{}, err
	}

	if s.deps.CheckCI == nil {
		return branch, forge.CI{}, nil
	}

	ci, err := s.deps.CheckCI(pull, branch.Head)

	return branch, ci, err
}

// hold keeps post until its pull request's CI passes, replacing any held
// before it, and sets the first of its reads of the CI.
func (s *server) hold(post announcePost, branch gitrepo.Branch) api.QueuedAnnouncement {
	s.held.mu.Lock()
	defer s.held.mu.Unlock()

	s.held.stopWatching()
	s.held.round++
	s.held.post, s.held.branch = post, branch.Name
	s.held.shown = api.QueuedAnnouncement{
		State: api.QueuedWaiting, Channel: post.delivery.Channel, Pull: post.pull.Number, Reason: nil,
	}
	s.held.check = s.checkHeldAfter(s.held.round)

	return s.held.shown
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
// sets the next read while it still waits.
func (s *server) checkHeld(round int) {
	select {
	case <-s.retired:
		return
	default:
	}

	branch, known := s.frameBranch()
	if known {
		s.settleHeld(branch, s.forgeReview(branch))
	}

	s.held.mu.Lock()
	defer s.held.mu.Unlock()

	if s.held.round == round && s.held.shown.State == api.QueuedWaiting {
		s.held.check = s.checkHeldAfter(round)
	}
}

// settleHeld settles the held announcement against branch's review: posts it
// once its CI has passed, drops it with the reason once its CI has failed or
// its pull request is no longer the branch's open one, and otherwise keeps it
// waiting. Only one caller can take it to post, so it is never posted twice.
func (s *server) settleHeld(branch gitrepo.Branch, review api.Review) {
	s.held.mu.Lock()

	if s.held.shown.State != api.QueuedWaiting {
		s.held.mu.Unlock()

		return
	}

	reason, verdict := s.heldVerdict(branch, review)

	switch verdict {
	case heldKeep:
		s.held.mu.Unlock()
	case heldDrop:
		s.held.settle(api.QueuedDropped, reason)
		s.held.mu.Unlock()
	case heldPost:
		post, round := s.held.post, s.held.round
		s.held.settle(api.QueuedAnnouncing, "")
		s.held.mu.Unlock()

		s.postHeld(post, round)
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

// heldVerdict is what becomes of the held announcement given branch's
// review, and the reason when it is dropped. The caller holds the lock.
func (s *server) heldVerdict(branch gitrepo.Branch, review api.Review) (string, heldVerdict) {
	number := s.held.shown.Pull

	switch {
	case branch.Name != s.held.branch || review.Pull == nil || review.Pull.Number != number:
		return s.pullName(number) + " is no longer this branch's " + s.noun(), heldDrop
	case review.Pull.State == api.Merged:
		return s.pullName(number) + " merged before its CI passed", heldDrop
	case review.Ci == nil:
		return "", heldKeep
	case review.Ci.State == api.Passed:
		return "", heldPost
	case review.Ci.State == api.Failed:
		return "CI failed at " + s.now().Format(failedAtFormat), heldDrop
	default:
		return "", heldKeep
	}
}

// postHeld posts the announcement taken to post in round, and records how it
// went, unless it was replaced or dropped meanwhile.
func (s *server) postHeld(post announcePost, round int) {
	err := loop.Deliver(s.deps.Post, post.memory, post.delivery)

	s.held.mu.Lock()
	defer s.held.mu.Unlock()

	if s.held.round != round {
		return
	}

	if err != nil {
		failure, _ := s.fault(err)
		s.held.settle(api.QueuedDropped, failure.Detail)

		return
	}

	s.held.settle(api.QueuedAnnounced, "")
}

// dropHeld drops the announcement waiting for CI, unposted, and forgets what
// was said of the last one, reporting whether one was waiting.
func (s *server) dropHeld() bool {
	s.held.mu.Lock()
	defer s.held.mu.Unlock()

	waiting := s.held.shown.State == api.QueuedWaiting

	s.held.stopWatching()
	s.held.round++
	s.held.post, s.held.branch, s.held.shown = announcePost{}, "", api.QueuedAnnouncement{}

	return waiting
}

// heldStatus is how the held announcement stands, for a frame: nil when none
// was held, or the last was dropped by hand or replaced by a post.
func (s *server) heldStatus() *api.QueuedAnnouncement {
	s.held.mu.Lock()
	defer s.held.mu.Unlock()

	if s.held.shown.State == "" {
		return nil
	}

	shown := s.held.shown

	return &shown
}

// CancelQueuedAnnouncement drops the announcement waiting for CI, unposted.
func (s *server) CancelQueuedAnnouncement(
	_ context.Context, _ api.CancelQueuedAnnouncementRequestObject,
) (api.CancelQueuedAnnouncementResponseObject, error) {
	if !s.dropHeld() {
		return api.CancelQueuedAnnouncement409ApplicationProblemPlusJSONResponse(
			problem(api.Conflict, errNothingHeld.Error())), nil
	}

	return api.CancelQueuedAnnouncement204Response{}, nil
}

// settle moves the held announcement to state, with the reason when it was
// dropped, and stops its reads of the CI. The caller holds the lock.
func (h *heldAnnouncement) settle(state api.QueuedAnnouncementState, reason string) {
	h.stopWatching()
	h.shown.State, h.shown.Reason = state, optional(reason)
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

// announcedCache is what the store remembers announcing in this repository,
// from any surface, read again once the forge's interval has passed — as the
// review it is told against is — so an announcement made from a terminal
// shows within one. One made here is added at once. A dry run reads no store,
// so it holds nothing. Its lock is its own, since every open stream reads it.
type announcedCache struct {
	mu     sync.Mutex
	held   bool
	readAt time.Time
	made   []loop.Announced
}

// recordedAnnouncements is every announcement the store remembers, read again
// once the forge's interval has passed, or none without a store or under a
// dry run.
func (s *server) recordedAnnouncements() []loop.Announced {
	if s.deps.Announced == nil || s.info.DryRun {
		return nil
	}

	s.announced.mu.Lock()
	defer s.announced.mu.Unlock()

	now := s.now()
	if !s.announced.held || now.Sub(s.announced.readAt) >= s.forgeInterval() {
		s.announced.made, s.announced.held, s.announced.readAt = s.deps.Announced(), true, now
	}

	return slices.Clone(s.announced.made)
}

// recordAnnouncement remembers an announcement just made, in the store and in
// what the next frame tells.
func (s *server) recordAnnouncement(made loop.Announced) {
	if s.deps.RecordAnnounce == nil {
		return
	}

	s.deps.RecordAnnounce(made)

	s.announced.mu.Lock()
	defer s.announced.mu.Unlock()

	s.announced.made = append(s.announced.made, made)
}

// announcedAlready reports that made was announced already, from any surface.
func (s *server) announcedAlready(made loop.Announced) bool {
	return loop.AnnounceMemory{Recorded: s.recordedAnnouncements}.Holds(made)
}

// reviewAnnounced reports that the review's pull request was announced at the
// moment it is at now, read from its pull request and CI as the terminal reads
// them.
func (s *server) reviewAnnounced(review api.Review) bool {
	if !review.Found || review.Pull == nil {
		return false
	}

	moment := messaging.MomentReady

	switch {
	case review.Pull.State == api.Merged:
		moment = messaging.MomentMerged
	case review.Ci != nil && review.Ci.State == api.Failed:
		moment = messaging.MomentCIRed
	}

	return s.announcedAlready(loop.Announced{Pull: review.Pull.Number, Moment: moment})
}
