// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

//go:build !unix

package pgroup

import "os/exec"

// Isolate is a no-op off Unix: a whole-group kill is a Unix session, and its
// Windows twin — a job object — cannot be exercised from here, so the default
// cancel, which kills the child, stands. The job object is FEAT-73 in
// FEATURES.md, to be built and watched on a real Windows runner.
func Isolate(_ *exec.Cmd) {}
