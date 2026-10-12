// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

// Every write the interface sends outlasts its session, which a quit or a
// switch ends, and ends with the program's lifetime, which a signal ends: the
// tests here hold each write seam in flight and end one, then the other.

import (
	"context"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/proc"
	"github.com/jacob-delgado/workflow/internal/tui"
)

// writeStart is what an interface sending a write opens on: its seams, its
// configuration, and the error loading it.
type writeStart struct {
	deps    tui.Deps
	cfg     config.Config
	loadErr error
}

// fromWorld is a start in the world build makes.
func fromWorld(build func() *world) func(*testing.T) writeStart {
	return func(*testing.T) writeStart {
		repo := build()

		return writeStart{deps: repo.deps(), cfg: repo.cfg, loadErr: nil}
	}
}

// fromFirstRun is a start with no configuration file, over a setup that
// writes one.
func fromFirstRun(t *testing.T) writeStart {
	t.Helper()

	return writeStart{
		deps: newFirstRun(t, http.StatusOK).deps(t), cfg: config.Default(), loadErr: config.ErrNotFound,
	}
}

// seamAt is the function field path names in deps, a dotted path such as
// Git.Stage, failing the test when the world binds none there.
func seamAt(t *testing.T, deps *tui.Deps, path string) reflect.Value {
	t.Helper()

	seam := reflect.ValueOf(deps).Elem()
	for name := range strings.SplitSeq(path, ".") {
		seam = seam.FieldByName(name)
	}

	if !seam.IsValid() || seam.Kind() != reflect.Func || seam.IsNil() {
		t.Fatalf("the world binds no seam %s", path)
	}

	return seam
}

// holdWrite arms held on the seam path names, so every call to it waits, and
// is the context the first call is asked under.
func holdWrite(t *testing.T, held *hold, deps *tui.Deps, path string) <-chan context.Context {
	t.Helper()

	seam := seamAt(t, deps, path)
	shape := seam.Type()
	asked := make(chan context.Context, 1)

	seam.Set(reflect.MakeFunc(shape, func(args []reflect.Value) []reflect.Value {
		ctx, _ := reflect.TypeAssert[context.Context](args[0])
		select {
		case asked <- ctx:
		default:
		}

		held.wait()

		answers := make([]reflect.Value, shape.NumOut())
		for index := range answers {
			answers[index] = reflect.Zero(shape.Out(index))
		}

		return answers
	}))
	held.armed.Store(true)

	return asked
}

// withChanges is a work tree with a file to stage and one to discard.
func withChanges() *world {
	repo := newWorld()
	repo.changes = workTree()

	return repo
}

// withMoves is an issue Jira offers transitions for.
func withMoves() *world {
	repo := newWorld()
	repo.moves = workflowMoves()

	return repo
}

// syncingTasks is Taskwarrior with a sync server configured.
func syncingTasks() *world {
	repo := withTasks()
	repo.tasks.install.SyncConfigured = true

	return repo
}

// writeFlow is the keys that send a write, and the start they are pressed in.
type writeFlow struct {
	start func(*testing.T) writeStart
	keys  []string
}

// writeFlows is the flow that sends each write seam, by its path in tui.Deps.
func writeFlows() map[string]writeFlow {
	scoped := append(append([]string{"3", "c", keyShiftTab}, letters("config")...), keyTab, "x", keyEnter)

	return map[string]writeFlow{
		"Jira.Transition":      {start: fromWorld(withMoves), keys: []string{"t", "j", keyEnter}},
		"Jira.Comment":         {start: fromWorld(newWorld), keys: []string{"c", "i", "x", keyEsc, keyEnter, keyEnter}},
		"Jira.Assign":          {start: fromWorld(newWorld), keys: []string{"a", "f", keyEnter}},
		"Jira.AddWorklog":      {start: fromWorld(newWorld), keys: []string{"w", "2", "h", keyEnter}},
		"Jira.LinkPullRequest": {start: fromWorld(withoutPull), keys: []string{"4", "n", keyEnter, keyEnter}},
		"Git.Stage":            {start: fromWorld(withChanges), keys: []string{"3", "j", keySpace}},
		"Git.Unstage":          {start: fromWorld(newWorld), keys: []string{"3", keySpace}},
		"Git.Discard":          {start: fromWorld(withChanges), keys: []string{"3", "j", "x", keyEnter}},
		"Git.CreateBranch":     {start: fromWorld(newWorld), keys: []string{"b", keyEnter}},
		"Git.Checkout":         {start: fromWorld(cleanSwitcher), keys: []string{"2", "s", keyEnter}},
		"Git.CreateWorktree":   {start: fromWorld(newWorld), keys: []string{"2", "b", keyWorktree, keyEnter}},
		"Git.Fetch":            {start: fromWorld(newWorld), keys: []string{"b", keyEnter}},
		"Git.Commit":           {start: fromWorld(newWorld), keys: commitKeys("x")},
		"Git.Push":             {start: fromWorld(unpushedWorld), keys: []string{"2", "P", keyEnter}},
		"Git.Amend":            {start: fromWorld(unpushedWorld), keys: []string{"3", "A", keyEnter}},
		"Git.Fixup":            {start: fromWorld(unpushedWorld), keys: []string{"3", "f", keyEnter}},
		"Git.Rebase":           {start: fromWorld(newWorld), keys: []string{"2", "u", keyEnter}},
		"Git.Finish":           {start: fromWorld(mergedBranch), keys: []string{"4", "F", keyEnter}},
		"Git.LinkIssue": {
			start: fromWorld(func() *world { return onOffConventionBranch(false) }), keys: []string{"2", "i", keyEnter},
		},
		"Git.UnlinkIssue":         {start: fromWorld(linkedBranch), keys: []string{"2", "i", "u"}},
		"Forge.CreatePullRequest": {start: fromWorld(withoutPull), keys: []string{"4", "n", keyEnter}},
		"Forge.EditPullRequest":   {start: fromWorld(newWorld), keys: []string{"4", "e", keyEnter}},
		"Forge.RewriteDescription": {
			start: fromWorld(func() *world { return onOffConventionBranch(true) }), keys: []string{"2", "i", keyEnter, keyEnter},
		},
		"Forge.Rerun":          {start: fromWorld(failedChecks), keys: []string{"4", "R", keyEnter}},
		"Forge.Merge":          {start: fromWorld(mergeable), keys: []string{"4", "M", keyEnter}},
		"Messaging.Post":       {start: fromWorld(newWorld), keys: []string{"5", "p", keyEnter}},
		"Hooks.Run":            {start: fromWorld(newWorld), keys: []string{"3", "h"}},
		"Hooks.Write":          {start: fromWorld(foldingWithHooks), keys: []string{"3", "g", keyEnter}},
		"Store.RecordScope":    {start: fromWorld(newWorld), keys: scoped},
		"Store.RecordAnnounce": {start: fromWorld(newWorld), keys: []string{"5", "p", keyEnter}},
		"Store.CacheIssues":    {start: fromWorld(newWorld), keys: nil},
		"Store.LinkOwner":      {start: fromWorld(onlyBen), keys: []string{"5", "P", "x"}},
		"Store.ForgetOwner":    {start: fromWorld(taggingWorld), keys: []string{"5", "P", "d", keyEnter}},
		"Store.SetRepoGroups":  {start: fromWorld(taggingWorld), keys: []string{"5", "P", keyTab, keySpace}},
		"Store.RecordGroups":   {start: fromWorld(taggingWorld), keys: []string{"5", "p", keyEnter}},
		"Store.Favor":          {start: fromWorld(reposWorld), keys: []string{reposKey, "f"}},
		"Store.Unfavor":        {start: fromWorld(reposWorld), keys: []string{reposKey, "j", "f"}},
		"Tasks.Add":            {start: fromWorld(withTasks), keys: []string{tasksPane, "a", "x", keyEnter}},
		"Tasks.Start":          {start: fromWorld(withTasks), keys: []string{tasksPane, downAction, "s"}},
		"Tasks.Stop":           {start: fromWorld(withTasks), keys: []string{tasksPane, "s"}},
		"Tasks.Done":           {start: fromWorld(withTasks), keys: []string{tasksPane, downAction, "d", keyEnter}},
		"Tasks.Annotate":       {start: fromWorld(withTasks), keys: []string{tasksPane, downAction, "A", "x", keyEnter}},
		"Tasks.Modify":         {start: fromWorld(withTasks), keys: []string{tasksPane, downAction, "e", "x", keyEnter}},
		"Tasks.Undo":           {start: fromWorld(syncingTasks), keys: []string{tasksPane, "u", keyEnter}},
		"Tasks.Sync":           {start: fromWorld(syncingTasks), keys: []string{tasksPane, "S", keyEnter}},
		"Settings.Save":        {start: fromWorld(newWorld), keys: editing(projectRow, editedProject)},
		"Settings.Setup.Write": {start: fromFirstRun, keys: append(throughEveryQuestion(), keyEnter)},
		"OpenURL":              {start: fromWorld(checkedCI), keys: []string{"4", "c", keyEnter}},
	}
}

func TestEveryWriteOutlastsItsSessionAndEndsWithItsProgram(t *testing.T) {
	t.Parallel()

	for path, flow := range writeFlows() {
		t.Run(path, func(t *testing.T) {
			t.Parallel()

			// Arrange
			start := flow.start(t)
			held := newHold(t)
			asked := holdWrite(t, held, &start.deps, path)
			session, endSession := context.WithCancel(t.Context())
			program, endProgram := context.WithCancel(t.Context())
			model := tui.New(start.cfg, start.loadErr, start.deps).WithContext(session).WithLifetime(program)
			model = sized(t, model, 140, 50)
			holding(t, held, drainPast(t, held, model, model.Init()), flow.keys...)
			held.requireReached(t)

			write := <-asked

			// Act: the session ends, as on a quit or a switch
			endSession()

			// Assert: the write goes on
			err := write.Err()
			if err != nil {
				t.Errorf("%s's context ended with %v once its session did, want it to go on", path, err)
			}

			// Act: the program ends, as on SIGINT or SIGTERM
			endProgram()

			// Assert: the write ends with it
			select {
			case <-write.Done():
			case <-time.After(quitPatience):
				t.Errorf("%s's context went on once its program ended, want it ended with the program", path)
			}
		})
	}
}

// runningOutput is a program that runs until it is stopped, streaming
// nothing.
func runningOutput() proc.Output {
	lines := make(chan string)

	return proc.Output{
		Lines: lines, Wait: func() error { return errRunStopped }, Stop: func() { close(lines) },
	}
}

// startRun presses keys up to the last, then the last, and delivers the run
// it starts, its program still running: it answers with the reading of its
// output, not yet run.
func startRun(t *testing.T, model tui.Model, keys []string) (tui.Model, func() tui.Model) {
	t.Helper()

	last := len(keys) - 1
	starting, start := pressed(t, typing(t, model, keys[:last]...), keys[last])
	running, readOutput := finish(t, starting, start)

	return running, func() tui.Model { return drain(t, running, readOutput) }
}

func TestAStreamedWriteKeepsItsContextUntilItsProgramExits(t *testing.T) {
	t.Parallel()

	for _, path := range []string{"Git.Commit", "Git.Push", "Git.Amend", "Git.Fixup", "Git.Rebase", "Hooks.Run"} {
		t.Run(path, func(t *testing.T) {
			t.Parallel()

			// Arrange
			flow := writeFlows()[path]
			start := flow.start(t)
			asked := make(chan context.Context, 1)
			seam := seamAt(t, &start.deps, path)
			seam.Set(reflect.MakeFunc(seam.Type(), func(args []reflect.Value) []reflect.Value {
				ctx, _ := reflect.TypeAssert[context.Context](args[0])
				asked <- ctx

				return []reflect.Value{reflect.ValueOf(runningOutput()), reflect.Zero(seam.Type().Out(1))}
			}))

			model := tui.New(start.cfg, start.loadErr, start.deps).WithLifetime(t.Context())
			model = sized(t, model, 140, 50)
			model = drain(t, model, model.Init())

			// Act: start the run
			running, readToTheEnd := startRun(t, model, flow.keys)
			write := <-asked

			// Assert: its program runs on under a live context
			err := write.Err()
			if err != nil {
				t.Errorf("%s's context ended with %v while its program ran, want it held until the program exits",
					path, err)
			}

			// Act: stop it, and read its output to the end
			typing(t, running, "s")
			readToTheEnd()

			// Assert: its context is released
			if write.Err() == nil {
				t.Errorf("%s's context was never released once its program exited", path)
			}
		})
	}
}

// Run gives a dry run its lifetime too, and the writes asked again under it
// are still held back: the issue list it loads is not kept.
func TestADryRunGivenALifetimeStillHoldsBackItsWrites(t *testing.T) {
	t.Parallel()

	// Arrange
	dry := newWorld()
	model := sized(t, dryInterface(dry).WithLifetime(t.Context()), 120, 40)

	// Act
	drain(t, model, model.Init())

	// Assert
	if searched := dry.asked("search"); len(searched) == 0 {
		t.Fatal("the dry run loaded no issue list")
	}

	if kept := dry.asked("cache"); len(kept) != 0 {
		t.Errorf("a dry run given a lifetime kept the issue list: %q", kept)
	}
}
