// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

// Package pgroup runs a streamed child in its own process group, so canceling
// its context kills the whole tree rather than the child alone. The behavior is
// Unix-only; off Unix Isolate is a no-op and the default child-only cancel
// stands. It lives in its own package because its two build-tagged halves are a
// twin gobco cannot read (see scripts/gobco-report.sh), which keeps that from
// costing the rest of internal/proc its branch coverage.
package pgroup
