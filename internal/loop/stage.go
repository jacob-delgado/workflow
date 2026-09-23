// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package loop

import (
	"errors"

	"github.com/jacob-delgado/workflow/internal/gitrepo"
)

// ErrStagingUnavailable refuses staging with no way to stage.
var ErrStagingUnavailable = errors.New("staging is not available")

// Stageable is every change "stage all" takes into the index: work the index
// does not hold yet — an edit, a deletion, a file git does not track, the rest
// of a partly staged file — and a conflict, which staging marks resolved. What
// is wholly staged already is left out.
func Stageable(changes []gitrepo.Change) []gitrepo.Change {
	var pending []gitrepo.Change

	for _, change := range changes {
		if change.HasUnstaged() || change.Conflicted() {
			pending = append(pending, change)
		}
	}

	return pending
}

// StageAll stages every Stageable change through stage, going on past a file
// git refuses so one failure does not hold back the rest, and reports every
// failure joined.
func StageAll(changes []gitrepo.Change, stage func(gitrepo.Change) error) error {
	if stage == nil {
		return ErrStagingUnavailable
	}

	pending := Stageable(changes)
	failures := make([]error, 0, len(pending))

	for _, change := range pending {
		failures = append(failures, stage(change))
	}

	return errors.Join(failures...)
}
