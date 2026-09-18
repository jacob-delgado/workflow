// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli_test

import "testing"

func TestStatusOutsideARepositoryReportsSo(t *testing.T) {
	// Act
	_, err := run(t, t.TempDir(), "status")

	// Assert
	if err == nil {
		t.Error("status outside a repository returned no error")
	}
}
