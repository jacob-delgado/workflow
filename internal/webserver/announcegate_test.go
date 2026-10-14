// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/httpx"
	"github.com/jacob-delgado/workflow/internal/webserver"
)

// errForgeDown is why a forge could not be reached, as a transport says it.
var errForgeDown = errors.New("connection refused")

// gatedDeps are filled deps over memory, posting nowhere but counting posts,
// whose CI reads answer reported and err.
func gatedDeps(memory *announceMemory, posts *int, reported forge.CI, err error) webserver.Deps {
	deps := memory.wire(filledDeps())
	deps.Messaging.Post = func(context.Context, string, string) error {
		*posts++

		return nil
	}
	deps.Forge.CheckStatus = func(context.Context, forge.PullRequest, string) (forge.CI, error) { return reported, err }

	return deps
}

func TestAnnouncingNowWithAFailedCheckAnswersCIFailed(t *testing.T) {
	t.Parallel()

	// Arrange
	memory, posts := &announceMemory{}, 0
	deps := gatedDeps(memory, &posts, forge.CI{State: forge.CIFailed}, nil)

	// Act
	recorder := postAnnounce(t, serve(t, deps, config.Default()), map[string]string{channelField: ""})

	// Assert
	failure := decode[api.Problem](t, recorder)
	if recorder.Code != http.StatusConflict || failure.Code != api.ProblemCodeCiFailed ||
		!strings.Contains(failure.Detail, "a check failed on #42") {
		t.Errorf("status %d, problem %+v; want 409 ci_failed naming #42", recorder.Code, failure)
	}

	if posts != 0 || len(memory.recorded) != 0 {
		t.Errorf("posted %d, recorded %+v; want nothing posted or recorded", posts, memory.recorded)
	}
}

func TestAnnouncingNowWhenCICannotBeReadPostsNothing(t *testing.T) {
	t.Parallel()

	// Arrange
	apiBase := "https://" + forgeHost + "/api/v3"
	unreachable := httpx.Unreachable(forge.ErrUnreachable, apiBase, errForgeDown)
	memory, posts := &announceMemory{}, 0
	deps := gatedDeps(memory, &posts, forge.CI{}, unreachable)

	// Act
	recorder := postAnnounce(t, serve(t, deps, config.Default()), map[string]string{channelField: ""})

	// Assert
	failure := decode[api.Problem](t, recorder)
	if recorder.Code != http.StatusBadGateway || failure.Code != api.ProblemCodeUnreachable ||
		strings.Contains(failure.Detail, forgeHost) {
		t.Errorf("status %d, problem %+v; want 502 unreachable naming no host", recorder.Code, failure)
	}

	if posts != 0 {
		t.Errorf("posted %d, want nothing posted when CI cannot be read", posts)
	}
}

func TestAnnouncingAMergeACheckFailedOnSaysToReRunTheChecks(t *testing.T) {
	t.Parallel()

	// Arrange
	// A merge cannot wait for CI, so it is not pointed at the wait.
	world := newForgeWorld()
	world.state, world.ci = forge.StateMerged, forge.CIFailed
	handler := serve(t, world.deps(), config.Default())

	// Act
	recorder := postAnnounce(t, handler, map[string]string{channelField: slackChannel})

	// Assert
	failure := decode[api.Problem](t, recorder)
	want := "a check failed on #42, so its merge was not announced; " +
		"re-run the failed checks on the forge, then announce it once they pass"

	if recorder.Code != http.StatusConflict || failure.Code != api.ProblemCodeCiFailed || failure.Detail != want {
		t.Errorf("status %d, problem %+v; want 409 ci_failed saying %q", recorder.Code, failure, want)
	}
}
