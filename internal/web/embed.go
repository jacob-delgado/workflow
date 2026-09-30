// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package web

import (
	"embed"
	"fmt"
	"io/fs"
)

//go:embed all:dist
var dist embed.FS

// Assets returns the embedded single-page app rooted at its top level.
//
// Trade-off TRADE-20: fs.Sub fails only for an invalid path, and "dist" is a
// constant, so no test runs the error arm.
func Assets() (fs.FS, error) {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		return nil, fmt.Errorf("reading the embedded web app: %w", err)
	}

	return sub, nil
}
