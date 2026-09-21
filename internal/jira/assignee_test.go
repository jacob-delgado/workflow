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

func TestAssignPutsTheAssigneeByNameToTheEscapedIssue(t *testing.T) {
	t.Parallel()

	// Arrange
	var (
		requested atomic.Value
		sent      atomic.Value
	)

	client := serve(t, func(writer http.ResponseWriter, request *http.Request) {
		requested.Store(request.Method + " " + request.URL.EscapedPath() + " " + request.Header.Get("Content-Type"))

		var body struct {
			Name string `json:"name"`
		}

		_ = json.NewDecoder(request.Body).Decode(&body)
		sent.Store(body.Name)

		writer.Header().Set("X-Ausername", "fred")
		writer.WriteHeader(http.StatusNoContent)
	})

	// Act
	err := client.Assign(t.Context(), "OPS/1", "fred")
	// Assert
	if err != nil {
		t.Fatalf("Assign returned %v, want nil", err)
	}

	if got := requested.Load(); got != "PUT /rest/api/2/issue/OPS%2F1/assignee "+jsonMediaType {
		t.Errorf("requested %v, want a JSON PUT to the escaped issue's assignee", got)
	}

	if got := sent.Load(); got != "fred" {
		t.Errorf("sent name %q, want the username exactly", got)
	}
}

func TestAssignReportsJirasReason(t *testing.T) {
	t.Parallel()

	// Arrange
	denied := `{"errorMessages":["You do not have permission to assign issues."]}`
	client := serve(t, failWith(http.StatusForbidden, denied, "fred"))

	// Act
	err := client.Assign(t.Context(), "OPS-1", "fred")

	// Assert
	if !errors.Is(err, jira.ErrRejected) || !strings.Contains(err.Error(), "do not have permission") {
		t.Errorf("Assign returned %v, want Jira's reason wrapped in ErrRejected", err)
	}
}
