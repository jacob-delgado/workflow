// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

import (
	"net/http"
	"testing"
	"testing/synctest"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/webserver"
	"github.com/jacob-delgado/workflow/internal/webserver/webtest"
)

func TestASettingsSaveIsRefusedWhileAnotherWorkflowHoldsTheSaveLock(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		// Arrange
		cfg := storedConfig(t)
		locks := config.SaveLocksIn(t.TempDir())

		unlock, err := locks.Hold(cfg.Layers())
		if err != nil {
			t.Fatalf("holding the save lock: %v", err)
		}

		defer unlock()

		next := cfg.Redacted()
		next.Jira.Project = "OSS"

		// Act
		recorder := webtest.PutConfig(t, webtest.Serve(t, webserver.Deps{SaveLocks: locks}, cfg), webtest.Marshal(t, next))

		// Assert
		onDisk, _, readErr := config.LoadLayersAt(cfg.Layers())
		if recorder.Code != http.StatusInternalServerError || readErr != nil || onDisk.Jira.Project == "OSS" {
			t.Errorf("status %d (%s); the file names %q (%v); want %d, the save refused while another workflow "+
				"held the lock for longer than a save waits, and the file as it was", recorder.Code,
				recorder.Body.String(), onDisk.Jira.Project, readErr, http.StatusInternalServerError)
		}
	})
}
