// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tasks

import (
	"context"
	"errors"
	"net/url"
	"slices"
	"sync"
	"time"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/report"
	"github.com/jacob-delgado/workflow/internal/taskwarrior"
	"github.com/jacob-delgado/workflow/internal/webserver/problem"
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
// seams were bound with as workflow started in or switched to this directory:
// a change applies at the next start or directory switch.
var errSettingsChanged = errors.New(
	"the taskwarrior settings changed since workflow started in or switched to this directory")

// detectionCache is the event stream's last search for Taskwarrior, held
// while it found none. search is held across a search, so streams that find
// one due together make one; mu is held around the detection alone, so the
// list letting go of it never waits on a search.
type detectionCache struct {
	search sync.Mutex
	mu     sync.Mutex
	held   detection
}

// detectStage is where the stream's search for Taskwarrior stands. It has two
// values, so it is compared, never switched on.
type detectStage int

const (
	// detectUnsearched holds no search: none made yet, one that found
	// Taskwarrior, or one the list let go of.
	detectUnsearched detectStage = iota
	// detectNoneFound holds a search that found none, which serves the
	// stream for taskwarriorHold.
	detectNoneFound
)

// detection is the stream's search for Taskwarrior as it stands: its stage,
// why it found none and when, and the round it is in, which each time the
// list lets go of it begins anew, so a search begun in an earlier round lands
// too late to be held.
type detection struct {
	stage  detectStage
	failed error
	readAt time.Time
	round  int
}

// serves reports whether d answers a frame at now in place of a search: a
// search that found none, within taskwarriorHold.
func (d detection) serves(now time.Time) bool {
	return d.stage == detectNoneFound && now.Sub(d.readAt) < taskwarriorHold
}

// landed is d once a search begun in round began answers err at now: one
// that found none is held, and one that found Taskwarrior holds nothing, but
// only while the round it began in stands; one the list let go of meanwhile
// leaves d as it is.
func (d detection) landed(began int, err error, now time.Time) detection {
	if began != d.round {
		return d
	}

	if err == nil {
		return detection{stage: detectUnsearched, failed: nil, readAt: time.Time{}, round: d.round}
	}

	return detection{stage: detectNoneFound, failed: err, readAt: now, round: d.round}
}

// forgotten is d let go of: nothing held, in a new round, so a search under
// way as the list found Taskwarrior is not held when it lands.
//
// Trade-off TRADE-76: the list lets go whenever its search began, so one
// begun before a failed search, and landing after it, erases that failure.
func (d detection) forgotten() detection {
	return detection{stage: detectUnsearched, failed: nil, readAt: time.Time{}, round: d.round + 1}
}

// ListTasks answers your pending tasks, or why no Taskwarrior can be asked —
// an answer about where the server runs, as the review queue's is, not a
// missing resource.
func (h *Handlers) ListTasks(ctx context.Context, _ api.ListTasksRequestObject) (api.ListTasksResponseObject, error) {
	list, err := h.readTaskList(ctx, "")
	if err != nil {
		return problem.Answer[api.ListTasksdefaultApplicationProblemPlusJSONResponse](h.taskFault(ctx, err)), nil
	}

	return api.ListTasks200JSONResponse(list), nil
}

// readTaskList is the list a read answers and every write ends on, asked under
// ctx, so the page replaces what it shows with what Taskwarrior holds now. said
// is what undo or sync printed, or what a write has to add. The clock is read
// once, so the tasks' states and the values the filter offers describe one
// moment.
func (h *Handlers) readTaskList(ctx context.Context, said string) (api.TaskList, error) {
	install, err := h.unlessSettingsChanged(ctx, h.listDetection)
	if err != nil {
		code, reason := unavailableReason(err)

		return api.TaskList{
			Available: false, Reason: reason, ReasonCode: &code, Tasks: []api.Task{}, FacetOrder: []api.TaskFacet{},
		}, nil
	}

	pending, err := h.seams.Tasks.Pending(ctx)
	if err != nil {
		return api.TaskList{}, err
	}

	tasks, now := knownTasks(pending.Tasks), h.kit.Now()

	return api.TaskList{
		Available: true, Reason: "", Context: pending.Context, SyncAvailable: install.SyncConfigured,
		Said: said, Tasks: h.tasksDTO(tasks, now), FacetOrder: taskFacetsDTO(taskwarrior.OfferedFacets(tasks, now)),
	}, nil
}

// unlessSettingsChanged is the Taskwarrior find answers, or errSettingsChanged
// without asking once the taskwarrior settings in effect are not the ones the
// seams were bound with: whatever find would say is about the old settings.
func (h *Handlers) unlessSettingsChanged(
	ctx context.Context, find func(context.Context) (taskwarrior.Install, error),
) (taskwarrior.Install, error) {
	if h.settingsChanged() {
		return taskwarrior.Install{}, errSettingsChanged
	}

	return find(ctx)
}

// settingsChanged reports taskwarrior settings in effect other than the ones
// the seams were bound with.
func (h *Handlers) settingsChanged() bool {
	return h.kit.Config() != h.bound
}

// settingsChangedRefusal is the refusal of a write once the taskwarrior
// settings in effect are not the ones the seams were bound with — the
// Taskwarrior the seams reach is the one the old settings found — in the words
// the list says it with, or nil while they stand.
func (h *Handlers) settingsChangedRefusal() *api.Problem {
	if !h.settingsChanged() {
		return nil
	}

	_, reason := unavailableReason(errSettingsChanged)
	prob := problem.New(api.ProblemCodeUnprocessable, reason)

	return &prob
}

// installed is the Taskwarrior that answers, asked under ctx, or why none can
// be asked: with no seam, the integration was turned off, or no task program
// was found, when workflow started in or switched to this directory.
func (h *Handlers) installed(ctx context.Context) (taskwarrior.Install, error) {
	if h.seams.Tasks.Install != nil {
		return h.seams.Tasks.Install(ctx)
	}

	if h.bound.Disabled {
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

	reasons := append(report.TaskwarriorReasons(),
		report.TaskwarriorReason{
			Cause: errTurnedOff, Code: api.TaskListReasonCodeTurnedOff, Text: "Turned off by taskwarrior.disabled.",
		},
		report.TaskwarriorReason{
			Cause: errSettingsChanged, Code: api.TaskListReasonCodeUnavailable,
			Text: "Taskwarrior settings changed; restart workflow or switch directory to apply.",
		})

	for _, reason := range reasons {
		if errors.Is(err, reason.Cause) {
			return reason.Code, reason.Text
		}
	}

	return api.TaskListReasonCodeUnavailable, "Taskwarrior could not be asked; workflow doctor says why."
}

// Summary is the stream's summary of your tasks: whether Taskwarrior can
// be asked, and why not; the started task; and every task linked to an issue.
// Until Taskwarrior has answered both reads the summary is unavailable, as the
// terminal draws no task mark until it has, so a page never takes a read that
// failed for "no task tracks this issue" and offers to track it again.
func (h *Handlers) Summary(ctx context.Context) api.TasksSummary {
	_, err := h.unlessSettingsChanged(ctx, h.heldDetection)
	if err != nil {
		_, reason := unavailableReason(err)

		return api.TasksSummary{Available: false, Reason: reason, Linked: []api.Task{}}
	}

	unanswered := api.TasksSummary{
		Available: false, Reason: "Taskwarrior could not be read; workflow doctor says why.", Linked: []api.Task{},
	}

	pending, err := h.seams.Tasks.Pending(ctx)
	if err != nil {
		return unanswered
	}

	linked, err := h.seams.Tasks.Linked(ctx)
	if err != nil {
		return unanswered
	}

	return h.tasksSummaryDTO(pending.Tasks, linked, h.kit.Now())
}

// listDetection is which Taskwarrior answers, for the list, which asks each
// time it is read: one found lets go of a search that found none the stream
// holds, so the next frame's summary says Taskwarrior is there, as the list
// does, rather than say otherwise for the rest of taskwarriorHold.
func (h *Handlers) listDetection(ctx context.Context) (taskwarrior.Install, error) {
	install, err := h.installed(ctx)
	if err == nil {
		h.detection.forget()
	}

	return install, err
}

// forget lets go of a search that found none, so the next frame searches
// again, and of one under way, so it is not held when it lands.
func (c *detectionCache) forget() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.held = c.held.forgotten()
}

// current is the detection as it stands.
func (c *detectionCache) current() detection {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.held
}

// land takes in a search begun in round began that answered err at now.
func (c *detectionCache) land(began int, err error, now time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.held = c.held.landed(began, err, now)
}

// heldDetection is which Taskwarrior answers, for the stream: a search that
// found none within taskwarriorHold stands for this one. Every stream shares
// the search, so it runs to its end even if the page whose frame began it
// leaves; a search canceled then would be held as none for every page. It ends
// when the server stops.
func (h *Handlers) heldDetection(ctx context.Context) (taskwarrior.Install, error) {
	h.detection.search.Lock()
	defer h.detection.search.Unlock()

	now, held := h.kit.Now(), h.detection.current()
	if held.serves(now) {
		return taskwarrior.Install{}, held.failed
	}

	shared, release := h.kit.Outliving(ctx)
	defer release()

	install, err := h.installed(shared)
	h.detection.land(held.round, err, now)

	return install, err
}

// tasksSummaryDTO maps what Taskwarrior answered onto the stream's summary, as
// the tasks stand at now: the started task and every linked task.
func (h *Handlers) tasksSummaryDTO(pending, linked []taskwarrior.Task, now time.Time) api.TasksSummary {
	known := knownTasks(linked)
	summary := api.TasksSummary{Available: true, Reason: "", Linked: h.tasksDTO(known, now)}
	summary.Active = h.activeTask(knownTasks(pending), known, summary.Linked, now)

	return summary
}

// activeTask is the first task started at now, pending or else linked, as the
// terminal's spine shows it, ranked among the list that holds it: your pending
// tasks, as the Tasks list ranks it, or the linked ones, given here as
// linkedDTO, when the active context hides it from the pending. It is nil when
// none is started; a started task with a wait still ahead is waiting, not
// started, as the list words it.
func (h *Handlers) activeTask(pending, linked []taskwarrior.Task, linkedDTO []api.Task, now time.Time) *api.Task {
	startedNow := func(task taskwarrior.Task) bool { return task.Started(now) }

	if started := slices.IndexFunc(pending, startedNow); started >= 0 {
		return &h.tasksDTO(pending, now)[started]
	}

	if started := slices.IndexFunc(linked, startedNow); started >= 0 {
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
func (h *Handlers) tasksDTO(tasks []taskwarrior.Task, now time.Time) []api.Task {
	ranks := taskwarrior.RanksOf(tasks, now)
	out := make([]api.Task, 0, len(tasks))

	for index, task := range tasks {
		out = append(out, h.taskDTO(task, ranks[index], now))
	}

	return out
}

// taskDTO maps a task onto the wire, each date it does not have absent, with
// where it stands at now, its facets, its ranks and the fields text matches.
func (h *Handlers) taskDTO(task taskwarrior.Task, ranks taskwarrior.Ranks, now time.Time) api.Task {
	return api.Task{
		UUID: task.UUID, ID: task.ID, Description: task.Description, Status: api.TaskStatus(task.Status),
		State: api.TaskState(task.State(now).String()), Project: task.Project, Priority: task.Priority,
		Tags: orEmpty(task.Tags),
		Due:  optionalTime(task.Due), Wait: optionalTime(task.Wait), Scheduled: optionalTime(task.Scheduled),
		Until: optionalTime(task.Until), Start: optionalTime(task.Start), End: optionalTime(task.End),
		Entry: task.Entry, Modified: task.Modified, Urgency: task.Urgency,
		Annotations: annotationsDTO(task.Annotations), IssueKey: task.IssueKey, IssueURL: h.taskIssueURL(task),
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
func (h *Handlers) taskIssueURL(task taskwarrior.Task) string {
	if task.IssueKey != "" && h.kit.BrowseURL != nil {
		return h.kit.BrowseURL(jira.Key(task.IssueKey))
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

// optionalTime is moment, or nil for the zero time, which a read gives for a
// time there is none of — a date a task does not have, one a tracker or a
// forge did not give or could not be read: the wire leaves such a time out,
// never sending the zero time.
func optionalTime(moment time.Time) *time.Time {
	if moment.IsZero() {
		return nil
	}

	return &moment
}

// orEmpty is texts, or an empty list rather than null on the wire.
func orEmpty(texts []string) []string {
	if texts == nil {
		return []string{}
	}

	return texts
}
