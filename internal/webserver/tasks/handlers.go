// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

// Package tasks serves the Taskwarrior operations of the web API behind
// `workflow --web`: the task list, the event stream's summary of your tasks,
// and the writes on them, over the seam the terminal's Tasks pane asks. It
// changes with the tool it drives, so it sits apart from the rest of the
// server, which embeds its Handlers and hands it what it takes from the server
// through a Kit.
package tasks

import (
	"context"
	"time"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/seams"
)

// Seams is what the handlers ask of the world.
type Seams struct {
	// Tasks is what the handlers ask of Taskwarrior. A nil Install, or one
	// that fails, answers the task list and the snapshot's summary as not
	// available — never a 404 — and a write with a nil function is refused as
	// unprocessable.
	Tasks seams.Tasks
	// Issue reads the issue a track adds a task for. Nil means no issue
	// tracker is configured, and a track is refused as unprocessable.
	Issue func(ctx context.Context, issueKey jira.Key) (jira.IssueDetail, error)
}

// Kit is what the handlers take from the server that embeds them. Fault,
// Config, Now and Outliving are required; BrowseURL and Home say what their
// absence means.
type Kit struct {
	// Fault maps a seam's error onto the server's problem, classified, for a
	// failure no Taskwarrior words explain.
	Fault func(err error) api.Problem
	// Config is the taskwarrior settings in effect, which a save in Settings
	// may have changed since the server started.
	Config func() config.Taskwarrior
	// Now is the time by the server's clock.
	Now func() time.Time
	// BrowseURL is the issue's page in the tracker. Nil means no tracker link
	// is wired, so a task links only its own jiraurl and a track annotates
	// nothing.
	BrowseURL func(key jira.Key) string
	// Outliving is a context with ctx's values that goes on when ctx ends and
	// ends when the server stops, and the function that releases it, for the
	// search for Taskwarrior every stream shares.
	Outliving func(ctx context.Context) (context.Context, context.CancelFunc)
	// Home is your home directory, which Taskwarrior's words can name and a
	// refusal shows as ~; empty leaves them naming it.
	Home string
}

// Handlers implements the Taskwarrior operations of api.StrictServerInterface,
// and the event stream's summary of your tasks.
type Handlers struct {
	seams Seams
	// bound is the taskwarrior settings Seams.Tasks was bound with when
	// workflow started in or switched to this directory. A change saved since
	// applies at the next start or directory switch, so while the settings in
	// effect differ the task list says so rather than a reason the new
	// settings would not give.
	bound config.Taskwarrior
	kit   Kit

	// detection is the stream's last search for Taskwarrior, when it found none.
	detection detectionCache
}

// New builds the handlers over seams, bound with the taskwarrior settings in
// bound, taking the rest from the server through kit.
func New(seams Seams, bound config.Taskwarrior, kit Kit) *Handlers {
	return &Handlers{seams: seams, bound: bound, kit: kit}
}

// browseURL is the issue's page in the tracker, or "" when no tracker link is
// wired.
func (h *Handlers) browseURL(key jira.Key) string {
	if h.kit.BrowseURL == nil {
		return ""
	}

	return h.kit.BrowseURL(key)
}
