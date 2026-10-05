// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package taskwarrior_test

import (
	"testing"
	"time"
)

func TestTouchedReadsTasksChangedSinceUnderTheActiveContext(t *testing.T) {
	t.Parallel()

	// Arrange
	// A task completed or deleted is still work done, so no status is left out
	// but deleted; the active context narrows it as it narrows the list.
	fake := &fakeTask{replies: map[string]reply{
		exportWord: {stdout: `[{"id": 0, "uuid": "a", "status": "completed", "description": "Ship it"}]`},
		showWord:   {stdout: workContext},
	}}
	since := time.Date(2026, 10, 2, 4, 0, 0, 0, time.FixedZone("EDT", -4*60*60))

	// Act
	tasks, err := fake.client().Touched(t.Context(), since)

	// Assert
	if err != nil || len(tasks) != 1 || tasks[0].Description != "Ship it" {
		t.Errorf("Touched = %+v, %v; want the completed task", tasks, err)
	}

	checkRuns(t, fake.calls, []call{
		showing(),
		reading("( project:Work )", "modified.after:2026-10-02T07:59:59Z", "status.not:deleted", exportWord),
	})
}
