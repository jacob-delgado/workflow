// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

//go:build !unix

package pgroup

import "os/exec"

// Isolate is a no-op off Unix: a whole-group kill is a Unix session, and its
// Windows twin — a job object — cannot be exercised from here, so the default
// cancel, which kills the child, stands. See TECH_DEBT DEBT-24 and DEBT-32.
func Isolate(_ *exec.Cmd) {}
