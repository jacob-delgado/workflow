// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package jira_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/jacob-delgado/workflow/internal/jira"
)

func TestAddWorklogPostsTheDurationAndNoteAndReturnsTheWorklog(t *testing.T) {
	t.Parallel()

	// Arrange
	var (
		requested   atomic.Value
		sentTime    atomic.Value
		sentComment atomic.Value
	)

	client := serve(t, func(writer http.ResponseWriter, request *http.Request) {
		requested.Store(request.Method + " " + request.URL.EscapedPath() + " " + request.Header.Get("Content-Type"))

		var body struct {
			TimeSpent string `json:"timeSpent"` //nolint:tagliatelle // Jira's field name on the wire, not ours to pick
			Comment   string `json:"comment"`
		}

		_ = json.NewDecoder(request.Body).Decode(&body)
		sentTime.Store(body.TimeSpent)
		sentComment.Store(body.Comment)

		writer.Header().Set("X-Ausername", "fred")
		writer.WriteHeader(http.StatusCreated)
		_, _ = writer.Write([]byte(`{"id":"10101","timeSpent":"2h"}`))
	})

	// Act
	worklog, err := client.AddWorklog(t.Context(), "OPS/1", "2h", "chased the flake")
	if err != nil {
		t.Fatalf("AddWorklog returned %v, want nil", err)
	}

	// Assert
	if got := requested.Load(); got != "POST /rest/api/2/issue/OPS%2F1/worklog "+jsonMediaType {
		t.Errorf("requested %v, want a JSON POST to the escaped issue's worklog", got)
	}

	if got := sentTime.Load(); got != "2h" {
		t.Errorf("sent timeSpent %q, want the duration exactly", got)
	}

	if got := sentComment.Load(); got != "chased the flake" {
		t.Errorf("sent comment %q, want the note exactly", got)
	}

	if worklog.ID != "10101" || worklog.TimeSpent != "2h" {
		t.Errorf("AddWorklog = %+v, want the worklog Jira recorded", worklog)
	}
}

func TestAddWorklogReportsJirasReason(t *testing.T) {
	t.Parallel()

	// Arrange
	bad := `{"errorMessages":[],"errors":{"timeLogged":"Time spent can not be null."}}`
	client := serve(t, failWith(http.StatusBadRequest, bad, "fred"))

	// Act
	_, err := client.AddWorklog(t.Context(), "OPS-1", "", "note")

	// Assert
	if !errors.Is(err, jira.ErrRejected) || !strings.Contains(err.Error(), "can not be null") {
		t.Errorf("AddWorklog returned %v, want Jira's reason wrapped in ErrRejected", err)
	}
}
