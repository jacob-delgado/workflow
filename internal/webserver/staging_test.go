// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/webserver"
)

// The staging endpoints, and the working tree their tests stage in.
const (
	stagePath   = "/api/stage"
	unstagePath = "/api/unstage"
	// renamedPath is where a renamed file is now; renamedFrom is where it was.
	renamedPath = "internal/config/settings.go"
	renamedFrom = "internal/config/config.go"
	// editedPath has edits the index does not hold; stagedPath is wholly staged.
	editedPath = "internal/tui/tui.go"
	stagedPath = "internal/web/web.go"
	notesPath  = "notes.txt"
	// everything is the body that stages or unstages every change.
	everything = `{"all":true}`
	// gitHost is a remote a partial clone's lazy fetch would name in git's own
	// words, which must not reach the page.
	gitHost = "git.internal.example.com"
	// indexHeld is how long a fake stage holds the index: long enough that a
	// second write sent beside it arrives while the first is under way.
	indexHeld = 20 * time.Millisecond
)

// workTree is one of each change the staging rules tell apart: wholly staged, a
// staged rename with more edits on it, an edit, and an untracked file.
func workTree() []gitrepo.Change {
	return []gitrepo.Change{
		{Path: stagedPath, Staged: 'M', Unstaged: ' '},
		{Path: renamedPath, OriginalPath: renamedFrom, Staged: 'R', Unstaged: 'M'},
		{Path: editedPath, Staged: ' ', Unstaged: 'M'},
		{Path: notesPath, Staged: '?', Unstaged: '?'},
	}
}

// staging is a working tree whose stage and unstage seams record each change
// they are handed.
type staging struct {
	staged, unstaged []gitrepo.Change
}

// deps wires the recording seams over workTree.
func (s *staging) deps() webserver.Deps {
	deps := filledDeps()
	deps.Changes = func() ([]gitrepo.Change, error) { return workTree(), nil }
	deps.Stage = func(change gitrepo.Change) error {
		s.staged = append(s.staged, change)

		return nil
	}
	deps.Unstage = func(change gitrepo.Change) error {
		s.unstaged = append(s.unstaged, change)

		return nil
	}

	return deps
}

// pathsOf is where each change is, in order.
func pathsOf(changes []gitrepo.Change) []string {
	paths := make([]string, 0, len(changes))
	for _, change := range changes {
		paths = append(paths, change.Path)
	}

	return paths
}

// sendStaging posts a staging request body to path.
func sendStaging(t *testing.T, deps webserver.Deps, path, body string) *httptest.ResponseRecorder {
	t.Helper()

	return send(t, serve(t, deps, config.Default()), http.MethodPost, path, body)
}

func TestStageAllStagesEveryChange(t *testing.T) {
	t.Parallel()

	// Arrange
	var tree staging

	// Act
	recorder := sendStaging(t, tree.deps(), stagePath, everything)

	// Assert
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", recorder.Code, recorder.Body.String())
	}

	// Every change the index does not hold all of yet, and not the one it does.
	want := []string{renamedPath, editedPath, notesPath}
	if got := pathsOf(tree.staged); !slices.Equal(got, want) {
		t.Errorf("staged %q, want %q", got, want)
	}

	if listed := decode[api.ChangeList](t, recorder); len(listed.Changes) != len(workTree()) {
		t.Errorf("answered %d changes, want the working tree's %d", len(listed.Changes), len(workTree()))
	}
}

func TestStageResolvesThePathAgainstTheChanges(t *testing.T) {
	t.Parallel()

	// Arrange
	var tree staging

	// Act
	recorder := sendStaging(t, tree.deps(), stagePath, `{"path":"`+renamedPath+`"}`)

	// Assert
	// The change git is handed is the one the server read, so a rename carries
	// the path it came from and both are staged.
	want := workTree()[1]
	if recorder.Code != http.StatusOK || len(tree.staged) != 1 || tree.staged[0] != want {
		t.Errorf("status = %d, staged %+v; want 200 and exactly %+v", recorder.Code, tree.staged, want)
	}
}

func TestStageRefusesAPathThatIsNotAChange(t *testing.T) {
	t.Parallel()

	// Arrange
	var tree staging

	// Act
	recorder := sendStaging(t, tree.deps(), stagePath, `{"path":"../../.ssh/id_ed25519"}`)

	// Assert
	failure := decode[api.Problem](t, recorder)
	if recorder.Code != http.StatusNotFound || failure.Code != api.NotFound || len(tree.staged) != 0 {
		t.Errorf("status/code = %d/%s, staged %+v; want 404/not_found and nothing staged",
			recorder.Code, failure.Code, tree.staged)
	}
}

func TestUnstageTakesTheNamedChangeOutOfTheIndex(t *testing.T) {
	t.Parallel()

	// Arrange
	var tree staging

	// Act
	recorder := sendStaging(t, tree.deps(), unstagePath, `{"path":"`+stagedPath+`"}`)

	// Assert
	if got := pathsOf(tree.unstaged); recorder.Code != http.StatusOK || !slices.Equal(got, []string{stagedPath}) {
		t.Errorf("status = %d, unstaged %q; want 200 and %s alone", recorder.Code, got, stagedPath)
	}
}

func TestUnstageAllTakesEveryStagedChangeOut(t *testing.T) {
	t.Parallel()

	// Arrange
	var tree staging

	// Act
	recorder := sendStaging(t, tree.deps(), unstagePath, everything)

	// Assert
	want := []string{stagedPath, renamedPath}
	if got := pathsOf(tree.unstaged); recorder.Code != http.StatusOK || !slices.Equal(got, want) {
		t.Errorf("status = %d, unstaged %q; want 200 and %q", recorder.Code, got, want)
	}
}

func TestStagingNeedsAPathOrAllButNotBoth(t *testing.T) {
	t.Parallel()

	cases := map[string]struct{ endpoint, body string }{
		"neither":         {endpoint: stagePath, body: `{}`},
		"all, but false":  {endpoint: stagePath, body: `{"all":false}`},
		"an empty path":   {endpoint: stagePath, body: `{"path":""}`},
		"a path and all":  {endpoint: stagePath, body: `{"path":"` + editedPath + `","all":true}`},
		"unstage neither": {endpoint: unstagePath, body: `{}`},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			var tree staging

			// Act
			recorder := sendStaging(t, tree.deps(), tt.endpoint, tt.body)

			// Assert
			failure := decode[api.Problem](t, recorder)
			if recorder.Code != http.StatusUnprocessableEntity || failure.Code != api.Unprocessable {
				t.Errorf("status/code = %d/%s, want 422/unprocessable", recorder.Code, failure.Code)
			}

			if len(tree.staged)+len(tree.unstaged) != 0 {
				t.Errorf("moved %+v %+v, want nothing", tree.staged, tree.unstaged)
			}
		})
	}
}

func TestStagingIsUnavailableWithoutItsSeams(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		unwire   func(*webserver.Deps)
		endpoint string
	}{
		"no stage seam":             {unwire: func(deps *webserver.Deps) { deps.Stage = nil }, endpoint: stagePath},
		"no unstage seam":           {unwire: func(deps *webserver.Deps) { deps.Unstage = nil }, endpoint: unstagePath},
		"no changes seam to stage":  {unwire: func(deps *webserver.Deps) { deps.Changes = nil }, endpoint: stagePath},
		"no changes seam to remove": {unwire: func(deps *webserver.Deps) { deps.Changes = nil }, endpoint: unstagePath},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			var tree staging

			deps := tree.deps()
			tt.unwire(&deps)

			// Act
			recorder := sendStaging(t, deps, tt.endpoint, everything)

			// Assert
			failure := decode[api.Problem](t, recorder)
			if recorder.Code != http.StatusUnprocessableEntity || !strings.Contains(failure.Detail, "not available") {
				t.Errorf("status = %d, detail %q; want 422 saying staging is not available", recorder.Code, failure.Detail)
			}
		})
	}
}

func TestStagingReportsAChangesReadFailure(t *testing.T) {
	t.Parallel()

	// Arrange
	var tree staging

	deps := tree.deps()
	deps.Changes = func() ([]gitrepo.Change, error) { return nil, errSeam }

	// Act
	recorder := sendStaging(t, deps, stagePath, `{"path":"`+editedPath+`"}`)

	// Assert
	failure := decode[api.Problem](t, recorder)
	if recorder.Code != http.StatusInternalServerError || strings.Contains(failure.Detail, errSeam.Error()) {
		t.Errorf("status = %d, detail %q; want an opaque 500", recorder.Code, failure.Detail)
	}
}

func TestStagingNeverForwardsGitsOwnWords(t *testing.T) {
	t.Parallel()

	// Unstaging in a partial clone fetches what the index needs, so git's own
	// words can name the remote; the detail names the file and never the host.
	refused := fmt.Errorf("unstaging %s: %w: fatal: unable to access 'https://%s/acme/repo.git/'",
		stagedPath, errSeam, gitHost)
	cases := map[string]struct {
		endpoint, body, want string
	}{
		"one file": {endpoint: unstagePath, body: `{"path":"` + stagedPath + `"}`, want: stagedPath},
		"all":      {endpoint: unstagePath, body: everything, want: "every"},
		"staging":  {endpoint: stagePath, body: `{"path":"` + editedPath + `"}`, want: editedPath},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			var tree staging

			deps := tree.deps()
			deps.Stage = func(gitrepo.Change) error { return refused }
			deps.Unstage = func(gitrepo.Change) error { return refused }

			// Act
			recorder := sendStaging(t, deps, tt.endpoint, tt.body)

			// Assert
			failure := decode[api.Problem](t, recorder)
			if recorder.Code != http.StatusUnprocessableEntity || !strings.Contains(failure.Detail, tt.want) {
				t.Errorf("status = %d, detail %q; want 422 naming %q", recorder.Code, failure.Detail, tt.want)
			}

			if strings.Contains(recorder.Body.String(), gitHost) {
				t.Errorf("body = %q, leaks the remote's host", recorder.Body.String())
			}
		})
	}
}

func TestStagingAnswersTheTreeAsReadBeforeWhenTheRereadFails(t *testing.T) {
	t.Parallel()

	// Arrange
	// The first read resolves the path; the second, after staging, fails. The
	// file is staged all the same, so the answer is a success, not a failure.
	var tree staging

	deps := tree.deps()
	reads := 0
	deps.Changes = func() ([]gitrepo.Change, error) {
		reads++
		if reads > 1 {
			return nil, errSeam
		}

		return workTree(), nil
	}

	// Act
	recorder := sendStaging(t, deps, stagePath, `{"path":"`+editedPath+`"}`)

	// Assert
	listed := decode[api.ChangeList](t, recorder)
	if recorder.Code != http.StatusOK || len(listed.Changes) != len(workTree()) || len(tree.staged) != 1 {
		t.Errorf("status = %d, answered %d changes, staged %d; want 200, the tree as read, one staged",
			recorder.Code, len(listed.Changes), len(tree.staged))
	}
}

func TestStagingAnswersTheTreeAsItNowStands(t *testing.T) {
	t.Parallel()

	// Arrange
	// The read before the stage finds the file edited; the read after finds it
	// staged, and that is the tree the answer carries.
	var tree staging

	deps := tree.deps()
	reads := 0
	deps.Changes = func() ([]gitrepo.Change, error) {
		reads++

		changes := workTree()
		if reads > 1 {
			edited := slices.IndexFunc(changes, func(change gitrepo.Change) bool { return change.Path == editedPath })
			changes[edited] = gitrepo.Change{Path: editedPath, Staged: 'M', Unstaged: ' '}
		}

		return changes, nil
	}

	// Act
	recorder := sendStaging(t, deps, stagePath, `{"path":"`+editedPath+`"}`)

	// Assert
	listed := decode[api.ChangeList](t, recorder)

	answered := slices.IndexFunc(listed.Changes, func(change api.Change) bool { return change.Path == editedPath })
	if recorder.Code != http.StatusOK || answered < 0 {
		t.Fatalf("status = %d, answered %+v; want 200 listing %s", recorder.Code, listed.Changes, editedPath)
	}

	if got := listed.Changes[answered]; !got.Staged || got.HasUnstaged {
		t.Errorf("answered %s as %+v, want it wholly staged, as the re-read found it", editedPath, got)
	}
}

func TestStagingWritesWaitForEachOther(t *testing.T) {
	t.Parallel()

	// Arrange
	// git lets one process write the index at a time, and one that finds it
	// taken fails on .git/index.lock rather than waits. This stage fails the
	// same way when it is reached while another is under way.
	var tree staging

	var writing atomic.Int32

	deps := tree.deps()
	deps.Stage = func(gitrepo.Change) error {
		defer writing.Add(-1)

		if writing.Add(1) > 1 {
			return errSeam
		}

		time.Sleep(indexHeld)

		return nil
	}
	handler := serve(t, deps, config.Default())

	// Stage all, and one file's own Stage pressed while it runs.
	bodies := []string{everything, `{"path":"` + editedPath + `"}`}
	codes := make([]int, len(bodies))

	var group sync.WaitGroup

	// Act
	for index, body := range bodies {
		group.Go(func() { codes[index] = send(t, handler, http.MethodPost, stagePath, body).Code })
	}

	group.Wait()

	// Assert
	if !slices.Equal(codes, []int{http.StatusOK, http.StatusOK}) {
		t.Errorf("statuses = %v, want both 200: a write waits for the one under way", codes)
	}
}

func TestDryRunRefusesStaging(t *testing.T) {
	t.Parallel()

	// The guard refuses by method, so it covers both endpoints and both forms
	// without either handler knowing about dry run; no seam is reached.
	cases := map[string]struct{ endpoint, body string }{
		"stage a file":   {endpoint: stagePath, body: `{"path":"` + editedPath + `"}`},
		"stage all":      {endpoint: stagePath, body: everything},
		"unstage a file": {endpoint: unstagePath, body: `{"path":"` + stagedPath + `"}`},
		"unstage all":    {endpoint: unstagePath, body: everything},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			var tree staging

			dryRun := webserver.Info{Version: testVersion, DryRun: true}
			handler := serveWith(t, tree.deps(), config.Default(), dryRun)

			// Act
			recorder := send(t, handler, http.MethodPost, tt.endpoint, tt.body)

			// Assert
			if recorder.Code != http.StatusForbidden || len(tree.staged)+len(tree.unstaged) != 0 {
				t.Errorf("status = %d, moved %+v %+v; want 403 and nothing moved",
					recorder.Code, tree.staged, tree.unstaged)
			}
		})
	}
}
