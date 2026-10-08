// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

// Package filelock takes an advisory lock on an open file, which the system
// lets go when the file is closed, whether by the process or by its end, so no
// lock outlives its holder. It is the platform glue alone, in a package of its
// own, so the code that waits on a lock is measured where it lives.
package filelock
