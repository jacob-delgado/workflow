// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package runs_test

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/hooks"
	"github.com/jacob-delgado/workflow/internal/proc"
	"github.com/jacob-delgado/workflow/internal/webserver"
	"github.com/jacob-delgado/workflow/internal/webserver/runs"
	"github.com/jacob-delgado/workflow/internal/webserver/webtest"
)

// The bodies that ask for each run but a fixup, which names a commit: a hook
// run of pre-commit, and of pre-push.
const (
	preCommitRun = `{"kind":"hook","hook":"pre-commit"}`
	prePushRun   = `{"kind":"hook","hook":"pre-push"}`
	rebaseRun    = `{"kind":"rebase"}`
	amendRun     = `{"kind":"amend"}`
)

// runsAt is where a run is started, and runningAt where the one going is
// stopped.
const (
	runsAt    = "/api/runs"
	runningAt = "/api/runs/current"
)

// errExit is a program that ended with a failure status.
var errExit = errors.New("exit status 1")

// finished is a run's output that has already been written in full, ending as
// exit says.
func finished(exit error, lines ...string) proc.Output {
	written := make(chan string, len(lines))
	for _, line := range lines {
		written <- line
	}

	close(written)

	return proc.Output{Lines: written, Wait: func() error { return exit }, Stop: func() {}}
}

// runCalls records what the git seams were asked to run.
type runCalls struct {
	mu    sync.Mutex
	calls []string
}

// record notes one call and answers output.
func (c *runCalls) record(call string, output proc.Output) (proc.Output, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.calls = append(c.calls, call)

	return output, nil
}

// asked is every call so far.
func (c *runCalls) asked() []string {
	c.mu.Lock()
	defer c.mu.Unlock()

	return slices.Clone(c.calls)
}

// configuredHooks is the tests' lefthook configuration.
func configuredHooks() hooks.Configured {
	return hooks.Configured{
		Runnable: []string{webtest.PreCommitHook, webtest.PrePushHook},
		GitOnly:  []hooks.GitOnly{{Name: webtest.CommitMsgHook, Reason: webtest.CommitMsgReason}},
	}
}

// runDeps is webtest.FilledDeps — a staged change, and one commit not yet
// pushed on fix/PROJ-412 off origin/main — with every run's seam answering
// output, and lefthook configuring configuredHooks.
func runDeps(calls *runCalls, output func() proc.Output) webserver.Deps {
	deps := webtest.FilledDeps()
	deps.Hooks.Run = func(_ context.Context, hook string) (proc.Output, error) {
		return calls.record("hook "+hook, output())
	}
	deps.Hooks.Configured = func(context.Context) (hooks.Configured, error) { return configuredHooks(), nil }
	deps.Git.Rebase = func(_ context.Context, base string) (proc.Output, error) {
		return calls.record("rebase "+base, output())
	}
	deps.Git.Amend = func(context.Context) (proc.Output, error) { return calls.record("amend", output()) }
	deps.Git.Fixup = func(_ context.Context, hash string) (proc.Output, error) {
		return calls.record("fixup "+hash, output())
	}

	return deps
}

// passing is a run that writes two lines and passes.
func passing() proc.Output {
	return finished(nil, "lint ok", "tests ok")
}

// startRun posts a run request against handler and returns the recorder.
func startRun(t *testing.T, handler http.Handler, body string) *httptest.ResponseRecorder {
	t.Helper()

	return webtest.Send(t, handler, http.MethodPost, runsAt, body)
}

// runEvents reads a run's stream, one event a line.
func runEvents(t *testing.T, recorder *httptest.ResponseRecorder) []api.RunEvent {
	t.Helper()

	if kind := recorder.Header().Get("Content-Type"); kind != "application/x-ndjson" {
		t.Fatalf("Content-Type = %q, want application/x-ndjson: %s", kind, recorder.Body)
	}

	var events []api.RunEvent

	scanner := bufio.NewScanner(strings.NewReader(recorder.Body.String()))
	for scanner.Scan() {
		var event api.RunEvent

		err := json.Unmarshal(scanner.Bytes(), &event)
		if err != nil {
			t.Fatalf("decoding the event %q: %v", scanner.Text(), err)
		}

		events = append(events, event)
	}

	return events
}

// lastRun is how the stream says the run ended.
func lastRun(t *testing.T, events []api.RunEvent) api.Run {
	t.Helper()

	if len(events) == 0 || events[len(events)-1].Run == nil {
		t.Fatalf("the stream %v does not end with the run", events)
	}

	return *events[len(events)-1].Run
}

func TestPreCommitStreamsItsOutputAsItRuns(t *testing.T) {
	t.Parallel()

	// Arrange
	calls := &runCalls{}
	handler := webtest.Serve(t, runDeps(calls, passing), config.Default())

	// Act
	events := runEvents(t, startRun(t, handler, preCommitRun))

	// Assert
	if len(events) != 4 || events[0].Run == nil || events[0].Run.State != api.RunStateInProgress {
		t.Fatalf("events = %+v, want the run starting, two lines, the run ended", events)
	}

	if *events[1].Line != "lint ok" || *events[2].Line != "tests ok" {
		t.Errorf("lines = %q, %q; want the hook's output in order", *events[1].Line, *events[2].Line)
	}

	ended := lastRun(t, events)
	if ended.State != api.RunStateSucceeded || ended.Outcome != "The pre-commit hook passed." {
		t.Errorf("the run ended %q, %q; want it passed", ended.State, ended.Outcome)
	}

	if !slices.Equal(calls.asked(), []string{"hook pre-commit"}) {
		t.Errorf("calls = %v, want the pre-commit hook run", calls.asked())
	}
}

func TestAHookRunRunsTheHookAskedFor(t *testing.T) {
	t.Parallel()

	// Arrange
	calls := &runCalls{}
	handler := webtest.Serve(t, runDeps(calls, passing), config.Default())

	// Act
	ended := lastRun(t, runEvents(t, startRun(t, handler, prePushRun)))

	// Assert
	if ended.Kind != api.RunKindHook || ended.Title != webtest.PrePushHook ||
		ended.Outcome != "The pre-push hook passed." {
		t.Errorf("the run = %+v, want pre-push run as a hook, and passed", ended)
	}

	if !slices.Equal(calls.asked(), []string{"hook pre-push"}) {
		t.Errorf("calls = %v, want the pre-push hook run", calls.asked())
	}
}

func TestARunThatFailsEndsRefusedWithItsOutput(t *testing.T) {
	t.Parallel()

	// Arrange
	failing := func() proc.Output { return finished(errExit, "\x1b[31mlint: 2 issues\x1b[0m") }
	handler := webtest.Serve(t, runDeps(&runCalls{}, failing), config.Default())

	// Act
	ended := lastRun(t, runEvents(t, startRun(t, handler, preCommitRun)))

	// Assert
	if ended.State != api.RunStateRefused || ended.Outcome != "The pre-commit hook failed." {
		t.Errorf("the run ended %q, %q; want it refused", ended.State, ended.Outcome)
	}

	if !slices.Equal(ended.Lines, []string{"lint: 2 issues"}) {
		t.Errorf("lines = %q, want the output with its terminal controls taken out", ended.Lines)
	}
}

func TestRebaseGoesOntoTheBaseGitNames(t *testing.T) {
	t.Parallel()

	// Arrange
	calls := &runCalls{}
	handler := webtest.Serve(t, runDeps(calls, passing), config.Default())

	// Act
	ended := lastRun(t, runEvents(t, startRun(t, handler, rebaseRun)))

	// Assert
	if ended.State != api.RunStateSucceeded || ended.Outcome != "Rebased onto main." {
		t.Errorf("the run ended %q, %q; want rebased onto main", ended.State, ended.Outcome)
	}

	if !slices.Equal(calls.asked(), []string{"rebase " + webtest.TestBase}) {
		t.Errorf("calls = %v, want a rebase onto %s", calls.asked(), webtest.TestBase)
	}
}

func TestAmendFoldsTheStagedChangesIntoTheLastCommit(t *testing.T) {
	t.Parallel()

	// Arrange
	calls := &runCalls{}
	handler := webtest.Serve(t, runDeps(calls, passing), config.Default())

	// Act
	ended := lastRun(t, runEvents(t, startRun(t, handler, amendRun)))

	// Assert
	if ended.Outcome != "Amended "+webtest.TestCommitSubject+"." || !slices.Equal(calls.asked(), []string{"amend"}) {
		t.Errorf("the run ended %q after %v, want the last commit amended", ended.Outcome, calls.asked())
	}
}

func TestFixupRecordsAFixupOfTheCommitNamed(t *testing.T) {
	t.Parallel()

	// Arrange
	calls := &runCalls{}
	handler := webtest.Serve(t, runDeps(calls, passing), config.Default())

	// Act
	ended := lastRun(t, runEvents(t, startRun(t, handler, `{"kind":"fixup","commit":"abc123"}`)))

	// Assert
	if ended.Outcome != "Recorded a fixup! of "+webtest.TestCommitSubject+"." ||
		!slices.Equal(calls.asked(), []string{"fixup abc123"}) {
		t.Errorf("the run ended %q after %v, want a fixup! of abc123", ended.Outcome, calls.asked())
	}
}

// A repeated key leaves which of its values counts to whoever reads the body,
// so the contract refuses the body, whichever value comes first.
func TestARunRequestThatRepeatsAKeyIsRefusedAndNothingRuns(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"a kind that is first a number":   `{"kind":5,"kind":"rebase"}`,
		"a commit that is first a number": `{"kind":"fixup","commit":7,"commit":"abc123"}`,
		"a kind that names two runs":      `{"kind":"amend","kind":"rebase"}`,
	}

	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			calls := &runCalls{}
			handler := webtest.Serve(t, runDeps(calls, passing), config.Default())

			// Act
			recorder := startRun(t, handler, body)

			// Assert
			failure := assertProblem(t, recorder, http.StatusBadRequest, "did not match the API contract")
			if failure.Code != api.ProblemCodeBadRequest {
				t.Errorf("code = %q, want %q", failure.Code, api.ProblemCodeBadRequest)
			}

			if len(calls.asked()) != 0 {
				t.Errorf("calls = %v, want nothing run", calls.asked())
			}
		})
	}
}

// StartRun is registered by hand and reads its own body, so it refuses one
// that does not decode whether or not the contract stands in front of it.
func TestStartRunRefusesABodyThatDoesNotDecodeAndRunsNothing(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"a kind that is a number":   `{"kind":5}`,
		"a commit that is a number": `{"kind":"fixup","commit":7}`,
		"a body cut short":          `{"kind":"rebase"`,
	}

	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			calls := &runCalls{}
			deps := runDeps(calls, passing)
			runner := runs.New(runs.Seams{Git: deps.Git, Hooks: deps.Hooks}, func(error) {}, &sync.Mutex{},
				func() int { return 1 })
			recorder := httptest.NewRecorder()

			// Act
			runner.StartRun(recorder, runRequest(t, body))

			// Assert
			failure := assertProblem(t, recorder, http.StatusBadRequest, "could not be understood")
			if failure.Code != api.ProblemCodeBadRequest {
				t.Errorf("code = %q, want %q", failure.Code, api.ProblemCodeBadRequest)
			}

			if len(calls.asked()) != 0 {
				t.Errorf("calls = %v, want nothing run", calls.asked())
			}
		})
	}
}

// runRequest is the page's request to start the run body asks for.
func runRequest(t *testing.T, body string) *http.Request {
	t.Helper()

	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, runsAt, strings.NewReader(body))
	request.Host = webtest.LoopbackHost
	request.Header.Set("Content-Type", "application/json")

	return request
}

func TestARunGoesToItsEndWhenThePageHasGone(t *testing.T) {
	t.Parallel()

	// Arrange
	calls := &runCalls{}
	handler := webtest.Serve(t, runDeps(calls, passing), config.Default())

	// Act
	handler.ServeHTTP(&webtest.FailingFlushWriter{}, runRequest(t, amendRun))

	// Assert
	again := startRun(t, handler, amendRun)
	if again.Code != http.StatusOK || len(calls.asked()) != 2 {
		t.Errorf("a run after one whose page went is %d after %v, want it to run: the first ended", again.Code, calls.asked())
	}
}

func TestARunWritesToAPageThatCannotFlush(t *testing.T) {
	t.Parallel()

	// Arrange
	calls := &runCalls{}
	handler := webtest.Serve(t, runDeps(calls, passing), config.Default())
	writer := &webtest.UnflushableWriter{}

	// Act
	handler.ServeHTTP(writer, runRequest(t, preCommitRun))

	// Assert
	if writer.Code != 0 && writer.Code != http.StatusOK || len(calls.asked()) != 1 {
		t.Errorf("status %d after %v, want the run made and written unflushed", writer.Code, calls.asked())
	}
}
