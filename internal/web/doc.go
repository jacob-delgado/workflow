// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

// Package web carries the built single-page app so `workflow --web` serves it
// from the one static binary. The assets are embedded only under the `embedui`
// build tag (see embed.go); the default build — tests, vet, and
// `go build ./...` — compiles the stub in noembed.go and needs no dist/
// directory, so a clean checkout builds before the frontend is ever built.
// Release builds (`task build`, `task release:binaries`) build the frontend and
// set the tag.
package web
