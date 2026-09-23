// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package loop

import (
	"errors"
	"slices"

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
	return each(Stageable(changes), stage)
}

// UnstageAll takes every staged change out of the index through unstage — a
// partly staged file's staged part too, leaving its work tree edits alone —
// going on past a file git refuses, and reports every failure joined.
func UnstageAll(changes []gitrepo.Change, unstage func(gitrepo.Change) error) error {
	staged := slices.DeleteFunc(slices.Clone(changes), func(change gitrepo.Change) bool { return !change.IsStaged() })

	return each(staged, unstage)
}

// each hands every change to act in turn, whatever the ones before it answered,
// and joins the failures.
func each(changes []gitrepo.Change, act func(gitrepo.Change) error) error {
	if act == nil {
		return ErrStagingUnavailable
	}

	failures := make([]error, 0, len(changes))

	for _, change := range changes {
		failures = append(failures, act(change))
	}

	return errors.Join(failures...)
}
