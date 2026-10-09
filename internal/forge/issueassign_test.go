// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package forge_test

import (
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/forge"
)

func TestAssignIssueOnGitHubAddsTheAssignee(t *testing.T) {
	t.Parallel()

	// Arrange
	client, seen := recordingForge(t, answering(http.StatusCreated, `{"number":42}`))

	// Act
	err := client.AssignIssue(t.Context(), githubRepo(), 42, userAna)

	// Assert
	sent := lastRequest(t, seen)
	if err != nil || sent.method != http.MethodPost || sent.path != githubIssuePath+"/assignees" ||
		!reflect.DeepEqual(sent.body["assignees"], []any{userAna}) {
		t.Errorf("AssignIssue = %v, sent %s %s %+v; want ana added", err, sent.method, sent.path, sent.body)
	}
}

func TestAssignIssueOnGitLabSetsTheAssigneeByID(t *testing.T) {
	t.Parallel()

	// Arrange
	client, seen := recordingForge(t, conversation(map[string]string{
		gitlabUsersPath: `[{"id":7}]`,
		gitlabIssuePath: `{"iid":7}`,
	}, nil))

	// Act
	err := client.AssignIssue(t.Context(), gitlabRepo(), 7, userAna)

	// Assert
	sent := requestTo(*seen, gitlabIssuePath)
	if err != nil || sent.method != http.MethodPut || !reflect.DeepEqual(sent.body["assignee_ids"], []any{float64(7)}) {
		t.Errorf("AssignIssue = %v, sent %s %+v; want the resolved id set", err, sent.method, sent.body)
	}
}

func TestAssignIssueOnGitLabSaysWhyNoOneWasAssigned(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		users        func(recorded) (int, string)
		want         error
		namesTheUser bool
	}{
		"a name GitLab knows no user by": {
			users: answering(http.StatusOK, "[]"), want: forge.ErrNoUser, namesTheUser: true,
		},
		"GitLab failing to look the name up": {
			users: answering(http.StatusBadGateway, `{"message":"502 Bad Gateway"}`), want: forge.ErrUnexpectedStatus,
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			client, seen := recordingForge(t, tt.users)

			// Act
			err := client.AssignIssue(t.Context(), gitlabRepo(), 7, userGhost)

			// Assert
			if !errors.Is(err, tt.want) || errors.Is(err, forge.ErrNoUser) != tt.namesTheUser ||
				strings.Contains(fmt.Sprint(err), userGhost) != tt.namesTheUser {
				t.Errorf("AssignIssue = %v, want %v, naming %s only when GitLab knows no one by it", err, tt.want, userGhost)
			}

			if assigned := requestTo(*seen, gitlabIssuePath); assigned.method != "" {
				t.Errorf("the issue was sent %s %+v, want nothing set once the name was not found", assigned.method,
					assigned.body)
			}
		})
	}
}
