// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"testing/synctest"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/webserver"
)

// internalHost is an address a seam's error carries that must never reach the
// page.
const internalHost = "git.internal.example"

func TestStreamPanelProblemsNeverCarryTheUnreachableHost(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := filledDeps()
	deps.Jira.Search = func(string, int) (jira.SearchResult, error) {
		return jira.SearchResult{}, fmt.Errorf("%w: https://%s/rest/api/2/search", jira.ErrUnreachable, internalHost)
	}
	deps.Forge.FindPullRequest = func(string) (forge.PullRequest, bool, error) {
		return forge.PullRequest{}, false, fmt.Errorf("%w: https://%s/api/v4", forge.ErrUnreachable, internalHost)
	}
	deps.Git.Changes = func() ([]gitrepo.Change, error) {
		return nil, fmt.Errorf("git status in /home/me/%s: %w", internalHost, errSeam)
	}

	// Act
	recorder := streamOnce(t, serve(t, deps, config.Default()), "/api/events")

	// Assert
	snap := firstSnapshot(t, recorder.Body.String())
	if snap.Problems == nil || snap.Problems.Issues == nil || snap.Problems.Review == nil ||
		snap.Problems.Changes == nil {
		t.Fatalf("problems = %+v, want the issues, review and changes problems", snap.Problems)
	}

	if snap.Problems.Review.Code != api.ProblemCodeUnreachable || snap.Problems.Issues.Code != api.ProblemCodeUnreachable {
		t.Errorf("problems = %+v, want the unreachable services classed as unreachable", snap.Problems)
	}

	said, err := json.Marshal(snap.Problems)
	if err != nil {
		t.Fatal(err)
	}

	if strings.Contains(string(said), internalHost) {
		t.Errorf("problems = %s, carry the unreachable host", said)
	}
}

func TestStreamSaysNoProblemOfAServiceThereIsNothingToAsk(t *testing.T) {
	t.Parallel()

	cases := map[string]func(*webserverDepsEdit){
		"no repository": func(edit *webserverDepsEdit) {
			edit.branchErr, edit.changesErr = gitrepo.ErrNotARepository, gitrepo.ErrNotARepository
		},
	}

	for name, arrange := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			var edit webserverDepsEdit
			arrange(&edit)

			// Act
			recorder := streamOnce(t, serve(t, edit.apply(filledDeps()), config.Default()), "/api/events")

			// Assert
			if snap := firstSnapshot(t, recorder.Body.String()); snap.Problems != nil {
				t.Errorf("problems = %+v, want none when there was nothing to ask", snap.Problems)
			}
		})
	}
}

func TestStreamSaysAForgeThatIsNotSetUpIsNotSetUp(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		cause error
		want  string
	}{
		"an origin on no forge workflow reads": {cause: forge.ErrNotARemote, want: "origin does not name"},
		"a forge workflow cannot tell":         {cause: forge.ErrUnknownForge, want: "forge.kind"},
		"no forge token":                       {cause: forge.ErrNoToken, want: "$GITHUB_TOKEN"},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			edit := webserverDepsEdit{pullErr: fmt.Errorf("%w: "+internalHost, tt.cause)}

			// Act
			recorder := streamOnce(t, serve(t, edit.apply(filledDeps()), config.Default()), "/api/events")

			// Assert
			problems := firstSnapshot(t, recorder.Body.String()).Problems
			if problems == nil || problems.Review == nil || problems.Review.Code != api.ProblemCodeNotSetUp ||
				!strings.Contains(problems.Review.Detail, tt.want) || strings.Contains(problems.Review.Detail, internalHost) {
				t.Errorf("problems = %+v, want the review not set up, saying %q, never the host", problems, tt.want)
			}
		})
	}
}

// webserverDepsEdit is which reads of filledDeps fail, and with what.
type webserverDepsEdit struct {
	branchErr  error
	changesErr error
	pullErr    error
}

// apply makes each read the edit names fail with its error.
func (e webserverDepsEdit) apply(deps webserver.Deps) webserver.Deps {
	if e.branchErr != nil {
		deps.Git.Branch = func() (gitrepo.Branch, error) { return gitrepo.Branch{}, e.branchErr }
	}

	if e.changesErr != nil {
		deps.Git.Changes = func() ([]gitrepo.Change, error) { return nil, e.changesErr }
	}

	if e.pullErr != nil {
		deps.Forge.FindPullRequest = func(string) (forge.PullRequest, bool, error) {
			return forge.PullRequest{}, false, e.pullErr
		}
	}

	return deps
}

func TestGetReviewSaysWhyItsCICouldNotBeRead(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := filledDeps()
	deps.Forge.CheckStatus = func(forge.PullRequest, string) (forge.CI, error) {
		return forge.CI{}, fmt.Errorf("%w: https://%s", forge.ErrUnreachable, internalHost)
	}

	// Act
	review := decode[api.Review](t, get(t, serve(t, deps, config.Default()), "/api/review"))

	// Assert
	if review.Ci != nil || review.CiError == nil || review.CiError.Code != api.ProblemCodeUnreachable ||
		strings.Contains(review.CiError.Detail, internalHost) {
		t.Errorf("review = ci %+v, ci_error %+v, want no CI and why, without the host", review.Ci, review.CiError)
	}
}

func TestAHeldAnnouncementWaitsOnWhileTheForgeCannotBeRead(t *testing.T) {
	t.Parallel()

	// Arrange
	// A commit lands, so nothing is held for the branch at its new head, and
	// the forge then cannot be read: that says nothing of the pull request.
	world := newForgeWorld()
	handler := serve(t, world.deps(), config.Default())
	cancelHeld(t, handler)
	announceWhenGreen(t, handler, nil)
	world.turn(func(w *forgeWorld) { w.head, w.pullErr = "def4567", forge.ErrUnreachable })

	// Act
	held := heldAnnouncement(t, handler)

	// Assert
	if held == nil || held.State != api.QueuedAnnouncementStateWaiting {
		t.Errorf("frame's held announcement = %+v, want it still waiting", held)
	}
}

func TestAHeldAnnouncementWithNoPageOpenOutlastsAReadThatFails(t *testing.T) {
	t.Parallel()

	cases := map[string]func(*forgeWorld){
		"the branch cannot be read": func(w *forgeWorld) { w.branchErr = errSeam },
		"the forge cannot be read":  func(w *forgeWorld) { w.head, w.pullErr = "def4567", forge.ErrUnreachable },
	}

	for name, fail := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			synctest.Test(t, func(t *testing.T) {
				// Arrange
				// The server reads for the held announcement on the CI
				// interval itself; three of its reads fail, then the CI is
				// seen to pass.
				world := newForgeWorld()
				cfg := config.Default()
				cfg.Timing.CIInterval = ciTick.String()
				handler := serve(t, world.deps(), cfg)
				cancelHeld(t, handler)
				announceWhenGreen(t, handler, nil)
				world.turn(fail)
				advance(3 * ciTick)

				// Act
				world.turn(func(w *forgeWorld) { w.branchErr, w.pullErr, w.ci = nil, nil, forge.CIPassed })
				advance(ciTick)

				// Assert
				if posts := world.posted(); len(posts) != 1 {
					t.Errorf("posted %q, want the held announcement once the reads answer again", posts)
				}
			})
		})
	}
}
