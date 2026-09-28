// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

//go:build unix

package proc_test

// A streamed run hands the program one end of a pipe and reads the other. When
// the process has no descriptor left for that pipe, Start refuses the run with
// the reason and starts nothing. A limit on the descriptors the process may
// hold is what refuses the pipe here: it holds for every file the process
// opens, so the test runs alone and lifts it the moment Start returns.

import (
	"errors"
	"strings"
	"syscall"
	"testing"

	"github.com/jacob-delgado/workflow/internal/proc"
	"github.com/jacob-delgado/workflow/internal/rlimit"
)

// standardDescriptors is how many descriptors a process holds before it opens
// anything: standard input, output and error.
const standardDescriptors = 3

//nolint:paralleltest // the descriptor limit holds for every file the process opens, so this runs alone.
func TestStartWithNoDescriptorLeftForThePipeSaysSo(t *testing.T) {
	// Arrange
	lift := rlimit.Lower(t, syscall.RLIMIT_NOFILE, standardDescriptors)

	// Act
	_, err := proc.Start(t.Context(), proc.Command{Name: "true"})

	lift()

	// Assert
	if !errors.Is(err, syscall.EMFILE) || !strings.Contains(err.Error(), "opening a pipe") {
		t.Errorf("Start = %v, want the pipe's own failure", err)
	}
}
