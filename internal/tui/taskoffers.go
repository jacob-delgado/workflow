// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"errors"
	"slices"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/taskwarrior"
)

// settled opens the follow-up once no overlay is open. Every route through
// Update ends here, so an offer made while another overlay had the keyboard
// opens the moment that overlay closes, and never on top of it.
func (m Model) settled(cmd tea.Cmd) (Model, tea.Cmd) {
	if m.overlay != nil || m.followUp == nil {
		return m, cmd
	}

	open := m.followUp
	m.followUp = nil
	opened, more := open(m)

	return opened, tea.Batch(cmd, more)
}

// canOffer reports whether a moment of the loop can offer a change of a task:
// Taskwarrior takes writes, and has answered, so which issues are tracked is
// known — an offer made after a failed read could track an issue twice.
func (m Model) canOffer() bool {
	return m.deps.Tasks.Start != nil && m.tasks.answered()
}

// offeredTask is the task an offer would change — the one that tracks an issue —
// and whether there is one to offer: none where no offer can be made at all.
func (m Model) offeredTask(issueKey jira.Key) (taskwarrior.Task, bool) {
	if !m.canOffer() {
		return taskwarrior.Task{}, false
	}

	return m.trackingTask(issueKey)
}

// offerStart offers, once work on an issue begins — its branch made, or switched
// to — to start the task that tracks it, or, where none does, to track the issue
// and start the new task. One task runs at a time, so every task started on
// anything else is stopped first. Nil when there is nothing to offer: no offer
// can be made, or the issue's own work is started already.
func (m Model) offerStart(issue jira.Issue) func(Model) (Model, tea.Cmd) {
	if !m.canOffer() || m.startedOn(issue.Key) {
		return nil
	}

	task, tracked := m.trackingTask(issue.Key)
	started := m.tasks.started()

	switch {
	case tracked && len(started) > 0:
		return m.offerSwitch(started, task)
	case tracked:
		return m.offerToStart(task)
	case len(started) > 0:
		return m.offerStopThenTrack(started, issue)
	default:
		return m.offerTrackAndStart(issue)
	}
}

// started is every started task, pending or linked, once each and in the
// Tasks pane's order. Where an offer is made none is the issue's own, so each is
// another issue's or none's, and stops before the issue's task starts.
func (s tasksState) started() []taskwarrior.Task {
	var started []taskwarrior.Task

	for _, task := range slices.Concat(s.pending, s.linked) {
		counted := slices.ContainsFunc(started, func(each taskwarrior.Task) bool { return each.UUID == task.UUID })
		if task.Active() && !counted {
			started = append(started, task)
		}
	}

	return started
}

// startedOn reports whether any task linked to an issue is started, whether or
// not the Tasks pane lists it: the issue's own work is running already.
func (m Model) startedOn(issueKey jira.Key) bool {
	return slices.ContainsFunc(m.tasks.linkedTo(issueKey), taskwarrior.Task.Active)
}

// listedIssue is an issue as the Issues pane lists it, or, where the pane has
// not read it — a branch switched to for an issue in another view — the issue
// known by its key alone, whose track line is then prefilled with no summary or
// priority.
func (m Model) listedIssue(issueKey jira.Key) jira.Issue {
	if issue, listed := m.issues.find(issueKey); listed {
		return issue
	}

	return jira.Issue{Key: issueKey}
}

// offerToStart offers to start the task that tracks an issue.
func (m Model) offerToStart(task taskwarrior.Task) func(Model) (Model, tea.Cmd) {
	start := m.deps.Tasks.Start

	return offering(lastLook{
		title: "Start the task", verb: verbStart, doing: "starting",
		body: "Start task " + describedTask(task) + " in Taskwarrior? Its hooks run, so a timewarrior hook " +
			"starts too.",
	}, "start task "+taskName(task), func() taskActed {
		return taskActed{verb: "started", uuid: task.UUID, id: shownID(task), said: "", err: start(task.UUID)}
	})
}

// offerTrackAndStart offers the line that tracks an issue in Taskwarrior,
// prefilled in its grammar as the track key's is, whose task is started once it
// is added and annotated.
func (m Model) offerTrackAndStart(issue jira.Issue) func(Model) (Model, tea.Cmd) {
	track := trackIssue{url: m.browseURL(issue.Key), thenStart: true}
	prefill, err := taskwarrior.TrackLine(taskwarrior.IssueLink{
		Key: string(issue.Key), Summary: issue.Summary, URL: track.url, Priority: issue.Priority,
	})
	line := taskLine{
		title: "Track and start " + shownKey(issue.Key), command: addCommand, write: addLine(m.deps.Tasks.Add),
		after: track.follow(m.deps.Tasks), heldBack: track.heldBack(), tracks: issue.Key,
	}

	return func(m Model) (Model, tea.Cmd) {
		if err != nil {
			return m.noticedFailure(err), nil
		}

		return m.openTaskLine(line, prefill)
	}
}

// offerSwitch offers to stop the started tasks and start the one that tracks the
// issue work begins on.
func (m Model) offerSwitch(started []taskwarrior.Task, task taskwarrior.Task) func(Model) (Model, tea.Cmd) {
	stop, start := m.deps.Tasks.Stop, m.deps.Tasks.Start

	return offering(lastLook{
		title: "Switch the task", verb: verbSwitch, doing: "switching",
		body: "Stop " + eachTask(started, describedTask) + " and start task " + describedTask(task) + "?",
	}, "stop "+eachTask(started, taskName)+" and start task "+taskName(task), func() taskActed {
		// The task is started only once every other has stopped, so a refusal never
		// leaves two started; tried again after the start was refused, each finds
		// itself stopped already.
		err := stopEach(stop, started)
		if err == nil {
			err = start(task.UUID)
		}

		return taskActed{
			verb: "stopped " + namesOf(started, taskName) + " and started", uuid: task.UUID, id: shownID(task), said: "",
			err: err,
		}
	})
}

// offerStopThenTrack offers to stop the started tasks and then track the issue
// work begins on: only once each has stopped does the line open that tracks the
// issue and starts the new task. A dry run says it would stop them and opens the
// line, whose own dry run says what it would add.
func (m Model) offerStopThenTrack(started []taskwarrior.Task, issue jira.Issue) func(Model) (Model, tea.Cmd) {
	stop, track := m.deps.Tasks.Stop, m.offerTrackAndStart(issue)
	look := lastLook{
		title: "Switch the task", verb: verbSwitch, doing: "stopping",
		body: "Stop " + eachTask(started, describedTask) + ", then track and start " + shownKey(issue.Key) + "?",
	}
	look.proceed = func(m Model) (Model, tea.Cmd) {
		switch {
		case m.dryRun:
			m = m.closeOverlay().noticed("dry run: would stop " + eachTask(started, taskName))
			m.followUp = track

			return m, nil
		case m.tasks.writing:
			return keepOpenWith[lastLook](m, errTaskWriteInFlight), nil
		}

		m.tasks.writing = true

		return m, func() tea.Msg {
			acted := taskActed{
				verb: "stopped " + namesOf(started, taskName), uuid: "", id: 0, said: "", err: stopEach(stop, started),
			}

			return offerAnswered{acted: acted, then: track}
		}
	}

	return opening(look)
}

// namesOf names each task as name does: "12 and 9".
func namesOf(tasks []taskwarrior.Task, name func(taskwarrior.Task) string) string {
	names := make([]string, len(tasks))
	for index, task := range tasks {
		names[index] = name(task)
	}

	return strings.Join(names, " and ")
}

// eachTask is namesOf with each after the word task: "task 12 and task 9".
func eachTask(tasks []taskwarrior.Task, name func(taskwarrior.Task) string) string {
	return namesOf(tasks, func(task taskwarrior.Task) string { return "task " + name(task) })
}

// stopEach stops each task in turn, until one is refused. A stop that changed
// nothing is taken as made: the task is not started — it was stopped since the
// tasks were read, or by a switch tried before — which is all a stop is for.
func stopEach(stop func(uuid string) error, tasks []taskwarrior.Task) error {
	for _, task := range tasks {
		err := stop(task.UUID)
		if err != nil && !errors.Is(err, taskwarrior.ErrNothingChanged) {
			return err
		}
	}

	return nil
}

// offerAnnotate offers, once a pull request is open for an issue, to note it on
// the task that tracks the issue: its number and URL, as an annotation. Nil when
// no task tracks the issue.
func (m Model) offerAnnotate(issueKey jira.Key, pull forge.PullRequest) func(Model) (Model, tea.Cmd) {
	task, tracked := m.offeredTask(issueKey)
	if !tracked {
		return nil
	}

	number := m.vocab.sigil + strconv.Itoa(pull.Number)
	note, annotate := number+" "+pull.URL, m.deps.Tasks.Annotate

	return offering(lastLook{
		title: "Note the " + m.vocab.noun, verb: "annotate", doing: "annotating",
		body: "Annotate task " + taskName(task) + " with " + number + " and its URL?",
	}, "annotate task "+taskName(task)+" with "+note, func() taskActed {
		return taskActed{verb: "annotated", uuid: task.UUID, id: shownID(task), said: "", err: annotate(task.UUID, note)}
	})
}

// offerMarkDone offers, once an issue's pull request is merged or the issue is
// done, to complete the task that tracks it. Nil when no task tracks the issue.
func (m Model) offerMarkDone(issueKey jira.Key) func(Model) (Model, tea.Cmd) {
	task, tracked := m.offeredTask(issueKey)
	if !tracked {
		return nil
	}

	return opening(m.doneAsked(task))
}

// doneAsked is the last look at marking task done, asked from the Tasks pane
// or offered once its issue is done: Taskwarrior runs the task's hooks, and
// only an undo while it is the last change takes it back.
func (m Model) doneAsked(task taskwarrior.Task) lastLook {
	done := m.deps.Tasks.Done

	return asking(lastLook{
		title: "Mark the task done", verb: "mark done", doing: "marking done",
		body: "Mark task " + describedTask(task) + " done?",
	}, "mark task "+taskName(task)+" done", func() tea.Msg {
		return offerAnswered{acted: taskActed{
			verb: "marked", after: " done", uuid: task.UUID, id: shownID(task), said: "", err: done(task.UUID),
		}, then: nil}
	})
}

// offerForMove is the follow-up once an issue has moved: to complete the task
// that tracks it, when the move is to done and a task does. A move that offers
// nothing keeps the offer that waited behind the status picker — the task's
// start after a branch, the pull request's note — which opens now instead.
func (m Model) offerForMove(issueKey jira.Key, to jira.Transition) func(Model) (Model, tea.Cmd) {
	if to.ToStatusCategory == jira.CategoryDone {
		if offer := m.offerMarkDone(issueKey); offer != nil {
			return offer
		}
	}

	return m.followUp
}

// offering is a follow-up that opens a last look at a change of a task, asking
// as asking does.
func offering(look lastLook, wouldDo string, change func() taskActed) func(Model) (Model, tea.Cmd) {
	return opening(asking(look, wouldDo, func() tea.Msg { return offerAnswered{acted: change(), then: nil} }))
}

// asking is look made to send a change of Taskwarrior once it is confirmed:
// send runs in one command whose answer closes the look or keeps it open with
// the reason. In a dry run the look closes saying what it would have done, and
// nothing is sent; while another write is on its way, the look stays open
// saying so, since one goes at a time.
func asking(look lastLook, wouldDo string, send tea.Cmd) lastLook {
	look.proceed = func(m Model) (Model, tea.Cmd) {
		switch {
		case m.dryRun:
			return m.closeOverlay().noticed("dry run: would " + wouldDo), nil
		case m.tasks.writing:
			return keepOpenWith[lastLook](m, errTaskWriteInFlight), nil
		}

		m.tasks.writing = true

		return m, send
	}

	return look
}

// opening is a follow-up that opens a last look.
func opening(look lastLook) func(Model) (Model, tea.Cmd) {
	return func(m Model) (Model, tea.Cmd) {
		look.leave = escSkip
		m.overlay = look

		return m, nil
	}
}

// offerAnswered reports how a change an offer's look asked for went: the change,
// told as any change of a task is; and what opens once the change is made, nil
// where nothing does.
type offerAnswered struct {
	acted taskActed
	then  func(Model) (Model, tea.Cmd)
}

var _ applier = offerAnswered{}

// apply closes the look that asked once the change is made, opening what
// follows it, or keeps the look open with why the change was refused; then tells
// the change. Only a look's own answer reaches it: a change from the Tasks pane,
// or the one that tracks an issue, is told and leaves any look as it is.
func (msg offerAnswered) apply(m Model) (Model, tea.Cmd) {
	m = m.answerLook(msg.acted.err)
	if msg.acted.err == nil && msg.then != nil {
		m.followUp = msg.then
	}

	return msg.acted.apply(m)
}

// answerLook closes the look that asked once its change is made, or keeps it
// open with why the change was refused. The look open is the one that asked: in
// flight, it keeps the keys until its answer.
func (m Model) answerLook(err error) Model {
	_, open := m.overlay.(lastLook)

	switch {
	case !open:
		return m
	case err != nil:
		return keepOpenWith[lastLook](m, err)
	default:
		return m.closeOverlay()
	}
}

// describedTask names a task and quotes what it is: 3 'PROJ-42: Fix it'.
func describedTask(task taskwarrior.Task) string {
	return taskName(task) + " '" + task.Description + "'"
}

// shownID is a task's id where the screen names the task by one, and zero where
// it names it by its uuid, so a change is told by the name its look used.
func shownID(task taskwarrior.Task) int {
	if taskNumber(task) == "" {
		return 0
	}

	return task.ID
}
