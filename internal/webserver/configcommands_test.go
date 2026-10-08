// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/webserver"
)

// fileRunningAsYou is a configuration file that names what workflow runs, or
// reads the environment by, as you: the program that prints the Jira token,
// the variable that holds it, and the task program.
const fileRunningAsYou = `{"jira": {"token_command": "pass show jira", "token_env": "JIRA_TOKEN"},
"taskwarrior": {"program": "task"}}`

// readToSave reads the configuration from a server over a file holding
// fileRunningAsYou, as Settings does before a save: the server, the file's
// path, what the read served, and the revision it named.
func readToSave(t *testing.T) (http.Handler, string, api.Config, string) {
	t.Helper()

	path := filepath.Join(t.TempDir(), config.FileName)
	rewrite(t, path, fileRunningAsYou)

	cfg, err := config.LoadFile(path)
	if err != nil {
		t.Fatalf("loading the file: %v", err)
	}

	handler := serve(t, webserver.Deps{}, cfg)
	read := get(t, handler, "/api/config")

	return handler, path, decode[api.Config](t, read), etagOf(read)
}

// body is cfg as the request body a save sends.
func body(t *testing.T, cfg api.Config) string {
	t.Helper()

	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshaling the save: %v", err)
	}

	return string(data)
}

func TestASaveCannotChangeWhatWorkflowRunsAsYou(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		setting string
		change  func(*api.Config)
	}{
		"the token command, changed": {
			setting: "jira.token_command",
			change:  func(cfg *api.Config) { cfg.Jira.TokenCommand = new("pass show other") },
		},
		"the token command, emptied": {
			setting: "jira.token_command",
			change:  func(cfg *api.Config) { cfg.Jira.TokenCommand = new("") },
		},
		"the token variable, changed": {
			setting: "jira.token_env",
			change:  func(cfg *api.Config) { cfg.Jira.TokenEnv = new("OTHER_TOKEN") },
		},
		"the task program, changed": {
			setting: "taskwarrior.program",
			change:  func(cfg *api.Config) { cfg.Taskwarrior.Program = new("/opt/other/task") },
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			handler, path, read, etag := readToSave(t)
			tt.change(&read)

			// Act
			recorder := putConfigOver(t, handler, body(t, read), etag)

			// Assert
			if recorder.Code != http.StatusUnprocessableEntity {
				t.Fatalf("status = %d, want 422: %s", recorder.Code, recorder.Body.String())
			}

			if got := decode[api.Problem](t, recorder); !strings.HasPrefix(got.Detail, tt.setting+" ") {
				t.Errorf("detail = %q, want it to name %s", got.Detail, tt.setting)
			}

			if got := onDisk(t, path); got != fileRunningAsYou {
				t.Errorf("the file holds %q, want it as it was", got)
			}
		})
	}
}

func TestASaveLeavingOutWhatWorkflowRunsAsYouKeepsIt(t *testing.T) {
	t.Parallel()

	// Arrange
	handler, path, read, etag := readToSave(t)
	read.Jira.TokenCommand, read.Jira.TokenEnv, read.Taskwarrior.Program = nil, nil, nil
	read.Jira.Project = new("PROJ")

	// Act
	recorder := putConfigOver(t, handler, body(t, read), etag)

	// Assert
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", recorder.Code, recorder.Body.String())
	}

	saved, err := config.LoadFile(path)
	if err != nil {
		t.Fatalf("reading the saved file: %v", err)
	}

	jira, task := saved.Jira, saved.Taskwarrior
	if jira.Project != "PROJ" || jira.TokenCommand != "pass show jira" || jira.TokenEnv != "JIRA_TOKEN" ||
		task.Program != "task" {
		t.Errorf("saved jira %+v and taskwarrior %+v, want the project written and the rest kept", jira, task)
	}
}
