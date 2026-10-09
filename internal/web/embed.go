// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package web

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var dist embed.FS

// Assets returns the embedded single-page app rooted at its top level. fs.Sub
// fails only for an invalid path, and "dist" is a constant the embed itself
// fails the build without, so a failure here is a build defect, which panics,
// as regexp.MustCompile does for a pattern that cannot compile.
func Assets() fs.FS {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		panic("reading the embedded web app: " + err.Error())
	}

	return sub
}
