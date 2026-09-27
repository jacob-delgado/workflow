// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package jira_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/jira"
)

func TestAWriteWithoutACredentialSendsNothing(t *testing.T) {
	t.Parallel()

	cases := map[string]func(context.Context, jira.Client) error{
		"assigning an issue": func(ctx context.Context, client jira.Client) error {
			return client.Assign(ctx, "OPS-1", servedUser)
		},
		"linking a pull request": func(ctx context.Context, client jira.Client) error {
			return client.LinkPullRequest(ctx, "OPS-1", "https://forge/pull/42", "fix: token")
		},
	}

	for name, write := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			var sent atomic.Bool

			client := jira.New(offline(&sent), config.Jira{BaseURL: exampleBaseURL, Token: "", User: ""})

			// Act
			err := write(t.Context(), client)

			// Assert
			if !errors.Is(err, jira.ErrNoCredential) || sent.Load() {
				t.Errorf("%s returned %v, sent %v; want ErrNoCredential and nothing sent", name, err, sent.Load())
			}
		})
	}
}
