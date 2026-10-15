// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package doctor_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/cli/clitest"
)

// slowJira answers Jira's myself after a second, long enough for the live
// line to be drawn while doctor waits.
func slowJira(writer http.ResponseWriter, request *http.Request) {
	select {
	case <-time.After(time.Second):
	case <-request.Context().Done():
		return
	}

	writer.Header().Set("Content-Type", "application/json")
	_, _ = writer.Write([]byte(clitest.JiraFixture))
}

// checkingJira is the live line's subject while doctor asks Jira, the name
// ended where the line goes on.
var checkingJira = regexp.MustCompile(`Checking Jira[^\pL]`)

func TestDoctorOnlineNamesTheServiceItIsAskingAndHowFarItHasGot(t *testing.T) {
	t.Parallel()

	// Arrange
	server := httptest.NewServer(http.HandlerFunc(slowJira))
	t.Cleanup(server.Close)

	dir := t.TempDir()
	clitest.WriteConfigFor(t, dir, server.URL)

	prompt := clitest.UnusedPrompt(t)
	prompt.IsTerminal = func(io.Writer) bool { return true }

	// Act
	printed, _ := clitest.RunStreams(t, dir, prompt, "doctor", "--online")

	// Assert
	// Jira is the first of three checks, and none is done while it is asked.
	if !checkingJira.MatchString(printed.Stderr) || !strings.Contains(printed.Stderr, "0 of 3") {
		t.Errorf("doctor's live line = %q, want it checking Jira, 0 of 3", printed.Stderr)
	}
}
