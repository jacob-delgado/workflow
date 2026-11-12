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
	"github.com/jacob-delgado/workflow/internal/armed"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/loop"
	"github.com/jacob-delgado/workflow/internal/messaging"
	"github.com/jacob-delgado/workflow/internal/webserver/problem"
)

var (
	// errNothingHeld is a stop asked for with no announcement waiting for CI.
	errNothingHeld = errors.New("no announcement is waiting for CI; it was posted, dropped, stopped, or never held")
	// errHeldAnnouncing is a stop, a hold or a post asked for while the
	// announcement held for CI is being posted, which it would race.
	errHeldAnnouncing = errors.New("the announcement held for CI is being posted now; it goes once, " +
		"so ask again once it has")
)

// heldSlot is the announcement held until its pull request's CI passes, as
// the terminal's w in the preview holds one: one armed.Hold, which the server
// steps on its own reads of the branch it was armed on and its CI, on the CI
// interval, whether or not a page is open, and which a frame only shows. It
// lives in memory alone: a server that stops, or is switched from, retires it,
// so one waiting is lost unposted, and a post under way finishes, its claim
// keeping any other from going. Its lock is its own, since the timed reads,
// the frames and the writes all reach it.
type heldSlot struct {
	mu   sync.Mutex
	hold armed.Hold
	// memory is what posting the hold remembers: the store's claims, and the
	// groups the post it was asked for tags.
	memory loop.AnnounceMemory
	// timer is the last read set for later; retired, once set, sets none.
	timer   *time.Timer
	retired bool
}

// retire stops the next read set and the hold's wait, as the server holding
// it ends: one waiting is lost, unposted, and one being posted finishes.
func (h *heldSlot) retire() {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.retired = true
	if h.timer != nil {
		h.timer.Stop()
	}

	h.hold = h.hold.Retire()
}

// announceWhenGreen holds a ready-for-review announcement until its pull
// request's CI passes, reading it at once under ctx: answered 200 when it has
// passed already and the post went, else 202 with the hold as it stands. One
// asked for over another pull request's waiting announcement replaces it, and
// the answer names it; one asked for while a held one is being posted, and
// one not ready for review, are 409.
func (s *server) announceWhenGreen(ctx context.Context, post announcePost) api.AnnounceResponseObject {
	if post.delivery.Moment != messaging.MomentReady {
		return api.Announce409ApplicationProblemPlusJSONResponse(problem.New(api.ProblemCodeConflict,
			"only a ready-for-review announcement waits for CI; this one announces a merge, so announce it now"))
	}

	started, err := s.arm(post)
	if err != nil {
		return api.Announce409ApplicationProblemPlusJSONResponse(problem.New(api.ProblemCodeConflict, heldRefusal(err)))
	}

	s.advance(ctx, started.Next)

	hold, _ := s.heldNow()
	if hold.Stage() == armed.StagePosted {
		posted := announcementDTO(hold.Delivery().Text, hold.Delivery().Channel)
		posted.Warning = heldWarning(hold)

		return api.Announce200JSONResponse(posted)
	}

	queued := s.queuedOf(hold)
	if replaced := started.Replaced; replaced != nil {
		queued.ReplacedPull = new(replaced.Pull())
	}

	return api.Announce202JSONResponse(queued)
}

// canWaitForCI reports whether an announcement at moment can be held for its
// pull request's CI, read as ci when read: it is ready for review and its CI
// has not passed.
func canWaitForCI(moment messaging.Moment, ci forge.CI, read bool) bool {
	return moment == messaging.MomentReady && (!read || ci.State != forge.CIPassed)
}

// previewCI is how pull's CI stands now, read under ctx at the commit the
// forge says it is at, fresh rather than from the stream's held answer, and
// whether it could be read.
func (s *server) previewCI(ctx context.Context, pull forge.PullRequest) (forge.CI, bool) {
	if s.deps.Forge.CheckStatus == nil {
		return forge.CI{}, false
	}

	ci, err := s.deps.Forge.CheckStatus(ctx, pull, pull.HeadCommit)

	return ci, err == nil
}

// arm arms post to go once its pull request's CI passes, on its branch, as
// the hold steps it: asked for, it replaces another pull request's waiting
// announcement; armed as its pull request opened, it never does, and tags
// whom it proposes as it posts.
func (s *server) arm(post announcePost) (armed.Armed, error) {
	armedAt := s.now()

	s.held.mu.Lock()
	defer s.held.mu.Unlock()

	started, err := s.held.hold.Arm(armedAt, armed.Arming{
		Delivery: post.delivery, Branch: post.branch, Opened: post.automatic, Unasked: post.automatic,
		Automatic: post.automatic, Seen: 0,
	})
	if err != nil {
		return started, err
	}

	s.held.hold, s.held.memory = started.Hold, post.memory

	return started, nil
}

// heldRefusal is why the hold refused what was asked of it: it is being
// posted, or, to stop it, nothing waits.
func heldRefusal(err error) string {
	if errors.Is(err, armed.ErrBeingPosted) {
		return errHeldAnnouncing.Error()
	}

	return errNothingHeld.Error()
}

// advance does what the hold asks, under ctx, until it asks for nothing more
// now: a read, its post, or a read once the CI interval has passed, which it
// sets for later.
func (s *server) advance(ctx context.Context, next armed.Next) {
	for next != armed.NextNothing {
		next = map[armed.Next]func() armed.Next{
			armed.NextNothing:   func() armed.Next { return armed.NextNothing },
			armed.NextRead:      func() armed.Next { return s.readHeld(ctx) },
			armed.NextReadLater: s.readHeldLater,
			armed.NextDeliver:   func() armed.Next { return s.deliverHeld(ctx) },
		}[next]()
	}
}

// heldNow is the hold as it stands, and the seams it reads and posts
// through, remembering what its post does.
func (s *server) heldNow() (armed.Hold, armed.Seams) {
	s.held.mu.Lock()
	defer s.held.mu.Unlock()

	return s.held.hold, s.heldSeams(s.held.memory)
}

// heldSeams are what the hold reads and posts through: the branch's pull
// request and its CI, read straight from the forge rather than through the
// stream's cache, which may stand a CI read before in for one that failed;
// the post and its gate; memory; and whom one armed as it opened tags, the
// linked owners of the changes against the base its pull request merges
// into and the groups that start checked.
func (s *server) heldSeams(memory loop.AnnounceMemory) armed.Seams {
	checkCI := s.deps.Forge.CheckStatus
	if checkCI == nil {
		checkCI = func(context.Context, forge.PullRequest, string) (forge.CI, error) {
			return forge.CI{}, loop.ErrCIUnread
		}
	}

	return armed.Seams{
		FindPull: s.deps.Forge.FindPullRequest, CheckCI: checkCI, Deliver: s.deliverSeams(), Memory: memory,
		UnaskedTags: func(ctx context.Context, delivery loop.Delivery) messaging.Mentions {
			return s.unaskedMentions(ctx, delivery.Pull.Base)
		},
	}
}

// deliverSeams are what a post goes through: the service, and the CI read
// its gate makes first.
func (s *server) deliverSeams() loop.DeliverSeams {
	return loop.DeliverSeams{Post: s.deps.Messaging.Post, CheckCI: s.deps.Forge.CheckStatus}
}

// readHeld reads the held pull request and its CI under ctx, and steps the
// hold on what it heard.
func (s *server) readHeld(ctx context.Context) armed.Next {
	hold, seams := s.heldNow()
	reading, failure := seams.Read(ctx, hold, s.now)

	return s.stepHeld(func(held armed.Hold) (armed.Hold, armed.Next) { return held.Hear(reading, failure) })
}

// deliverHeld posts the held announcement under ctx, and steps the hold on
// how it went.
func (s *server) deliverHeld(ctx context.Context) armed.Next {
	hold, seams := s.heldNow()
	outcome := seams.DeliverHeld(ctx, hold, s.now)
	s.noteDelivered(outcome.Err)

	return s.stepHeld(func(held armed.Hold) (armed.Hold, armed.Next) { return held.Delivered(outcome) })
}

// noteDelivered hands Unexpected what a post's outcome says that its frame
// leaves out: why the store could not confirm a post that went, which can
// name its file, and a failure no class tells. The gate's refusal and a claim
// another took first are the hold's own to say.
func (s *server) noteDelivered(err error) {
	_, refused := errors.AsType[loop.GateRefusalError](err)
	_, told := faultProblem(err)

	if err != nil && !refused && !told && !errors.Is(err, loop.ErrAnnouncedAlready) {
		s.unexpected(err)
	}
}

// stepHeld steps the hold on event, answering what it asks next.
func (s *server) stepHeld(event func(armed.Hold) (armed.Hold, armed.Next)) armed.Next {
	s.held.mu.Lock()
	defer s.held.mu.Unlock()

	var next armed.Next

	s.held.hold, next = event(s.held.hold)

	return next
}

// readHeldLater sets the next read of the hold for once the CI interval has
// passed, unless the server is retired. The read runs on the timer's own
// goroutine, as the server's shutdown runs on context.AfterFunc's, under the
// server's lifetime: no page asked for it.
func (s *server) readHeldLater() armed.Next {
	interval := s.forgeInterval()

	s.held.mu.Lock()
	defer s.held.mu.Unlock()

	if !s.held.retired {
		round := s.held.hold.Round()
		s.held.timer = time.AfterFunc(interval, func() { s.readHeldIn(round) })
	}

	return armed.NextNothing
}

// readHeldIn reads the hold, set to in round, while it still waits in that
// round: one stopped, replaced or posted meanwhile is read no more.
func (s *server) readHeldIn(round int) {
	hold, _ := s.heldNow()
	if hold.Round() == round && hold.Stage().Waiting() {
		s.advance(s.worlds.lifetime, armed.NextRead)
	}
}

// supersede settles a waiting hold of made as announced, a post of it asked
// for now having gone out in its place.
func (s *server) supersede(made loop.Announced) {
	s.held.mu.Lock()
	defer s.held.mu.Unlock()

	s.held.hold = s.held.hold.Superseded(made)
}

// heldBeingPosted reports whether the held announcement is being posted now.
func (s *server) heldBeingPosted() bool {
	hold, _ := s.heldNow()

	return hold.Stage() == armed.StageDue
}

// heldStatus is how the held announcement stands, for a frame: nil when none
// was ever held here.
func (s *server) heldStatus() *api.QueuedAnnouncement {
	hold, _ := s.heldNow()
	if hold.Stage() == armed.StageIdle {
		return nil
	}

	shown := s.queuedOf(hold)

	return &shown
}

// CancelQueuedAnnouncement stops the announcement waiting for CI, unposted.
// One not waiting, or being posted, is a 409 that changes nothing.
func (s *server) CancelQueuedAnnouncement(
	_ context.Context, _ api.CancelQueuedAnnouncementRequestObject,
) (api.CancelQueuedAnnouncementResponseObject, error) {
	s.held.mu.Lock()
	defer s.held.mu.Unlock()

	stopped, err := s.held.hold.Stop()
	if err != nil {
		return api.CancelQueuedAnnouncement409ApplicationProblemPlusJSONResponse(
			problem.New(api.ProblemCodeConflict, heldRefusal(err))), nil
	}

	s.held.hold = stopped

	return api.CancelQueuedAnnouncement204Response{}, nil
}

// queuedOf is how hold stands, on the wire: which hold it is, its state,
// what it waits on and the CI it last heard while it waits, why it stands
// there, and, posted, whether the store did not confirm it.
func (s *server) queuedOf(hold armed.Hold) api.QueuedAnnouncement {
	stage := hold.Stage()

	return api.QueuedAnnouncement{
		ID: hold.Round(), State: queuedState(stage), WaitingOn: waitingOn(stage), Ci: heardCI(hold),
		Channel: hold.Delivery().Channel, Pull: hold.Pull(), Reason: optional(s.heldReason(hold)),
		Automatic: optionalTrue(hold.Automatic()), Warning: heldWarning(hold),
	}
}

// queuedState maps the stage a hold stands at onto its wire word. A map, not
// a switch, so there is no last-case arm gobco can never see; exhaustive
// keeps it complete. An idle hold has no word, since a frame then shows no
// held announcement at all.
func queuedState(stage armed.Stage) api.QueuedAnnouncementState {
	return map[armed.Stage]api.QueuedAnnouncementState{
		armed.StageIdle:           "",
		armed.StageListing:        api.QueuedAnnouncementStateWaiting,
		armed.StageAwaitingChecks: api.QueuedAnnouncementStateWaiting,
		armed.StageRunning:        api.QueuedAnnouncementStateWaiting,
		armed.StageAwaitingFix:    api.QueuedAnnouncementStateWaiting,
		armed.StageUnheard:        api.QueuedAnnouncementStateWaiting,
		armed.StageDue:            api.QueuedAnnouncementStateAnnouncing,
		armed.StagePosted:         api.QueuedAnnouncementStateAnnounced,
		armed.StageElsewhere:      api.QueuedAnnouncementStateElsewhere,
		armed.StageDropped:        api.QueuedAnnouncementStateDropped,
		armed.StageStopped:        api.QueuedAnnouncementStateStopped,
	}[stage]
}

// waitingOn is what a hold at stage waits on, on the wire, or nil when it
// does not wait.
func waitingOn(stage armed.Stage) *api.QueuedAnnouncementWaitingOn {
	waits := func(word api.QueuedAnnouncementWaitingOn) *api.QueuedAnnouncementWaitingOn { return &word }

	return map[armed.Stage]*api.QueuedAnnouncementWaitingOn{
		armed.StageIdle: nil, armed.StageDue: nil, armed.StagePosted: nil, armed.StageElsewhere: nil,
		armed.StageDropped: nil, armed.StageStopped: nil,
		armed.StageListing:        waits(api.QueuedAnnouncementWaitingOnListing),
		armed.StageRunning:        waits(api.QueuedAnnouncementWaitingOnCi),
		armed.StageAwaitingChecks: waits(api.QueuedAnnouncementWaitingOnChecks),
		armed.StageAwaitingFix:    waits(api.QueuedAnnouncementWaitingOnFix),
		armed.StageUnheard:        waits(api.QueuedAnnouncementWaitingOnUnread),
	}[stage]
}

// heardCI is the CI hold last heard, while it waits on CI, a check or a fix,
// or nil.
func heardCI(hold armed.Hold) *api.CIState {
	heard := map[armed.Stage]bool{
		armed.StageRunning: true, armed.StageAwaitingChecks: true, armed.StageAwaitingFix: true,
		armed.StageIdle: false, armed.StageListing: false, armed.StageUnheard: false, armed.StageDue: false,
		armed.StagePosted: false, armed.StageElsewhere: false, armed.StageDropped: false, armed.StageStopped: false,
	}
	if !heard[hold.Stage()] {
		return nil
	}

	return new(ciState(hold.CI().State))
}

// heldWarning is what is said of a hold posted whose post the store did not
// confirm, or nil: why the store did not goes to the log alone, since its
// error can name its file.
func heldWarning(hold armed.Hold) *string {
	if hold.Stage() != armed.StagePosted || hold.Reason() == nil {
		return nil
	}

	return new(loop.UnconfirmedWarning)
}

// heldReason is why hold stands where it does, in words that name no host,
// webhook or file: what failed while it waits unread, why it was dropped,
// stopped, or not posted here; nothing at any other stage. A map, not a
// switch, so there is no arm for the stages that say nothing.
func (s *server) heldReason(hold armed.Hold) string {
	none := func() string { return "" }

	return map[armed.Stage]func() string{
		armed.StageUnheard: func() string { return briefly(hold.Reason()) },
		armed.StageDropped: func() string { return s.droppedWords(hold) },
		armed.StageElsewhere: func() string {
			already, _ := errors.AsType[loop.AnnouncedAlreadyError](hold.Reason())

			return s.claimedBefore(hold.Pull(), already.Claim).Detail
		},
		armed.StageStopped: func() string { return hold.Reason().Error() },
		armed.StageIdle:    none, armed.StageListing: none, armed.StageAwaitingChecks: none, armed.StageRunning: none,
		armed.StageAwaitingFix: none, armed.StageDue: none, armed.StagePosted: none,
	}[hold.Stage()]()
}

// droppedWords is why hold was dropped: the forge never listed its pull
// request, or nothing was heard of it, for too long; no check was reported;
// it merged, or began merging, first or is no longer its branch's; or how its
// post failed.
func (s *server) droppedWords(hold armed.Hold) string {
	name, reason := s.pullName(hold.Pull()), hold.Reason()
	patience := minutes(armed.UnheardPatience)

	// In order: an UnheardError matches ErrNotListedYet when the forge never
	// listed it, which says more.
	causes := []struct {
		cause error
		words func() string
	}{
		{armed.ErrNotListedYet, func() string { return "the forge did not list " + name + " within " + patience }},
		{armed.ErrUnheard, func() string {
			unheard, _ := errors.AsType[armed.UnheardError](reason)

			return "nothing could be heard of " + name + " for " + patience + ": " + briefly(unheard.Last)
		}},
		{armed.ErrNoCI, func() string {
			return "no checks reported on " + name + " within " + minutes(armed.NoCIPatience) + "; announce it by hand"
		}},
		{armed.ErrRunningTooLong, func() string {
			return "CI on " + name + " has been running for " + hours(armed.RunningPatience)
		}},
		{armed.ErrMergedFirst, func() string { return name + " merged before its CI passed" }},
		{armed.ErrMergingFirst, func() string { return name + " began merging before its CI passed" }},
		{armed.ErrPullChanged, func() string { return name + " is no longer its branch's " + s.noun() }},
		{loop.ErrPostUncertain, func() string { return postUncertain }},
	}

	for _, told := range causes {
		if errors.Is(reason, told.cause) {
			return told.words()
		}
	}

	return briefly(reason)
}

// briefly is err in the words fault gives its class, which name no host,
// webhook or file.
func briefly(err error) string {
	told, _ := faultProblem(err)

	return told.Detail
}

// minutes is patience in whole minutes, as "10m".
func minutes(patience time.Duration) string {
	return strconv.Itoa(int(patience.Minutes())) + "m"
}

// hours is patience in whole hours, as "2h".
func hours(patience time.Duration) string {
	return strconv.Itoa(int(patience.Hours())) + "h"
}
