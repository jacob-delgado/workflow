// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

//go:build embedui

package web

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var dist embed.FS

// Assets returns the embedded single-page app rooted at its top level, and true.
func Assets() (fs.FS, bool) {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		return nil, false
	}

	return sub, true
}
