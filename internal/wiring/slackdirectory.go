// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package wiring

import (
	"cmp"
	"context"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/loop"
	"github.com/jacob-delgado/workflow/internal/messaging"
	"github.com/jacob-delgado/workflow/internal/seams"
)

// DirectoryTTL is how long a directory read holds before Slack is asked again.
const DirectoryTTL = 10 * time.Minute

// SlackDirectory is the Slack directory as this session has read it: the
// channels named so far, their members, the workspace's users and its user
// groups. A read that fails is not held, so it is asked again. While the
// settings in effect post with no Slack user token, every read answers
// messaging.ErrNoCredential and nothing is held, so a surface reads tagging as
// unavailable until a token is set up, and then reads the directory afresh.
//
// Trade-off TRADE-26: users.list is read whole, once a session, to label a
// channel's members, rather than each member looked up on its own.
type SlackDirectory struct {
	client func() (messaging.Client, error)
	now    func() time.Time

	lock     sync.Mutex
	readFrom time.Time
	channels map[string]string
	members  map[string][]messaging.SlackUserID
	users    []messaging.SlackTarget
	groups   []messaging.SlackTarget
}

// NewSlackDirectory is an empty directory reading through client, which is
// asked for anew on every read so settings saved meanwhile are used, and
// timing its reads by now. client answers messaging.ErrNoCredential while the
// settings have no Slack user token to read with.
func NewSlackDirectory(client func() (messaging.Client, error), now func() time.Time) *SlackDirectory {
	directory := &SlackDirectory{client: client, now: now}
	directory.Refresh()

	return directory
}

// Refresh drops everything read so far, so the next read asks Slack again.
func (d *SlackDirectory) Refresh() {
	d.lock.Lock()
	defer d.lock.Unlock()

	d.forget()
}

// ChannelMembers is everyone in channel — a name, with or without its "#", or
// an ID — who users.list says can be tagged, labeled by their Slack name and
// ordered by it.
func (d *SlackDirectory) ChannelMembers(ctx context.Context, channel string) ([]loop.SlackTarget, error) {
	// The lock is held across Slack's answer on purpose: two surfaces asking at
	// once wait for one read rather than paging the directory twice.
	d.lock.Lock()
	defer d.lock.Unlock()

	slack, err := d.slack()
	if err != nil {
		return nil, err
	}

	members, err := d.membersOf(ctx, slack, channel)
	if err != nil {
		return nil, err
	}

	users, err := d.everyone(ctx, slack)
	if err != nil {
		return nil, err
	}

	return labeled(members, users), nil
}

// UserGroups is every enabled user group in the workspace.
func (d *SlackDirectory) UserGroups(ctx context.Context) ([]loop.SlackTarget, error) {
	d.lock.Lock()
	defer d.lock.Unlock()

	slack, err := d.slack()
	if err != nil {
		return nil, err
	}

	d.expire()

	if d.groups == nil {
		groups, err := slack.UserGroups(ctx)
		if err != nil {
			return nil, err
		}

		d.groups = groups
	}

	return asLoopTargets(d.groups), nil
}

// slack is the client to read with, or why there is none — when everything
// held is forgotten, since it was read under other settings. The caller holds
// the lock.
func (d *SlackDirectory) slack() (messaging.Client, error) {
	client, err := d.client()
	if err != nil {
		d.forget()

		return messaging.Client{}, err
	}

	return client, nil
}

// membersOf is the user ID of everyone in channel, read once.
func (d *SlackDirectory) membersOf(
	ctx context.Context, slack messaging.Client, channel string,
) ([]messaging.SlackUserID, error) {
	d.expire()

	channelID, err := d.channelID(ctx, slack, channel)
	if err != nil {
		return nil, err
	}

	if members, held := d.members[channelID]; held {
		return members, nil
	}

	members, err := slack.ChannelMembers(ctx, channelID)
	if err != nil {
		return nil, err
	}

	d.members[channelID] = members

	return members, nil
}

// channelID is the ID of the channel named, read once.
func (d *SlackDirectory) channelID(ctx context.Context, slack messaging.Client, channel string) (string, error) {
	if known, held := d.channels[channel]; held {
		return known, nil
	}

	found, err := slack.ChannelID(ctx, channel)
	if err != nil {
		return "", err
	}

	d.channels[channel] = found

	return found, nil
}

// everyone is every taggable user in the workspace, read once.
func (d *SlackDirectory) everyone(ctx context.Context, slack messaging.Client) ([]messaging.SlackTarget, error) {
	if d.users == nil {
		users, err := slack.Users(ctx)
		if err != nil {
			return nil, err
		}

		d.users = users
	}

	return d.users, nil
}

// expire forgets everything once DirectoryTTL has passed since the reads
// began. The caller holds the lock.
func (d *SlackDirectory) expire() {
	if d.now().Sub(d.readFrom) >= DirectoryTTL {
		d.forget()
	}
}

// forget empties the directory and starts its time over. The caller holds the
// lock.
func (d *SlackDirectory) forget() {
	d.readFrom = d.now()
	d.channels = map[string]string{}
	d.members = map[string][]messaging.SlackUserID{}
	d.users, d.groups = nil, nil
}

// labeled is each member users names, under that name, ordered by it. A member
// users leaves out — a bot, a deactivated account — is not someone to tag.
func labeled(members []messaging.SlackUserID, users []messaging.SlackTarget) []loop.SlackTarget {
	targets := make([]loop.SlackTarget, 0, len(members))

	for _, member := range members {
		index := slices.IndexFunc(users, func(user messaging.SlackTarget) bool { return user.ID == member.String() })
		if index >= 0 {
			targets = append(targets, loop.SlackTarget{ID: users[index].ID, Label: users[index].Label})
		}
	}

	slices.SortStableFunc(targets, func(left, right loop.SlackTarget) int {
		return cmp.Compare(strings.ToLower(left.Label), strings.ToLower(right.Label))
	})

	return targets
}

// asLoopTargets is targets as the loop names them.
func asLoopTargets(targets []messaging.SlackTarget) []loop.SlackTarget {
	converted := make([]loop.SlackTarget, 0, len(targets))
	for _, target := range targets {
		converted = append(converted, loop.SlackTarget{ID: target.ID, Label: target.Label})
	}

	return converted
}

// slackUserClient is the client a directory reads with: built from the
// settings in effect, and messaging.ErrNoCredential while they post with no
// Slack user token — a webhook, Teams or Discord cannot read the directory.
// Only Slack has a user-token mode, so the mode alone says the service is
// Slack.
func slackUserClient(setup messagingSetup) func() (messaging.Client, error) {
	return func() (messaging.Client, error) {
		if setup.settings().Mode() != config.MessagingUser {
			return messaging.Client{}, messaging.ErrNoCredential
		}

		return messagingClient(setup), nil
	}
}

// withDirectory is bound with directory's reads.
func withDirectory(ctx context.Context, bound seams.Messaging, directory *SlackDirectory) seams.Messaging {
	bound.ChannelMembers = func(channel string) ([]loop.SlackTarget, error) {
		return directory.ChannelMembers(ctx, channel)
	}
	bound.UserGroups = func() ([]loop.SlackTarget, error) { return directory.UserGroups(ctx) }
	bound.RefreshDirectory = directory.Refresh

	return bound
}
