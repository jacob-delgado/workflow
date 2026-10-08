// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver

import (
	"slices"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/report"
)

// faultClasses are the failures fault tells apart: those every surface tells
// alike, report.SharedFaults, with the server's own placed among them where
// they belong, the more specific first.
func faultClasses() []report.Class {
	shared := report.SharedFaults()

	return slices.Concat(shared.Git, branchFaults(), shared.Transport, shared.Jira, shared.Forge, shared.Messaging,
		taskWriteFaults(), shared.Taskwarrior, shared.Directory, switchFaults(), localDataFaults(), peopleFaults())
}

// branchFaults are the server's own refusals of a new branch.
func branchFaults() []report.Class {
	return []report.Class{
		{Causes: []error{errBranchExists}, Code: api.ProblemCodeConflict, Detail: errBranchExists.Error()},
	}
}

// taskWriteFaults are Taskwarrior's writes the server words itself: a start
// it declined, ahead of Taskwarrior's own classes, since it has words of its
// own.
func taskWriteFaults() []report.Class {
	return []report.Class{
		{
			Causes: []error{errStartDeclined},
			Code:   api.ProblemCodeConflict, Detail: "the task is already started, or no such task exists",
		},
	}
}

// switchFaults are the server's refusals of a favorite or a switch to another
// directory, in words that never repeat its path, which the error itself
// carries.
func switchFaults() []report.Class {
	return []report.Class{
		{
			Causes: []error{errFavoritesNotKept, errNoSwitching}, Code: api.ProblemCodeUnprocessable,
			Detail: "that is not available here: the store is turned off, or the server cannot switch",
		},
		{
			Causes: []error{ErrConfigurationUnreadable}, Code: api.ProblemCodeUnprocessable,
			Detail: "the configuration there did not load; workflow doctor there says why",
		},
		{
			Causes: []error{ErrConfigurationRefused}, Code: api.ProblemCodeUnprocessable,
			Detail: "the configuration there binds keys workflow refuses; workflow doctor there names them",
		},
	}
}
