// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

// Package places is where an issue can be, for narrowing the Issues list: one
// of its tracker's status names, or one of workflow's own marks.
package places

import (
	"slices"

	"github.com/jacob-delgado/workflow/internal/jira"
)

// Trade-off TRADE-21: these words and the place rules below are written again
// in web/src/features/issues/issuePlaces.ts, and both copies answer to the one
// case file testdata/twins/places.json, so a change to either alone fails its
// own tests.

// The marks an issue can be in, in the order the picker lists them, as both
// surfaces word them.
const (
	InFlight   = "in flight"
	TaskActive = "task active"
	Tracked    = "tracked"
	TaskDone   = "task done"
	Forge      = "forge issue"
)

// Kind is which of the two groups a place is in. Places in one group widen the
// list, and the two groups narrow it together.
type Kind string

// The two groups, as the web names them.
const (
	Status Kind = "status"
	Mark   Kind = "mark"
)

// Place is where an issue can be: one of its tracker's status names, or one of
// workflow's own marks.
type Place struct {
	Kind Kind
	Name string
}

// Choice is a place on offer, with how many of the issues are in it.
type Choice struct {
	Place Place
	Count int
}

// Standing is what an issue's marks are read from: whether a branch names it,
// the words for how its linked tasks stand (empty with none, or none known),
// and whether it is the forge's issue rather than Jira's.
type Standing struct {
	InFlight bool
	Task     string
	Forge    bool
}

// Marks is the marks an issue so standing is in, in the order the picker lists
// them.
func (s Standing) Marks() []string {
	var marks []string

	if s.InFlight {
		marks = append(marks, InFlight)
	}

	if s.Task != "" {
		marks = append(marks, s.Task)
	}

	if s.Forge {
		marks = append(marks, Forge)
	}

	return marks
}

// Admits reports whether an issue in status, with marks, is in the picked
// places: in any picked status, when a status is picked, and holding any picked
// mark, when a mark is picked.
func Admits(picked []Place, status string, marks []string) bool {
	return admitsIn(picked, Status, []string{status}) && admitsIn(picked, Mark, marks)
}

// admitsIn reports whether any of names is picked in the kind's group, or
// nothing is picked there at all.
func admitsIn(picked []Place, kind Kind, names []string) bool {
	constrained := false

	for _, chosen := range picked {
		if chosen.Kind != kind {
			continue
		}

		constrained = true

		if slices.Contains(names, chosen.Name) {
			return true
		}
	}

	return !constrained
}

// Choices is every place the issues are in, with how many are in each —
// statuses by category, not started first, then as they first appear, then the
// marks in their fixed order — and every picked place no issue is in, at zero,
// so it can still be unpicked.
func Choices(issues []jira.Issue, marksOf func(jira.Key) []string, picked []Place) []Choice {
	counts := map[Place]int{}

	for _, issue := range issues {
		counts[Place{Kind: Status, Name: issue.Status}]++

		for _, mark := range marksOf(issue.Key) {
			counts[Place{Kind: Mark, Name: mark}]++
		}
	}

	var choices []Choice

	for _, offering := range slices.Concat(statusPlaces(issues), pickedStatusesGone(issues, picked), markPlaces()) {
		if counts[offering] > 0 || slices.Contains(picked, offering) {
			choices = append(choices, Choice{Place: offering, Count: counts[offering]})
		}
	}

	return choices
}

// statusPlaces is each status the issues are in, once, not started first, then
// in flight, then done, then any in a category Jira does not name (its "No
// Category", or none), and within each in the order they first appear.
func statusPlaces(issues []jira.Issue) []Place {
	var statuses []Place

	add := func(keep func(jira.StatusCategory) bool) {
		for _, issue := range issues {
			status := Place{Kind: Status, Name: issue.Status}
			if keep(issue.StatusCategory) && !slices.Contains(statuses, status) {
				statuses = append(statuses, status)
			}
		}
	}

	named := []jira.StatusCategory{jira.CategoryNew, jira.CategoryIndeterminate, jira.CategoryDone}
	for _, category := range named {
		add(func(of jira.StatusCategory) bool { return of == category })
	}

	add(func(of jira.StatusCategory) bool { return !slices.Contains(named, of) })

	return statuses
}

// pickedStatusesGone is each picked status no issue is in any more, in the
// order it was picked, so the picker can still offer to unpick it.
func pickedStatusesGone(issues []jira.Issue, picked []Place) []Place {
	var gone []Place

	for _, chosen := range picked {
		if chosen.Kind == Status && !slices.ContainsFunc(issues, func(issue jira.Issue) bool {
			return issue.Status == chosen.Name
		}) {
			gone = append(gone, chosen)
		}
	}

	return gone
}

// markPlaces is every mark, in the order the picker lists them.
func markPlaces() []Place {
	marks := []string{InFlight, TaskActive, Tracked, TaskDone, Forge}
	offered := make([]Place, 0, len(marks))

	for _, mark := range marks {
		offered = append(offered, Place{Kind: Mark, Name: mark})
	}

	return offered
}
