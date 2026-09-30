// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver

import (
	"io/fs"
	"net/http"
	"path"
	"strings"
)

// spaHandler serves the embedded single-page app. It returns the requested file
// when one exists and index.html for every other path, so the client-side router
// owns the routes that name no file. It is reached only for requests the API
// subtree did not claim, and serves GET and HEAD alone — the app is read-only.
func spaHandler(assets fs.FS) http.Handler {
	files := http.FileServerFS(assets)

	return http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet && request.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "the web interface is read-only", http.StatusMethodNotAllowed)

			return
		}

		if hasFile(assets, request.URL.Path) {
			files.ServeHTTP(w, request)

			return
		}

		http.ServeFileFS(w, request, assets, "index.html")
	})
}

// hasFile reports whether urlPath resolves to a regular file in assets. The root
// and any directory resolve to no served file, so index.html serves them through
// the fallback rather than the file server listing a directory.
func hasFile(assets fs.FS, urlPath string) bool {
	name := strings.TrimPrefix(path.Clean("/"+urlPath), "/")
	if name == "" {
		return false
	}

	info, err := fs.Stat(assets, name)

	return err == nil && !info.IsDir()
}
