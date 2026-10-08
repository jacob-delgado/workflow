// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

import (
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/messaging"
	"github.com/jacob-delgado/workflow/internal/webserver"
)

// Where an announcement is posted or held, and where a held one is dropped,
// and the fields an edit is posted with.
const (
	announcePath    = "/api/announce"
	queuedPath      = "/api/announce/queued"
	channelField    = "channel"
	textField       = "text"
	editedTextField = "edited_text"
	editedText      = "Please review #42 today"
	// notTheBranchesPull is why a held announcement is dropped once its pull
	// request is not the branch's open one.
	notTheBranchesPull = "#42 is no longer this branch's pull request"
)

// forgeWorld is a forge and a messaging service a test can change under a
// running server, each read behind a lock, since a held announcement is
// settled on a goroutine of the server's own as well as on a frame.
type forgeWorld struct {
	mu     sync.Mutex
	branch string
	// head, when set, is the commit the branch is at, in place of filledDeps'.
	head string
	// pullErr, when set, is how the forge fails to find the pull request.
	pullErr error
	// branchErr, when set, is how git fails to read the branch.
	branchErr error
	ci        forge.CIState
	ciErr     error
	pull      int
	gone      bool
	state     forge.PullState
	now       time.Time
	posts     []string
	fail      error
}

// newForgeWorld is pull request 42, open, its CI running, at a fixed time.
func newForgeWorld() *forgeWorld {
	return &forgeWorld{
		branch: testBranchName, ci: forge.CIRunning, pull: 42, state: forge.StateOpen,
		now: time.Date(2026, 10, 6, 9, 0, 0, 0, time.UTC),
	}
}

// deps is filledDeps reading the world's pull request, CI and clock, and
// posting into it.
func (w *forgeWorld) deps() webserver.Deps {
	deps := filledDeps()
	checkedOut := deps.Git.Branch
	deps.Git.Branch = func() (gitrepo.Branch, error) {
		branch, err := checkedOut()

		w.mu.Lock()
		defer w.mu.Unlock()

		branch.Name = w.branch
		if w.head != "" {
			branch.Head = w.head
		}

		if w.branchErr != nil {
			return gitrepo.Branch{}, w.branchErr
		}

		return branch, err
	}
	deps.Forge.FindPullRequest = func(string) (forge.PullRequest, bool, error) {
		w.mu.Lock()
		defer w.mu.Unlock()

		url := fmt.Sprintf("https://x/%d", w.pull)

		return forge.PullRequest{Number: w.pull, URL: url, Title: "Redact tokens", State: w.state}, !w.gone, w.pullErr
	}
	deps.Forge.CheckStatus = func(forge.PullRequest, string) (forge.CI, error) {
		w.mu.Lock()
		defer w.mu.Unlock()

		return forge.CI{State: w.ci, Total: 1}, w.ciErr
	}
	deps.Clock = func() time.Time {
		w.mu.Lock()
		defer w.mu.Unlock()

		return w.now
	}
	deps.Messaging.Post = func(_, text string) error {
		w.mu.Lock()
		defer w.mu.Unlock()

		if w.fail != nil {
			return w.fail
		}

		w.posts = append(w.posts, text)

		return nil
	}

	return deps
}

// turn sets the CI to state, or the pull request to another number, and moves
// the clock past the forge's interval, so the next frame asks the forge again.
func (w *forgeWorld) turn(change func(*forgeWorld)) {
	w.mu.Lock()
	defer w.mu.Unlock()

	change(w)
	w.now = w.now.Add(time.Hour)
}

// posted is every text posted so far.
func (w *forgeWorld) posted() []string {
	w.mu.Lock()
	defer w.mu.Unlock()

	return slices.Clone(w.posts)
}

// announceWhenGreen asks handler to post the previewed announcement to #dev
// once CI passes, with the extra fields given.
func announceWhenGreen(t *testing.T, handler http.Handler, extra map[string]string) *httptest.ResponseRecorder {
	t.Helper()

	previewed := previewAnnouncement(t, handler)
	fields := map[string]string{channelField: slackChannel, textField: previewed, "when": "ci_passes"}
	maps.Copy(fields, extra)

	return postAnnounce(t, handler, fields)
}

// heldAnnouncement is the held announcement one frame of the stream reports,
// or nil when it reports none.
func heldAnnouncement(t *testing.T, handler http.Handler) *api.QueuedAnnouncement {
	t.Helper()

	return firstSnapshot(t, streamOnce(t, handler, "/api/events").Body.String()).QueuedAnnouncement
}

// cancelHeld drops a held announcement once the test is over, so the server's
// own wait for it ends with the test.
func cancelHeld(t *testing.T, handler http.Handler) {
	t.Helper()

	t.Cleanup(func() { send(t, handler, http.MethodDelete, queuedPath, "") })
}

func TestAnnouncingWhenCIPassesHoldsItWhileCIRuns(t *testing.T) {
	t.Parallel()

	// Arrange
	world := newForgeWorld()
	handler := serve(t, world.deps(), config.Default())
	cancelHeld(t, handler)

	// Act
	recorder := announceWhenGreen(t, handler, nil)

	// Assert
	if recorder.Code != http.StatusAccepted {
		t.Fatalf("status = %d (%s), want 202", recorder.Code, recorder.Body.String())
	}

	want := api.QueuedAnnouncement{State: api.QueuedAnnouncementStateWaiting, Channel: slackChannel, Pull: 42}
	if held := decode[api.QueuedAnnouncement](t, recorder); held != want {
		t.Errorf("answer = %+v, want %+v", held, want)
	}

	if held := heldAnnouncement(t, handler); held == nil || *held != want {
		t.Errorf("frame's held announcement = %+v, want %+v", held, want)
	}

	if posts := world.posted(); len(posts) != 0 {
		t.Errorf("posted %q, want nothing before CI passes", posts)
	}
}

func TestAHeldAnnouncementGoesOnTheFirstFrameWithGreenCI(t *testing.T) {
	t.Parallel()

	// Arrange
	world := newForgeWorld()
	handler := serve(t, world.deps(), config.Default())
	announceWhenGreen(t, handler, map[string]string{editedTextField: "Ready for review: #42"})
	world.turn(func(w *forgeWorld) { w.ci = forge.CIPassed })

	// Act: the first frame with green CI
	held := heldAnnouncement(t, handler)

	// Assert: it went
	if held == nil || held.State != api.QueuedAnnouncementStateAnnounced {
		t.Errorf("frame's held announcement = %+v, want it announced", held)
	}

	// Act: another frame
	heldAnnouncement(t, handler)

	// Assert: it went once
	if posts := world.posted(); !slices.Equal(posts, []string{"Ready for review: #42"}) {
		t.Errorf("posted %q, want the edited text once", posts)
	}
}

func TestAHeldAnnouncementIsDroppedWithTheReason(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		change     func(*forgeWorld)
		wantReason string
	}{
		"CI failed": {
			change:     func(w *forgeWorld) { w.ci = forge.CIFailed },
			wantReason: "CI failed at 10:00",
		},
		"another pull request": {
			change:     func(w *forgeWorld) { w.pull = 43 },
			wantReason: notTheBranchesPull,
		},
		"merged first": {
			change:     func(w *forgeWorld) { w.state = forge.StateMerged },
			wantReason: "#42 merged before its CI passed",
		},
		"another branch checked out": {
			change:     func(w *forgeWorld) { w.branch = "feat/PROJ-500" },
			wantReason: notTheBranchesPull,
		},
		"no pull request any more": {
			change:     func(w *forgeWorld) { w.gone = true },
			wantReason: notTheBranchesPull,
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			world := newForgeWorld()
			handler := serve(t, world.deps(), config.Default())
			announceWhenGreen(t, handler, nil)
			world.turn(tt.change)

			// Act
			held := heldAnnouncement(t, handler)

			// Assert
			if held == nil || held.State != api.QueuedAnnouncementStateDropped || orEmpty(held.Reason) != tt.wantReason {
				t.Errorf("frame's held announcement = %+v, want it dropped because %q", held, tt.wantReason)
			}

			if posts := world.posted(); len(posts) != 0 {
				t.Errorf("posted %q, want nothing", posts)
			}
		})
	}
}

func TestAHeldAnnouncementThatCannotBePostedSaysWhyWithoutTheWebhook(t *testing.T) {
	t.Parallel()

	// Arrange
	world := newForgeWorld()
	handler := serve(t, world.deps(), config.Default())
	announceWhenGreen(t, handler, nil)
	world.turn(func(w *forgeWorld) {
		w.ci = forge.CIPassed
		w.fail = fmt.Errorf("%w: https://hooks.slack.com/services/%s", messaging.ErrUnreachable, webhookSecret)
	})

	// Act
	held := heldAnnouncement(t, handler)

	// Assert
	if held == nil || held.State != api.QueuedAnnouncementStateDropped || orEmpty(held.Reason) == "" {
		t.Fatalf("frame's held announcement = %+v, want it dropped with why", held)
	}

	if strings.Contains(*held.Reason, webhookSecret) || strings.Contains(*held.Reason, "hooks.slack.com") {
		t.Errorf("reason = %q, carries the webhook", *held.Reason)
	}
}

// ciTick is the CI interval a held announcement is read on in a synctest
// bubble, where its length costs nothing.
const ciTick = time.Minute

func TestAHeldAnnouncementGoesWithNoPageOpen(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		// Arrange
		// The server reads the CI for the held announcement on the CI
		// interval itself; no frame is ever asked for.
		world := newForgeWorld()
		cfg := config.Default()
		cfg.Timing.CIInterval = ciTick.String()
		handler := serve(t, world.deps(), cfg)
		cancelHeld(t, handler)
		announceWhenGreen(t, handler, nil)
		world.turn(func(w *forgeWorld) { w.ci = forge.CIPassed })

		// Act
		advance(ciTick)

		// Assert
		if posts := world.posted(); len(posts) != 1 {
			t.Errorf("posted %q, want the held announcement once, with no page open", posts)
		}
	})
}

func TestAnnouncingWhenCIPassesPostsAtOnceWhenItHasPassed(t *testing.T) {
	t.Parallel()

	// Arrange
	world := newForgeWorld()
	world.ci = forge.CIPassed
	handler := serve(t, world.deps(), config.Default())

	// Act
	recorder := announceWhenGreen(t, handler, nil)

	// Assert
	if recorder.Code != http.StatusOK || len(world.posted()) != 1 {
		t.Errorf("status = %d, posted %q; want 200 and the announcement posted at once", recorder.Code, world.posted())
	}
}

func TestOnlyAReadyAnnouncementWithCIRunningWaits(t *testing.T) {
	t.Parallel()

	cases := map[string]func(*forgeWorld){
		"no CI":            func(w *forgeWorld) { w.ci = forge.CINone },
		"a merged request": func(w *forgeWorld) { w.state = forge.StateMerged },
	}

	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			world := newForgeWorld()
			change(world)
			handler := serve(t, world.deps(), config.Default())

			// Act
			recorder := announceWhenGreen(t, handler, nil)

			// Assert
			if failure := decode[api.Problem](t, recorder); recorder.Code != http.StatusConflict ||
				failure.Code != api.ProblemCodeConflict || len(world.posted()) != 0 {
				t.Errorf("status/code = %d/%s, posted %q; want 409 and nothing posted",
					recorder.Code, failure.Code, world.posted())
			}
		})
	}
}

func TestAHeldAnnouncementWaitsOnWhileItsCICannotBeRead(t *testing.T) {
	t.Parallel()

	// Arrange
	world := newForgeWorld()
	handler := serve(t, world.deps(), config.Default())
	cancelHeld(t, handler)
	announceWhenGreen(t, handler, nil)
	world.turn(func(w *forgeWorld) { w.ciErr = forge.ErrUnreachable })

	// Act
	held := heldAnnouncement(t, handler)

	// Assert
	if held == nil || held.State != api.QueuedAnnouncementStateWaiting {
		t.Errorf("frame's held announcement = %+v, want it still waiting", held)
	}

	if posts := world.posted(); len(posts) != 0 {
		t.Errorf("posted %q, want nothing", posts)
	}
}

func TestAnnouncingWhenCIPassesWithNoCISeamIsAConflict(t *testing.T) {
	t.Parallel()

	// Arrange
	world := newForgeWorld()
	deps := world.deps()
	deps.Forge.CheckStatus = nil
	handler := serve(t, deps, config.Default())

	// Act
	recorder := announceWhenGreen(t, handler, nil)

	// Assert
	if failure := decode[api.Problem](t, recorder); recorder.Code != http.StatusConflict ||
		!strings.Contains(failure.Detail, "no running CI") || len(world.posted()) != 0 {
		t.Errorf("status/detail = %d/%q, posted %q; want 409, no running CI, nothing posted",
			recorder.Code, failure.Detail, world.posted())
	}
}

func TestAnnouncingWhenCIPassesWithCIUnreadableIsUnreachable(t *testing.T) {
	t.Parallel()

	// Arrange
	world := newForgeWorld()
	world.ciErr = fmt.Errorf("%w: https://forge.internal.example", forge.ErrUnreachable)
	handler := serve(t, world.deps(), config.Default())

	// Act
	recorder := announceWhenGreen(t, handler, nil)

	// Assert
	failure := decode[api.Problem](t, recorder)
	if recorder.Code != http.StatusBadGateway || failure.Code != api.ProblemCodeUnreachable || len(world.posted()) != 0 {
		t.Errorf("status/code = %d/%s, posted %q; want 502/unreachable and nothing posted",
			recorder.Code, failure.Code, world.posted())
	}

	if strings.Contains(failure.Detail, "forge.internal.example") {
		t.Errorf("detail = %q, names the forge's host", failure.Detail)
	}
}

func TestAnnouncingNowDropsTheHeldAnnouncement(t *testing.T) {
	t.Parallel()

	// Arrange
	world := newForgeWorld()
	handler := serve(t, world.deps(), config.Default())
	announceWhenGreen(t, handler, nil)
	previewed := previewAnnouncement(t, handler)

	// Act: announce now
	recorder := postAnnounce(t, handler, map[string]string{channelField: slackChannel, textField: previewed})

	// Assert: it went
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}

	// Act: CI passes
	world.turn(func(w *forgeWorld) { w.ci = forge.CIPassed })

	// Assert: nothing is held, and the held one never follows
	if held := heldAnnouncement(t, handler); held != nil {
		t.Errorf("held = %+v, want nothing held", held)
	}

	if posts := world.posted(); len(posts) != 1 {
		t.Errorf("posted %d times, want once: the channel never reads it twice", len(posts))
	}
}

func TestCancelingDropsTheHeldAnnouncementUnposted(t *testing.T) {
	t.Parallel()

	// Arrange
	world := newForgeWorld()
	handler := serve(t, world.deps(), config.Default())
	announceWhenGreen(t, handler, nil)

	// Act: drop it
	recorder := send(t, handler, http.MethodDelete, queuedPath, "")

	// Assert: it was dropped
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", recorder.Code)
	}

	// Act: CI passes
	world.turn(func(w *forgeWorld) { w.ci = forge.CIPassed })

	// Assert: nothing is held, and nothing goes
	if held := heldAnnouncement(t, handler); held != nil {
		t.Errorf("held = %+v, want nothing held", held)
	}

	if posts := world.posted(); len(posts) != 0 {
		t.Errorf("posted %q, want nothing", posts)
	}
}

func TestCancelingWithNothingHeldIsAConflict(t *testing.T) {
	t.Parallel()

	// Arrange
	handler := serve(t, newForgeWorld().deps(), config.Default())

	// Act
	recorder := send(t, handler, http.MethodDelete, queuedPath, "")

	// Assert
	failure := decode[api.Problem](t, recorder)
	if recorder.Code != http.StatusConflict || failure.Code != api.ProblemCodeConflict {
		t.Errorf("status/code = %d/%s, want 409/conflict", recorder.Code, failure.Code)
	}
}

func TestAnEditedAnnouncementIsPostedAsEdited(t *testing.T) {
	t.Parallel()

	// Arrange
	world := newForgeWorld()
	world.ci = forge.CIPassed
	handler := serve(t, world.deps(), config.Default())
	previewed := previewAnnouncement(t, handler)

	// Act
	recorder := postAnnounce(t, handler, map[string]string{
		channelField: slackChannel, textField: previewed, editedTextField: editedText,
	})

	// Assert
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d (%s), want 200", recorder.Code, recorder.Body.String())
	}

	if posts := world.posted(); !slices.Equal(posts, []string{editedText}) {
		t.Errorf("posted %q, want the edited text", posts)
	}

	if answer := decode[api.Announcement](t, recorder); answer.Text != editedText {
		t.Errorf("answer = %+v, want the text as posted", answer)
	}
}

func TestAnEditedAnnouncementIsRefusedWhereItCannotGo(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		fields     func(previewed string) map[string]string
		turn       func(*forgeWorld)
		wantStatus int
	}{
		"blank edited text": {
			fields: func(previewed string) map[string]string {
				return map[string]string{channelField: slackChannel, textField: previewed, editedTextField: " \n "}
			},
			wantStatus: http.StatusUnprocessableEntity,
		},
		"an edit without what it began from": {
			fields: func(string) map[string]string {
				return map[string]string{channelField: slackChannel, editedTextField: "Ship it"}
			},
			wantStatus: http.StatusUnprocessableEntity,
		},
		"an edit of an announcement that changed since": {
			fields: func(previewed string) map[string]string {
				return map[string]string{channelField: slackChannel, textField: previewed, editedTextField: "Ship it"}
			},
			turn:       func(w *forgeWorld) { w.ci = forge.CIFailed },
			wantStatus: http.StatusConflict,
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			world := newForgeWorld()
			world.ci = forge.CIPassed
			handler := serve(t, world.deps(), config.Default())
			previewed := previewAnnouncement(t, handler)

			if tt.turn != nil {
				world.turn(tt.turn)
			}

			// Act
			recorder := postAnnounce(t, handler, tt.fields(previewed))

			// Assert
			if recorder.Code != tt.wantStatus || len(world.posted()) != 0 {
				t.Errorf("status = %d, posted %q; want %d and nothing posted", recorder.Code, world.posted(), tt.wantStatus)
			}
		})
	}
}

func TestAPreviewSaysWhetherItCanWaitForCI(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		ci   forge.CIState
		want bool
	}{
		"CI running": {ci: forge.CIRunning, want: true},
		"CI passed":  {ci: forge.CIPassed, want: false},
		"no CI":      {ci: forge.CINone, want: false},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			world := newForgeWorld()
			world.ci = tt.ci

			// Act
			recorder := get(t, serve(t, world.deps(), config.Default()), "/api/announcement")

			// Assert
			preview := decode[api.Announcement](t, recorder)
			if got := preview.CanWaitForCi != nil && *preview.CanWaitForCi; got != tt.want {
				t.Errorf("can_wait_for_ci = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDryRunRefusesToHoldOrDropAnAnnouncement(t *testing.T) {
	t.Parallel()

	cases := map[string]struct{ method, path, body string }{
		"holding one": {
			method: http.MethodPost, path: announcePath, body: mustJSON(t, map[string]string{
				channelField: slackChannel, "when": "ci_passes",
			}),
		},
		"dropping one": {method: http.MethodDelete, path: queuedPath},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			world := newForgeWorld()
			dryRun := webserver.Info{Version: testVersion, DryRun: true}
			handler := serveWith(t, world.deps(), config.Default(), dryRun)

			// Act
			recorder := send(t, handler, tt.method, tt.path, tt.body)

			// Assert
			if recorder.Code != http.StatusForbidden || heldAnnouncement(t, handler) != nil {
				t.Errorf("status = %d, want 403 and nothing held", recorder.Code)
			}
		})
	}
}

// mustJSON encodes fields as a JSON object.
func mustJSON(t *testing.T, fields map[string]string) string {
	t.Helper()

	body, err := json.Marshal(fields)
	if err != nil {
		t.Fatalf("encoding %v: %v", fields, err)
	}

	return string(body)
}

// orEmpty is the string a pointer holds, or "" for none.
func orEmpty(text *string) string {
	if text == nil {
		return ""
	}

	return *text
}
