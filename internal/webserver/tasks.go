// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver

import (
	"context"
	"errors"
	"net/url"
	"slices"
	"sync"
	"time"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/taskwarrior"
)

// taskwarriorHold is how long a search for Taskwarrior that found none serves
// the event stream: each search runs every task program on PATH, go-task among
// them, and one that found none finds none again a frame later.
const taskwarriorHold = time.Minute

// errTurnedOff is why there is no Taskwarrior to ask when taskwarrior.disabled
// turned the integration off, which leaves no seam to ask.
var errTurnedOff = errors.New("turned off by taskwarrior.disabled")

// errSettingsChanged is why the task list and the stream answer from no
// Taskwarrior once the taskwarrior settings in effect are not the ones the
// seams were bound with at start: a change applies at the next start.
var errSettingsChanged = errors.New("the taskwarrior settings changed since workflow started")

// detectionCache is the event stream's last search for Taskwarrior when it
// found none, held for taskwarriorHold so a frame does not search again; a
// search that finds Taskwarrior holds nothing. Its lock is held across the
// search, so streams that find one due together make one.
type detectionCache struct {
	mu     sync.Mutex
	failed error
	readAt time.Time
}

// ListTasks answers your pending tasks, or why no Taskwarrior can be asked —
// an answer about where the server runs, as the review queue's is, not a
// missing resource.
func (s *server) ListTasks(_ context.Context, _ api.ListTasksRequestObject) (api.ListTasksResponseObject, error) {
	list, err := s.readTaskList("")
	if err != nil {
		return problemAnswer[api.ListTasksdefaultApplicationProblemPlusJSONResponse](s.taskFault(err)), nil
	}

	return api.ListTasks200JSONResponse(list), nil
}

// readTaskList is the list a read answers and every write ends on, so the
// page replaces what it shows with what Taskwarrior holds now. said is what
// undo or sync printed, or what a write has to add. The clock is read once, so
// the tasks' states and the values the filter offers describe one moment.
func (s *server) readTaskList(said string) (api.TaskList, error) {
	install, err := s.unlessSettingsChanged(s.installed)
	if err != nil {
		code, reason := unavailableReason(err)

		return api.TaskList{
			Available: false, Reason: reason, ReasonCode: &code, Tasks: []api.Task{}, FacetOrder: []api.TaskFacet{},
		}, nil
	}

	pending, err := s.deps.Tasks.Pending()
	if err != nil {
		return api.TaskList{}, err
	}

	tasks, now := knownTasks(pending.Tasks), s.now()

	return api.TaskList{
		Available: true, Reason: "", Context: pending.Context, SyncAvailable: install.SyncConfigured,
		Said: said, Tasks: s.tasksDTO(tasks, now), FacetOrder: taskFacetsDTO(taskwarrior.OfferedFacets(tasks, now)),
	}, nil
}

// unlessSettingsChanged is the Taskwarrior find answers, or errSettingsChanged
// without asking once the taskwarrior settings in effect are not the ones the
// seams were bound with: whatever find would say is about the old settings.
func (s *server) unlessSettingsChanged(find func() (taskwarrior.Install, error)) (taskwarrior.Install, error) {
	if s.config().Taskwarrior != s.info.Taskwarrior {
		return taskwarrior.Install{}, errSettingsChanged
	}

	return find()
}

// installed is the Taskwarrior that answers, or why none can be asked: with no
// seam, the integration was turned off at start or no task program was found.
func (s *server) installed() (taskwarrior.Install, error) {
	if s.deps.Tasks.Install != nil {
		return s.deps.Tasks.Install()
	}

	if s.info.Taskwarrior.Disabled {
		return taskwarrior.Install{}, errTurnedOff
	}

	return taskwarrior.Install{}, taskwarrior.ErrNotInstalled
}

// malformedTaskrc is the fixed words Taskwarrior's refusal has for a taskrc
// line it cannot read, in place of the line, which can hold a secret.
const malformedTaskrc = "taskrc has a malformed line"

// unavailableReason is why no Taskwarrior can be asked, as a code and in words,
// never the error's own, which name the task program's path: a taskrc with a
// line Taskwarrior cannot read, or a Taskwarrior that could not start for
// another reason, which only a search meets; the reasons a write shares; the
// integration turned off; settings saved since the start; and else where to see
// why.
func unavailableReason(err error) (api.TaskListReasonCode, string) {
	switch {
	case errors.Is(err, taskwarrior.ErrRefused) && refusalWords(err) == malformedTaskrc:
		return api.TaskListReasonCodeMalformedTaskrc, "Taskwarrior's taskrc has a malformed line."
	case errors.Is(err, taskwarrior.ErrRefused):
		return api.TaskListReasonCodeUnavailable, "Taskwarrior could not start; workflow doctor says why."
	}

	reasons := append(taskwarriorReasons(),
		taskwarriorReason{
			cause: errTurnedOff, code: api.TaskListReasonCodeTurnedOff, text: "Turned off by taskwarrior.disabled.",
		},
		taskwarriorReason{
			cause: errSettingsChanged, code: api.TaskListReasonCodeUnavailable,
			text: "Taskwarrior settings changed; restart workflow to apply.",
		})

	for _, reason := range reasons {
		if errors.Is(err, reason.cause) {
			return reason.code, reason.text
		}
	}

	return api.TaskListReasonCodeUnavailable, "Taskwarrior could not be asked; workflow doctor says why."
}

// snapshotTasks is the stream's summary of your tasks: whether Taskwarrior can
// be asked, and why not; the started task; and every task linked to an issue.
// Until Taskwarrior has answered both reads the summary is unavailable, as the
// terminal draws no task mark until it has, so a page never takes a read that
// failed for "no task tracks this issue" and offers to track it again.
func (s *server) snapshotTasks() api.TasksSummary {
	_, err := s.unlessSettingsChanged(s.heldDetection)
	if err != nil {
		_, reason := unavailableReason(err)

		return api.TasksSummary{Available: false, Reason: reason, Linked: []api.Task{}}
	}

	unanswered := api.TasksSummary{
		Available: false, Reason: "Taskwarrior could not be read; workflow doctor says why.", Linked: []api.Task{},
	}

	pending, err := s.deps.Tasks.Pending()
	if err != nil {
		return unanswered
	}

	linked, err := s.deps.Tasks.Linked()
	if err != nil {
		return unanswered
	}

	return s.tasksSummaryDTO(pending.Tasks, linked, s.now())
}

// heldDetection is which Taskwarrior answers, for the stream: a search that
// found none within taskwarriorHold stands for this one.
func (s *server) heldDetection() (taskwarrior.Install, error) {
	s.detection.mu.Lock()
	defer s.detection.mu.Unlock()

	now := s.now()
	if s.detection.failed != nil && now.Sub(s.detection.readAt) < taskwarriorHold {
		return taskwarrior.Install{}, s.detection.failed
	}

	install, err := s.installed()
	s.detection.failed, s.detection.readAt = err, now

	return install, err
}

// tasksSummaryDTO maps what Taskwarrior answered onto the stream's summary, as
// the tasks stand at now: the started task and every linked task.
func (s *server) tasksSummaryDTO(pending, linked []taskwarrior.Task, now time.Time) api.TasksSummary {
	known := knownTasks(linked)
	summary := api.TasksSummary{Available: true, Reason: "", Linked: s.tasksDTO(known, now)}
	summary.Active = s.activeTask(knownTasks(pending), known, summary.Linked, now)

	return summary
}

// activeTask is the first started task, pending or else linked, as the
// terminal's spine shows it, ranked among the list that holds it: your pending
// tasks, as the Tasks list ranks it, or the linked ones, given here as
// linkedDTO, when the active context hides it from the pending. It is nil when
// none is started.
func (s *server) activeTask(pending, linked []taskwarrior.Task, linkedDTO []api.Task, now time.Time) *api.Task {
	if started := slices.IndexFunc(pending, taskwarrior.Task.Active); started >= 0 {
		return &s.tasksDTO(pending, now)[started]
	}

	if started := slices.IndexFunc(linked, taskwarrior.Task.Active); started >= 0 {
		return &linkedDTO[started]
	}

	return nil
}

// knownTasks is tasks but each whose status the spec does not know: a synced
// replica can hold one, and a status outside the spec's enum fails the page's
// parse of the whole answer, or the whole frame. The active task is pending,
// so never one.
func knownTasks(tasks []taskwarrior.Task) []taskwarrior.Task {
	return slices.DeleteFunc(slices.Clone(tasks), func(task taskwarrior.Task) bool {
		return !api.TaskStatus(task.Status).Valid()
	})
}

// tasksDTO maps tasks onto the wire, an empty list rather than null, each as
// it stands at now, with its place in every order among them.
func (s *server) tasksDTO(tasks []taskwarrior.Task, now time.Time) []api.Task {
	ranks := taskwarrior.RanksOf(tasks, now)
	out := make([]api.Task, 0, len(tasks))

	for index, task := range tasks {
		out = append(out, s.taskDTO(task, ranks[index], now))
	}

	return out
}

// taskDTO maps a task onto the wire, each date it does not have absent, with
// where it stands at now, its facets, its ranks and the fields text matches.
func (s *server) taskDTO(task taskwarrior.Task, ranks taskwarrior.Ranks, now time.Time) api.Task {
	return api.Task{
		UUID: task.UUID, ID: task.ID, Description: task.Description, Status: api.TaskStatus(task.Status),
		State: api.TaskState(task.State(now).String()), Project: task.Project, Priority: task.Priority,
		Tags: orEmpty(task.Tags),
		Due:  optionalTime(task.Due), Wait: optionalTime(task.Wait), Scheduled: optionalTime(task.Scheduled),
		Until: optionalTime(task.Until), Start: optionalTime(task.Start), End: optionalTime(task.End),
		Entry: task.Entry, Modified: task.Modified, Urgency: task.Urgency,
		Annotations: annotationsDTO(task.Annotations), IssueKey: task.IssueKey, IssueURL: s.taskIssueURL(task),
		Facets: taskFacetsDTO(task.Facets(now)), Ranks: ranksDTO(ranks), Searchable: task.Searchable(),
	}
}

// ranksDTO maps a task's place in each order onto the wire.
func ranksDTO(ranks taskwarrior.Ranks) api.TaskRanks {
	return api.TaskRanks{
		Urgency: ranks.Urgency, State: ranks.State, ID: ranks.ID, Tag: ranks.Tag, Issue: ranks.Issue,
		Priority: ranks.Priority,
	}
}

// taskFacetsDTO maps task facets onto the wire, each with its label, an empty
// list rather than null. A map, so exhaustive keeps the kinds complete.
func taskFacetsDTO(facets []taskwarrior.Facet) []api.TaskFacet {
	kinds := map[taskwarrior.FacetKind]api.TaskFacetKind{
		taskwarrior.FacetState: api.TaskFacetKindState, taskwarrior.FacetPriority: api.TaskFacetKindPriority,
		taskwarrior.FacetProject: api.TaskFacetKindProject, taskwarrior.FacetTag: api.TaskFacetKindTag,
		taskwarrior.FacetIssue: api.TaskFacetKindIssue,
	}

	out := make([]api.TaskFacet, 0, len(facets))
	for _, facet := range facets {
		out = append(out, api.TaskFacet{Kind: kinds[facet.Kind], Value: facet.Value, Label: facet.Label()})
	}

	return out
}

// taskIssueURL is the page of the issue a task tracks: the tracker's, as the
// terminal opens it, for a task naming an issue where the tracker builds pages;
// otherwise the task's own jiraurl, but only an http or https address — it is
// Taskwarrior's data, which a hand edit or any synced replica can have written;
// else none.
func (s *server) taskIssueURL(task taskwarrior.Task) string {
	if task.IssueKey != "" && s.deps.Jira.BrowseURL != nil {
		return s.deps.Jira.BrowseURL(jira.Key(task.IssueKey))
	}

	address, err := url.Parse(task.IssueURL)
	if err != nil || (address.Scheme != "http" && address.Scheme != "https") || address.Host == "" {
		return ""
	}

	return task.IssueURL
}

// annotationsDTO maps a task's notes onto the wire, an empty list rather than
// null.
func annotationsDTO(annotations []taskwarrior.Annotation) []api.TaskAnnotation {
	out := make([]api.TaskAnnotation, 0, len(annotations))
	for _, annotation := range annotations {
		out = append(out, api.TaskAnnotation{Entry: annotation.Entry, Description: annotation.Description})
	}

	return out
}

// orEmpty is texts, or an empty list rather than null on the wire.
func orEmpty(texts []string) []string {
	if texts == nil {
		return []string{}
	}

	return texts
}
