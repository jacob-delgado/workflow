// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package web_test

import (
	"io/fs"
	"regexp"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/web"
)

func TestAssetsCarriesTheBuiltApp(t *testing.T) {
	t.Parallel()

	// Act
	assets := web.Assets()

	// Assert
	info, err := fs.Stat(assets, "index.html")
	if err != nil {
		t.Fatalf("fs.Stat(Assets(), index.html) = %v, want the built app's entry page", err)
	}

	if !info.Mode().IsRegular() {
		t.Errorf("index.html mode = %v, want a regular file", info.Mode())
	}
}

func TestTheBuiltAppRunsNoInlineScript(t *testing.T) {
	t.Parallel()

	// Arrange
	// The server's content policy runs scripts from the app's own origin only,
	// so a script written into the page itself would never run.
	index, err := fs.ReadFile(web.Assets(), "index.html")
	if err != nil {
		t.Fatalf("reading the built app's index.html: %v", err)
	}

	// Act
	scripts := regexp.MustCompile(`<script\b[^>]*>`).FindAllString(string(index), -1)

	// Assert
	if len(scripts) == 0 {
		t.Fatal("index.html holds no script, want the app's own")
	}

	for _, script := range scripts {
		if !strings.Contains(script, " src=") {
			t.Errorf("index.html holds %q, a script with no src, which the content policy would refuse", script)
		}
	}
}
