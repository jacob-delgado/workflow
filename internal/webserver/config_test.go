// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

import (
	"encoding/json"
	"errors"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/webserver"
)

// marshal renders a configuration as the request body a client would send.
func marshal(t *testing.T, cfg config.Config) string {
	t.Helper()

	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshaling the config body: %v", err)
	}

	return string(data)
}

// putConfig saves body as Settings does: it reads the configuration first, and
// saves over the revision that read returned.
func putConfig(t *testing.T, handler http.Handler, body string) *httptest.ResponseRecorder {
	t.Helper()

	return putConfigOver(t, handler, body, get(t, handler, "/api/config").Header().Get("ETag"))
}

// putConfigOver saves body over the revision etag names, sent as If-Match; an
// empty etag sends none.
func putConfigOver(t *testing.T, handler http.Handler, body, etag string) *httptest.ResponseRecorder {
	t.Helper()

	request := httptest.NewRequestWithContext(t.Context(), http.MethodPut, "/api/config", strings.NewReader(body))
	request.Host = loopbackHost
	request.Header.Set("Content-Type", "application/json")

	if etag != "" {
		request.Header.Set("If-Match", etag)
	}

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	return recorder
}

func TestGetConfigMasksSecrets(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := config.Default()
	cfg.Jira.BaseURL = "https://jira.example.com"
	cfg.Jira.Token = "jira-secret-abcd"

	// Act
	out := decode[api.Config](t, get(t, serve(t, webserver.Deps{}, cfg), "/api/config"))

	// Assert
	if out.Jira.Token == nil || !strings.HasPrefix(*out.Jira.Token, "****") {
		t.Errorf("jira.token = %v, want masked", out.Jira.Token)
	}

	if out.Jira.BaseURL == nil || *out.Jira.BaseURL != "https://jira.example.com" {
		t.Errorf("jira.base_url = %v, want the value in the clear", out.Jira.BaseURL)
	}
}

func TestUpdateConfigWritesTheFile(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := config.Default()
	cfg.Path = filepath.Join(t.TempDir(), ".workflow.json")

	next := config.Default()
	next.Jira.BaseURL = "https://new.example.com"

	// Act
	recorder := putConfig(t, serve(t, webserver.Deps{}, cfg), marshal(t, next))

	// Assert
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", recorder.Code, recorder.Body.String())
	}

	saved, _, err := config.LoadLayersAt(config.Files{Home: cfg.Path})
	if err != nil {
		t.Fatalf("reading the saved file: %v", err)
	}

	if saved.Jira.BaseURL != "https://new.example.com" {
		t.Errorf("saved jira.base_url = %q, want the written value", saved.Jira.BaseURL)
	}
}

// layeredConfig is a configuration read from a home file and a repository's
// file over it, as the server reads one working in a repository.
func layeredConfig(t *testing.T) config.Config {
	t.Helper()

	home, repo := filepath.Join(t.TempDir(), config.FileName), filepath.Join(t.TempDir(), config.FileName)

	for path, contents := range map[string]string{home: `{}`, repo: `{"jira": {"project": "OSS"}}`} {
		err := os.WriteFile(path, []byte(contents), config.FileMode)
		if err != nil {
			t.Fatalf("writing %s: %v", path, err)
		}
	}

	cfg, _, err := config.LoadLayersAt(config.Files{Home: home, Repo: repo})
	if err != nil {
		t.Fatalf("reading the layers: %v", err)
	}

	return cfg
}

func TestUpdateConfigKeepsWhatOnlyTheHomeFileMayHoldOutOfTheRepositoryFile(t *testing.T) {
	t.Parallel()

	const typedToken = "forge-token-typed-in-settings"

	// A program to run is one a save never changes at all, so it is refused
	// naming the setting before the repository's file is considered.
	cases := map[string]struct {
		change func(*config.Config)
		want   string
	}{
		"a typed credential": {func(cfg *config.Config) { cfg.Forge.Token = typedToken }, "home"},
		"a program to run": {
			func(cfg *config.Config) { cfg.Taskwarrior.Program = "/opt/homebrew/bin/task" }, "taskwarrior.program",
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			cfg := layeredConfig(t)
			next := cfg
			tt.change(&next)

			// Act
			recorder := putConfig(t, serve(t, webserver.Deps{}, cfg), marshal(t, next))

			// Assert
			failure := decode[api.Problem](t, recorder)
			if recorder.Code != http.StatusUnprocessableEntity || failure.Code != api.ProblemCodeUnprocessable ||
				!strings.Contains(failure.Detail, tt.want) || strings.Contains(failure.Detail, typedToken) {
				t.Errorf("status %d, problem %+v; want 422 naming %q, without the token",
					recorder.Code, failure, tt.want)
			}
		})
	}
}

func TestUpdateConfigRejectsAnInvalidConfig(t *testing.T) {
	t.Parallel()

	// The refusal names the setting or the value refused, and never a
	// credential the posted configuration carries beside it.
	const jiraToken = "jira-token-not-for-the-page"

	cases := map[string]struct {
		refuse func(*config.Config)
		want   string
	}{
		"a timing that is not a duration": {func(c *config.Config) { c.Timing.RequestTimeout = "soon" }, `"soon"`},
		"a refs trailer with a colon":     {func(c *config.Config) { c.Commit.RefsTrailer = "Refs:" }, "refs_trailer"},
		"an unknown title source":         {func(c *config.Config) { c.PullRequest.TitleSource = "branch" }, "title_source"},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			cfg := config.Default()
			cfg.Path = filepath.Join(t.TempDir(), ".workflow.json")

			bad := config.Default()
			bad.Jira.Token = jiraToken
			bad.Messaging.WebhookURL = config.Secret("https://hooks.slack.com/services/" + webhookSecret)
			tt.refuse(&bad)

			// Act
			recorder := putConfig(t, serve(t, webserver.Deps{}, cfg), marshal(t, bad))

			// Assert
			failure := decode[api.Problem](t, recorder)
			if recorder.Code != http.StatusUnprocessableEntity || failure.Code != api.ProblemCodeUnprocessable {
				t.Errorf("status = %d, code %q; want 422 and unprocessable", recorder.Code, failure.Code)
			}

			if !strings.Contains(failure.Detail, tt.want) {
				t.Errorf("detail = %q, want it to name %s", failure.Detail, tt.want)
			}

			if body := recorder.Body.String(); strings.Contains(body, jiraToken) || strings.Contains(body, webhookSecret) {
				t.Errorf("body = %q, carries a credential the configuration holds", body)
			}
		})
	}
}

func TestUpdateConfigNamesEveryRefusalOnOneLine(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := config.Default()
	cfg.Path = filepath.Join(t.TempDir(), ".workflow.json")

	bad := config.Default()
	bad.Timing.RequestTimeout = "soon"
	bad.Commit.RefsTrailer = "Refs:"

	// Act
	recorder := putConfig(t, serve(t, webserver.Deps{}, cfg), marshal(t, bad))

	// Assert
	// The detail names no file, since a posted configuration is not one yet,
	// and joins the timing's reason to the trailer's on the one line.
	detail := decode[api.Problem](t, recorder).Detail
	if !strings.HasPrefix(detail, "the configuration is not valid: ") ||
		strings.Contains(detail, config.ErrInvalid.Error()) || strings.Contains(detail, "\n") {
		t.Errorf("detail = %q, want one line after the refusal, naming no file", detail)
	}

	if !strings.Contains(detail, `"soon"; invalid commit default`) {
		t.Errorf("detail = %q, want the timing's reason and the trailer's joined by \"; \"", detail)
	}
}

func TestUpdateConfigRefusesABaseURLCarryingALogin(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := config.Default()
	cfg.Path = filepath.Join(t.TempDir(), ".workflow.json")
	posted := cfg
	posted.Jira.BaseURL = "https://user:s3cret@jira.example.com"

	// Act
	recorder := putConfig(t, serve(t, webserver.Deps{}, cfg), marshal(t, posted))

	// Assert
	if recorder.Code != http.StatusUnprocessableEntity || strings.Contains(recorder.Body.String(), "s3cret") {
		t.Errorf("status = %d, body %s; want 422 without the password", recorder.Code, recorder.Body.String())
	}
}

func TestUpdateConfigKeepsAMaskedSecret(t *testing.T) {
	t.Parallel()

	// Arrange
	// The config holds a real token; a client edits and sends back the masked
	// form the read returned. The stored secret must survive.
	cfg := config.Default()
	cfg.Path = filepath.Join(t.TempDir(), ".workflow.json")
	cfg.Jira.Token = "real-secret-wxyz"

	// Act
	recorder := putConfig(t, serve(t, webserver.Deps{}, cfg), marshal(t, cfg.Redacted()))

	// Assert
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", recorder.Code, recorder.Body.String())
	}

	saved, _, err := config.LoadLayersAt(config.Files{Home: cfg.Path})
	if err != nil {
		t.Fatalf("reading the saved file: %v", err)
	}

	if saved.Jira.Token.Reveal() != "real-secret-wxyz" {
		t.Errorf("saved jira.token = %q, want the stored secret kept, not the mask", saved.Jira.Token.Reveal())
	}
}

func TestUpdateConfigSetsANewSecret(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := config.Default()
	cfg.Path = filepath.Join(t.TempDir(), ".workflow.json")
	cfg.Jira.Token = "old-secret-0000"

	next := cfg
	next.Jira.Token = "brand-new-secret-1111" // a real new value, neither empty nor the mask

	// Act
	recorder := putConfig(t, serve(t, webserver.Deps{}, cfg), marshal(t, next))

	// Assert
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", recorder.Code, recorder.Body.String())
	}

	saved, _, err := config.LoadLayersAt(config.Files{Home: cfg.Path})
	if err != nil {
		t.Fatalf("reading the saved file: %v", err)
	}

	if saved.Jira.Token.Reveal() != "brand-new-secret-1111" {
		t.Errorf("saved token = %q, want the new value written", saved.Jira.Token.Reveal())
	}
}

func TestUpdateConfigResolvesHeaders(t *testing.T) {
	t.Parallel()

	// Arrange
	// One header comes back masked (keep the stored value); one comes back with a
	// new value (replace it).
	cfg := config.Default()
	cfg.Path = filepath.Join(t.TempDir(), ".workflow.json")
	cfg.Jira.Headers = map[string]config.Secret{"CF-Id": "stored-id-secret", "CF-Team": "stored-team"}

	next := cfg
	next.Jira.Headers = map[string]config.Secret{
		"CF-Id": config.Secret(config.Redact("stored-id-secret")), "CF-Team": "new-team",
	}

	// Act
	recorder := putConfig(t, serve(t, webserver.Deps{}, cfg), marshal(t, next))

	// Assert
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", recorder.Code, recorder.Body.String())
	}

	saved, _, err := config.LoadLayersAt(config.Files{Home: cfg.Path})
	if err != nil {
		t.Fatalf("reading the saved file: %v", err)
	}

	if got := saved.Jira.Headers["CF-Id"].Reveal(); got != "stored-id-secret" {
		t.Errorf("CF-Id = %q, want the stored value kept behind the mask", got)
	}

	if got := saved.Jira.Headers["CF-Team"].Reveal(); got != "new-team" {
		t.Errorf("CF-Team = %q, want the new value written", got)
	}
}

func TestUpdateConfigReportsASaveFailure(t *testing.T) {
	t.Parallel()

	// Arrange
	// A path whose parent directory does not exist cannot be written.
	cfg := config.Default()
	cfg.Path = filepath.Join(t.TempDir(), "missing", ".workflow.json")

	// Act
	recorder := putConfig(t, serve(t, webserver.Deps{}, cfg), marshal(t, config.Default()))

	// Assert
	if recorder.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500 when the file cannot be written", recorder.Code)
	}
}

func TestUpdateConfigRefusesAUIValueOutsideTheContract(t *testing.T) {
	t.Parallel()

	cases := map[string]config.UI{
		"a negative comments_shown": {Mouse: true, CommentsShown: -1},
		"a misspelled color":        {Mouse: true, Color: "nevr"},
	}

	for name, outOfContract := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			cfg := config.Default()
			cfg.Path = filepath.Join(t.TempDir(), ".workflow.json")

			bad := config.Default()
			bad.UI = outOfContract

			// Act
			recorder := putConfig(t, serve(t, webserver.Deps{}, cfg), marshal(t, bad))

			// Assert
			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400: %s", recorder.Code, recorder.Body.String())
			}

			if failure := decode[api.Problem](t, recorder); failure.Code != api.ProblemCodeBadRequest {
				t.Errorf("code = %q, want bad_request", failure.Code)
			}
		})
	}
}

// errCommitOnTaken is a keymap check's refusal, in the words the terminal
// interface's own check uses.
var errCommitOnTaken = errors.New(`ui.keys binds two actions to one key in the same context: ` +
	`stage and commit both bind "space" in the Branch and Commits panes`)

// refuseAMovedCommit is a keymap check that refuses a map moving commit and
// accepts any other.
func refuseAMovedCommit(keys map[string]string) error {
	if _, moved := keys["commit"]; moved {
		return errCommitOnTaken
	}

	return nil
}

func TestUpdateConfigRefusesAKeymapTheInterfaceWouldRefuse(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := config.Default()
	cfg.Path = filepath.Join(t.TempDir(), ".workflow.json")

	next := config.Default()
	next.UI.Keys = map[string]string{"commit": "space"}

	handler := serve(t, webserver.Deps{CheckKeys: refuseAMovedCommit}, cfg)

	// Act
	recorder := putConfig(t, handler, marshal(t, next))

	// Assert
	if recorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422: %s", recorder.Code, recorder.Body.String())
	}

	failure := decode[api.Problem](t, recorder)
	if failure.Code != api.ProblemCodeUnprocessable || !strings.Contains(failure.Detail, errCommitOnTaken.Error()) {
		t.Errorf("problem = %q saying %q, want unprocessable saying %q", failure.Code, failure.Detail, errCommitOnTaken)
	}

	_, err := os.Stat(cfg.Path)
	if !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("stat of the configuration file = %v, want it never written", err)
	}
}

func TestUpdateConfigSavesAKeymapTheInterfaceAccepts(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := config.Default()
	cfg.Path = filepath.Join(t.TempDir(), ".workflow.json")

	next := config.Default()
	next.UI.Keys = map[string]string{"comment": "ctrl+e"}

	handler := serve(t, webserver.Deps{CheckKeys: refuseAMovedCommit}, cfg)

	// Act
	recorder := putConfig(t, handler, marshal(t, next))

	// Assert
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", recorder.Code, recorder.Body.String())
	}

	saved, _, err := config.LoadLayersAt(config.Files{Home: cfg.Path})
	if err != nil {
		t.Fatalf("reading the saved file: %v", err)
	}

	if saved.UI.Keys["comment"] != "ctrl+e" {
		t.Errorf("saved ui.keys = %v, want comment on ctrl+e", saved.UI.Keys)
	}
}

// taskwarriorProgram is a taskwarrior.program a user might set.
const taskwarriorProgram = "/opt/homebrew/bin/task"

func TestTheWebConfigRoundTripKeepsTaskwarrior(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := config.Default()
	cfg.Path = filepath.Join(t.TempDir(), ".workflow.json")
	cfg.Taskwarrior.Program = taskwarriorProgram

	handler := serve(t, webserver.Deps{}, cfg)

	// Act: read the configuration
	response := get(t, handler, "/api/config")
	read := decode[api.Config](t, response)

	// Assert: it carries the taskwarrior section
	if read.Taskwarrior.Program == nil || *read.Taskwarrior.Program != taskwarriorProgram {
		t.Fatalf("the read served %s, want taskwarrior.program set", response.Body.String())
	}

	// Act: save what the read served, with taskwarrior turned off
	disabled := true
	read.Taskwarrior.Disabled = &disabled

	body, err := json.Marshal(read)
	if err != nil {
		t.Fatalf("marshaling the read back: %v", err)
	}

	recorder := putConfigOver(t, handler, string(body), etagOf(response))

	// Assert: the file holds the change, and the program the read carried
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", recorder.Code, recorder.Body.String())
	}

	saved, _, err := config.LoadLayersAt(config.Files{Home: cfg.Path})
	if err != nil {
		t.Fatalf("reading the saved file: %v", err)
	}

	if saved.Taskwarrior.Program != taskwarriorProgram || !saved.Taskwarrior.Disabled {
		t.Errorf("saved taskwarrior = %+v, want the program kept and disabled written", saved.Taskwarrior)
	}
}
