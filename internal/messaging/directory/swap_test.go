// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package directory_test

// A swap of settings and the directory's forgetting are one step: a read
// begins under the old settings and what they read, or under the new ones
// and nothing read yet, never under the new settings and the old reads.

import (
	"sync"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/messaging/directory"
)

// otherWorkspace is auth.test's answer in the workspace the settings swap to.
const otherWorkspace = `{"ok":true,"team":"Other","user":"workflow","team_id":"T0OTHER"}`

func TestAReadBegunDuringASwapDoesNotJoinTheOldWorkspacesRead(t *testing.T) {
	t.Parallel()

	// Arrange
	// Workspace A's auth.test is under way when the settings swap to B's
	// token, and a read is begun from inside the swap.
	before, beforeClient := startSlack(t, directoryBodies())
	arrived, release := before.hold(t, "/auth.test")

	afterBodies := directoryBodies()
	afterBodies["/auth.test"] = otherWorkspace
	_, afterClient := startSlack(t, afterBodies)

	settings := &switchable{client: beforeClient, available: true}
	slackDirectory := directory.New(settings.current, time.Now)

	var readers sync.WaitGroup

	readers.Go(func() { _, _ = slackDirectory.Workspace(t.Context()) })
	<-arrived

	var during string

	// Act
	slackDirectory.Swap(func() {
		settings.use(afterClient)
		readers.Go(func() { during, _ = slackDirectory.Workspace(t.Context()) })
	})
	release()
	readers.Wait()

	// Assert
	if during != "T0OTHER" {
		t.Errorf("a read begun during the swap answered %q, joined workspace A's read; want B's T0OTHER", during)
	}
}
