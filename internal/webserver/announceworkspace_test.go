// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

// Links are kept per Slack workspace, so a token whose workspace Slack will
// not name tags nobody: the preview says why, and People and groups is
// refused with why.

import (
	"net/http"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/messaging"
	"github.com/jacob-delgado/workflow/internal/webserver"
)

// unknownWorkspace is the note an announcement that cannot tell its
// workspace carries.
const unknownWorkspace = "can't tell which Slack workspace this token is for"

func TestGetAnnouncementSaysWhyItTagsNoOneInAnUnknownWorkspace(t *testing.T) {
	t.Parallel()

	// Arrange
	fake := newFakeKept()
	fake.workspaceErr = messaging.ErrNoWorkspace
	handler := keptServer(t, fake, webserver.Info{Version: testVersion})

	// Act
	recorder := get(t, handler, "/api/announcement")

	// Assert
	tagging := decode[api.Announcement](t, recorder).Tagging
	if tagging == nil || tagging.Available || len(tagging.Owners)+len(tagging.Groups) != 0 {
		t.Fatalf("tagging = %+v, want it unavailable, proposing no one", tagging)
	}

	if reason := tagging.UnavailableReason; reason == nil || !strings.HasPrefix(*reason, unknownWorkspace) {
		t.Errorf("unavailable_reason = %v, want it to say %q", reason, unknownWorkspace)
	}
}

func TestPeopleIsRefusedWithWhyInAnUnknownWorkspace(t *testing.T) {
	t.Parallel()

	// Arrange
	fake := newFakeKept()
	fake.workspaceErr = messaging.ErrNoWorkspace
	handler := keptServer(t, fake, webserver.Info{Version: testVersion})

	// Act
	recorder := get(t, handler, peoplePath)

	// Assert
	if detail := decode[api.Problem](t, recorder).Detail; recorder.Code != http.StatusUnprocessableEntity ||
		!strings.Contains(detail, unknownWorkspace) {
		t.Errorf("status %d, detail %q; want 422 saying %q", recorder.Code, detail, unknownWorkspace)
	}
}
