// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/proc"
	"github.com/jacob-delgado/workflow/internal/webserver"
)

// filledHead is the head commit of filledDeps' branch, its one commit.
const filledHead = "abc123"

// The bodies that ask for each run but a fixup, which names a commit.
const (
	preCommitRun = `{"kind":"pre_commit"}`
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

// heldRun is a run's output that writes a line, then waits until it is
// stopped, as a hook that hangs does.
type heldRun struct {
	lines   chan string
	once    sync.Once
	stopped chan struct{}
}

// newHeldRun is a run that has written its first line.
func newHeldRun(first string) *heldRun {
	held := &heldRun{lines: make(chan string, 1), stopped: make(chan struct{})}
	held.lines <- first

	return held
}

// output is the run as a seam answers it.
func (h *heldRun) output() proc.Output {
	return proc.Output{
		Lines: h.lines,
		Wait: func() error {
			<-h.stopped

			return errExit
		},
		Stop: func() {
			h.once.Do(func() {
				close(h.lines)
				close(h.stopped)
			})
		},
	}
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

// runDeps is filledDeps — a staged change, and one commit not yet pushed on
// fix/PROJ-412 off origin/main — with every run's seam answering output.
func runDeps(calls *runCalls, output func() proc.Output) webserver.Deps {
	deps := filledDeps()
	deps.RunHook = func(hook string) (proc.Output, error) { return calls.record("hook "+hook, output()) }
	deps.Rebase = func(base string) (proc.Output, error) { return calls.record("rebase "+base, output()) }
	deps.Amend = func() (proc.Output, error) { return calls.record("amend", output()) }
	deps.Fixup = func(hash string) (proc.Output, error) { return calls.record("fixup "+hash, output()) }

	return deps
}

// passing is a run that writes two lines and passes.
func passing() proc.Output {
	return finished(nil, "lint ok", "tests ok")
}

// startRun posts a run request against handler and returns the recorder.
func startRun(t *testing.T, handler http.Handler, body string) *httptest.ResponseRecorder {
	t.Helper()

	return send(t, handler, http.MethodPost, runsAt, body)
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
	handler := serve(t, runDeps(calls, passing), config.Default())

	// Act
	events := runEvents(t, startRun(t, handler, preCommitRun))

	// Assert
	if len(events) != 4 || events[0].Run == nil || events[0].Run.State != api.InProgress {
		t.Fatalf("events = %+v, want the run starting, two lines, the run ended", events)
	}

	if *events[1].Line != "lint ok" || *events[2].Line != "tests ok" {
		t.Errorf("lines = %q, %q; want the hook's output in order", *events[1].Line, *events[2].Line)
	}

	ended := lastRun(t, events)
	if ended.State != api.Succeeded || ended.Outcome != "The pre-commit hook passed." {
		t.Errorf("the run ended %q, %q; want it passed", ended.State, ended.Outcome)
	}

	if !slices.Equal(calls.asked(), []string{"hook pre-commit"}) {
		t.Errorf("calls = %v, want the pre-commit hook run", calls.asked())
	}
}

func TestARunThatFailsEndsRefusedWithItsOutput(t *testing.T) {
	t.Parallel()

	// Arrange
	failing := func() proc.Output { return finished(errExit, "\x1b[31mlint: 2 issues\x1b[0m") }
	handler := serve(t, runDeps(&runCalls{}, failing), config.Default())

	// Act
	ended := lastRun(t, runEvents(t, startRun(t, handler, preCommitRun)))

	// Assert
	if ended.State != api.Refused || ended.Outcome != "The pre-commit hook failed." {
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
	handler := serve(t, runDeps(calls, passing), config.Default())

	// Act
	ended := lastRun(t, runEvents(t, startRun(t, handler, rebaseRun)))

	// Assert
	if ended.State != api.Succeeded || ended.Outcome != "Rebased onto main." {
		t.Errorf("the run ended %q, %q; want rebased onto main", ended.State, ended.Outcome)
	}

	if !slices.Equal(calls.asked(), []string{"rebase " + testBase}) {
		t.Errorf("calls = %v, want a rebase onto %s", calls.asked(), testBase)
	}
}

func TestAmendFoldsTheStagedChangesIntoTheLastCommit(t *testing.T) {
	t.Parallel()

	// Arrange
	calls := &runCalls{}
	handler := serve(t, runDeps(calls, passing), config.Default())

	// Act
	ended := lastRun(t, runEvents(t, startRun(t, handler, amendRun)))

	// Assert
	if ended.Outcome != "Amended "+testCommitSubject+"." || !slices.Equal(calls.asked(), []string{"amend"}) {
		t.Errorf("the run ended %q after %v, want the last commit amended", ended.Outcome, calls.asked())
	}
}

func TestFixupRecordsAFixupOfTheCommitNamed(t *testing.T) {
	t.Parallel()

	// Arrange
	calls := &runCalls{}
	handler := serve(t, runDeps(calls, passing), config.Default())

	// Act
	ended := lastRun(t, runEvents(t, startRun(t, handler, `{"kind":"fixup","commit":"abc123"}`)))

	// Assert
	if ended.Outcome != "Recorded a fixup! of "+testCommitSubject+"." ||
		!slices.Equal(calls.asked(), []string{"fixup abc123"}) {
		t.Errorf("the run ended %q after %v, want a fixup! of abc123", ended.Outcome, calls.asked())
	}
}

func TestARunWithNothingToWorkOnIsRefusedBeforeItRuns(t *testing.T) {
	t.Parallel()

	onBase := func() (gitrepo.Branch, error) {
		return gitrepo.Branch{Name: prBase, Base: testBase, Head: filledHead, PushRemote: gitrepo.DefaultRemote}, nil
	}
	pushed := func() (gitrepo.Branch, error) {
		return gitrepo.Branch{
			Name: testBranchName, Base: testBase, Upstream: gitrepo.DefaultRemote + "/" + testBranchName,
			PushRemote: gitrepo.DefaultRemote, Commits: []gitrepo.Commit{{Hash: filledHead, Subject: testCommitSubject}},
		}, nil
	}
	unstaged := func() ([]gitrepo.Change, error) {
		return []gitrepo.Change{{Path: "a.go", Staged: ' ', Unstaged: 'M'}}, nil
	}

	cases := map[string]struct {
		body    string
		branch  func() (gitrepo.Branch, error)
		changes func() ([]gitrepo.Change, error)
		status  int
		saying  string
	}{
		"a rebase on the base itself": {
			body: rebaseRun, branch: onBase, status: http.StatusConflict, saying: "nothing to rebase",
		},
		"an amend with nothing staged": {
			body: amendRun, changes: unstaged, status: http.StatusConflict, saying: "nothing to amend",
		},
		"an amend of a pushed commit": {
			body: amendRun, branch: pushed, status: http.StatusConflict, saying: "nothing to amend",
		},
		"a fixup of a commit not on the branch": {
			body: `{"kind":"fixup","commit":"fff9999"}`, status: http.StatusUnprocessableEntity,
			saying: "only a commit not yet pushed",
		},
		"a fixup naming no commit": {
			body: `{"kind":"fixup"}`, status: http.StatusUnprocessableEntity, saying: "only a commit not yet pushed",
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			calls := &runCalls{}
			deps := runDeps(calls, passing)

			if tt.branch != nil {
				deps.Branch = tt.branch
			}

			if tt.changes != nil {
				deps.Changes = tt.changes
			}

			// Act
			recorder := startRun(t, serve(t, deps, config.Default()), tt.body)

			// Assert
			assertProblem(t, recorder, tt.status, tt.saying)

			if len(calls.asked()) != 0 {
				t.Errorf("calls = %v, want nothing run", calls.asked())
			}
		})
	}
}

func TestARunThatIsNotWiredIsNotAvailable(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := runDeps(&runCalls{}, passing)
	deps.RunHook = nil

	// Act
	recorder := startRun(t, serve(t, deps, config.Default()), preCommitRun)

	// Assert
	assertProblem(t, recorder, http.StatusUnprocessableEntity, "not available")
}

func TestARunThatCannotStartKeepsGitsWordsOff(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := runDeps(&runCalls{}, passing)
	deps.Rebase = func(string) (proc.Output, error) {
		return proc.Output{}, fmt.Errorf("git -C %s rebase: %w", repoPath, errSeam)
	}

	// Act
	recorder := startRun(t, serve(t, deps, config.Default()), rebaseRun)

	// Assert
	assertProblem(t, recorder, http.StatusUnprocessableEntity, "could not be started")

	if strings.Contains(recorder.Body.String(), repoPath) {
		t.Errorf("the refusal %s names where the repository is", recorder.Body)
	}
}

// startHeld starts a run that hangs after its first line on handler, and
// returns the recorder its stream is written to, once that line is written,
// and a channel closed when the stream ends.
func startHeld(t *testing.T, handler http.Handler) (*syncRecorder, chan struct{}) {
	t.Helper()

	recorder := newSyncRecorder()
	done := make(chan struct{})

	go func() {
		defer close(done)

		request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, runsAt,
			strings.NewReader(preCommitRun))
		request.Host = loopbackHost
		request.Header.Set("Content-Type", "application/json")
		handler.ServeHTTP(recorder, request)
	}()

	deadline := time.Now().Add(5 * time.Second)
	for !strings.Contains(recorder.body(), "hanging") {
		if time.Now().After(deadline) {
			t.Fatalf("the held run's first line never came; body %q", recorder.body())
		}

		time.Sleep(5 * time.Millisecond)
	}

	return recorder, done
}

func TestOneRunGoesAtATimeAndAStopEndsIt(t *testing.T) {
	t.Parallel()

	// Arrange
	held := newHeldRun("hanging")
	handler := serve(t, runDeps(&runCalls{}, held.output), config.Default())
	first, done := startHeld(t, handler)

	// Act: start a second while the first is going
	second := startRun(t, handler, preCommitRun)

	// Assert: it is refused
	assertProblem(t, second, http.StatusConflict, "already going")

	// Act: stop the first
	stopped := send(t, handler, http.MethodDelete, runningAt, "")

	<-done

	// Assert: it ends stopped
	if stopped.Code != http.StatusNoContent {
		t.Fatalf("stop status = %d, want 204: %s", stopped.Code, stopped.Body)
	}

	if !strings.Contains(first.body(), `"state":"stopped"`) {
		t.Errorf("the stream %s does not end with the run stopped", first.body())
	}
}

func TestTheStreamCarriesTheRunGoingNow(t *testing.T) {
	t.Parallel()

	// Arrange
	held := newHeldRun("hanging")
	handler := serve(t, runDeps(&runCalls{}, held.output), config.Default())
	_, done := startHeld(t, handler)

	// Act
	snap := firstSnapshot(t, streamOnce(t, handler, "/api/events").Body.String())

	// Assert
	held.output().Stop()
	<-done

	if snap.Run == nil || snap.Run.State != api.InProgress || !slices.Equal(snap.Run.Lines, []string{"hanging"}) {
		t.Errorf("the frame's run = %+v, want pre-commit in progress with its line so far", snap.Run)
	}
}

func TestTheStreamCarriesNoRunOnceItEnded(t *testing.T) {
	t.Parallel()

	// Arrange
	handler := serve(t, runDeps(&runCalls{}, passing), config.Default())
	startRun(t, handler, preCommitRun)

	// Act
	snap := firstSnapshot(t, streamOnce(t, handler, "/api/events").Body.String())

	// Assert
	if snap.Run != nil {
		t.Errorf("the frame's run = %+v, want none once it ended", snap.Run)
	}
}

func TestStoppingWithNoRunGoingIsNotFound(t *testing.T) {
	t.Parallel()

	// Act
	recorder := send(t, serve(t, runDeps(&runCalls{}, passing), config.Default()), http.MethodDelete, runningAt, "")

	// Assert
	assertProblem(t, recorder, http.StatusNotFound, "no run is going")
}

func TestARunIsHeldBackUnderADryRun(t *testing.T) {
	t.Parallel()

	// Arrange
	calls := &runCalls{}
	handler := serveWith(t, runDeps(calls, passing), config.Default(), webserver.Info{Version: testVersion, DryRun: true})

	// Act
	recorder := startRun(t, handler, rebaseRun)

	// Assert
	if recorder.Code != http.StatusForbidden || len(calls.asked()) != 0 {
		t.Errorf("status = %d after %v, want 403 and nothing run", recorder.Code, calls.asked())
	}
}

// syncRecorder is a ResponseRecorder another goroutine may read while the
// handler writes to it.
type syncRecorder struct {
	mu       sync.Mutex
	recorder *httptest.ResponseRecorder
}

// newSyncRecorder is an empty syncRecorder.
func newSyncRecorder() *syncRecorder {
	return &syncRecorder{recorder: httptest.NewRecorder()}
}

// Header is the response's header.
func (r *syncRecorder) Header() http.Header {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.recorder.Header()
}

// Write records a part of the body.
func (r *syncRecorder) Write(part []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.recorder.Write(part) //nolint:wrapcheck // a recorder's write is the write the handler made
}

// WriteHeader records the status.
func (r *syncRecorder) WriteHeader(status int) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.recorder.WriteHeader(status)
}

// Flush does nothing: the body is recorded as it is written.
func (r *syncRecorder) Flush() {}

// body is what has been written so far.
func (r *syncRecorder) body() string {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.recorder.Body.String()
}

func TestTheBranchMarksTheCommitsNotYetPushed(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := filledDeps()
	deps.Branch = func() (gitrepo.Branch, error) {
		return gitrepo.Branch{
			Name: testBranchName, Base: testBase, Upstream: gitrepo.DefaultRemote + "/" + testBranchName,
			PushRemote: gitrepo.DefaultRemote, Ahead: 1,
			Commits: []gitrepo.Commit{{Hash: "aaa1111", Subject: "feat: first"}, {Hash: "bbb2222", Subject: "fix: second"}},
		}, nil
	}

	// Act
	snap := firstSnapshot(t, streamOnce(t, serve(t, deps, config.Default()), "/api/events").Body.String())

	// Assert
	marked := make([]bool, 0, len(snap.Branch.Commits))
	for _, commit := range snap.Branch.Commits {
		marked = append(marked, commit.Unpushed)
	}

	if !slices.Equal(marked, []bool{false, true}) {
		t.Errorf("unpushed = %v, want only the commit the upstream lacks", marked)
	}
}

func TestARunWhoseReadsFailHasNothingToWorkOn(t *testing.T) {
	t.Parallel()

	failingBranch := func() (gitrepo.Branch, error) { return gitrepo.Branch{}, errSeam }
	failingChanges := func() ([]gitrepo.Change, error) { return nil, errSeam }

	cases := map[string]struct {
		body    string
		branch  func() (gitrepo.Branch, error)
		changes func() ([]gitrepo.Change, error)
	}{
		"a rebase whose branch cannot be read": {body: rebaseRun, branch: failingBranch},
		"an amend whose branch cannot be read": {body: amendRun, branch: failingBranch},
		"a fixup whose changes cannot be read": {body: `{"kind":"fixup","commit":"abc123"}`, changes: failingChanges},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			calls := &runCalls{}
			deps := runDeps(calls, passing)

			if tt.branch != nil {
				deps.Branch = tt.branch
			}

			if tt.changes != nil {
				deps.Changes = tt.changes
			}

			// Act
			recorder := startRun(t, serve(t, deps, config.Default()), tt.body)

			// Assert
			assertProblem(t, recorder, http.StatusConflict, "nothing to")

			if len(calls.asked()) != 0 {
				t.Errorf("calls = %v, want nothing run", calls.asked())
			}
		})
	}
}

func TestEveryRunIsUnavailableWithoutItsSeam(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		body  string
		unset func(*webserver.Deps)
	}{
		"a rebase":          {body: rebaseRun, unset: func(deps *webserver.Deps) { deps.Rebase = nil }},
		"an amend":          {body: amendRun, unset: func(deps *webserver.Deps) { deps.Amend = nil }},
		"a fixup":           {body: `{"kind":"fixup"}`, unset: func(deps *webserver.Deps) { deps.Fixup = nil }},
		"an amend, no tree": {body: amendRun, unset: func(deps *webserver.Deps) { deps.Changes = nil }},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			deps := runDeps(&runCalls{}, passing)
			tt.unset(&deps)

			// Act
			recorder := startRun(t, serve(t, deps, config.Default()), tt.body)

			// Assert
			assertProblem(t, recorder, http.StatusUnprocessableEntity, "not available")
		})
	}
}

func TestARunGoesToItsEndWhenThePageHasGone(t *testing.T) {
	t.Parallel()

	// Arrange
	calls := &runCalls{}
	handler := serve(t, runDeps(calls, passing), config.Default())
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, runsAt, strings.NewReader(amendRun))
	request.Host = loopbackHost
	request.Header.Set("Content-Type", "application/json")

	// Act
	handler.ServeHTTP(&failingFlushWriter{}, request)

	// Assert
	again := startRun(t, handler, amendRun)
	if again.Code != http.StatusOK || len(calls.asked()) != 2 {
		t.Errorf("a run after one whose page went is %d after %v, want it to run: the first ended", again.Code, calls.asked())
	}
}
