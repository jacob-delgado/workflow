// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

import (
	"bufio"
	"context"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/webserver"
	"github.com/jacob-delgado/workflow/internal/webserver/webtest"
)

func TestASwitchServesTheDirectorySwitchedTo(t *testing.T) {
	t.Parallel()

	// Arrange
	var reached []string

	handler := webtest.Serve(t, reachable(repositoryDeps(favoritesKept()), &reached), config.Default())

	// Act
	recorder := webtest.Send(t, handler, http.MethodPut, reposPath+"/here", `{"dir":"`+webRoot+`"}`)

	// Assert
	after := webtest.Decode[api.Repositories](t, webtest.Get(t, handler, reposPath))
	if recorder.Code != http.StatusOK || after.Here.Dir != webRoot || !slices.Equal(reached, []string{webRoot}) {
		t.Errorf("status %d, now in %q, reached %v; want 200 and web served", recorder.Code, after.Here.Dir, reached)
	}
}

func TestASwitchToADirectoryNotThereIsRefused(t *testing.T) {
	t.Parallel()

	// Arrange
	var reached []string

	handler := webtest.Serve(t, reachable(repositoryDeps(favoritesKept()), &reached), config.Default())

	// Act
	recorder := webtest.Send(t, handler, http.MethodPut, reposPath+"/here", `{"dir":"/secret/gone"}`)

	// Assert
	after := webtest.Decode[api.Repositories](t, webtest.Get(t, handler, reposPath))

	named := strings.Contains(recorder.Body.String(), "secret")
	if recorder.Code != http.StatusNotFound || after.Here.Dir != apiCmd || named {
		t.Errorf("status %d, body %s, now in %q; want 404 naming no path, still in api", recorder.Code,
			recorder.Body.String(), after.Here.Dir)
	}
}

func TestASwitchWaitsForNoWriteButIsRefusedWhileOneRuns(t *testing.T) {
	t.Parallel()

	// Arrange
	// A favorite being marked is a write the switch would leave unknown.
	var reached []string

	kept := favoritesKept()
	kept.hold, kept.entered = make(chan struct{}), make(chan struct{})
	handler := webtest.Serve(t, reachable(repositoryDeps(kept), &reached), config.Default())

	writing := make(chan struct{})
	go func() {
		defer close(writing)

		webtest.Send(t, handler, http.MethodPut, reposPath+"/favorites", `{"dir":"/srv/web"}`)
	}()

	<-kept.entered

	// Act
	recorder := webtest.Send(t, handler, http.MethodPut, reposPath+"/here", `{"dir":"`+webRoot+`"}`)

	// Assert
	close(kept.hold)
	<-writing

	if recorder.Code != http.StatusConflict || len(reached) != 0 {
		t.Errorf("status %d, reached %v; want 409 and no switch", recorder.Code, reached)
	}
}

func TestAWriteFromAPageShowingAnotherDirectoryIsRefused(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		shown string
		want  int
	}{
		"the directory the server works in": {shown: apiCmd, want: http.StatusOK},
		"another":                           {shown: webRoot, want: http.StatusConflict},
		// A header carries bytes, not text, so the page escapes the path as a
		// URL path is escaped.
		"escaped":          {shown: "%2Fhome%2Fana%2Fsrc%2Fapi%2Fcmd", want: http.StatusOK},
		"escaped, another": {shown: "%2Fhome%2Fjos%C3%A9", want: http.StatusConflict},
		// One that does not unescape names no directory, so not this one.
		"escaped badly": {shown: "%2Fhome%zz", want: http.StatusConflict},
	}

	for name, page := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			handler := webtest.Serve(t, repositoryDeps(favoritesKept()), config.Default())
			request := httptest.NewRequestWithContext(t.Context(), http.MethodPut, reposPath+"/favorites",
				strings.NewReader(`{"dir":"/srv/web"}`))
			request.Host = webtest.LoopbackHost
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set(webserver.HereHeader, page.shown)

			recorder := httptest.NewRecorder()

			// Act
			handler.ServeHTTP(recorder, request)

			// Assert
			if recorder.Code != page.want {
				t.Errorf("status %d, want %d", recorder.Code, page.want)
			}
		})
	}
}

func TestASwitchEndsTheEventStreamSoThePageReconnects(t *testing.T) {
	t.Parallel()

	// Arrange
	var reached []string

	handler := webtest.Serve(t, reachable(repositoryDeps(favoritesKept()), &reached), config.Default())
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL+"/api/events", nil)
	if err != nil {
		t.Fatal(err)
	}

	response, err := server.Client().Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = response.Body.Close() }()

	lines := bufio.NewScanner(response.Body)
	for lines.Scan() && !strings.HasPrefix(lines.Text(), "data: ") {
		continue
	}

	// Act
	webtest.Send(t, handler, http.MethodPut, reposPath+"/here", `{"dir":"`+webRoot+`"}`)

	// Assert
	var rest []string
	for lines.Scan() {
		rest = append(rest, lines.Text())
	}

	if !slices.Contains(rest, "retry: 1000") {
		t.Errorf("the stream went on with %q, want it to end asking the page to reconnect", rest)
	}
}

func TestASwitchIsToThePathWrittenPlainly(t *testing.T) {
	t.Parallel()

	// Arrange
	// Where the server works is a path written plainly, as a favorite must be.
	var reached []string

	handler := webtest.Serve(t, reachable(repositoryDeps(favoritesKept()), &reached), config.Default())

	// Act
	recorder := webtest.Send(t, handler, http.MethodPut, reposPath+"/here", `{"dir":"/home/ana/src/api/../web/"}`)

	// Assert
	if recorder.Code != http.StatusOK || !slices.Equal(reached, []string{webRoot}) {
		t.Errorf("status %d, reached %v; want %s", recorder.Code, reached, webRoot)
	}
}
