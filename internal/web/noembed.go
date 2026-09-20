// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

//go:build !embedui

package web

import "io/fs"

// Assets reports that this build embeds no single-page app: it returns a nil
// filesystem and false. This is the default build — the one tests and
// `go build ./...` compile — so a checkout builds without the frontend built
// first. A build with the `embedui` tag embeds the app instead; see embed.go.
func Assets() (fs.FS, bool) {
	return nil, false
}
