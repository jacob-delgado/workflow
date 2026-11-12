// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package loop

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/messaging"
)

var (
	// ErrAnnounceUnavailable refuses an announcement with no way to post one.
	ErrAnnounceUnavailable = errors.New("announcing is not available")
	// ErrCIFailed refuses an announcement of a pull request a check failed
	// on: what it announces is not ready for anyone to look at.
	ErrCIFailed = errors.New("a check failed on the pull request")
	// ErrCIUnread refuses an announcement whose pull request's CI could not
	// be read, so nothing failing on it could not be confirmed. It wraps the
	// read's own error.
	ErrCIUnread = errors.New("the pull request's CI could not be read")
	// ErrNotClaimed refuses to post an announcement the store could not
	// claim: unclaimed, a post could go out twice, so none went. It wraps the
	// store's error.
	ErrNotClaimed = errors.New("the announcement could not be claimed, so it was not posted")
	// ErrAnnouncedAlready refuses to post an announcement another session or
	// process claimed first: it was posted, or a post of it began.
	ErrAnnouncedAlready = errors.New("the announcement was already made")
	// ErrNotDelivered is a post the service surely did not deliver, whose
	// claim was given up, so it can be announced again. It wraps the
	// service's error.
	ErrNotDelivered = errors.New("the post was not delivered")
	// ErrPostUncertain is a post that may have gone out — the service did
	// not answer, or answered in a way that says neither — whose claim is
	// kept, so nothing posts it again unasked. It wraps the service's error.
	ErrPostUncertain = errors.New("the post may have gone out")
	// ErrUnconfirmed is a post that went out whose claim the store could not
	// confirm: the claim still keeps it from going twice, and it reads as a
	// post that began and was not seen to finish. It wraps the store's error.
	ErrUnconfirmed = errors.New("posted, but the store did not confirm it")
)

// UnconfirmedWarning is what every surface says of an announcement Deliver
// posted but answered ErrUnconfirmed for, after the line saying where it
// went. The terminal and workflow announce follow it with why; the web leaves
// why to the server's log, since the store's error can name its file.
const UnconfirmedWarning = "The store did not confirm it; it is not offered again."

// Announced is one announcement made: a pull request, and the moment it marked.
// A pull request is announced once at each of its moments — ready for review,
// then merged — so the pair is what a later session is told was made.
type Announced struct {
	Pull   int
	Moment messaging.Moment
}

// ClaimKind is what a claim on an announcement came to.
type ClaimKind int

const (
	// ClaimHeld is a claim another took first. It is the zero kind, so a
	// claim that says nothing errs toward not posting.
	ClaimHeld ClaimKind = iota
	// ClaimTaken is a claim this caller took: it, and no one else, posts.
	ClaimTaken
	// ClaimNotKept is no claim, from a store that keeps nothing: the post
	// goes unguarded.
	ClaimNotKept
)

// Claim is what a claim on an announcement came to, and the claim as it
// stands: its generation, when it was taken, and whether its post was
// confirmed.
type Claim struct {
	Kind       ClaimKind
	Generation int
	ClaimedAt  time.Time
	Posted     bool
}

// AnnounceRecord is one announcement claimed in this repository, by any
// session: what it made, its claim's generation and time, and whether its
// post was confirmed. One never confirmed began and was not seen to finish.
type AnnounceRecord struct {
	Made       Announced
	Generation int
	ClaimedAt  time.Time
	Posted     bool
}

// Standing is how a pull request stands, announced, at the moment it is at.
type Standing int

const (
	// StandingNone is not announced at its moment, or not one the loop
	// follows.
	StandingNone Standing = iota
	// StandingPosted was announced, its post confirmed.
	StandingPosted
	// StandingUnconfirmed was claimed, its post begun and never confirmed:
	// it may have gone out, so it is not posted again unasked.
	StandingUnconfirmed
)

// RecordOf is the record among records of made, and whether there is one.
func RecordOf(records []AnnounceRecord, made Announced) (AnnounceRecord, bool) {
	for _, record := range records {
		if record.Made == made {
			return record, true
		}
	}

	return AnnounceRecord{}, false
}

// StandingOf is how pull, as a find answered it, stands among records at the
// moment it is at now: the one rule every surface reads the announce stage,
// and whether to say the pull request was announced, by. One the loop does
// not follow — none found, or closed without merging — stands as none.
func StandingOf(records []AnnounceRecord, pull forge.PullRequest, found bool) Standing {
	if !followed(pull, found) {
		return StandingNone
	}

	record, claimed := RecordOf(records, Announced{Pull: pull.Number, Moment: AnnounceMoment(pull)})

	switch {
	case !claimed:
		return StandingNone
	case record.Posted:
		return StandingPosted
	default:
		return StandingUnconfirmed
	}
}

// AnnounceMemory is what the store keeps of the announcements made in this
// repository, by any session: every one claimed, the claim a post takes
// before it goes, its confirmation once it has gone, its release when it
// surely did not, and the claim taken again for a post asked for again. A nil
// Recorded knows of none; a nil Claim or Reclaim keeps nothing, so the post
// goes unguarded; a nil Confirm or Release does nothing.
//
// RecordGroups remembers the user groups an announcement tagged, as this
// repository's last choice. A nil RecordGroups remembers none; a surface sets
// it only for an announcement that offered groups to choose, since a choice
// of none is a choice too.
type AnnounceMemory struct {
	Recorded     func(ctx context.Context) ([]AnnounceRecord, error)
	Claim        func(ctx context.Context, made Announced) (Claim, error)
	Confirm      func(ctx context.Context, made Announced, generation int) error
	Release      func(ctx context.Context, made Announced, generation int) error
	Reclaim      func(ctx context.Context, made Announced, seen int) (Claim, error)
	RecordGroups func(ctx context.Context, ids []string) error
}

// Records is every announcement claimed, read under ctx: none without a way
// to read them, and the read's error when it fails.
func (m AnnounceMemory) Records(ctx context.Context) ([]AnnounceRecord, error) {
	if m.Recorded == nil {
		return nil, nil
	}

	return m.Recorded(ctx)
}

// notKept is the claim a memory that keeps nothing answers.
func notKept() Claim {
	return Claim{Kind: ClaimNotKept, Generation: 0, ClaimedAt: time.Time{}, Posted: false}
}

// claim claims made under ctx, or keeps nothing without a way to.
func (m AnnounceMemory) claim(ctx context.Context, made Announced) (Claim, error) {
	if m.Claim == nil {
		return notKept(), nil
	}

	return m.Claim(ctx, made)
}

// reclaimFrom is the claim of made again, under ctx, if its claim is still at
// generation seen, or nothing kept without a way to.
func (m AnnounceMemory) reclaimFrom(seen int) func(ctx context.Context, made Announced) (Claim, error) {
	return func(ctx context.Context, made Announced) (Claim, error) {
		if m.Reclaim == nil {
			return notKept(), nil
		}

		return m.Reclaim(ctx, made, seen)
	}
}

// Delivery is an announcement on its way out: the channel it goes to — empty
// for the service's own — its text, the pull request it announces and the
// head commit its CI is read at, the moment it marks, and whom it tags. The
// zero Mentions tags no one.
type Delivery struct {
	Channel  string
	Text     string
	Pull     forge.PullRequest
	Head     string
	Moment   messaging.Moment
	Mentions messaging.Mentions
}

// Made is the announcement the delivery makes: its pull request at its
// moment.
func (d Delivery) Made() Announced {
	return Announced{Pull: d.Pull.Number, Moment: d.Moment}
}

// Made is the announcement posting the composed one makes: its pull request
// at its moment.
func (c Composed) Made() Announced {
	return Announced{Pull: c.Pull.Number, Moment: c.Announcement.Moment}
}

// Delivery is the composed announcement on its way out as text, to channel,
// tagging mentions.
func (c Composed) Delivery(channel, text string, mentions messaging.Mentions) Delivery {
	return Delivery{
		Channel: channel, Text: text, Pull: c.Pull, Head: c.Pull.HeadCommit, Moment: c.Announcement.Moment,
		Mentions: mentions,
	}
}

// DeliverSeams are what a delivery goes out through: the post, and the read
// of the CI the gate makes before it. A nil Post cannot announce; a nil
// CheckCI cannot confirm that nothing failed, so it refuses too.
type DeliverSeams struct {
	Post    func(ctx context.Context, channel, text string) error
	CheckCI func(ctx context.Context, pull forge.PullRequest, head string) (forge.CI, error)
}

// GateRefusalError is a delivery the gate refused before anything was claimed
// or posted, with the CI it read, so every surface can name the checks that
// failed: a check failed, which unwraps to ErrCIFailed, or the CI could not
// be read, which unwraps to ErrCIUnread.
type GateRefusalError struct {
	CI  forge.CI
	Err error
}

var _ error = GateRefusalError{}

// Error is the gate's own words.
func (r GateRefusalError) Error() string {
	return r.Err.Error()
}

// Unwrap lets errors.Is match ErrCIFailed or ErrCIUnread, and the read's error.
func (r GateRefusalError) Unwrap() error {
	return r.Err
}

// AnnouncedAlreadyError is a delivery refused because another session or
// process claimed it first, with the claim as it stands: confirmed, or a post
// that began at its time and was not seen to finish.
type AnnouncedAlreadyError struct {
	Claim Claim
}

var _ error = AnnouncedAlreadyError{}

// Error says it was made already, or began and was not confirmed.
func (e AnnouncedAlreadyError) Error() string {
	if e.Claim.Posted {
		return ErrAnnouncedAlready.Error()
	}

	return ErrAnnouncedAlready.Error() + ": a post began at " + e.Claim.ClaimedAt.UTC().Format(time.RFC3339) +
		" and was not confirmed"
}

// Unwrap lets errors.Is match ErrAnnouncedAlready.
func (AnnouncedAlreadyError) Unwrap() error {
	return ErrAnnouncedAlready
}

// Deliver posts the delivery under ctx, its tags on a line after the text, at
// most once across every session and process: the gate first, then the
// claim, then the post, then its confirmation. The gate refusing is a
// GateRefusalError, and nothing is claimed. A claim the store cannot take is
// ErrNotClaimed, and one another took is an AnnouncedAlreadyError; neither
// posts. A post the service surely did not deliver gives up its claim and is
// ErrNotDelivered; one that may have gone out keeps it and is
// ErrPostUncertain. Once it has gone out, the groups it tagged are recorded,
// and the claim confirmed, whether or not ctx has ended since; a claim not
// confirmed is ErrUnconfirmed, a post made all the same. A store that keeps
// nothing claims nothing, so the post goes unguarded and a failed one is the
// service's own error.
func Deliver(ctx context.Context, seams DeliverSeams, memory AnnounceMemory, delivery Delivery) error {
	return deliver(ctx, seams, memory, delivery, memory.claim)
}

// Redeliver posts the delivery again, as asked, as Deliver does, claiming it
// again only if its claim is still at generation seen, so of two sessions
// asking again at once, one posts. A claim another took meanwhile is an
// AnnouncedAlreadyError.
func Redeliver(ctx context.Context, seams DeliverSeams, memory AnnounceMemory, delivery Delivery, seen int) error {
	return deliver(ctx, seams, memory, delivery, memory.reclaimFrom(seen))
}

// deliver is Deliver, claiming through claim.
func deliver(
	ctx context.Context, seams DeliverSeams, memory AnnounceMemory, delivery Delivery,
	claim func(context.Context, Announced) (Claim, error),
) error {
	if seams.Post == nil {
		return ErrAnnounceUnavailable
	}

	// Trade-off TRADE-40: the gate reads the CI again as it posts, whatever a
	// surface read before, so a reading an interval old cannot let a failed
	// check through.
	reported, err := ClearToAnnounce(ctx, seams.CheckCI, delivery.Pull, delivery.Head)
	if err != nil {
		return GateRefusalError{CI: reported, Err: err}
	}

	made := delivery.Made()

	taken, err := claim(ctx, made)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrNotClaimed, err)
	}

	if taken.Kind == ClaimHeld {
		return AnnouncedAlreadyError{Claim: taken}
	}

	err = seams.Post(ctx, delivery.Channel, delivery.textWithTags())
	if err != nil {
		return failedPost(ctx, memory, made, taken, err)
	}

	remember(ctx, memory, delivery)

	return confirm(ctx, memory, made, taken)
}

// failedPost is how a post of made that failed with err ends: with no claim
// taken, the service's error; one surely not delivered gives its claim up,
// even if its asker has left meanwhile, and is ErrNotDelivered; any other
// keeps its claim and is ErrPostUncertain, as is one whose claim could not be
// given up, which now reads as a post begun.
func failedPost(ctx context.Context, memory AnnounceMemory, made Announced, taken Claim, err error) error {
	if taken.Kind != ClaimTaken {
		return err
	}

	if !messaging.Undelivered(err) {
		return fmt.Errorf("%w: %w", ErrPostUncertain, err)
	}

	if memory.Release != nil {
		// The release outlasts its asker, so a claim on a post that surely did
		// not go is not left to read as one that may have.
		releaseErr := memory.Release(context.WithoutCancel(ctx), made, taken.Generation)
		if releaseErr != nil {
			return fmt.Errorf("%w: %w; giving up its claim: %w", ErrPostUncertain, err, releaseErr)
		}
	}

	return notDeliveredError{why: err}
}

// notDeliveredError is ErrNotDelivered carrying the service's refusal, which
// says what went wrong in the service's own words: the claim's giving up is
// for the caller to tell apart, not to say.
type notDeliveredError struct {
	why error
}

var _ error = notDeliveredError{}

func (e notDeliveredError) Error() string {
	return e.why.Error()
}

func (e notDeliveredError) Unwrap() []error {
	return []error{ErrNotDelivered, e.why}
}

// confirm confirms the claim taken on made, which has gone out, even if its
// asker has left meanwhile: one never confirmed reads as a post begun.
func confirm(ctx context.Context, memory AnnounceMemory, made Announced, taken Claim) error {
	if taken.Kind != ClaimTaken || memory.Confirm == nil {
		return nil
	}

	// The confirmation outlasts its asker, since the post it confirms went
	// out.
	err := memory.Confirm(context.WithoutCancel(ctx), made, taken.Generation)
	if err != nil {
		return unconfirmedError{why: err}
	}

	return nil
}

// unconfirmedError is ErrUnconfirmed carrying why the store could not
// confirm the post Deliver made.
type unconfirmedError struct {
	why error
}

var _ error = unconfirmedError{}

func (e unconfirmedError) Error() string {
	return ErrUnconfirmed.Error() + ": " + e.why.Error()
}

func (e unconfirmedError) Unwrap() []error {
	return []error{ErrUnconfirmed, e.why}
}

// UnconfirmedReason reports whether err is Deliver's word that it posted an
// announcement whose claim the store could not confirm, wrapped or not, and
// why the store could not: what the terminal and workflow announce say after
// UnconfirmedWarning.
func UnconfirmedReason(err error) (string, bool) {
	var unconfirmed unconfirmedError
	if !errors.As(err, &unconfirmed) {
		return "", false
	}

	return unconfirmed.why.Error(), true
}

// ClearToAnnounce reads pull's CI at head under ctx and answers it when
// nothing has failed on it: none reported, running or passed. A failed check
// is ErrCIFailed; a CI that cannot be read, no way to read it, or no head to
// read it at, is ErrCIUnread, wrapping the read's error, since nothing failing
// cannot be confirmed.
func ClearToAnnounce(
	ctx context.Context, check func(context.Context, forge.PullRequest, string) (forge.CI, error),
	pull forge.PullRequest, head string,
) (forge.CI, error) {
	if check == nil || head == "" {
		return forge.CI{}, ErrCIUnread
	}

	reported, err := check(ctx, pull, head)
	if err != nil {
		return forge.CI{}, fmt.Errorf("%w: %w", ErrCIUnread, err)
	}

	if reported.State == forge.CIFailed {
		return reported, ErrCIFailed
	}

	return reported, nil
}

// remember records the groups the delivery tagged, once it has gone out.
// The post is what the caller asked for: a choice not remembered is only
// offered unchecked next time, so its failure is not the caller's.
func remember(ctx context.Context, memory AnnounceMemory, delivery Delivery) {
	if memory.RecordGroups != nil {
		_ = memory.RecordGroups(context.WithoutCancel(ctx), delivery.groupIDs())
	}
}

// textWithTags is the text, then the tags on a line of their own when there
// are any.
func (d Delivery) textWithTags() string {
	line := d.Mentions.Line()
	if line == "" {
		return d.Text
	}

	return d.Text + "\n" + line
}

// groupIDs is the ID of every group the delivery tags.
func (d Delivery) groupIDs() []string {
	ids := make([]string, 0, len(d.Mentions.Groups))
	for _, group := range d.Mentions.Groups {
		ids = append(ids, group.String())
	}

	return ids
}
