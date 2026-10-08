// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"errors"
	"fmt"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/seams"
	"github.com/jacob-delgado/workflow/internal/taskwarrior"
)

// taskFollow is what follows once Taskwarrior has taken a task line: a command,
// given the task's uuid.
type taskFollow func(uuid string) tea.Cmd

// trackIssue is what follows adding a task for an issue: the issue's page as an
// annotation, where the tracker gives one, and the task started, when the line
// was offered as work on the issue began.
type trackIssue struct {
	url       string
	thenStart bool
}

// follow annotates the added task with the issue's page, then starts it when
// asked to, in one command whose answer reads the tasks again. A failed
// annotation keeps the task, and says how to add the page by hand.
func (t trackIssue) follow(tasks seams.Tasks) taskFollow {
	annotate, start := tasks.Annotate, tasks.Start
	verb := "added task"

	if t.thenStart {
		verb = "added and started task"
	}

	return func(uuid string) tea.Cmd {
		return func() tea.Msg {
			err := t.annotation(annotate, uuid)
			if t.thenStart {
				err = errors.Join(err, start(uuid))
			}

			return taskActed{verb: verb, uuid: uuid, id: 0, said: "", err: err}
		}
	}
}

// heldBack is how a dry run tells what would follow adding the task: annotating
// it with the issue's page, where the tracker gives one, and starting it, when
// asked to.
func (t trackIssue) heldBack() string {
	var steps []string

	if t.url != "" {
		steps = append(steps, "annotate it with "+t.url)
	}

	if t.thenStart {
		steps = append(steps, "start it")
	}

	if len(steps) == 0 {
		return ""
	}

	return " then " + strings.Join(steps, " and ")
}

// annotation adds the issue's page to the task, when there is one: an issue with
// no page adds nothing, which Taskwarrior would refuse.
func (t trackIssue) annotation(annotate func(uuid, text string) error, uuid string) error {
	if t.url == "" {
		return nil
	}

	err := annotate(uuid, t.url)
	if err != nil {
		return fmt.Errorf("%w: %w", taskwarrior.ErrAnnotateFailed, err)
	}

	return nil
}

// trackSelectedIssue goes to the task that tracks the selected issue, or says
// why the Tasks pane does not list it, and otherwise opens the line that tracks
// the issue in Taskwarrior, prefilled in its grammar.
func (m Model) trackSelectedIssue() (Model, tea.Cmd) {
	selected, _ := m.issues.current()
	if task, tracked := m.trackingTask(selected.Key); tracked {
		return m.goToTrackingTask(selected.Key, task), nil
	}

	track := trackIssue{url: m.issues.selectedURL(m.deps), thenStart: false}

	line, err := taskwarrior.TrackLine(taskwarrior.IssueLink{
		Key: string(selected.Key), Summary: selected.Summary, URL: track.url, Priority: selected.Priority,
	})
	if err != nil {
		return m.noticedFailure(err), nil
	}

	return m.openTaskLine(taskLine{
		title: "Track " + shownKey(selected.Key), command: addCommand, write: addLine(m.deps.Tasks.Add),
		after: track.follow(m.deps.Tasks), heldBack: track.heldBack(), tracks: selected.Key,
	}, line)
}

// canTrack reports whether the Issues pane can track an issue, or go to the task
// that does: a task can be added, and Taskwarrior has answered, so which issues
// are tracked already is known — and, for an issue no task tracks, no write is
// on its way, since one goes at a time.
func (m Model) canTrack(issueKey jira.Key) bool {
	if m.deps.Tasks.Add == nil || !m.tasks.answered() {
		return false
	}

	_, tracked := m.trackingTask(issueKey)

	return tracked || !m.tasks.writing
}

// trackKey is the track key as the Issues footer offers it: going to the task
// where one tracks the issue already.
func (m Model) trackKey(issueKey jira.Key) key.Binding {
	if _, tracked := m.trackingTask(issueKey); tracked {
		return relabel(m.keys.trackIssue, "go to task")
	}

	return m.keys.trackIssue
}
