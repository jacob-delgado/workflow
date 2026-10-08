// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

// Package pgroup runs a streamed child in its own process group, so canceling
// its context kills the whole tree rather than the child alone. The behavior is
// Unix-only; off Unix Isolate is a no-op and the default child-only cancel
// stands. Its two build-tagged halves are a twin, which gobco reads only a file
// at a time, each standing alone (see scripts/gobco-report.sh), so they live in
// a package of their own rather than ask that of every file in internal/proc.
package pgroup
