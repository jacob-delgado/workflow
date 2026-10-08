// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/cli"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/webserver"
)

// laterToken is a Jira token the server takes up only after it starts — from
// the directory it switches to, or from a save in Settings — and a failure
// then quotes.
const laterToken = "jira-secret-2222"

// errQuotesTheLaterToken is a search failure that quotes laterToken.
var errQuotesTheLaterToken = errors.New("the tracker answered " + laterToken + " oddly")

func TestTheWebServerMasksACredentialOfTheDirectoryItSwitchedTo(t *testing.T) {
	t.Parallel()

	// Arrange
	switched := config.Default()
	switched.Jira.Token = laterToken

	deps := webserver.Deps{
		Reach: func(string) (webserver.World, error) {
			return webserver.World{Deps: failingSearch(errQuotesTheLaterToken), Config: switched}, nil
		},
	}

	// Act
	notes := notesAfter(t, config.Default(), deps, func(base string) {
		status := sendJSON(t, http.MethodPut, base+"/api/repositories/here", `{"dir":"/elsewhere"}`, "")
		if status != http.StatusOK {
			t.Fatalf("the switch answered %d, want 200", status)
		}

		askOnce(t, base+"/api/issues")
	})

	// Assert
	if strings.Contains(notes, laterToken) {
		t.Errorf("the web server said %q, which shows the token of the directory it switched to", notes)
	}

	if !strings.Contains(notes, "workflow web: the tracker answered ****2222 oddly\n") {
		t.Errorf("the web server said %q, want the failure with the switched-to token masked", notes)
	}
}

func TestTheWebServerMasksACredentialSavedInSettings(t *testing.T) {
	t.Parallel()

	// Arrange
	dir := t.TempDir()
	path := filepath.Join(dir, config.FileName)

	err := os.WriteFile(path, []byte(`{"jira":{"token":"`+shownToken+`"}}`), config.FileMode)
	if err != nil {
		t.Fatalf("writing the configuration: %v", err)
	}

	cfg, err := config.Load(dir, t.TempDir())
	if err != nil {
		t.Fatalf("loading the configuration: %v", err)
	}

	// Act
	notes := notesAfter(t, cfg, failingSearch(errQuotesTheLaterToken), func(base string) {
		saveJiraToken(t, base, laterToken)
		askOnce(t, base+"/api/issues")
	})

	// Assert
	if strings.Contains(notes, laterToken) {
		t.Errorf("the web server said %q, which shows the token saved in Settings", notes)
	}

	if !strings.Contains(notes, "workflow web: the tracker answered ****2222 oddly\n") {
		t.Errorf("the web server said %q, want the failure with the saved token masked", notes)
	}
}

// failingSearch is a web server's seams whose search fails with cause.
func failingSearch(cause error) webserver.Deps {
	return webserver.Deps{
		Search: func(string, int) (jira.SearchResult, error) { return jira.SearchResult{}, cause },
	}
}

// notesAfter serves the web API over deps and cfg, has use make its requests
// at the base URL it serves, stops it, and returns what it said.
func notesAfter(t *testing.T, cfg config.Config, deps webserver.Deps, use func(base string)) string {
	t.Helper()

	ctx, cancel := context.WithCancel(t.Context())
	notes := &sharedNotes{}
	done := make(chan error, 1)

	go func() { done <- cli.WebServerAt("127.0.0.1:0")(ctx, cfg, deps, webserver.Info{}, notes) }()

	base := servingAt(t, notes, done)
	awaitServing(t, base)
	use(base)
	cancel()

	err := <-done
	if err != nil {
		t.Fatalf("the web server stopped with %v, want a clean stop", err)
	}

	return notes.String()
}

// saveJiraToken saves token as Jira's in Settings at base: it reads the
// configuration, sets the token, and saves over the revision it read.
func saveJiraToken(t *testing.T, base, token string) {
	t.Helper()

	response, err := getURL(t, base+"/api/config")
	if err != nil {
		t.Fatalf("reading the configuration: %v", err)
	}

	defer func() { _ = response.Body.Close() }()

	var read map[string]any

	err = json.NewDecoder(response.Body).Decode(&read)
	if err != nil {
		t.Fatalf("decoding the configuration: %v", err)
	}

	jiraSettings, _ := read["jira"].(map[string]any)
	jiraSettings["token"] = token

	body, err := json.Marshal(read)
	if err != nil {
		t.Fatalf("encoding the configuration: %v", err)
	}

	status := sendJSON(t, http.MethodPut, base+"/api/config", string(body), response.Header.Get("ETag"))
	if status != http.StatusOK {
		t.Fatalf("saving the configuration answered %d, want 200", status)
	}
}

// sendJSON sends body as JSON to target with method, over the revision etag
// names when it is not empty, and returns the status it was answered with.
func sendJSON(t *testing.T, method, target, body, etag string) int {
	t.Helper()

	request, err := http.NewRequestWithContext(t.Context(), method, target, strings.NewReader(body))
	if err != nil {
		t.Fatalf("building a request for %s: %v", target, err)
	}

	request.Header.Set("Content-Type", "application/json")

	if etag != "" {
		request.Header.Set("If-Match", etag)
	}

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatalf("sending %s %s: %v", method, target, err)
	}

	_, _ = io.Copy(io.Discard, response.Body)
	_ = response.Body.Close()

	return response.StatusCode
}
