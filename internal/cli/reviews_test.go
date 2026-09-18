// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli_test

import "testing"

func TestReviewsWithoutAForgeReportsSo(t *testing.T) {
	// Act
	_, err := run(t, t.TempDir(), "reviews")

	// Assert
	if err == nil {
		t.Error("reviews without a forge to ask returned no error")
	}
}
