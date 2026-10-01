// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"errors"
	"slices"
	"strings"

	"github.com/jacob-delgado/workflow/internal/loop"
	"github.com/jacob-delgado/workflow/internal/seams"
)

// Slack fixtures the tagging and people tests share.
const (
	carlaID    = "U0CARLA"
	carlaName  = "Carla Diaz"
	benID      = "U0BEN"
	benName    = "Ben Ortiz"
	podID      = "S0POD"
	podName    = "control-plane-pod"
	apiID      = "S0API"
	apiName    = "api-reviewers"
	podTeam    = "acme/control-plane"
	ownerBen   = "ben"
	ownerCarla = "carla"
	// benLinked and benNotOnSlack are the store's record of each decision
	// about ben.
	benLinked     = "link-owner " + ownerBen + " " + benID
	benNotOnSlack = "link-owner " + ownerBen + " nobody"
	// teamChannel is a second channel a post can go to.
	teamChannel = "#team-b"
)

// errDirectoryDown is a Slack directory read that failed for a reason of its
// own, neither a scope nor a missing feature.
var errDirectoryDown = errors.New("slack is down")

// slackWorld is the Slack directory and the kept associations, faked. A world
// without one has no directory seams and no kept store, as a webhook or a
// dry run has none.
type slackWorld struct {
	members    map[string][]loop.SlackTarget
	membersErr error
	groups     []loop.SlackTarget
	groupsErr  error
	noGroups   bool
	// noDirectory keeps the store but reads no directory, as a webhook does.
	noDirectory bool

	links      []loop.OwnerLink
	linksErr   error
	linkErr    error
	forgetErr  error
	repoGroups []loop.SlackTarget
	repoErr    error
	setErr     error
	last       []string
	lastChosen bool
}

// newSlackWorld is a channel with Carla and Ben in it, two user groups, and
// nothing decided yet.
func newSlackWorld() *slackWorld {
	return &slackWorld{
		members: map[string][]loop.SlackTarget{
			devChannel: {{ID: carlaID, Label: carlaName}, {ID: benID, Label: benName}},
		},
		groups: []loop.SlackTarget{{ID: podID, Label: podName}, {ID: apiID, Label: apiName}},
	}
}

// messagingDeps fakes the messaging service: posts are recorded, and with a
// slackWorld its directory answers.
func (w *world) messagingDeps() seams.Messaging {
	deps := seams.Messaging{Post: func(channel, text string) error {
		w.record("post " + text)
		w.rememberChannel(channel)

		if w.postParked != nil {
			w.postParked <- struct{}{}

			<-w.postRelease
		}

		return w.postErr
	}}

	if w.slack == nil || w.slack.noDirectory {
		return deps
	}

	deps.ChannelMembers = func(channel string) ([]loop.SlackTarget, error) {
		w.record("members " + channel)

		if w.slack.membersErr != nil {
			return nil, w.slack.membersErr
		}

		return slices.Clone(w.slack.members[channel]), nil
	}
	deps.RefreshDirectory = func() { w.record("refresh-directory") }

	if !w.slack.noGroups {
		deps.UserGroups = func() ([]loop.SlackTarget, error) {
			w.record("user-groups")

			if w.slack.groupsErr != nil {
				return nil, w.slack.groupsErr
			}

			return slices.Clone(w.slack.groups), nil
		}
	}

	return deps
}

// withKept adds the kept associations to a store, when the world has a
// slackWorld to keep them in.
func (w *world) withKept(store seams.Store) seams.Store {
	if w.slack == nil {
		return store
	}

	store.OwnerLinks = w.ownerLinks
	store.LinkOwner = w.linkOwner
	store.ForgetOwner = w.forgetOwner
	store.RepoGroups = func() ([]loop.SlackTarget, error) {
		w.mu.Lock()
		defer w.mu.Unlock()

		if w.slack.repoErr != nil {
			return nil, w.slack.repoErr
		}

		return slices.Clone(w.slack.repoGroups), nil
	}
	store.SetRepoGroups = w.setRepoGroups
	store.LastGroups = func() ([]string, bool) { return w.slack.last, w.slack.lastChosen }
	store.RecordGroups = func(ids []string) error {
		w.record("record-groups " + strings.Join(ids, ","))

		return nil
	}

	return store
}

// ownerLinks is what was decided so far.
func (w *world) ownerLinks() ([]loop.OwnerLink, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.slack.linksErr != nil {
		return nil, w.slack.linksErr
	}

	return slices.Clone(w.slack.links), nil
}

// linkOwner records a decision, replacing any earlier one, unless it is
// refused.
func (w *world) linkOwner(owner string, target *loop.SlackTarget) error {
	call, link := "link-owner "+owner+" nobody", loop.OwnerLink{Owner: owner}
	if target != nil {
		call, link = "link-owner "+owner+" "+target.ID, loop.OwnerLink{Owner: owner, OnSlack: true, Slack: *target}
	}

	w.record(call)

	w.mu.Lock()
	defer w.mu.Unlock()

	if w.slack.linkErr != nil {
		return w.slack.linkErr
	}

	w.slack.links = append(slices.DeleteFunc(w.slack.links, func(old loop.OwnerLink) bool { return old.Owner == owner }),
		link)

	return nil
}

// forgetOwner drops a decision, unless it is refused.
func (w *world) forgetOwner(owner string) error {
	w.record("forget-owner " + owner)

	w.mu.Lock()
	defer w.mu.Unlock()

	if w.slack.forgetErr != nil {
		return w.slack.forgetErr
	}

	w.slack.links = slices.DeleteFunc(w.slack.links, func(old loop.OwnerLink) bool { return old.Owner == owner })

	return nil
}

// setRepoGroups replaces the repository's groups, unless it is refused.
func (w *world) setRepoGroups(groups []loop.SlackTarget) error {
	ids := make([]string, 0, len(groups))
	for _, group := range groups {
		ids = append(ids, group.ID)
	}

	w.record("set-repo-groups " + strings.Join(ids, ","))

	w.mu.Lock()
	defer w.mu.Unlock()

	if w.slack.setErr != nil {
		return w.slack.setErr
	}

	w.slack.repoGroups = slices.Clone(groups)

	return nil
}
