// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli

// Execute wires a signal-cancelable context that every command runs under.
// Raising a real signal in a test is flaky, so this drives the internal execute
// with a context it controls, which is the same context ExecuteContext delivers.

import (
	"bytes"
	"context"
	"testing"
)

func TestExecuteRunsCommandsUnderTheGivenContext(t *testing.T) {
	t.Parallel()

	// Arrange
	// A live context in this repository lets `status` read the branch, so a
	// failure under the canceled one below is the cancellation reaching git, not
	// a missing repository. Without ExecuteContext the command would run under
	// context.Background and ignore the cancellation entirely.
	live := execute(context.Background(), []string{"status"}, &bytes.Buffer{}, &bytes.Buffer{}, Prompt{})
	if live != nil {
		t.Skipf("status is not readable here, so cancellation cannot be told apart: %v", live)
	}

	canceled, cancel := context.WithCancel(context.Background())
	cancel()

	// Act
	err := execute(canceled, []string{"status"}, &bytes.Buffer{}, &bytes.Buffer{}, Prompt{})

	// Assert
	if err == nil {
		t.Error("status ran to completion under a canceled context; the context did not reach its git subprocess")
	}
}
