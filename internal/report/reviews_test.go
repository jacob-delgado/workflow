// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package report_test

import (
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/report"
)

func TestNoReviewRequestIsAnEmptyListNotNull(t *testing.T) {
	t.Parallel()

	// Act
	queue := report.ReviewRequests(nil)

	// Assert
	if queue == nil || len(queue) != 0 {
		t.Errorf("ReviewRequests(nil) = %#v, want an empty list", queue)
	}
}

func TestAReviewRequestCarriesItsCIAndWhenItOpened(t *testing.T) {
	t.Parallel()

	// Arrange
	opened := time.Date(2026, time.March, 3, 9, 0, 0, 0, time.UTC)
	request := forge.ReviewRequest{
		Number: 7, URL: "https://github.com/owner/repo/pull/7", Title: "Redact tokens", Author: "octo",
		Repository: "owner/repo", CI: forge.CIPassed, OpenedAt: opened,
	}

	// Act
	queue := report.ReviewRequests([]forge.ReviewRequest{request})

	// Assert
	got := queue[0]
	passed := api.CIState(forge.CIPassed.Word())

	if got.Number != 7 || got.Ci != passed || got.OpenedAt == nil || !got.OpenedAt.Equal(opened) {
		t.Errorf("the request = %+v, want #7, passed, opened when it was", got)
	}
}

func TestAReviewRequestTheForgeGaveNoOpeningForLeavesItOut(t *testing.T) {
	t.Parallel()

	// Act
	queue := report.ReviewRequests([]forge.ReviewRequest{{Number: 7}})

	// Assert
	if queue[0].OpenedAt != nil {
		t.Errorf("OpenedAt = %v, want it left out", queue[0].OpenedAt)
	}
}
