// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package loop_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/loop"
	"github.com/jacob-delgado/workflow/internal/taskwarrior"
)

func TestEveryNotSetUpCauseSaysWhatToSet(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		cause error
		names string
	}{
		"no Jira token":           {cause: jira.ErrNoCredential, names: "jira.token"},
		"no forge token":          {cause: forge.ErrNoToken, names: "forge.token"},
		"origin on no forge":      {cause: forge.ErrNotARemote, names: "origin"},
		"an unknown forge":        {cause: forge.ErrUnknownForge, names: "forge.kind and forge.host"},
		"no git identity":         {cause: gitrepo.ErrNoIdentity, names: "git config user.email"},
		"no Taskwarrior":          {cause: taskwarrior.ErrNotInstalled, names: "taskwarrior.program"},
		"another task program":    {cause: taskwarrior.ErrNotTaskwarrior, names: "taskwarrior.program"},
		"a Taskwarrior never run": {cause: taskwarrior.ErrNotConfigured, names: "run it once"},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			advice, notSetUp := loop.SetUpAdvice(fmt.Errorf("reading: %w", tt.cause))

			// Assert
			if !notSetUp || !strings.Contains(advice, tt.names) {
				t.Errorf("SetUpAdvice = %q, %v; want words naming %q", advice, notSetUp, tt.names)
			}
		})
	}
}

func TestARefusalHasNoSetUpAdvice(t *testing.T) {
	t.Parallel()

	// Act
	advice, notSetUp := loop.SetUpAdvice(fmt.Errorf("searching: %w", jira.ErrUnauthorized))

	// Assert
	if notSetUp || advice != "" {
		t.Errorf("SetUpAdvice = %q, %v; want none for a refusal", advice, notSetUp)
	}
}
