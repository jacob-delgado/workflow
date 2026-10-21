// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/proc"
	"github.com/jacob-delgado/workflow/internal/webserver"
	"github.com/jacob-delgado/workflow/internal/webserver/webtest"
)

// stageNotes is the body that stages notes.txt; commitAt and pushAt are
// where a commit and a push are asked for.
const (
	stageNotes = `{"path":"` + notesPath + `"}`
	commitAt   = "/api/commit"
	pushAt     = "/api/push"
)

// liveTree is a working tree whose one change, notes.txt, the stage and
// unstage seams move in and out of the index, so a frame read after either
// shows it.
type liveTree struct {
	mu     sync.Mutex
	staged bool
}

// changes is the tree as it stands: notes.txt, untracked or staged.
func (l *liveTree) changes(context.Context) ([]gitrepo.Change, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.staged {
		return []gitrepo.Change{{Path: notesPath, Staged: 'A', Unstaged: ' '}}, nil
	}

	return []gitrepo.Change{{Path: notesPath, Staged: '?', Unstaged: '?'}}, nil
}

// stage moves notes.txt into the index.
func (l *liveTree) stage(context.Context, gitrepo.Change) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.staged = true

	return nil
}

// wire binds deps' changes read and stage write to the tree.
func (l *liveTree) wire(deps webserver.Deps) webserver.Deps {
	deps.Git.Changes = l.changes
	deps.Git.Stage = l.stage

	return deps
}

// notesStaged reports a frame that shows notes.txt staged.
func notesStaged(frame api.Snapshot) bool {
	return slices.ContainsFunc(frame.Changes.Changes, func(change api.Change) bool {
		return change.Path == notesPath && change.Staged
	})
}

// streamOver opens an event stream on a server over deps and cfg that
// re-pushes every streamTick, inside the caller's synctest bubble, and waits
// for its first frame; it answers the handler and the stream.
func streamOver(t *testing.T, deps webserver.Deps, cfg config.Config) (http.Handler, *webtest.OpenedStream) {
	t.Helper()

	info := webserver.Info{Version: webtest.TestVersion, StreamInterval: streamTick}
	handler := webtest.ServeWith(t, deps, cfg, info)
	stream := webtest.OpenStream(t, handler, "/api/events")

	synctest.Wait()

	return handler, stream
}

func TestAWriteAnsweredPushesAFrameAtOnce(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		// Arrange
		tree := &liveTree{}
		handler, stream := streamOver(t, tree.wire(webtest.FilledDeps()), config.Default())

		// Act
		staged := webtest.Send(t, handler, http.MethodPost, stagePath, stageNotes)

		synctest.Wait()

		// Assert
		if staged.Code != http.StatusOK {
			t.Fatalf("the stage answered %d: %s", staged.Code, staged.Body.String())
		}

		pushed := webtest.Snapshots(t, stream.Close())
		if len(pushed) != 2 || !notesStaged(pushed[len(pushed)-1]) {
			t.Errorf("pushed %d frames, the last staging %v; want 2 with no time passing, the last showing %s staged",
				len(pushed), notesStaged(pushed[len(pushed)-1]), notesPath)
		}
	})
}

// heldOnCall is a seam call that, on its numbered call alone, waits on held
// until the test lets it go.
func heldOnCall(number int32, held *webtest.HeldCall) func(context.Context) error {
	var calls atomic.Int32

	return func(ctx context.Context) error {
		if calls.Add(1) == number {
			return held.Wait(ctx)
		}

		return nil
	}
}

func TestAFrameReadWhileAWriteLandsIsReadAgain(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		// Arrange
		// A frame lists the branches last, after the changes: the second
		// frame's read of the remote's is held while a stage lands, so the
		// changes that frame read are those before it.
		tree, held := &liveTree{}, webtest.NewHeld()
		deps := tree.wire(webtest.FilledDeps())
		deps.Git.Branches = func(context.Context) ([]string, error) { return []string{webtest.TestBranchName}, nil }
		holdSecond := heldOnCall(2, held)
		deps.Git.RemoteBranches = func(ctx context.Context) ([]string, error) { return nil, holdSecond(ctx) }
		handler, stream := streamOver(t, deps, config.Default())

		advance(streamTick)

		// Act
		staged := webtest.Send(t, handler, http.MethodPost, stagePath, stageNotes)

		close(held.Release)
		synctest.Wait()

		// Assert
		if staged.Code != http.StatusOK {
			t.Fatalf("the stage answered %d: %s", staged.Code, staged.Body.String())
		}

		pushed := webtest.Snapshots(t, stream.Close())
		for index, frame := range pushed[1:] {
			if !notesStaged(frame) {
				t.Errorf("frame %d after the first shows %s unstaged, want no frame read across the stage sent",
					index+1, notesPath)
			}
		}

		if !notesStaged(pushed[len(pushed)-1]) {
			t.Errorf("the last frame shows %s unstaged, want it staged", notesPath)
		}
	})
}

// openedPull is the number of the pull request the open tests open.
const openedPull = 128

// laggingForgeDeps is openableDeps whose forge opens #128 but goes on finding
// no pull request for the branch, as a forge whose list lags behind its
// opens does, with messaging configured to post.
func laggingForgeDeps() webserver.Deps {
	deps := openableDeps()
	deps.Forge.CreatePullRequest = func(context.Context, forge.NewPullRequest) (forge.PullRequest, error) {
		return forge.PullRequest{Number: openedPull, URL: webtest.PRURL, Title: webtest.PRTitle, State: forge.StateOpen}, nil
	}
	deps.Messaging.Post = func(context.Context, string, string) error { return nil }

	return deps
}

// stageState is the state of step in frame, or "" when the frame has none.
func stageState(frame api.Snapshot, step api.StageStep) api.StageState {
	at := slices.IndexFunc(frame.Stages, func(stage api.Stage) bool { return stage.Step == step })
	if at < 0 {
		return ""
	}

	return frame.Stages[at].State
}

func TestAPullRequestOpenedShowsOnTheFrameItsAnswerWakes(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		// Arrange
		handler, stream := streamOver(t, laggingForgeDeps(), webhookConfig())
		cancelHeld(t, handler)

		// Act
		opening := webtest.Send(t, handler, http.MethodPost, "/api/pull-request", openRequestBody)

		synctest.Wait()

		// Assert
		if opening.Code != http.StatusOK {
			t.Fatalf("the open answered %d: %s", opening.Code, opening.Body.String())
		}

		last := lastSnapshot(t, stream.Close())
		if !last.Review.Found || last.Review.Pull.Number != openedPull {
			t.Errorf("review = %+v, want #%d found on the frame the open woke", last.Review, openedPull)
		}

		held := last.QueuedAnnouncement
		if held == nil || held.State != api.QueuedAnnouncementStateWaiting || held.Pull != openedPull {
			t.Errorf("queued announcement = %+v, want #%d's waiting", held, openedPull)
		}

		if state := stageState(last, api.StageStepReview); state != api.StageStateInFlight {
			t.Errorf("review stage = %q, want %q", state, api.StageStateInFlight)
		}
	})
}

// wakingDeps is a working tree of one change of each kind, on a branch ahead
// of its remote with no upstream, whose stage, unstage, commit, push and
// start-work writes all succeed.
func wakingDeps(branch gitrepo.Branch) webserver.Deps {
	deps := (&staging{}).deps()
	deps.Git.Branch = func(context.Context) (gitrepo.Branch, error) { return branch, nil }
	deps.Git.Commit = func(context.Context, string) (proc.Output, error) { return fakeOutput(nil, nil), nil }
	deps.Git.Push = func(context.Context, string) (proc.Output, error) { return fakeOutput(nil, nil), nil }
	deps.Git.CreateBranch = func(context.Context, string, string) error { return nil }

	return deps
}

// aheadBranch is webtest.FilledDeps' branch: ahead, with no upstream, so it
// can be pushed.
func aheadBranch() gitrepo.Branch {
	return gitrepo.Branch{
		Name: webtest.TestBranchName, Base: webtest.TestBase, Ahead: 2, Head: webtest.FilledHead,
		PushRemote: gitrepo.DefaultRemote, BaseRemote: gitrepo.DefaultRemote,
		Commits: []gitrepo.Commit{{Hash: webtest.FilledHead, Subject: webtest.TestCommitSubject}},
	}
}

func TestEveryWriteWakesTheStream(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		method, path, body string
		branch             gitrepo.Branch
		wantStatus         int
	}{
		"staging a change": {
			method: http.MethodPost, path: stagePath, body: `{"path":"` + editedPath + `"}`,
			branch: aheadBranch(), wantStatus: http.StatusOK,
		},
		"unstaging one": {
			method: http.MethodPost, path: unstagePath, body: `{"path":"` + stagedPath + `"}`,
			branch: aheadBranch(), wantStatus: http.StatusOK,
		},
		"committing": {
			method: http.MethodPost, path: commitAt, body: `{"type":"fix","subject":"stream a frame"}`,
			branch: aheadBranch(), wantStatus: http.StatusOK,
		},
		"pushing": {method: http.MethodPost, path: pushAt, branch: aheadBranch(), wantStatus: http.StatusOK},
		"starting work": {
			method: http.MethodPost, path: branchesAt, body: `{"issue_key":"` + startIssue + `"}`,
			branch: aheadBranch(), wantStatus: http.StatusOK,
		},
		"a push refused with nothing to push": {
			method: http.MethodPost, path: pushAt,
			branch: gitrepo.Branch{
				Name: webtest.TestBranchName, Upstream: "origin/" + webtest.TestBranchName,
				PushRemote: gitrepo.DefaultRemote, Commits: aheadBranch().Commits,
			},
			wantStatus: http.StatusConflict,
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			synctest.Test(t, func(t *testing.T) {
				// Arrange
				handler, stream := streamOver(t, wakingDeps(tt.branch), config.Default())

				// Act
				answer := webtest.Send(t, handler, tt.method, tt.path, tt.body)

				synctest.Wait()

				// Assert
				if answer.Code != tt.wantStatus {
					t.Fatalf("answered %d: %s; want %d", answer.Code, answer.Body.String(), tt.wantStatus)
				}

				if pushed := len(webtest.Snapshots(t, stream.Close())); pushed != 2 {
					t.Errorf("pushed %d frames with no time passing, want 2: on connect, and once answered", pushed)
				}
			})
		})
	}
}

func TestAReadWakesNoStream(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		// Arrange
		handler, stream := streamOver(t, webtest.FilledDeps(), config.Default())

		// Act
		read := webtest.Get(t, handler, "/api/branch")

		synctest.Wait()

		// Assert
		if read.Code != http.StatusOK {
			t.Fatalf("the branch read answered %d: %s", read.Code, read.Body.String())
		}

		if pushed := len(webtest.Snapshots(t, stream.Close())); pushed != 1 {
			t.Errorf("pushed %d frames, want only the one on connect: a read changes nothing", pushed)
		}
	})
}

// rebasingTree is a branch one commit behind its base whose rebase the test
// holds until it lets the program end, which leaves the branch level.
type rebasingTree struct {
	mu      sync.Mutex
	rebased bool
	lines   chan string
}

// newRebasingTree is a branch not yet rebased, with a rebase's program that
// writes nothing until the test ends it.
func newRebasingTree() *rebasingTree {
	return &rebasingTree{lines: make(chan string)}
}

// branch is the branch as it stands: behind its base until the rebase ends.
func (r *rebasingTree) branch(context.Context) (gitrepo.Branch, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	branch := aheadBranch()
	if !r.rebased {
		branch.Behind = 1
	}

	return branch, nil
}

// end lets the rebase's program end, the branch rebased.
func (r *rebasingTree) end() {
	r.mu.Lock()
	r.rebased = true
	r.mu.Unlock()

	close(r.lines)
}

// wire binds deps' branch read and rebase to the tree.
func (r *rebasingTree) wire(deps webserver.Deps) webserver.Deps {
	deps.Git.Branch = r.branch
	deps.Git.Rebase = func(context.Context, string) (proc.Output, error) {
		return proc.Output{Lines: r.lines, Wait: func() error { return nil }, Stop: func() {}}, nil
	}

	return deps
}

func TestARunsEndPushesAFrame(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		// Arrange
		tree := newRebasingTree()
		handler, stream := streamOver(t, tree.wire(webtest.FilledDeps()), config.Default())
		run := httptest.NewRecorder()
		done := make(chan struct{})

		go func() {
			defer close(done)

			request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/runs",
				strings.NewReader(`{"kind":"rebase"}`))
			request.Host = webtest.LoopbackHost
			request.Header.Set("Content-Type", "application/json")
			handler.ServeHTTP(run, request)
		}()

		synctest.Wait()

		// Act
		tree.end()
		<-done
		synctest.Wait()

		// Assert
		if !run.Flushed {
			t.Error("the run's lines were not flushed, want each flushed through to the page")
		}

		if last := lastSnapshot(t, stream.Close()); last.Branch.Behind != 0 {
			t.Errorf("the last frame's branch is %d behind, want the rebased branch level with no time passing",
				last.Branch.Behind)
		}
	})
}

// assignableTracker is a tracker whose lenient search answers the issues
// assigned to you, and whose assign makes an issue yours.
type assignableTracker struct {
	mu    sync.Mutex
	yours []jira.Key
}

// search answers your issues among the scoping tests' branches.
func (a *assignableTracker) search(context.Context, string, int) (jira.SearchResult, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	issues := make([]jira.Issue, 0, len(a.yours))
	for _, key := range a.yours {
		issues = append(issues, jira.Issue{Key: key})
	}

	return jira.SearchResult{Issues: issues, Total: len(issues)}, nil
}

// assign makes key yours.
func (a *assignableTracker) assign(_ context.Context, key jira.Key, _ string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.yours = append(a.yours, key)

	return nil
}

func TestAnAssignmentShowsOnTheFrameItWakes(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		// Arrange
		var asked atomic.Int32

		tracker := &assignableTracker{yours: []jira.Key{yourIssue}}
		deps := scopedDeps(nil, nil, &asked)
		deps.Git.RemoteBranches = nil
		deps.Jira.SearchLenient, deps.Jira.Assign = tracker.search, tracker.assign
		handler, stream := streamOver(t, deps, scopedConfig())

		// Act
		assigned := webtest.Send(t, handler, http.MethodPut, "/api/issues/PROJ-2/assignee", `{"assignee":"me"}`)

		synctest.Wait()

		// Assert
		if assigned.Code != http.StatusOK {
			t.Fatalf("the assignment answered %d: %s", assigned.Code, assigned.Body.String())
		}

		want := []string{yourLocalBranch, otherLocalBranch}
		if got := listed(lastSnapshot(t, stream.Close()).Branches); !slices.Equal(got, want) {
			t.Errorf("the woken frame's branches = %v, want %v once PROJ-2 is yours", got, want)
		}
	})
}

func TestAStreamWhosePageLeavesAsAWriteLandsReadsNoFrameAgain(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		// Arrange
		var reads atomic.Int32

		tree, held := &liveTree{}, webtest.NewHeld()
		deps := tree.wire(webtest.FilledDeps())
		deps.Git.Branches = func(context.Context) ([]string, error) { return []string{webtest.TestBranchName}, nil }
		holdSecond := heldOnCall(2, held)
		deps.Git.RemoteBranches = func(ctx context.Context) ([]string, error) {
			reads.Add(1)

			return nil, holdSecond(ctx)
		}
		handler, stream := streamOver(t, deps, config.Default())

		advance(streamTick)
		webtest.Send(t, handler, http.MethodPost, stagePath, stageNotes)

		// Act
		stream.Cancel()
		synctest.Wait()
		close(held.Release)
		<-stream.Done

		// Assert
		if got := reads.Load(); got != 2 {
			t.Errorf("the stream read %d frames, want 2: one on connect, and the one its page left during", got)
		}
	})
}

func TestARunWrittenToAWriterThatCannotFlushRunsToItsEnd(t *testing.T) {
	t.Parallel()

	// Arrange
	var ended atomic.Bool

	deps := webtest.FilledDeps()
	deps.Git.Branch = func(context.Context) (gitrepo.Branch, error) {
		branch := aheadBranch()
		branch.Behind = 1

		return branch, nil
	}
	deps.Git.Rebase = func(context.Context, string) (proc.Output, error) {
		output := fakeOutput([]string{"rebasing"}, nil)
		output.Wait = func() error {
			ended.Store(true)

			return nil
		}

		return output, nil
	}
	handler := webtest.Serve(t, deps, config.Default())
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/runs",
		strings.NewReader(`{"kind":"rebase"}`))
	request.Host = webtest.LoopbackHost
	request.Header.Set("Content-Type", "application/json")

	// Act
	handler.ServeHTTP(&webtest.UnflushableWriter{}, request)

	// Assert
	if !ended.Load() {
		t.Error("the run did not go to its end on a writer that cannot flush")
	}
}

// laggingTracker is assignableTracker whose ask of which issues are yours, once
// the test says so, waits until the test lets it go, and then answers what
// the tracker said as it began.
type laggingTracker struct {
	*assignableTracker
	*slowRead
}

// search answers as the tracker stood when it was asked, once let go.
func (s *laggingTracker) search(ctx context.Context, jql string, startAt int) (jira.SearchResult, error) {
	answer, err := s.assignableTracker.search(ctx, jql, startAt)
	if strings.HasPrefix(jql, keyInYourSearch) {
		s.call()
	}

	return answer, err
}

func TestAnAssignmentMadeDuringAnAskIsAskedAboutAgain(t *testing.T) {
	t.Parallel()

	// Arrange
	// The ask under way began before PROJ-2 was assigned, so it says PROJ-2
	// is not yours; the assignment tells the cache the tracker says more now.
	var asked atomic.Int32

	tracker := &laggingTracker{
		assignableTracker: &assignableTracker{yours: []jira.Key{yourIssue}}, slowRead: newSlowRead(),
	}
	deps := scopedDeps(nil, nil, &asked)
	deps.Git.RemoteBranches = nil
	deps.Jira.SearchLenient, deps.Jira.Assign = tracker.search, tracker.assign
	handler := webtest.Serve(t, deps, scopedConfig())

	tracker.slowDown()
	reading := frameInFlight(t, handler, tracker.slowRead)

	if assigned := webtest.Send(t, handler, http.MethodPut, "/api/issues/PROJ-2/assignee",
		`{"assignee":"me"}`); assigned.Code != http.StatusOK {
		t.Fatalf("the assignment answered %d: %s", assigned.Code, assigned.Body.String())
	}

	close(tracker.release)
	<-reading

	// Act
	frame := webtest.FirstSnapshot(t, webtest.StreamOnce(t, handler, "/api/events").Body.String())

	// Assert
	want := []string{yourLocalBranch, otherLocalBranch}
	if got := listed(frame.Branches); !slices.Equal(got, want) {
		t.Errorf("branches = %v, want %v from an ask made after the assignment", got, want)
	}
}
