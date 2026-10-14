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

// writeFlow is the keys that send a write, the start they are pressed in, and
// what the write is called while it is in flight.
type writeFlow struct {
	start func(*testing.T) writeStart
	keys  []string
	name  string
}

// pullNoun is what a GitHub forge calls a pull request.
const pullNoun = "pull request"

// streamedWrites are the write seams whose program streams its output on
// after the seam returns.
func streamedWrites() []string {
	return []string{"Git.Commit", "Git.Push", "Git.Amend", "Git.Fixup", "Git.Rebase", "Hooks.Run"}
}

// writeFlows is the flow that sends each write seam, by its path in tui.Deps.
func writeFlows() map[string]writeFlow {
	scoped := append(append([]string{"3", "c", keyShiftTab}, letters("config")...), keyTab, "x", keyEnter)

	return map[string]writeFlow{
		"Jira.Transition": {
			start: fromWorld(withMoves), keys: []string{"t", "j", keyEnter},
			name: "moving an issue",
		},
		"Jira.Comment": {
			start: fromWorld(newWorld), keys: []string{"c", "i", "x", keyEsc, keyEnter, keyEnter},
			name: "commenting on an issue",
		},
		"Jira.Assign": {
			start: fromWorld(newWorld), keys: []string{"a", "f", keyEnter},
			name: "assigning an issue",
		},
		"Jira.AddWorklog": {
			start: fromWorld(newWorld), keys: []string{"w", "2", "h", keyEnter},
			name: "logging work on an issue",
		},
		"Jira.LinkPullRequest": {
			start: fromWorld(withoutPull), keys: []string{"4", "n", keyEnter, keyEnter, keyEnter},
			name: "linking an issue to its " + pullNoun,
		},
		"Git.Stage": {
			start: fromWorld(withChanges), keys: []string{"3", "j", keySpace},
			name: "staging a change",
		},
		"Git.Unstage": {start: fromWorld(newWorld), keys: []string{"3", keySpace}, name: "unstaging a change"},
		"Git.Discard": {
			start: fromWorld(withChanges), keys: []string{"3", "j", "x", keyEnter},
			name: "discarding a change",
		},
		"Git.CreateBranch": {start: fromWorld(newWorld), keys: []string{"b", keyEnter}, name: "creating a branch"},
		"Git.Checkout": {
			start: fromWorld(cleanSwitcher), keys: []string{"2", "s", keyEnter},
			name: "checking out a branch",
		},
		"Git.CreateWorktree": {
			start: fromWorld(newWorld), keys: []string{"2", "b", keyWorktree, keyEnter},
			name: "adding a worktree",
		},
		"Git.Fetch":  {start: fromWorld(newWorld), keys: []string{"b", keyEnter}, name: "fetching"},
		"Git.Commit": {start: fromWorld(newWorld), keys: commitKeys("x"), name: "committing"},
		"Git.Push":   {start: fromWorld(unpushedWorld), keys: []string{"2", "P", keyEnter}, name: "pushing"},
		"Git.Amend": {
			start: fromWorld(unpushedWorld), keys: []string{"3", "A", keyEnter},
			name: "amending the last commit",
		},
		"Git.Fixup": {
			start: fromWorld(unpushedWorld), keys: []string{"3", "f", keyEnter},
			name: "making a fixup commit",
		},
		"Git.Rebase": {start: fromWorld(newWorld), keys: []string{"2", "u", keyEnter}, name: "rebasing"},
		"Git.Finish": {
			start: fromWorld(mergedBranch), keys: []string{"4", "F", keyEnter},
			name: "finishing the branch",
		},
		"Git.LinkIssue": {
			start: fromWorld(func() *world { return onOffConventionBranch(false) }), keys: []string{"2", "i", keyEnter},
			name: "linking the branch to an issue",
		},
		"Git.UnlinkIssue": {
			start: fromWorld(linkedBranch), keys: []string{"2", "i", "u"},
			name: "unlinking the branch from its issue",
		},
		"Forge.CreatePullRequest": {
			start: fromWorld(withoutPull), keys: []string{"4", "n", keyEnter, keyEnter},
			name: "opening a " + pullNoun,
		},
		"Forge.EditPullRequest": {
			start: fromWorld(newWorld), keys: []string{"4", "e", keyEnter},
			name: "editing the " + pullNoun,
		},
		"Forge.RewriteDescription": {
			start: fromWorld(func() *world { return onOffConventionBranch(true) }), keys: []string{"2", "i", keyEnter, keyEnter},
			name: "rewriting the " + pullNoun + "'s description",
		},
		"Forge.Rerun": {
			start: fromWorld(failedChecks), keys: []string{"4", "R", keyEnter},
			name: "rerunning a check",
		},
		"Forge.Merge": {
			start: fromWorld(mergeable), keys: []string{"4", "M", keyEnter},
			name: "merging the " + pullNoun,
		},
		"Messaging.Post": {
			start: fromWorld(newWorld), keys: []string{"5", "p", keyEnter},
			name: "posting a message",
		},
		"Hooks.Run": {start: fromWorld(newWorld), keys: []string{"3", "h", keyEnter}, name: "running the hooks"},
		"Hooks.Write": {
			start: fromWorld(foldingWithHooks), keys: []string{"3", "g", keyEnter},
			name: "setting up lefthook",
		},
		"Store.RecordScope": {start: fromWorld(newWorld), keys: scoped, name: "remembering the commit scope"},
		"Store.RecordAnnounce": {
			start: fromWorld(newWorld), keys: []string{"5", "p", keyEnter},
			name: "remembering an announcement",
		},
		"Store.CacheIssues": {start: fromWorld(newWorld), keys: nil, name: "caching the issue list"},
		"Store.LinkOwner":   {start: fromWorld(onlyBen), keys: []string{"5", "P", "x"}, name: "linking a code owner"},
		"Store.ForgetOwner": {
			start: fromWorld(taggingWorld), keys: []string{"5", "P", "d", keyEnter},
			name: "forgetting a code owner's link",
		},
		"Store.SetRepoGroups": {
			start: fromWorld(taggingWorld), keys: []string{"5", "P", keyTab, keySpace},
			name: "setting the repository's groups",
		},
		"Store.RecordGroups": {
			start: fromWorld(taggingWorld), keys: []string{"5", "p", keyEnter},
			name: "remembering the groups tagged",
		},
		"Store.Favor": {start: fromWorld(reposWorld), keys: []string{reposKey, "f"}, name: "adding a favorite"},
		"Store.Unfavor": {
			start: fromWorld(reposWorld), keys: []string{reposKey, "j", "f"},
			name: "removing a favorite",
		},
		"Tasks.Add": {
			start: fromWorld(withTasks), keys: []string{tasksPane, "a", "x", keyEnter},
			name: "adding a task",
		},
		"Tasks.Start": {
			start: fromWorld(withTasks), keys: []string{tasksPane, downAction, "s"},
			name: "starting a task",
		},
		"Tasks.Stop": {start: fromWorld(withTasks), keys: []string{tasksPane, "s"}, name: "stopping a task"},
		"Tasks.Done": {
			start: fromWorld(withTasks), keys: []string{tasksPane, downAction, "d", keyEnter},
			name: "marking a task done",
		},
		"Tasks.Annotate": {
			start: fromWorld(withTasks), keys: []string{tasksPane, downAction, "A", "x", keyEnter},
			name: "annotating a task",
		},
		"Tasks.Modify": {
			start: fromWorld(withTasks), keys: []string{tasksPane, downAction, "e", "x", keyEnter},
			name: "modifying a task",
		},
		"Tasks.Undo": {
			start: fromWorld(syncingTasks), keys: []string{tasksPane, "u", keyEnter},
			name: "undoing a task change",
		},
		"Tasks.Sync": {
			start: fromWorld(syncingTasks), keys: []string{tasksPane, "S", keyEnter},
			name: "syncing tasks",
		},
		"Settings.Save": {
			start: fromWorld(newWorld), keys: editing(projectRow, editedProject),
			name: "saving settings",
		},
		"Settings.Setup.Write": {
			start: fromFirstRun, keys: append(throughEveryQuestion(), keyEnter),
			name: "writing the configuration file",
		},
	}
}

// openURLPath is the seam that opens a page in the browser, which outlasts its
// session as a write does, and counts in flight nowhere.
const openURLPath = "OpenURL"

// pageOpening is the flow that opens a check's page in the browser.
func pageOpening() writeFlow {
	return writeFlow{start: fromWorld(checkedCI), keys: []string{"4", "c", keyEnter}, name: ""}
}

// outlastingFlows are the flows whose seam outlasts its session: every
// write's, and opening a page's.
func outlastingFlows() map[string]writeFlow {
	flows := writeFlows()
	flows[openURLPath] = pageOpening()

	return flows
}

func TestEveryWriteOutlastsItsSessionAndEndsWithItsProgram(t *testing.T) {
	t.Parallel()

	for path, flow := range outlastingFlows() {
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

	for _, path := range streamedWrites() {
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
