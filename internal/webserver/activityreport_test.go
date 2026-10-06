// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/webserver"
)

func TestAFaultsDetailIsTheProblemsWordsWithoutTheHost(t *testing.T) {
	t.Parallel()

	// Arrange
	unreachable := fmt.Errorf("%w: https://jira.internal.example", jira.ErrUnreachable)

	// Act
	detail := webserver.FaultDetail(unreachable)

	// Assert
	if strings.Contains(detail, "internal.example") || !strings.Contains(detail, "could not be reached") {
		t.Errorf("FaultDetail = %q, want the problem's words for an unreachable service, no host", detail)
	}
}
