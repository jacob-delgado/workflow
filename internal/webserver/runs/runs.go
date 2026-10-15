// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package runs

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
	"github.com/jacob-delgado/workflow/internal/webserver/problem"
)

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
	errHookNotRunnable = errors.New("that hook is not one lefthook runs by hand here; " +
		"GET /api/hooks lists those it does")
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
	start     func(ctx context.Context) (proc.Output, error)
	succeeded string
	refused   string
}

// StartRun starts the git run the request names and streams it, one RunEvent
// a line: the run as it starts, each line its program writes, and the run as
// it ended. It is registered by hand, as the event stream is, since the strict
// one-response interface cannot stream. A refusal is a problem written before
// anything runs. The run holds the index while it goes, and goes on to its end
// on this connection's goroutine whether or not the page stays to read it.
func (r *Runner) StartRun(w http.ResponseWriter, request *http.Request) {
	var asked api.RunRequest

	// A body the contract admitted can still fail to decode: the contract
	// reads a repeated key's last value, and the decoder fails on its first.
	err := json.NewDecoder(request.Body).Decode(&asked)
	if err != nil {
		problem.WriteRequestError(w, request, err)

		return
	}

	ctx := request.Context()

	planned, err := r.planRun(ctx, asked)
	if err != nil {
		writeRunRefusal(w, err)

		return
	}

	if !r.claimRun(planned) {
		problem.Write(w, api.ProblemCodeConflict, errRunGoing.Error())

		return
	}

	defer r.freeRun()

	r.indexWrites.Lock()
	defer r.indexWrites.Unlock()

	output, err := planned.start(ctx)
	if err != nil {
		problem.Write(w, api.ProblemCodeUnprocessable,
			planned.title+" could not be started; run it from a terminal to see why")

		return
	}

	r.streamRun(w, planned, output)
}

// writeRunRefusal answers a run refused before it started.
func writeRunRefusal(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, errNothingToRebase), errors.Is(err, errNothingToFold):
		problem.Write(w, api.ProblemCodeConflict, err.Error())
	default:
		problem.Write(w, api.ProblemCodeUnprocessable, err.Error())
	}
}

// planRun is the run asked for, read against the branch and the changes as
// they stand: a rebase onto the base git names, and an amend or a fixup only of
// a commit not yet pushed. They are read under ctx.
func (r *Runner) planRun(ctx context.Context, asked api.RunRequest) (plannedRun, error) {
	planners := map[api.RunKind]func(context.Context, api.RunRequest) (plannedRun, error){
		api.RunKindHook:   r.planHook,
		api.RunKindRebase: r.planRebase,
		api.RunKindAmend:  r.planAmend,
		api.RunKindFixup:  r.planFixup,
	}

	// The contract admits only these kinds, so every request names one.
	return planners[asked.Kind](ctx, asked)
}

// planHook runs the hook asked for, as the terminal's h does, once lefthook's
// configuration, read under ctx, lists it as one a run by hand can run: no
// name lefthook did not list reaches its command line.
func (r *Runner) planHook(ctx context.Context, asked api.RunRequest) (plannedRun, error) {
	if r.seams.Hooks.Run == nil || r.seams.Hooks.Configured == nil {
		return plannedRun{}, errRunUnavailable
	}

	configured, err := r.seams.Hooks.Configured(ctx)
	if err != nil {
		r.unexpected(err)

		return plannedRun{}, errHooksUnreadable
	}

	hook := orZero(asked.Hook)
	if !slices.Contains(configured.Runnable, hook) {
		return plannedRun{}, errHookNotRunnable
	}

	run := r.seams.Hooks.Run

	return plannedRun{
		kind: api.RunKindHook, title: hook,
		start:     func(ctx context.Context) (proc.Output, error) { return run(ctx, hook) },
		succeeded: "The " + hook + " hook passed.", refused: "The " + hook + " hook failed.",
	}, nil
}

// planRebase replays the checked-out branch onto its base.
func (r *Runner) planRebase(ctx context.Context, _ api.RunRequest) (plannedRun, error) {
	if r.seams.Git.Rebase == nil || r.seams.Git.Branch == nil {
		return plannedRun{}, errRunUnavailable
	}

	branch, err := r.seams.Git.Branch(ctx)
	if err != nil || !loop.CanRebase(branch) {
		return plannedRun{}, errNothingToRebase
	}

	rebase := r.seams.Git.Rebase

	return plannedRun{
		kind: api.RunKindRebase, title: "git rebase",
		start:     func(ctx context.Context) (proc.Output, error) { return rebase(ctx, branch.Base) },
		succeeded: "Rebased onto " + branch.BaseName() + ".",
		refused: "The rebase stopped: resolve the conflict in a terminal, " +
			"then go on with git rebase --continue, or undo it with git rebase --abort.",
	}, nil
}

// planAmend folds the staged changes into the branch's last commit, which
// must be one not yet pushed.
func (r *Runner) planAmend(ctx context.Context, _ api.RunRequest) (plannedRun, error) {
	if r.seams.Git.Amend == nil {
		return plannedRun{}, errRunUnavailable
	}

	foldable, err := r.foldable(ctx)
	if err != nil {
		return plannedRun{}, err
	}

	last := foldable[len(foldable)-1]

	return plannedRun{
		kind: api.RunKindAmend, title: "git commit --amend", start: r.seams.Git.Amend,
		succeeded: "Amended " + last.Subject + ".", refused: "The amend was refused.",
	}, nil
}

// planFixup records a fixup! of the commit asked for, which must be one not
// yet pushed.
func (r *Runner) planFixup(ctx context.Context, asked api.RunRequest) (plannedRun, error) {
	if r.seams.Git.Fixup == nil {
		return plannedRun{}, errRunUnavailable
	}

	foldable, err := r.foldable(ctx)
	if err != nil {
		return plannedRun{}, err
	}

	at := slices.IndexFunc(foldable, func(commit gitrepo.Commit) bool { return commit.Hash == orZero(asked.Commit) })
	if at < 0 {
		return plannedRun{}, errNotFoldable
	}

	chosen, fixup := foldable[at], r.seams.Git.Fixup

	return plannedRun{
		kind: api.RunKindFixup, title: "git commit --fixup",
		start:     func(ctx context.Context) (proc.Output, error) { return fixup(ctx, chosen.Hash) },
		succeeded: "Recorded a fixup! of " + chosen.Subject + ".", refused: "The fixup was refused.",
	}, nil
}

// foldable is the commits staged changes can be folded into, by loop's rule,
// or errNothingToFold, read under ctx.
func (r *Runner) foldable(ctx context.Context) ([]gitrepo.Commit, error) {
	if r.seams.Git.Branch == nil || r.seams.Git.Changes == nil {
		return nil, errRunUnavailable
	}

	branch, err := r.seams.Git.Branch(ctx)
	if err != nil {
		return nil, errNothingToFold
	}

	changes, err := r.seams.Git.Changes(ctx)
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
func (r *Runner) claimRun(planned plannedRun) bool {
	r.run.mu.Lock()
	defer r.run.mu.Unlock()

	if r.run.going != nil {
		return false
	}

	r.run.going = &goingRun{run: api.Run{
		Kind: planned.kind, Title: planned.title, State: api.RunStateInProgress, Lines: []string{}, Outcome: "",
	}}

	return true
}

// freeRun lets the next run start.
func (r *Runner) freeRun() {
	r.run.mu.Lock()
	defer r.run.mu.Unlock()

	r.run.going = nil
}

// streamRun writes the run as it starts, each line its program writes as it
// comes, and the run as it ended. A write that fails — the page went away —
// stops the writing, never the run: its lines are still read to the end.
func (r *Runner) streamRun(w http.ResponseWriter, planned plannedRun, output proc.Output) {
	w.Header().Set("Content-Type", "application/x-ndjson")
	w.Header().Set("Cache-Control", "no-cache")

	stream := runStream{w: w}
	stream.send(api.RunEvent{Run: r.runStarted(output.Stop)})

	for line := range output.Lines {
		shown := sanitize.Line(line)
		r.addRunLine(shown)
		stream.send(api.RunEvent{Line: &shown})
	}

	stream.send(api.RunEvent{Run: r.runEnded(planned, output.Wait())})
}

// runStarted records how to stop the run, stopping it at once if it was asked
// to stop before its program started, and answers it as it starts.
func (r *Runner) runStarted(stop func()) *api.Run {
	r.run.mu.Lock()
	defer r.run.mu.Unlock()

	r.run.going.stop = stop
	if r.run.going.stopped {
		stop()
	}

	started := r.run.going.run

	return &started
}

// addRunLine records a line the run wrote.
func (r *Runner) addRunLine(line string) {
	r.run.mu.Lock()
	defer r.run.mu.Unlock()

	r.run.going.run.Lines = append(r.run.going.run.Lines, line)
}

// runEnded records how the run ended — stopped, refused or passed — and
// answers it whole.
func (r *Runner) runEnded(planned plannedRun, exit error) *api.Run {
	r.run.mu.Lock()
	defer r.run.mu.Unlock()

	going := r.run.going

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

// RunShown is the run going now, with its last runTail lines, for a frame of
// the event stream, or nil when none is going.
func (r *Runner) RunShown() *api.Run {
	r.run.mu.Lock()
	defer r.run.mu.Unlock()

	if r.run.going == nil {
		return nil
	}

	shown := r.run.going.run
	shown.Lines = slices.Clone(shown.Lines[max(0, len(shown.Lines)-runTail):])

	return &shown
}

// StopRun stops the run going — its program's whole process group — as the
// terminal's interrupt does; its stream then ends with it stopped. With none
// going it is a 404.
func (r *Runner) StopRun(context.Context, api.StopRunRequestObject) (api.StopRunResponseObject, error) {
	r.run.mu.Lock()
	defer r.run.mu.Unlock()

	going := r.run.going
	if going == nil {
		return api.StopRun404ApplicationProblemPlusJSONResponse(
			problem.New(api.ProblemCodeNotFound, errNoRunGoing.Error())), nil
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
