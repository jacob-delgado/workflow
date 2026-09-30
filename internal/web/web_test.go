// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package web_test

import (
	"io/fs"
	"testing"

	"github.com/jacob-delgado/workflow/internal/web"
)

func TestAssetsCarriesTheBuiltApp(t *testing.T) {
	t.Parallel()

	// Act
	assets, err := web.Assets()
	// Assert
	if err != nil {
		t.Fatalf("Assets() = %v, want the embedded app", err)
	}

	info, err := fs.Stat(assets, "index.html")
	if err != nil {
		t.Fatalf("fs.Stat(Assets(), index.html) = %v, want the built app's entry page", err)
	}

	if !info.Mode().IsRegular() {
		t.Errorf("index.html mode = %v, want a regular file", info.Mode())
	}
}
