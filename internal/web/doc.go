// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

// Package web carries the built single-page app so `workflow --web` serves it
// from the one static binary. The build output in dist/ is committed, not only
// produced at release, because `go install` from the module proxy fetches
// committed files alone and cannot run the frontend build; every build —
// `go install` included — embeds it. `task web:build` regenerates it, and
// `task web:dist:check` fails when the committed copy is stale.
package web
