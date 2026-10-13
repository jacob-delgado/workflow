// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/codeowners"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/proc"
	"github.com/jacob-delgado/workflow/internal/webserver"
)

// draftNotArmed is what an opened draft's answer says of its announcement.
const draftNotArmed = "a draft is not announced; announce it once it is ready"

// openingDeps are the forge world's deps with no pull request yet for the
// branch, which opening one makes the one found; draft opens it as a draft.
func openingDeps(world *forgeWorld, draft bool) webserver.Deps {
	world.gone = true
	deps := world.deps()
	deps.Git.Push = func(context.Context, string) (proc.Output, error) { return fakeOutput(nil, nil), nil }
	deps.Forge.CreatePullRequest = func(context.Context, forge.NewPullRequest) (forge.PullRequest, error) {
		world.mu.Lock()
		defer world.mu.Unlock()

		world.gone = false

		return forge.PullRequest{
			Number: world.pull, URL: fmt.Sprintf("https://x/%d", world.pull), Title: "Redact tokens",
			State: forge.StateOpen, Draft: draft,
		}, nil
	}

	return deps
}

// webhookConfig posts announcements through a Slack webhook, announcing a
// pull request opened here once its CI passes, as it does by default.
func webhookConfig() config.Config {
	cfg := config.Default()
	cfg.Messaging.Kind = config.KindSlack
	cfg.Messaging.WebhookURL = "https://hooks.slack.com/services/T000/B000/XXXX"

	return cfg
}

// opened opens the pull request against handler and answers what it said.
func opened(t *testing.T, handler http.Handler) api.OpenedPullRequest {
	t.Helper()

	recorder := send(t, handler, http.MethodPost, "/api/pull-request", openRequestBody)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d (%s), want the pull request opened", recorder.Code, recorder.Body.String())
	}

	return decode[api.OpenedPullRequest](t, recorder)
}

// armedAndWaiting reports an announcement of #42 armed as it opened, waiting.
func armedAndWaiting(armed *api.QueuedAnnouncement) bool {
	return armed != nil && armed.State == api.QueuedAnnouncementStateWaiting && armed.Pull == 42 &&
		armed.Automatic != nil && *armed.Automatic
}

func TestOpeningAPullRequestArmsItsAnnouncement(t *testing.T) {
	t.Parallel()

	// Arrange
	world := newForgeWorld()
	handler := serve(t, openingDeps(world, false), webhookConfig())
	cancelHeld(t, handler)

	// Act: open the pull request
	answer := opened(t, handler)

	// Assert: its announcement is armed, and waits
	if !armedAndWaiting(answer.Announcement) {
		t.Fatalf("announcement = %+v, want #42 armed and waiting", answer.Announcement)
	}

	if held := heldAnnouncement(t, handler); held == nil || held.State != api.QueuedAnnouncementStateWaiting {
		t.Errorf("frame's held announcement = %+v, want it waiting", held)
	}

	// Act: CI passes
	world.turn(func(w *forgeWorld) { w.ci = forge.CIPassed })

	held := heldAnnouncement(t, handler)

	// Assert: it went, once
	if held == nil || held.State != api.QueuedAnnouncementStateAnnounced || len(world.posted()) != 1 {
		t.Errorf("frame's held announcement = %+v, posted %q; want it announced once", held, world.posted())
	}
}

func TestOpeningArmsNothingWithAutoAnnounceOff(t *testing.T) {
	t.Parallel()

	// Arrange
	world := newForgeWorld()
	cfg := webhookConfig()
	cfg.Messaging.AutoAnnounce = false
	handler := serve(t, openingDeps(world, false), cfg)

	// Act
	answer := opened(t, handler)

	// Assert
	if answer.Announcement != nil || heldAnnouncement(t, handler) != nil {
		t.Errorf("announcement = %+v, want none armed", answer.Announcement)
	}
}

func TestOpeningArmsNothingWithoutMessaging(t *testing.T) {
	t.Parallel()

	// Arrange
	world := newForgeWorld()
	handler := serve(t, openingDeps(world, false), config.Default())

	// Act
	answer := opened(t, handler)

	// Assert
	if answer.Announcement != nil || answer.NotArmed != nil {
		t.Errorf("announcement = %+v, not armed %q; want neither", answer.Announcement, orEmpty(answer.NotArmed))
	}
}

func TestOpeningADraftArmsNothing(t *testing.T) {
	t.Parallel()

	// Arrange
	world := newForgeWorld()
	handler := serve(t, openingDeps(world, true), webhookConfig())

	// Act
	answer := opened(t, handler)

	// Assert
	if answer.Announcement != nil || orEmpty(answer.NotArmed) != draftNotArmed {
		t.Errorf("announcement = %+v, not armed %q; want none armed, saying %q",
			answer.Announcement, orEmpty(answer.NotArmed), draftNotArmed)
	}
}

func TestStopWaitingDropsAnArmedAnnouncement(t *testing.T) {
	t.Parallel()

	// Arrange
	world := newForgeWorld()
	handler := serve(t, openingDeps(world, false), webhookConfig())
	opened(t, handler)

	// Act
	recorder := send(t, handler, http.MethodDelete, queuedPath, "")

	// Assert
	world.turn(func(w *forgeWorld) { w.ci = forge.CIPassed })
	heldAnnouncement(t, handler)

	if recorder.Code != http.StatusNoContent || len(world.posted()) != 0 {
		t.Errorf("status = %d, posted %q; want 204 and nothing posted", recorder.Code, world.posted())
	}
}

func TestARefusedPostNowKeepsTheArmedAnnouncement(t *testing.T) {
	t.Parallel()

	// Arrange
	// The armed announcement waits; a check fails, and Announce now is asked.
	world := newForgeWorld()
	handler := serve(t, openingDeps(world, false), webhookConfig())
	cancelHeld(t, handler)
	opened(t, handler)
	world.turn(func(w *forgeWorld) { w.ci = forge.CIFailed })

	// Act: announce now
	refused := postAnnounce(t, handler, map[string]string{channelField: ""})

	// Assert: refused, and the armed one still waits
	if refused.Code != http.StatusConflict {
		t.Fatalf("status = %d (%s), want 409 for a failed check", refused.Code, refused.Body.String())
	}

	if held := heldAnnouncement(t, handler); held == nil || held.State != api.QueuedAnnouncementStateWaiting {
		t.Fatalf("frame's held announcement = %+v, want the armed one still waiting", held)
	}

	// Act: a run passes
	world.turn(func(w *forgeWorld) { w.ci = forge.CIPassed })

	held := heldAnnouncement(t, handler)

	// Assert: it goes, once
	if held == nil || held.State != api.QueuedAnnouncementStateAnnounced || len(world.posted()) != 1 {
		t.Errorf("frame's held announcement = %+v, posted %q; want it announced once", held, world.posted())
	}
}

func TestAnArmedAnnouncementTagsAsAnnounceYesDoes(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		workspaceErr error
		tagged       bool
	}{
		"a workspace Slack names":         {workspaceErr: nil, tagged: true},
		"a workspace Slack will not name": {workspaceErr: errSeam, tagged: false},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			world := newForgeWorld()
			deps := openingDeps(world, false)
			kept := newFakeKept()
			kept.workspaceErr = tt.workspaceErr
			kept.wire(&deps)
			handler := serve(t, deps, slackUserConfig())
			opened(t, handler)

			// Act
			world.turn(func(w *forgeWorld) { w.ci = forge.CIPassed })
			heldAnnouncement(t, handler)

			// Assert
			posts := world.posted()
			if len(posts) != 1 || strings.Contains(posts[0], "<@"+carla().ID+">") != tt.tagged {
				t.Errorf("posted %q, want one, tagging the linked owner: %t", posts, tt.tagged)
			}
		})
	}
}

func TestOpeningWhileAHeldAnnouncementIsPostedArmsNothing(t *testing.T) {
	t.Parallel()

	// Arrange
	// #42's announcement, held for its CI, is being posted when a new pull
	// request opens on the branch.
	world, held := newForgeWorld(), newHeldPost()
	deps := openingDeps(world, false)
	world.gone = false
	deps.Messaging.Post = held.post
	handler := serve(t, deps, webhookConfig())
	announceWhenGreen(t, handler, nil)
	world.turn(func(w *forgeWorld) { w.ci = forge.CIPassed })

	posting := make(chan struct{})

	go func() {
		defer close(posting)

		heldAnnouncement(t, handler)
	}()

	held.awaitStart(t)
	world.turn(func(w *forgeWorld) { w.gone, w.pull = true, 43 })

	// Act
	answer := opened(t, handler)

	// Assert
	close(held.release)
	<-posting

	if answer.Announcement != nil {
		t.Errorf("announcement = %+v, want none armed while #42's is being posted", answer.Announcement)
	}
}

func TestAnArmedAnnouncementGoesOnceAFixLandsOnANewHead(t *testing.T) {
	t.Parallel()

	// Arrange
	// A check failed on the commit it opened at, and the fix was pushed as a
	// new commit whose run passes.
	world := newForgeWorld()
	handler := serve(t, fixedOnANewHead(openingDeps(world, false)), webhookConfig())
	cancelHeld(t, handler)
	opened(t, handler)
	world.turn(func(w *forgeWorld) { w.head = fixedHead })

	// Act
	held := heldAnnouncement(t, handler)

	// Assert
	if held == nil || held.State != api.QueuedAnnouncementStateAnnounced || len(world.posted()) != 1 {
		t.Errorf("frame's held announcement = %+v, posted %q; want it announced once", held, world.posted())
	}
}

func TestAnArmedAnnouncementTagsTheOwnersAgainstTheBaseItOpenedInto(t *testing.T) {
	t.Parallel()

	// Arrange
	// It opens into release, where carla owns the changes; against the
	// branch's own base nobody does.
	const release = "release"

	world := newForgeWorld()
	deps := openingDeps(world, false)
	newFakeKept().wire(&deps)
	deps.Git.CodeOwnersAt = func(_ context.Context, base string) (codeowners.File, bool, error) {
		if base != release {
			return codeowners.File{}, false, nil
		}

		return codeowners.Parse("internal/ @carla\n", codeowners.GitHub), true, nil
	}
	create := deps.Forge.CreatePullRequest
	deps.Forge.CreatePullRequest = func(ctx context.Context, request forge.NewPullRequest) (forge.PullRequest, error) {
		pull, err := create(ctx, request)
		pull.Base = request.Base

		return pull, err
	}
	handler := serve(t, deps, slackUserConfig())
	body := `{"title":"` + prTitle + `","base":"` + release + `","body":"why"}`

	if recorder := send(t, handler, http.MethodPost, "/api/pull-request", body); recorder.Code != http.StatusOK {
		t.Fatalf("status = %d (%s), want the pull request opened", recorder.Code, recorder.Body.String())
	}

	// Act
	world.turn(func(w *forgeWorld) { w.ci = forge.CIPassed })
	heldAnnouncement(t, handler)

	// Assert
	if posts := world.posted(); len(posts) != 1 || !strings.Contains(posts[0], "<@"+carla().ID+">") {
		t.Errorf("posted %q, want one tagging carla, who owns the changes against release", posts)
	}
}
