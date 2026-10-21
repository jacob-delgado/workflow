// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package demo_test

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
	"time"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/demo"
	"github.com/jacob-delgado/workflow/internal/webserver/webtest"
)

func TestTheWebDemoSavesSettingsInTheDirectoryItIsGiven(t *testing.T) {
	t.Parallel()

	// Arrange
	kept := t.TempDir()
	world := demo.New(func() time.Duration { return 0 })
	handler := webtest.ServeWorld(t, world.Web(webtest.TestVersion, kept), fstest.MapFS{})
	shown := webtest.Get(t, handler, "/api/config").Body.String()

	// Act
	saved := webtest.PutConfig(t, handler, shown)

	// Assert
	if saved.Code != http.StatusOK {
		t.Fatalf("saving Settings answered %d: %s", saved.Code, saved.Body.String())
	}

	_, err := os.Stat(filepath.Join(kept, config.FileName))
	if err != nil {
		t.Errorf("saving Settings left no %s in the directory the demo was given: %v", config.FileName, err)
	}
}
