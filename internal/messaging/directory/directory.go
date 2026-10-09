// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

// Package directory reads the Slack directory a session tags people from: a
// channel's members, the workspace's users and user groups, and which
// workspace the token is for, each read once however many ask at a time and
// held for a while, waiting out Slack's rate limits within its patience. It is
// behavior over the messaging client rather than a binding of it, so it lives
// beside messaging, below the wiring that binds it into the seams.
package directory

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/jacob-delgado/workflow/internal/httpx"
	"github.com/jacob-delgado/workflow/internal/messaging"
)

// TTL is how long a directory read holds before Slack is asked again.
const TTL = 10 * time.Minute

// Slack is the Slack directory as this session has read it: the channels
// named so far, their members, the workspace's users and its user groups, and
// which workspace it is. Each is read once however many ask for it at a time,
// and held for TTL from when Slack answered it; no lock is held while Slack
// answers, so Refresh never waits on a read. A read that fails is not held, so
// it is asked again. While the settings in effect post with no Slack user
// token, every read answers messaging.ErrNoCredential and nothing is held, so
// a surface reads tagging as unavailable until a token is set up, and then
// reads the directory afresh.
//
// Trade-off TRADE-26: users.list is read whole, once a session, to label a
// channel's members, rather than each member looked up on its own — unless
// the workspace is too large to list, when each member is.
type Slack struct {
	client func() (messaging.Client, error)
	now    func() time.Time

	lock sync.Mutex
	held *heldReads
}

// heldReads is one generation of the directory: every read begun since it was
// last forgotten, each finished or in flight. Forgetting replaces it whole, so
// a read in flight then lands in a generation no one reads from any more.
type heldReads struct {
	channels map[string]*flight[string]
	members  map[string]*flight[[]messaging.SlackUserID]
	users    map[string]*flight[roster]
	profiles map[string]*flight[profile]
	groups   map[string]*flight[[]messaging.SlackTarget]
	identity map[string]*flight[messaging.Identity]
}

// roster is users.list read whole: each taggable user's label by their ID, or
// tooLarge when the workspace is past messaging.UserListPages, which is held
// too so it is not paged again until the directory expires.
type roster struct {
	labels   map[string]string
	tooLarge bool
}

// profile is one user as users.info answers: their label, or not taggable.
type profile struct {
	label    string
	taggable bool
}

// wholeDirectory is the key of a read that has only one answer.
const wholeDirectory = ""

// flight is one read from Slack, shared by everyone who asks while it runs
// and after, until TTL has passed since it was answered: value, err and
// answeredAt are set before done closes.
type flight[V any] struct {
	done       chan struct{}
	value      V
	err        error
	answeredAt time.Time
}

// expired reports a read answered TTL or more before now. One still in flight
// has not expired.
func (f *flight[V]) expired(now time.Time) bool {
	select {
	case <-f.done:
		return now.Sub(f.answeredAt) >= TTL
	default:
		return false
	}
}

// New is an empty directory reading through client, which is asked for anew
// on every read so settings saved meanwhile are used, and timing its reads by
// now. client answers messaging.ErrNoCredential while the settings have no
// Slack user token to read with.
func New(client func() (messaging.Client, error), now func() time.Time) *Slack {
	directory := &Slack{client: client, now: now}
	directory.Refresh()

	return directory
}

// Refresh drops everything read so far, so the next read asks Slack again. A
// read in flight finishes for those already waiting on it, and is not held.
func (d *Slack) Refresh() {
	d.lock.Lock()
	defer d.lock.Unlock()

	d.forget()
}

// ChannelMembers is everyone in channel — a name, with or without its "#", or
// an ID — who users.list says can be tagged, labeled by their Slack name and
// ordered by it.
func (d *Slack) ChannelMembers(ctx context.Context, channel string) ([]messaging.SlackTarget, error) {
	slack, held, err := d.begin()
	if err != nil {
		return nil, err
	}

	channelID, err := shared(ctx, d, held.channels, channel, func(ctx context.Context) (string, error) {
		return slack.ChannelID(ctx, channel)
	})
	if err != nil {
		return nil, err
	}

	members, err := shared(ctx, d, held.members, channelID, func(ctx context.Context) ([]messaging.SlackUserID, error) {
		return slack.ChannelMembers(ctx, channelID)
	})
	if err != nil {
		return nil, err
	}

	return d.label(ctx, slack, held, members)
}

// UserGroups is every enabled user group in the workspace.
func (d *Slack) UserGroups(ctx context.Context) ([]messaging.SlackTarget, error) {
	slack, held, err := d.begin()
	if err != nil {
		return nil, err
	}

	groups, err := shared(ctx, d, held.groups, wholeDirectory, slack.UserGroups)
	if err != nil {
		return nil, err
	}

	return groups, nil
}

// Workspace is the ID of the Slack workspace the user token is for, which
// keys every link the store keeps. It is held as the directory's reads are,
// so a switch of settings, which drops them, reads it again.
func (d *Slack) Workspace(ctx context.Context) (string, error) {
	identity, err := d.identity(ctx)
	if err != nil {
		return "", err
	}

	return identity.Workspace()
}

// Grant is the scopes Slack lists the user token as granted, read with the
// workspace in one auth.test, so a surface can tell a scope tagging needs is
// missing without reading the directory.
func (d *Slack) Grant(ctx context.Context) (messaging.Grant, error) {
	identity, err := d.identity(ctx)

	return identity.Granted, err
}

// identity is auth.test's answer for the user token, held as the directory's
// reads are.
func (d *Slack) identity(ctx context.Context) (messaging.Identity, error) {
	slack, held, err := d.begin()
	if err != nil {
		return messaging.Identity{}, err
	}

	return shared(ctx, d, held.identity, wholeDirectory, slack.AuthTest)
}

// label is members under their Slack names, ordered by them: from users.list
// read whole, or one by one through users.info in a workspace too large to
// list.
func (d *Slack) label(
	ctx context.Context, slack messaging.Client, held *heldReads, members []messaging.SlackUserID,
) ([]messaging.SlackTarget, error) {
	users, err := shared(ctx, d, held.users, wholeDirectory, func(ctx context.Context) (roster, error) {
		return byID(slack.Users(ctx))
	})
	if err != nil {
		return nil, err
	}

	if users.tooLarge {
		return d.oneByOne(ctx, slack, held, members)
	}

	return labeled(members, users.labels), nil
}

// slackPatience is how long labeling a channel one by one waits, in all, on
// Slack asking it to slow down: users.info is Tier 4, so a large channel can
// meet the limit, and Slack names a short wait. A longer one fails the read,
// keeping every member already labeled, so the next read goes on from there.
const slackPatience = time.Minute

// oneByOne labels each member through users.info, reading each once, and
// waits when Slack asks, while its patience lasts.
func (d *Slack) oneByOne(
	ctx context.Context, slack messaging.Client, held *heldReads, members []messaging.SlackUserID,
) ([]messaging.SlackTarget, error) {
	labels := make(map[string]string, len(members))
	patience := &waiting{left: slackPatience}

	for _, member := range members {
		found, err := d.profile(ctx, slack, held, member, patience)
		if err != nil {
			return nil, err
		}

		if found.taggable {
			labels[member.String()] = found.label
		}
	}

	return labeled(members, labels), nil
}

// profile is member as users.info answers, read once, waiting as long as
// Slack asks to while patience lasts.
func (d *Slack) profile(
	ctx context.Context, slack messaging.Client, held *heldReads, member messaging.SlackUserID, patience *waiting,
) (profile, error) {
	for {
		found, err := shared(ctx, d, held.profiles, member.String(), func(ctx context.Context) (profile, error) {
			return profileOf(slack.User(ctx, member))
		})

		limit, limited := errors.AsType[*httpx.RateLimitError](err)
		if !limited || !patience.spend(limit.Wait) {
			return found, err
		}

		err = pause(ctx, limit.Wait)
		if err != nil {
			return profile{}, err
		}
	}
}

// waiting is how much longer a read will wait on Slack.
type waiting struct {
	left time.Duration
}

// spend takes wait from what is left, and reports false, taking nothing, when
// too little is.
func (w *waiting) spend(wait time.Duration) bool {
	if wait > w.left {
		return false
	}

	w.left -= wait

	return true
}

// pause waits for wait, or until ctx ends.
func pause(ctx context.Context, wait time.Duration) error {
	timer := time.NewTimer(wait)
	defer timer.Stop()

	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("waiting on Slack: %w", ctx.Err())
	}
}

// begin is the client to read with and the generation to read into, or why
// there is none — when everything held is forgotten, since it was read under
// other settings.
func (d *Slack) begin() (messaging.Client, *heldReads, error) {
	d.lock.Lock()
	defer d.lock.Unlock()

	client, err := d.client()
	if err != nil {
		d.forget()

		return messaging.Client{}, nil, err
	}

	return client, d.held, nil
}

// shared is what read answers for key, read once: the first to ask reads, and
// everyone asking meanwhile waits on that read without holding the lock, each
// until it answers or their own ctx ends. A failure is dropped once it is
// answered, so the next ask reads again.
func shared[V any](
	ctx context.Context, directory *Slack, held map[string]*flight[V], key string,
	read func(context.Context) (V, error),
) (V, error) {
	directory.lock.Lock()
	current, inFlight := held[key]

	if inFlight && current.expired(directory.now()) {
		inFlight = false
	}

	if !inFlight {
		current = &flight[V]{done: make(chan struct{})}
		held[key] = current
	}
	directory.lock.Unlock()

	if !inFlight {
		// Others may be waiting on this read, so it runs to its end even if
		// the one who began it leaves; their leaving would fail it for all.
		current.value, current.err = read(context.WithoutCancel(ctx))
		current.answeredAt = directory.now()
		close(current.done)

		if current.err != nil {
			drop(directory, held, key, current)
		}
	}

	select {
	case <-current.done:
		return current.value, current.err
	case <-ctx.Done():
		var none V

		return none, fmt.Errorf("reading the Slack directory: %w", ctx.Err())
	}
}

// drop removes the failed read from held, unless another has taken its place.
func drop[V any](directory *Slack, held map[string]*flight[V], key string, failed *flight[V]) {
	directory.lock.Lock()
	defer directory.lock.Unlock()

	if held[key] == failed {
		delete(held, key)
	}
}

// forget starts a new generation, empty. The caller holds the lock.
func (d *Slack) forget() {
	d.held = &heldReads{
		channels: map[string]*flight[string]{},
		members:  map[string]*flight[[]messaging.SlackUserID]{},
		users:    map[string]*flight[roster]{},
		profiles: map[string]*flight[profile]{},
		groups:   map[string]*flight[[]messaging.SlackTarget]{},
		identity: map[string]*flight[messaging.Identity]{},
	}
}

// byID is each user's label by their ID, or a workspace too large to list.
func byID(users []messaging.SlackTarget, err error) (roster, error) {
	if errors.Is(err, messaging.ErrDirectoryTooLarge) {
		return roster{labels: nil, tooLarge: true}, nil
	}

	if err != nil {
		return roster{}, err
	}

	labels := make(map[string]string, len(users))
	for _, user := range users {
		labels[user.ID] = user.Label
	}

	return roster{labels: labels, tooLarge: false}, nil
}

// profileOf is users.info's answer as a profile: someone not to tag is held as
// such, not as a failure to read again.
func profileOf(user messaging.SlackTarget, err error) (profile, error) {
	if errors.Is(err, messaging.ErrNotTaggable) {
		return profile{label: "", taggable: false}, nil
	}

	if err != nil {
		return profile{}, err
	}

	return profile{label: user.Label, taggable: true}, nil
}

// labeled is each member users names, under that name, ordered by it. A member
// users leaves out — a bot, a deactivated account — is not someone to tag.
func labeled(members []messaging.SlackUserID, users map[string]string) []messaging.SlackTarget {
	targets := make([]messaging.SlackTarget, 0, len(members))

	for _, member := range members {
		if label, known := users[member.String()]; known {
			targets = append(targets, messaging.SlackTarget{ID: member.String(), Label: label})
		}
	}

	slices.SortStableFunc(targets, func(left, right messaging.SlackTarget) int {
		return cmp.Compare(strings.ToLower(left.Label), strings.ToLower(right.Label))
	})

	return targets
}
