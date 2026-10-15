// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package status_test

import (
	"errors"
	"testing"

	"github.com/jacob-delgado/workflow/internal/cli/clitest"
	"github.com/jacob-delgado/workflow/internal/config"
)

func TestStatusRefusesAConfigurationItCannotRead(t *testing.T) {
	t.Parallel()

	// A file that exists but does not parse is not replaced by the defaults:
	// status read on settings nobody chose would read the wrong things.

	// Arrange
	dir := t.TempDir()
	clitest.WriteFile(t, dir, "{not json")

	// Act
	_, err := clitest.Run(t, dir, statusCommand)

	// Assert
	if !errors.Is(err, config.ErrInvalid) {
		t.Errorf("status with a malformed configuration = %v, want it refused", err)
	}

	wantExit(t, err, 3)
}
