// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/webserver"
)

// The credentials the removal and redaction tests store and type.
const (
	storedJiraToken   = "stored-jira-token-7531"
	typedJiraToken    = "typed-jira-token-8642"
	typedHeaderSecret = "typed-header-secret-9753"
)

// The values the tests set: a Jira address, a timing that is not a duration,
// and a channel added in Settings.
const (
	jiraBaseURL  = "https://jira.example.com"
	notADuration = "never-soon"
	addedChannel = "#announcements"
)

// withValue is body, a configuration's JSON, with the value at a dotted path
// replaced: nil sends null, and a missing value leaves the key out.
func withValue(t *testing.T, body, path string, value any, missing bool) string {
	t.Helper()

	var root map[string]any

	err := json.Unmarshal([]byte(body), &root)
	if err != nil {
		t.Fatalf("reading the body: %v", err)
	}

	section, name, _ := strings.Cut(path, ".")
	inner, _ := root[section].(map[string]any)

	if missing {
		delete(inner, name)
	} else {
		inner[name] = value
	}

	data, err := json.Marshal(root)
	if err != nil {
		t.Fatalf("writing the body: %v", err)
	}

	return string(data)
}

// storedConfig is a configuration with a Jira token, written where a save
// goes.
func storedConfig(t *testing.T) config.Config {
	t.Helper()

	cfg := config.Default()
	cfg.Path = filepath.Join(t.TempDir(), ".workflow.json")
	cfg.Jira.BaseURL = jiraBaseURL
	cfg.Jira.Token = storedJiraToken

	err := config.Save(cfg.Path, cfg)
	if err != nil {
		t.Fatalf("writing the file: %v", err)
	}

	return cfg
}

func TestUpdateConfigRemovesACredentialSentAsNull(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := storedConfig(t)
	body := withValue(t, marshal(t, cfg.Redacted()), "jira.token", nil, false)

	// Act
	recorder := putConfig(t, serve(t, webserver.Deps{}, cfg), body)

	// Assert
	written, err := os.ReadFile(cfg.Path)
	if recorder.Code != http.StatusOK || err != nil || strings.Contains(string(written), storedJiraToken) {
		t.Errorf("status %d (%s), %v; the file still holds the token: %t; want it written without it",
			recorder.Code, recorder.Body.String(), err, strings.Contains(string(written), storedJiraToken))
	}
}

func TestUpdateConfigRefusesABodyLeavingACredentialOut(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := storedConfig(t)
	body := withValue(t, marshal(t, cfg.Redacted()), "jira.token", nil, true)

	// Act
	recorder := putConfig(t, serve(t, webserver.Deps{}, cfg), body)

	// Assert
	saved, _, err := config.LoadLayersAt(config.Files{Home: cfg.Path})
	if recorder.Code != http.StatusBadRequest || err != nil || saved.Jira.Token.Reveal() != storedJiraToken {
		t.Errorf("status %d, %v, token kept %t; want 400 and the token kept: leaving it out says neither "+
			"keep nor remove", recorder.Code, err, saved.Jira.Token.Reveal() == storedJiraToken)
	}
}

func TestACredentialTypedIntoSettingsIsNeverAnsweredUnmasked(t *testing.T) {
	t.Parallel()

	typed := storedConfig(t)
	typed.Jira.Token = typedJiraToken
	typed.Jira.Headers = map[string]config.Secret{"CF-Access-Client-Secret": typedHeaderSecret}

	invalid := typed
	invalid.Timing.CIInterval = notADuration

	cases := map[string]struct {
		body string
		deps webserver.Deps
	}{
		"a save that writes":            {body: marshal(t, typed)},
		"a save refused as not valid":   {body: marshal(t, invalid)},
		"a save refused for its keymap": {body: marshal(t, typed), deps: webserver.Deps{CheckKeys: refuseAMovedCommit}},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			cfg := storedConfig(t)
			body := tt.body

			if tt.deps.CheckKeys != nil {
				body = withValue(t, body, "ui.keys", map[string]string{"commit": "C"}, false)
			}

			handler := serve(t, tt.deps, cfg)

			// Act
			saved := putConfig(t, handler, body)
			read := get(t, handler, "/api/config")

			// Assert
			for _, answer := range []string{saved.Body.String(), read.Body.String()} {
				if strings.Contains(answer, typedJiraToken) || strings.Contains(answer, typedHeaderSecret) ||
					strings.Contains(answer, storedJiraToken) {
					t.Errorf("an answer carries a credential unmasked: %s", answer)
				}
			}
		})
	}
}

func TestAViewAddedInSettingsIsInTheConfigurationTheInterfaceReads(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := storedConfig(t)
	next := cfg.Redacted()
	next.Jira.Views = []config.JiraView{{Name: "Review", JQL: "status = Review"}}

	// Act
	recorder := putConfig(t, serve(t, webserver.Deps{}, cfg), marshal(t, next))

	// Assert
	saved, _, err := config.LoadLayersAt(config.Files{Home: cfg.Path})
	if recorder.Code != http.StatusOK || err != nil || !slices.Contains(saved.Jira.Views, next.Jira.Views[0]) {
		t.Errorf("status %d, %v, views %v; want the added view in the file the interface reads",
			recorder.Code, err, saved.Jira.Views)
	}
}

func TestAChannelAddedInSettingsIsOfferedForTheAnnouncement(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := storedConfig(t)
	next := cfg.Redacted()
	next.Messaging.ClientID, next.Messaging.Channel = "1234.5678", "#dev"
	next.Messaging.Channels = []string{addedChannel}
	handler := serve(t, webserver.Deps{}, cfg)
	putConfig(t, handler, marshal(t, next))

	// Act
	destination := decode[api.MessagingDestination](t, get(t, handler, "/api/messaging"))

	// Assert
	if !slices.Contains(destination.Channels, addedChannel) {
		t.Errorf("channels offered = %v, want the one added in Settings among them", destination.Channels)
	}
}
