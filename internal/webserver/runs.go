// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"sync"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/loop"
	"github.com/jacob-delgado/workflow/internal/proc"
	"github.com/jacob-delgado/workflow/internal/sanitize"
)

// preCommitHook is the hook a pre_commit run runs, as the terminal's h does.
const preCommitHook = "pre-commit"

// runTail is how many of a run's last lines the event stream carries: enough
// to see where it is, from a page opened while it runs.
const runTail = 200

// Why a run was not started.
var (
	errRunGoing        = errors.New("a run is already going; wait for it to end, or stop it")
	errNoRunGoing      = errors.New("no run is going")
	errRunUnavailable  = errors.New("that run is not available here")
	errNothingToRebase = errors.New("there is nothing to rebase: check out a branch of its own, " +
		"with a base to replay it onto")
	errNothingToFold = errors.New("there is nothing to amend or fix up: stage changes, " +
		"and have a commit not yet pushed to fold them into")
	errNotFoldable = errors.New("that commit cannot be fixed up: only a commit not yet pushed can, " +
		"as the branch lists it")
)

// runSlot is the one git run going, if any: POST /api/runs claims it before it
// starts a program and frees it once the program has ended. Its lock is its
// own, since the stream reads it while the run writes to it.
type runSlot struct {
	mu    sync.Mutex
	going *goingRun
}

// goingRun is the run going now: how it stands, how to stop it once its
// program has started, and whether it was asked to stop.
type goingRun struct {
	run     api.Run
	stop    func()
	stopped bool
}

// plannedRun is a run ready to start: what it is, how to start its program, and
// what it says once it has ended either way.
type plannedRun struct {
	kind      api.RunKind
	title     string
	start     func() (proc.Output, error)
	succeeded string
	refused   string
}

// startRun starts the git run the request names and streams it, one RunEvent
// a line: the run as it starts, each line its program writes, and the run as
// it ended. It is registered by hand, as the event stream is, since the strict
// one-response interface cannot stream. A refusal is a problem written before
// anything runs. The run holds the index while it goes, and goes on to its end
// on this connection's goroutine whether or not the page stays to read it.
func (s *server) startRun(w http.ResponseWriter, request *http.Request) {
	var asked api.RunRequest

	err := json.NewDecoder(request.Body).Decode(&asked)
	if err != nil {
		writeRequestError(w, request, err)

		return
	}

	planned, err := s.planRun(asked)
	if err != nil {
		writeRunRefusal(w, err)

		return
	}

	if !s.claimRun(planned) {
		writeProblem(w, api.ProblemCodeConflict, errRunGoing.Error())

		return
	}

	defer s.freeRun()

	s.indexWrites.Lock()
	defer s.indexWrites.Unlock()

	output, err := planned.start()
	if err != nil {
		writeProblem(w, api.ProblemCodeUnprocessable,
			planned.title+" could not be started; run it from a terminal to see why")

		return
	}

	s.streamRun(w, planned, output)
}

// writeRunRefusal answers a run refused before it started.
func writeRunRefusal(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, errNothingToRebase), errors.Is(err, errNothingToFold):
		writeProblem(w, api.ProblemCodeConflict, err.Error())
	default:
		writeProblem(w, api.ProblemCodeUnprocessable, err.Error())
	}
}

// planRun is the run asked for, read against the branch and the changes as
// they stand: a rebase onto the base git names, and an amend or a fixup only of
// a commit not yet pushed.
func (s *server) planRun(asked api.RunRequest) (plannedRun, error) {
	planners := map[api.RunKind]func(api.RunRequest) (plannedRun, error){
		api.RunKindPreCommit: s.planPreCommit,
		api.RunKindRebase:    s.planRebase,
		api.RunKindAmend:     s.planAmend,
		api.RunKindFixup:     s.planFixup,
	}

	// The contract admits only these kinds, so every request names one.
	return planners[asked.Kind](asked)
}

// planPreCommit runs the pre-commit hook on what is staged.
func (s *server) planPreCommit(api.RunRequest) (plannedRun, error) {
	if s.deps.RunHook == nil {
		return plannedRun{}, errRunUnavailable
	}

	run := s.deps.RunHook

	return plannedRun{
		kind: api.RunKindPreCommit, title: preCommitHook,
		start:     func() (proc.Output, error) { return run(preCommitHook) },
		succeeded: "The pre-commit hook passed.", refused: "The pre-commit hook failed.",
	}, nil
}

// planRebase replays the checked-out branch onto its base.
func (s *server) planRebase(api.RunRequest) (plannedRun, error) {
	if s.deps.Rebase == nil || s.deps.Branch == nil {
		return plannedRun{}, errRunUnavailable
	}

	branch, err := s.deps.Branch()
	if err != nil || !loop.CanRebase(branch) {
		return plannedRun{}, errNothingToRebase
	}

	rebase := s.deps.Rebase

	return plannedRun{
		kind: api.RunKindRebase, title: "git rebase",
		start:     func() (proc.Output, error) { return rebase(branch.Base) },
		succeeded: "Rebased onto " + branch.BaseName() + ".",
		refused: "The rebase stopped: resolve the conflict in a terminal, " +
			"then go on with git rebase --continue, or undo it with git rebase --abort.",
	}, nil
}

// planAmend folds the staged changes into the branch's last commit, which
// must be one not yet pushed.
func (s *server) planAmend(api.RunRequest) (plannedRun, error) {
	if s.deps.Amend == nil {
		return plannedRun{}, errRunUnavailable
	}

	foldable, err := s.foldable()
	if err != nil {
		return plannedRun{}, err
	}

	last := foldable[len(foldable)-1]

	return plannedRun{
		kind: api.RunKindAmend, title: "git commit --amend", start: s.deps.Amend,
		succeeded: "Amended " + last.Subject + ".", refused: "The amend was refused.",
	}, nil
}

// planFixup records a fixup! of the commit asked for, which must be one not
// yet pushed.
func (s *server) planFixup(asked api.RunRequest) (plannedRun, error) {
	if s.deps.Fixup == nil {
		return plannedRun{}, errRunUnavailable
	}

	foldable, err := s.foldable()
	if err != nil {
		return plannedRun{}, err
	}

	at := slices.IndexFunc(foldable, func(commit gitrepo.Commit) bool { return commit.Hash == orZero(asked.Commit) })
	if at < 0 {
		return plannedRun{}, errNotFoldable
	}

	chosen, fixup := foldable[at], s.deps.Fixup

	return plannedRun{
		kind: api.RunKindFixup, title: "git commit --fixup",
		start:     func() (proc.Output, error) { return fixup(chosen.Hash) },
		succeeded: "Recorded a fixup! of " + chosen.Subject + ".", refused: "The fixup was refused.",
	}, nil
}

// foldable is the commits staged changes can be folded into, by loop's rule,
// or errNothingToFold.
func (s *server) foldable() ([]gitrepo.Commit, error) {
	if s.deps.Branch == nil || s.deps.Changes == nil {
		return nil, errRunUnavailable
	}

	branch, err := s.deps.Branch()
	if err != nil {
		return nil, errNothingToFold
	}

	changes, err := s.deps.Changes()
	if err != nil {
		return nil, errNothingToFold
	}

	foldable := loop.Foldable(changes, branch)
	if len(foldable) == 0 {
		return nil, errNothingToFold
	}

	return foldable, nil
}

// claimRun makes planned the run going, unless one is going already.
func (s *server) claimRun(planned plannedRun) bool {
	s.run.mu.Lock()
	defer s.run.mu.Unlock()

	if s.run.going != nil {
		return false
	}

	s.run.going = &goingRun{run: api.Run{
		Kind: planned.kind, Title: planned.title, State: api.RunStateInProgress, Lines: []string{}, Outcome: "",
	}}

	return true
}

// freeRun lets the next run start.
func (s *server) freeRun() {
	s.run.mu.Lock()
	defer s.run.mu.Unlock()

	s.run.going = nil
}

// streamRun writes the run as it starts, each line its program writes as it
// comes, and the run as it ended. A write that fails — the page went away —
// stops the writing, never the run: its lines are still read to the end.
func (s *server) streamRun(w http.ResponseWriter, planned plannedRun, output proc.Output) {
	w.Header().Set("Content-Type", "application/x-ndjson")
	w.Header().Set("Cache-Control", "no-cache")

	stream := runStream{w: w}
	stream.send(api.RunEvent{Run: s.runStarted(output.Stop)})

	for line := range output.Lines {
		shown := sanitize.Line(line)
		s.addRunLine(shown)
		stream.send(api.RunEvent{Line: &shown})
	}

	stream.send(api.RunEvent{Run: s.runEnded(planned, output.Wait())})
}

// runStarted records how to stop the run, stopping it at once if it was asked
// to stop before its program started, and answers it as it starts.
func (s *server) runStarted(stop func()) *api.Run {
	s.run.mu.Lock()
	defer s.run.mu.Unlock()

	s.run.going.stop = stop
	if s.run.going.stopped {
		stop()
	}

	started := s.run.going.run

	return &started
}

// addRunLine records a line the run wrote.
func (s *server) addRunLine(line string) {
	s.run.mu.Lock()
	defer s.run.mu.Unlock()

	s.run.going.run.Lines = append(s.run.going.run.Lines, line)
}

// runEnded records how the run ended — stopped, refused or passed — and
// answers it whole.
func (s *server) runEnded(planned plannedRun, exit error) *api.Run {
	s.run.mu.Lock()
	defer s.run.mu.Unlock()

	going := s.run.going

	switch {
	case going.stopped:
		going.run.State, going.run.Outcome = api.RunStateStopped, "Stopped "+planned.title+"."
	case exit != nil:
		going.run.State, going.run.Outcome = api.RunStateRefused, planned.refused
	default:
		going.run.State, going.run.Outcome = api.RunStateSucceeded, planned.succeeded
	}

	ended := going.run

	return &ended
}

// runShown is the run going now, with its last runTail lines, for a frame of
// the event stream, or nil when none is going.
func (s *server) runShown() *api.Run {
	s.run.mu.Lock()
	defer s.run.mu.Unlock()

	if s.run.going == nil {
		return nil
	}

	shown := s.run.going.run
	shown.Lines = slices.Clone(shown.Lines[max(0, len(shown.Lines)-runTail):])

	return &shown
}

// StopRun stops the run going — its program's whole process group — as the
// terminal's interrupt does; its stream then ends with it stopped. With none
// going it is a 404.
func (s *server) StopRun(context.Context, api.StopRunRequestObject) (api.StopRunResponseObject, error) {
	s.run.mu.Lock()
	defer s.run.mu.Unlock()

	going := s.run.going
	if going == nil {
		return api.StopRun404ApplicationProblemPlusJSONResponse(problem(api.ProblemCodeNotFound, errNoRunGoing.Error())), nil
	}

	going.stopped = true
	if going.stop != nil {
		going.stop()
	}

	return api.StopRun204Response{}, nil
}

// runStream writes a run's events, one JSON object a line, flushing each, and
// stops writing once a write fails.
type runStream struct {
	w      http.ResponseWriter
	failed bool
}

// send writes one event, unless an earlier write failed.
func (r *runStream) send(event api.RunEvent) {
	if r.failed {
		return
	}

	err := json.NewEncoder(r.w).Encode(event)
	if err != nil {
		r.failed = true

		return
	}

	if flusher, ok := r.w.(http.Flusher); ok {
		flusher.Flush()
	}
}
