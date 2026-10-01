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

// errOtherWorkspace is what the kept fakes answer when asked in a Slack
// workspace other than the token's, which nothing should ask in.
var errOtherWorkspace = errors.New("asked in another Slack workspace")

// worldWorkspace is the Slack workspace the world's token is for.
const worldWorkspace = "T0WORLD"

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
	// workspace is the Slack workspace the token is for, which the kept fakes
	// answer in alone, and workspaceErr why Slack would not say.
	workspace    string
	workspaceErr error

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
		groups:    []loop.SlackTarget{{ID: podID, Label: podName}, {ID: apiID, Label: apiName}},
		workspace: worldWorkspace,
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
	deps.Workspace = func() (string, error) { return w.slack.workspace, w.slack.workspaceErr }

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
	store.RepoGroups = func(workspace string) ([]loop.SlackTarget, error) {
		w.mu.Lock()
		defer w.mu.Unlock()

		err := errors.Join(w.otherWorkspace(workspace), w.slack.repoErr)
		if err != nil {
			return nil, err
		}

		return slices.Clone(w.slack.repoGroups), nil
	}
	store.SetRepoGroups = w.setRepoGroups
	store.LastGroups = func(workspace string) ([]string, bool) {
		if w.otherWorkspace(workspace) != nil {
			return nil, false
		}

		return w.slack.last, w.slack.lastChosen
	}
	store.RecordGroups = func(workspace string, ids []string) error {
		w.record("record-groups " + strings.Join(ids, ","))

		return w.otherWorkspace(workspace)
	}

	return store
}

// otherWorkspace refuses a kept read or write in a workspace other than the
// token's.
func (w *world) otherWorkspace(workspace string) error {
	if workspace != w.slack.workspace {
		return errOtherWorkspace
	}

	return nil
}

// ownerLinks is what was decided so far.
func (w *world) ownerLinks(workspace string) ([]loop.OwnerLink, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	err := errors.Join(w.otherWorkspace(workspace), w.slack.linksErr)
	if err != nil {
		return nil, err
	}

	return slices.Clone(w.slack.links), nil
}

// linkOwner records a decision, replacing any earlier one, unless it is
// refused.
func (w *world) linkOwner(workspace string, link loop.OwnerLink) error {
	owner, call := link.Owner, "link-owner "+link.Owner+" nobody"
	if link.OnSlack {
		call = "link-owner " + owner + " " + link.Slack.ID
	}

	w.record(call)

	w.mu.Lock()
	defer w.mu.Unlock()

	err := errors.Join(w.otherWorkspace(workspace), w.slack.linkErr)
	if err != nil {
		return err
	}

	w.slack.links = append(slices.DeleteFunc(w.slack.links, func(old loop.OwnerLink) bool { return old.Owner == owner }),
		link)

	return nil
}

// forgetOwner drops a decision, unless it is refused.
func (w *world) forgetOwner(workspace, owner string) error {
	w.record("forget-owner " + owner)

	w.mu.Lock()
	defer w.mu.Unlock()

	err := errors.Join(w.otherWorkspace(workspace), w.slack.forgetErr)
	if err != nil {
		return err
	}

	w.slack.links = slices.DeleteFunc(w.slack.links, func(old loop.OwnerLink) bool { return old.Owner == owner })

	return nil
}

// setRepoGroups replaces the repository's groups, unless it is refused.
func (w *world) setRepoGroups(workspace string, groups []loop.SlackTarget) error {
	ids := make([]string, 0, len(groups))
	for _, group := range groups {
		ids = append(ids, group.ID)
	}

	w.record("set-repo-groups " + strings.Join(ids, ","))

	w.mu.Lock()
	defer w.mu.Unlock()

	err := errors.Join(w.otherWorkspace(workspace), w.slack.setErr)
	if err != nil {
		return err
	}

	w.slack.repoGroups = slices.Clone(groups)

	return nil
}
