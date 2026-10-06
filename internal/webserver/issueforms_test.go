// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

import (
	"fmt"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/webserver"
)

// Where the issue forms' requests go, for testKey.
const (
	statusChangesPath = "/api/issues/" + testKey + "/transitions"
	assigneePath      = "/api/issues/" + testKey + "/assignee"
	worklogPath       = "/api/issues/" + testKey + "/worklog"
	// resolved and blocked are where the two changes testKey offers lead.
	resolved = "Resolved"
	blocked  = "Blocked"
	// assignAna and logAnHour are an assignment and a worklog that can be made.
	assignAna = `{"assignee":"ana"}`
	logAnHour = `{"time_spent":"1h"}`
	// resolveBody is a change to Resolved with every field it needs filled.
	resolveBody = `{"transition_id":"5","fields":[` +
		`{"id":"duedate","text":"2026-10-09"},` +
		`{"id":"fixVersions","option_ids":["10","11"]},` +
		`{"id":"resolution","option_id":"2"}]}`
)

// resolveMove is a change to Resolved that needs a resolution, the fix
// versions and a due date, beside one that needs nothing.
func resolveMove() jira.Transition {
	return jira.Transition{
		ID: "5", Name: "Resolve Issue", ToStatus: resolved, ToStatusCategory: "done",
		Fields: []jira.Field{
			{ID: "duedate", Name: "Due date", Kind: jira.FieldDate},
			{
				ID: "fixVersions", Name: "Fix Version/s", Kind: jira.FieldOptionList,
				Options: []jira.Option{{ID: "10", Name: "1.0"}, {ID: "11", Name: "1.1"}},
			},
			{
				ID: "resolution", Name: "Resolution", Kind: jira.FieldOption,
				Options: []jira.Option{{ID: "1", Name: "Fixed"}, {ID: "2", Name: "Won't Fix"}},
			},
		},
	}
}

// statusChange is one change the seam was asked to make.
type statusChange struct {
	to     jira.Transition
	values []jira.FieldValue
}

// issueWrites records what the issue forms' seams were asked.
type issueWrites struct {
	changes   []statusChange
	assigned  []string
	worklogs  []string
	worklogOn []jira.Key
}

// formDeps is filledDeps with the issue forms' seams wired over writes:
// testKey offers a change to Blocked that needs nothing and resolveMove.
func formDeps(writes *issueWrites) webserver.Deps {
	deps := filledDeps()
	deps.Transitions = func(jira.Key) ([]jira.Transition, error) {
		return []jira.Transition{
			{ID: "11", Name: "Block", ToStatus: blocked, ToStatusCategory: "new"},
			resolveMove(),
		}, nil
	}
	deps.Transition = func(_ jira.Key, to jira.Transition, values []jira.FieldValue) error {
		writes.changes = append(writes.changes, statusChange{to: to, values: values})

		return nil
	}
	deps.Assign = func(issueKey jira.Key, assignee string) error {
		writes.assigned = append(writes.assigned, string(issueKey)+" "+assignee)

		return nil
	}
	deps.AddWorklog = func(issueKey jira.Key, timeSpent, comment string) (jira.Worklog, error) {
		writes.worklogs = append(writes.worklogs, timeSpent+" "+comment)
		writes.worklogOn = append(writes.worklogOn, issueKey)

		return jira.Worklog{ID: "900", TimeSpent: timeSpent}, nil
	}

	return deps
}

// sent is everything the issue forms' seams were asked to write.
func (w issueWrites) sent() int {
	return len(w.changes) + len(w.assigned) + len(w.worklogs)
}

func TestStatusChangesListEachWithTheFieldsItNeeds(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := formDeps(&issueWrites{})
	deps.Transitions = func(jira.Key) ([]jira.Transition, error) {
		move := resolveMove()
		move.Fields = append(move.Fields, jira.Field{ID: "components", Name: "Component tree"})

		return []jira.Transition{move}, nil
	}

	// Act
	recorder := get(t, serve(t, deps, config.Default()), statusChangesPath)

	// Assert
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d (%s), want 200", recorder.Code, recorder.Body.String())
	}

	changes := decode[[]api.StatusChange](t, recorder)
	if len(changes) != 1 || changes[0].ID != "5" || changes[0].ToStatus != resolved ||
		changes[0].Name != "Resolve Issue" || changes[0].ToStatusCategory != api.StatusCategoryDone {
		t.Fatalf("changes = %+v, want the one change to Resolved", changes)
	}

	requireResolveFields(t, changes[0].Fields)
}

// requireResolveFields fails the test unless fields are resolveMove's with a
// field only Jira fills after them: a date, a list, an option with its two
// values, and the one only Jira fills.
func requireResolveFields(t *testing.T, fields []api.StatusChangeField) {
	t.Helper()

	kinds := make([]api.StatusChangeFieldKind, 0, len(fields))
	for _, field := range fields {
		kinds = append(kinds, field.Kind)
	}

	want := []api.StatusChangeFieldKind{
		api.FieldDateKind, api.FieldOptionListKind, api.FieldOptionKind, api.FieldOnlyJiraKind,
	}
	if !slices.Equal(kinds, want) || len(fields[2].Options) != 2 || fields[2].Options[1].Name != "Won't Fix" {
		t.Errorf("fields = %+v, want a date, a list, an option with its two values, and one only Jira fills", fields)
	}
}

func TestStatusChangesAreUnavailableWithoutATracker(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := formDeps(&issueWrites{})
	deps.Transitions = nil

	// Act
	recorder := get(t, serve(t, deps, config.Default()), statusChangesPath)

	// Assert
	if failure := decode[api.Problem](t, recorder); recorder.Code != http.StatusUnprocessableEntity ||
		failure.Code != api.Unprocessable {
		t.Errorf("status/code = %d/%s, want 422/unprocessable", recorder.Code, failure.Code)
	}
}

func TestAStatusChangeIsMadeWithTheFieldsItNeeds(t *testing.T) {
	t.Parallel()

	// Arrange
	var writes issueWrites

	handler := serve(t, formDeps(&writes), config.Default())

	// Act
	recorder := send(t, handler, http.MethodPost, statusChangesPath, resolveBody)

	// Assert
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d (%s), want 200", recorder.Code, recorder.Body.String())
	}

	if moved := decode[api.MovedIssue](t, recorder); moved.Key != testKey || moved.Status != resolved {
		t.Errorf("answer = %+v, want %s now Resolved", moved, testKey)
	}

	if len(writes.changes) != 1 || writes.changes[0].to.ID != "5" {
		t.Fatalf("changes = %+v, want the change to Resolved made once", writes.changes)
	}

	requireResolveValues(t, writes.changes[0].values)
}

// requireResolveValues fails the test unless values are resolveBody's: the
// date, both versions and Won't Fix, each with its field.
func requireResolveValues(t *testing.T, values []jira.FieldValue) {
	t.Helper()

	if len(values) != 3 || values[0].Text != "2026-10-09" || !slices.Equal(values[1].OptionIDs, []string{"10", "11"}) ||
		values[2].OptionID != "2" || values[2].Field.Kind != jira.FieldOption {
		t.Errorf("values = %+v, want the date, both versions and Won't Fix, each with its field", values)
	}
}

func TestAStatusChangeNeedingNoFieldsIsMade(t *testing.T) {
	t.Parallel()

	// Arrange
	var writes issueWrites

	handler := serve(t, formDeps(&writes), config.Default())

	// Act
	recorder := send(t, handler, http.MethodPost, statusChangesPath, `{"transition_id":"11","fields":[]}`)

	// Assert
	if recorder.Code != http.StatusOK || len(writes.changes) != 1 || writes.changes[0].to.ToStatus != blocked ||
		len(writes.changes[0].values) != 0 {
		t.Errorf("status = %d, changes = %+v; want 200 and the change to Blocked made with no fields",
			recorder.Code, writes.changes)
	}
}

func TestAStatusChangeIsRefusedWithAFieldItCannotTake(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		body      string
		wantInIt  string
		transform func(*webserver.Deps)
	}{
		"a field left out": {
			body:     `{"transition_id":"5","fields":[{"id":"resolution","option_id":"1"}]}`,
			wantInIt: "Due date needs a value",
		},
		"a date that is not one": {
			body: `{"transition_id":"5","fields":[{"id":"duedate","text":"Friday"},` +
				`{"id":"fixVersions","option_ids":["10"]},{"id":"resolution","option_id":"1"}]}`,
			wantInIt: "Due date must be a date like 2026-09-21",
		},
		"an option the field does not offer": {
			body: `{"transition_id":"5","fields":[{"id":"duedate","text":"2026-10-09"},` +
				`{"id":"fixVersions","option_ids":["10"]},{"id":"resolution","option_id":"7"}]}`,
			wantInIt: "Resolution is not one of its values",
		},
		"a field it does not ask for": {
			body:     `{"transition_id":"11","fields":[{"id":"summary","text":"renamed"}]}`,
			wantInIt: "Block does not ask for summary",
		},
		"a field only Jira fills": {
			body:     `{"transition_id":"5","fields":[]}`,
			wantInIt: "Resolve Issue needs Component tree, which only Jira's own screen can fill",
			transform: func(deps *webserver.Deps) {
				deps.Transitions = func(jira.Key) ([]jira.Transition, error) {
					move := resolveMove()
					move.Fields = append(move.Fields, jira.Field{ID: "components", Name: "Component tree"})

					return []jira.Transition{move}, nil
				}
			},
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			var writes issueWrites

			deps := formDeps(&writes)
			if tt.transform != nil {
				tt.transform(&deps)
			}

			// Act
			recorder := send(t, serve(t, deps, config.Default()), http.MethodPost, statusChangesPath, tt.body)

			// Assert
			failure := decode[api.Problem](t, recorder)
			if recorder.Code != http.StatusUnprocessableEntity || writes.sent() != 0 {
				t.Errorf("status = %d, changes = %+v; want 422 and nothing sent", recorder.Code, writes.changes)
			}

			if !strings.Contains(failure.Detail, tt.wantInIt) {
				t.Errorf("detail = %q, want it to say %q", failure.Detail, tt.wantInIt)
			}
		})
	}
}

func TestAStatusChangeNoLongerOfferedIsRefused(t *testing.T) {
	t.Parallel()

	// Arrange
	var writes issueWrites

	handler := serve(t, formDeps(&writes), config.Default())

	// Act
	recorder := send(t, handler, http.MethodPost, statusChangesPath, `{"transition_id":"99","fields":[]}`)

	// Assert
	failure := decode[api.Problem](t, recorder)
	if recorder.Code != http.StatusConflict || failure.Code != api.Conflict || writes.sent() != 0 {
		t.Errorf("status/code = %d/%s, sent %d; want 409/conflict and nothing sent", recorder.Code, failure.Code,
			writes.sent())
	}
}

func TestAnIssueIsAssigned(t *testing.T) {
	t.Parallel()

	// Arrange
	var writes issueWrites

	handler := serve(t, formDeps(&writes), config.Default())

	// Act
	recorder := send(t, handler, http.MethodPut, assigneePath, `{"assignee":" ana.lopez "}`)

	// Assert
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d (%s), want 200", recorder.Code, recorder.Body.String())
	}

	if !slices.Equal(writes.assigned, []string{testKey + " ana.lopez"}) {
		t.Errorf("assigned = %v, want %s assigned to ana.lopez once", writes.assigned, testKey)
	}

	if answer := decode[api.AssignedIssue](t, recorder); answer.Key != testKey || answer.Assignee != "ana.lopez" {
		t.Errorf("answer = %+v, want %s assigned to ana.lopez", answer, testKey)
	}
}

func TestWorkIsLoggedWithItsNote(t *testing.T) {
	t.Parallel()

	// Arrange
	var writes issueWrites

	handler := serve(t, formDeps(&writes), config.Default())

	// Act
	recorder := send(t, handler, http.MethodPost, worklogPath, `{"time_spent":"1h 30m","comment":"pairing"}`)

	// Assert
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d (%s), want 200", recorder.Code, recorder.Body.String())
	}

	if !slices.Equal(writes.worklogs, []string{"1h 30m pairing"}) {
		t.Errorf("worklogs = %v, want 1h 30m logged with the note", writes.worklogs)
	}

	if logged := decode[api.LoggedWork](t, recorder); logged.Key != testKey || logged.TimeSpent != "1h 30m" {
		t.Errorf("answer = %+v, want 1h 30m logged on %s", logged, testKey)
	}
}

func TestAnIssueWriteIsRefusedWhereItCannotBeMade(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		method, path, body string
		unwire             func(*webserver.Deps)
	}{
		"a blank assignee":       {method: http.MethodPut, path: assigneePath, body: `{"assignee":"   "}`},
		"no tracker to assign":   {method: http.MethodPut, path: assigneePath, body: assignAna, unwire: noAssign},
		"a blank duration":       {method: http.MethodPost, path: worklogPath, body: `{"time_spent":"  "}`},
		"no tracker to log work": {method: http.MethodPost, path: worklogPath, body: logAnHour, unwire: noWorklog},
		"work on a forge issue": {
			method: http.MethodPost, path: "/api/issues/42/worklog", body: logAnHour,
		},
		"no tracker to change status": {
			method: http.MethodPost, path: statusChangesPath, body: `{"transition_id":"11","fields":[]}`,
			unwire: func(deps *webserver.Deps) { deps.Transition = nil },
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			var writes issueWrites

			deps := formDeps(&writes)
			if tt.unwire != nil {
				tt.unwire(&deps)
			}

			// Act
			recorder := send(t, serve(t, deps, config.Default()), tt.method, tt.path, tt.body)

			// Assert
			failure := decode[api.Problem](t, recorder)
			if recorder.Code != http.StatusUnprocessableEntity || failure.Code != api.Unprocessable || writes.sent() != 0 {
				t.Errorf("status/code = %d/%s, sent %d; want 422/unprocessable and nothing sent",
					recorder.Code, failure.Code, writes.sent())
			}
		})
	}
}

// noAssign unwires the assign seam.
func noAssign(deps *webserver.Deps) { deps.Assign = nil }

// noWorklog unwires the worklog seam.
func noWorklog(deps *webserver.Deps) { deps.AddWorklog = nil }

func TestDryRunRefusesTheIssueForms(t *testing.T) {
	t.Parallel()

	cases := map[string]struct{ method, path, body string }{
		"a status change": {method: http.MethodPost, path: statusChangesPath, body: resolveBody},
		"an assignee":     {method: http.MethodPut, path: assigneePath, body: assignAna},
		"a worklog":       {method: http.MethodPost, path: worklogPath, body: logAnHour},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			var writes issueWrites

			dryRun := webserver.Info{Version: testVersion, DryRun: true}
			handler := serveWith(t, formDeps(&writes), config.Default(), dryRun)

			// Act
			recorder := send(t, handler, tt.method, tt.path, tt.body)

			// Assert
			if recorder.Code != http.StatusForbidden || writes.sent() != 0 {
				t.Errorf("status = %d, sent %d; want 403 and nothing sent", recorder.Code, writes.sent())
			}
		})
	}
}

func TestAnIssueFormsFailureNeverCarriesTheTrackersHost(t *testing.T) {
	t.Parallel()

	failing := func(cause error) error { return fmt.Errorf("%w: https://%s/rest/api/2/issue", cause, jiraHost) }
	cases := map[string]struct {
		method, path, body string
		cause              error
		status             int
	}{
		"listing the changes, unreachable": {
			method: http.MethodGet, path: statusChangesPath, cause: jira.ErrUnreachable, status: http.StatusBadGateway,
		},
		"a change Jira refuses": {
			method: http.MethodPost, path: statusChangesPath, body: `{"transition_id":"11","fields":[]}`,
			cause: jira.ErrRejected, status: http.StatusUnprocessableEntity,
		},
		"an assignee Jira refuses": {
			method: http.MethodPut, path: assigneePath, body: `{"assignee":"nobody"}`,
			cause: jira.ErrRejected, status: http.StatusUnprocessableEntity,
		},
		"a worklog, unreachable": {
			method: http.MethodPost, path: worklogPath, body: logAnHour,
			cause: jira.ErrUnreachable, status: http.StatusBadGateway,
		},
		"an issue Jira lacks": {
			method: http.MethodPut, path: assigneePath, body: assignAna,
			cause: jira.ErrNotFound, status: http.StatusNotFound,
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			deps := formDeps(&issueWrites{})
			failWith(&deps, failing(tt.cause))

			// Act
			recorder := send(t, serve(t, deps, config.Default()), tt.method, tt.path, tt.body)

			// Assert
			if recorder.Code != tt.status {
				t.Errorf("status = %d (%s), want %d", recorder.Code, recorder.Body.String(), tt.status)
			}

			if strings.Contains(recorder.Body.String(), jiraHost) {
				t.Errorf("body = %q, leaks the Jira host", recorder.Body.String())
			}
		})
	}
}

// failWith has every issue form's tracker call fail with err, but the read
// of the changes a change is checked against when err is a refusal, so the
// change itself is what is refused.
func failWith(deps *webserver.Deps, err error) {
	if !strings.Contains(err.Error(), jira.ErrRejected.Error()) {
		deps.Transitions = func(jira.Key) ([]jira.Transition, error) { return nil, err }
	}

	deps.Transition = func(jira.Key, jira.Transition, []jira.FieldValue) error { return err }
	deps.Assign = func(jira.Key, string) error { return err }
	deps.AddWorklog = func(jira.Key, string, string) (jira.Worklog, error) { return jira.Worklog{}, err }
}
