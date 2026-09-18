// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package wiring

// The CLI transport translates a forge request into a gh/glab `api` invocation
// and parses the reply, none of which a black-box test can reach through the
// wiring. These drive it with a fake capture, so no gh or glab need be
// installed.

import (
	"context"
	"errors"
	"io"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/proc"
)

// errCommandFailed is a CLI that ran and failed; errBodyRead is a request body
// that cannot be read.
var (
	errCommandFailed = errors.New("gh exploded")
	errBodyRead      = errors.New("body read failed")
)

// failingReader is a request body that errors when read.
type failingReader struct{}

func (failingReader) Read([]byte) (int, error) { return 0, errBodyRead }

// okResponse is a well-formed HTTP response for the fake CLI to hand back.
const okResponse = "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\n\r\n{\"ok\":true}"

// recordingCapture answers with a well-formed response and records the command
// and body it saw.
func recordingCapture() (forgeCapture, *proc.Command, *[]byte) {
	var (
		seen  proc.Command
		input []byte
	)

	capture := func(_ context.Context, program proc.Command, body []byte) ([]byte, error) {
		seen, input = program, body

		return []byte(okResponse), nil
	}

	return capture, &seen, &input
}

// cliRequest is a request for the given method and URL, with an optional body.
func cliRequest(t *testing.T, method, rawURL, body string) *http.Request {
	t.Helper()

	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}

	request, err := http.NewRequestWithContext(t.Context(), method, rawURL, reader)
	if err != nil {
		t.Fatalf("building the request: %v", err)
	}

	return request
}

func TestForgeCLIDoerReadsThroughGH(t *testing.T) {
	t.Parallel()

	// Arrange
	capture, seen, _ := recordingCapture()
	doer := forgeCLIDoer(context.Background(), capture, "gh", "https://api.github.com", forge.KindGitHub)
	request := cliRequest(t, http.MethodGet, "https://api.github.com/repos/ex/repo/pulls", "")

	// Act
	response, err := doer(request)
	if err != nil {
		t.Fatalf("forgeCLIDoer: %v", err)
	}
	defer func() { _ = response.Body.Close() }()

	// Assert
	body, _ := io.ReadAll(response.Body)
	if response.StatusCode != http.StatusOK || string(body) != `{"ok":true}` {
		t.Errorf("response = %d %q, want 200 and the JSON body", response.StatusCode, body)
	}

	if args := seen.Args; seen.Name != "gh" || args[0] != "api" ||
		args[len(args)-1] != "https://api.github.com/repos/ex/repo/pulls" {
		t.Errorf("gh was called as %s %v, want it to read the full URL", seen.Name, args)
	}
}

func TestForgeCLIDoerWritesWithABodyOnStdin(t *testing.T) {
	t.Parallel()

	// Arrange
	capture, seen, input := recordingCapture()
	doer := forgeCLIDoer(context.Background(), capture, "gh", "https://api.github.com", forge.KindGitHub)
	request := cliRequest(t, http.MethodPost, "https://api.github.com/repos/ex/repo/pulls", `{"title":"x"}`)

	// Act
	response, err := doer(request)
	if err != nil {
		t.Fatalf("forgeCLIDoer: %v", err)
	}
	defer func() { _ = response.Body.Close() }()

	// Assert
	if !containsAll(seen.Args, "-X", http.MethodPost, "--input", "-") || string(*input) != `{"title":"x"}` {
		t.Errorf("a write was not sent with its body on stdin: args=%v input=%q", seen.Args, *input)
	}
}

func TestForgeCLIDoerTrimsTheBaseForGitLab(t *testing.T) {
	t.Parallel()

	// Arrange
	capture, seen, _ := recordingCapture()
	doer := forgeCLIDoer(context.Background(), capture, "glab", "https://gitlab.com/api/v4", forge.KindGitLab)
	request := cliRequest(t, http.MethodGet, "https://gitlab.com/api/v4/projects/ex%2Frepo/merge_requests", "")

	// Act
	response, err := doer(request)
	if err != nil {
		t.Fatalf("forgeCLIDoer: %v", err)
	}
	defer func() { _ = response.Body.Close() }()

	// Assert
	if endpoint := seen.Args[len(seen.Args)-1]; endpoint != "projects/ex%2Frepo/merge_requests" {
		t.Errorf("glab endpoint = %q, want the path under its API root", endpoint)
	}
}

func TestForgeCLIDoerReportsAnUnreadableReply(t *testing.T) {
	t.Parallel()

	// Arrange
	capture := func(context.Context, proc.Command, []byte) ([]byte, error) { return []byte("not http"), nil }
	doer := forgeCLIDoer(context.Background(), capture, "gh", "https://api.github.com", forge.KindGitHub)
	request := cliRequest(t, http.MethodGet, "https://api.github.com/x", "")

	// Act
	response, err := doer(request)

	// Assert
	if response != nil {
		_ = response.Body.Close()
	}

	if err == nil || !strings.Contains(err.Error(), "unreadable response") {
		t.Errorf("forgeCLIDoer returned %v, want an unreadable-response error", err)
	}
}

func TestForgeCLIDoerReportsTheCommandFailure(t *testing.T) {
	t.Parallel()

	// Arrange
	capture := func(context.Context, proc.Command, []byte) ([]byte, error) {
		return []byte("boom"), errCommandFailed
	}
	doer := forgeCLIDoer(context.Background(), capture, "gh", "https://api.github.com", forge.KindGitHub)
	request := cliRequest(t, http.MethodGet, "https://api.github.com/x", "")

	// Act
	response, err := doer(request)

	// Assert
	if response != nil {
		_ = response.Body.Close()
	}

	if !errors.Is(err, errCommandFailed) {
		t.Errorf("forgeCLIDoer returned %v, want the command failure", err)
	}
}

func TestForgeCLIDoerReportsABodyThatCannotBeRead(t *testing.T) {
	t.Parallel()

	// Arrange
	capture, _, _ := recordingCapture()
	doer := forgeCLIDoer(context.Background(), capture, "gh", "https://api.github.com", forge.KindGitHub)
	request := cliRequest(t, http.MethodPost, "https://api.github.com/x", "")
	request.Body = io.NopCloser(failingReader{})

	// Act
	response, err := doer(request)

	// Assert
	if response != nil {
		_ = response.Body.Close()
	}

	if err == nil || !strings.Contains(err.Error(), "reading the request body") {
		t.Errorf("forgeCLIDoer returned %v, want a body-read error", err)
	}
}

func TestForgeProgramForEachForge(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		kind forge.Kind
		want string
		ok   bool
	}{
		"GitHub":  {kind: forge.KindGitHub, want: "gh", ok: true},
		"GitLab":  {kind: forge.KindGitLab, want: "glab", ok: true},
		"unknown": {kind: forge.KindUnknown, want: "", ok: false},
		"unnamed": {kind: forge.Kind(99), want: "", ok: false},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			program, ok := forgeProgram(tt.kind)

			// Assert
			if program != tt.want || ok != tt.ok {
				t.Errorf("forgeProgram(%v) = %q, %v, want %q, %v", tt.kind, program, ok, tt.want, tt.ok)
			}
		})
	}
}

func TestForgeTransportChoosesTheTransport(t *testing.T) {
	t.Parallel()

	github := forge.Repo{Kind: forge.KindGitHub}
	present := func(string) bool { return true }
	absent := func(string) bool { return false }

	cases := map[string]struct {
		settings  config.Forge
		repo      forge.Repo
		available func(string) bool
		wantCLI   bool
	}{
		"HTTP unless asked":       {settings: config.Forge{}, repo: github, available: present, wantCLI: false},
		"CLI when asked and here": {settings: config.Forge{CLI: true}, repo: github, available: present, wantCLI: true},
		"HTTP when the tool is missing": {
			settings: config.Forge{CLI: true}, repo: github, available: absent, wantCLI: false,
		},
		"HTTP for a forge with no CLI": {
			settings: config.Forge{CLI: true}, repo: forge.Repo{Kind: forge.KindUnknown}, available: present, wantCLI: false,
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			doer, usingCLI := forgeTransport(
				context.Background(), tt.settings, tt.repo, "https://api.github.com", RequestTimeout, tt.available)

			// Assert
			if usingCLI != tt.wantCLI || doer == nil {
				t.Errorf("forgeTransport chose CLI=%v (doer nil: %v), want CLI=%v", usingCLI, doer == nil, tt.wantCLI)
			}
		})
	}
}

// containsAll reports whether values all appear in args.
func containsAll(args []string, values ...string) bool {
	for _, value := range values {
		if !slices.Contains(args, value) {
			return false
		}
	}

	return true
}
