// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package doctor_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jacob-delgado/workflow/internal/cli/clitest"
	"github.com/jacob-delgado/workflow/internal/cli/doctor"
)

// stallingMidAnswer begins a JSON answer and sends nothing more until the
// client goes.
func stallingMidAnswer(writer http.ResponseWriter, request *http.Request) {
	writer.Header().Set("Content-Type", "application/json")
	_, _ = writer.Write([]byte(`{"name":`))

	if flusher, ok := writer.(http.Flusher); ok {
		flusher.Flush()
	}

	<-request.Context().Done()
}

func TestDoctorOnlineCountsAnAnswerNeverFinishedAsUnreachable(t *testing.T) {
	t.Parallel()

	// Arrange
	// Jira begins its answer and never finishes it, which says nothing of the
	// credential: it may answer in full another time.
	server := httptest.NewServer(http.HandlerFunc(stallingMidAnswer))
	t.Cleanup(server.Close)

	dir := t.TempDir()
	clitest.WriteFile(t, dir, `{"jira": {"base_url": "`+server.URL+`", "token": "t"}, `+clitest.SlackWebhook+
		`, "timing": {"request_timeout": "1s"}}`)

	// Act
	output, err := clitest.Run(t, dir, "doctor", "--online")

	// Assert
	if !errors.Is(err, doctor.ErrUnreachable) || errors.Is(err, doctor.ErrCredentialRejected) {
		t.Errorf("doctor --online = %v, want the unfinished answer told as unreachable, not rejected:\n%s", err, output)
	}

	wantExit(t, err, 5)
}
