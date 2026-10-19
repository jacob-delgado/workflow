// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package demo

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/jacob-delgado/workflow/internal/forge"
)

// What the demo's GitHub refuses, as GitHub itself does.
var (
	errHeadNotPushed = errors.New("the head branch is not on GitHub; push it first")
	errAlreadyOpen   = errors.New("a pull request for the branch is already open")
	errNoSuchPull    = errors.New("no pull request of that number")
)

// firstPull is the number the demo's first pull request takes, the web
// mock's (mockSnapshot.ts).
const firstPull = 128

// scheduledCheck is a CI check and how long after its pull request opens it
// passes.
type scheduledCheck struct {
	name  string
	takes time.Duration
}

// checkSchedule is the demo's CI: four checks, the last passing four
// seconds in, so a recording watches it go from running to passed over two
// of the interface's reads without holding the GIF on a wait.
func checkSchedule() []scheduledCheck {
	return []scheduledCheck{
		{name: "lint", takes: time.Second},
		{name: "build", takes: 2 * time.Second},
		{name: "test", takes: 3 * time.Second},
		{name: "e2e", takes: 4 * time.Second},
	}
}

// createPullRequest opens a pull request for a branch origin holds, one per
// branch, as GitHub does.
func (w *World) createPullRequest(_ context.Context, request forge.NewPullRequest) (forge.PullRequest, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	head := request.Head.Branch

	local := w.repo.branches[head]
	if local.upstream == "" {
		return forge.PullRequest{}, fmt.Errorf("%s: %w: %w", head, forge.ErrRejected, errHeadNotPushed)
	}

	if _, open := w.pulls[head]; open {
		return forge.PullRequest{}, fmt.Errorf("%s: %w: %w", head, forge.ErrRejected, errAlreadyOpen)
	}

	number := firstPull + len(w.pulls)
	pull := forge.PullRequest{
		Number: number, URL: forgeURL + "/pull/" + strconv.Itoa(number), Title: request.Title, Body: request.Body,
		Draft: request.Draft, Base: request.Base, State: forge.StateOpen, HeadCommit: headOf(local),
		Mergeable: forge.MergeClean,
	}
	w.pulls[head], w.opened[number] = number, openedPull{pull: pull, at: w.now()}

	return pull, nil
}

// checkStatus is how a pull request's CI stands: each check running until
// its time since the pull request opened has passed.
func (w *World) checkStatus(_ context.Context, pull forge.PullRequest, _ string) (forge.CI, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	opened, known := w.opened[pull.Number]
	if !known {
		return forge.CI{}, fmt.Errorf("#%d: %w", pull.Number, errNoSuchPull)
	}

	ran := w.now().Sub(opened.at)
	standing := forge.CI{State: forge.CIRunning}

	for _, check := range checkSchedule() {
		state := forge.CIRunning
		if ran >= check.takes {
			state, standing.Done = forge.CIPassed, standing.Done+1
		}

		standing.Checks = append(standing.Checks, forge.Check{
			ID: check.name, Name: check.name, State: state, URL: forgeURL + "/actions/runs/" + check.name,
		})
	}

	standing.Total = len(standing.Checks)
	if standing.Done == standing.Total {
		standing.State = forge.CIPassed
	}

	return standing, nil
}

// pullsOpened are the pull requests the demo has opened, as the forge's
// activity tells them.
func (w *World) pullsOpened() []forge.Event {
	w.mu.Lock()
	defer w.mu.Unlock()

	events := make([]forge.Event, 0, len(w.pulls))

	for number := firstPull; number < firstPull+len(w.opened); number++ {
		opened := w.opened[number]
		events = append(events, forge.Event{
			At: opened.at, Kind: forge.EventOpened, Number: number, Title: opened.pull.Title, URL: opened.pull.URL,
			Repository: forgeRepository,
		})
	}

	return events
}
