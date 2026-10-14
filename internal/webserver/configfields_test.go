// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

import (
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/webserver"
)

// refusedFields are a problem's refused settings, none when it names none.
func refusedFields(failure api.Problem) []api.RefusedSetting {
	if failure.Fields == nil {
		return nil
	}

	return *failure.Fields
}

func TestARefusedSaveNamesEachSettingItRefused(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := config.Default()
	cfg.Path = filepath.Join(t.TempDir(), ".workflow.json")

	bad := config.Default()
	bad.Timing.RequestTimeout = "later"
	bad.Commit.RefsTrailer = "Closes:"

	// Act
	recorder := putConfig(t, serve(t, webserver.Deps{}, cfg), marshal(t, bad))

	// Assert
	fields := refusedFields(decode[api.Problem](t, recorder))
	if recorder.Code != http.StatusUnprocessableEntity || len(fields) != 2 ||
		fields[0].Key != "timing.request_timeout" || !strings.Contains(fields[0].Detail, `"later"`) ||
		fields[1].Key != "commit.refs_trailer" || !strings.Contains(fields[1].Detail, `"Closes:"`) {
		t.Errorf("status %d, fields %+v; want 422 naming the timeout, then the trailer, each with why", recorder.Code, fields)
	}
}

func TestARefusedSettingNeverCarriesASecret(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := config.Default()
	cfg.Path = filepath.Join(t.TempDir(), ".workflow.json")
	posted := cfg
	posted.Jira.BaseURL = "https://user:s3cret@jira.example.com"
	posted.Jira.Token = "jira-token-wxyz"

	// Act
	recorder := putConfig(t, serve(t, webserver.Deps{}, cfg), marshal(t, posted))

	// Assert
	fields := refusedFields(decode[api.Problem](t, recorder))
	if len(fields) != 1 || fields[0].Key != "jira.base_url" ||
		strings.Contains(recorder.Body.String(), "s3cret") || strings.Contains(recorder.Body.String(), "wxyz") {
		t.Errorf("fields %+v, body %s; want jira.base_url refused with no secret", fields, recorder.Body.String())
	}
}
