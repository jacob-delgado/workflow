// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package taskwarrior

import (
	"slices"
	"strconv"
	"strings"
	"time"
)

// FacetKind is one of the ways the Tasks list can be narrowed.
type FacetKind int

// The kinds, in the order the list offers them.
const (
	FacetState FacetKind = iota
	FacetPriority
	FacetProject
	FacetTag
	FacetIssue
)

// The two values of the issue facet.
const (
	WithIssue = "linked"
	NoIssue   = "unlinked"
)

// Facet is one value a task can hold in one kind: a state as the list words
// it, a priority, a project or a tag ("" for none), or whether it is linked to
// an issue.
type Facet struct {
	Kind  FacetKind
	Value string
}

// Label is the facet as the list words it.
func (f Facet) Label() string {
	if f.Value == "" {
		return noneOf(f.Kind)
	}

	switch f.Kind {
	case FacetPriority:
		return "priority " + f.Value
	case FacetProject:
		return "project " + f.Value
	case FacetTag:
		return "+" + f.Value
	case FacetIssue:
		return issueLabel(f.Value)
	case FacetState:
		return f.Value
	}

	return f.Value
}

// noneOf is what a kind's empty value is called.
func noneOf(kind FacetKind) string {
	words := [...]string{"no state", "no priority", "no project", "no tag", "no issue"}

	return words[kind]
}

// issueLabel words the issue facet's two values.
func issueLabel(value string) string {
	if value == WithIssue {
		return "with issue"
	}

	return "no issue"
}

// FacetChoice is a value the list offers to narrow to, with how many listed
// tasks hold it.
type FacetChoice struct {
	Facet Facet
	Count int
}

// Narrowing is what the Tasks list is narrowed to: values picked, which widen
// within a kind and narrow across kinds, and text typed, which a task matches
// when one of its fields holds it, ignoring case.
type Narrowing struct {
	Picked []Facet
	Text   string
}

// Narrows reports a narrowing that leaves any task out: something picked or
// typed.
func (n Narrowing) Narrows() bool {
	return len(n.Picked) > 0 || n.Text != ""
}

// ListsWaiting reports a narrowing that asks for the waiting tasks, which the
// list otherwise only counts.
func (n Narrowing) ListsWaiting() bool {
	return slices.Contains(n.Picked, Facet{Kind: FacetState, Value: StateWaiting.String()})
}

// Matches reports a task the narrowing lets through at now: the typed text in
// one of its fields, and, in every kind with a value picked, one of them held.
func (n Narrowing) Matches(task Task, now time.Time) bool {
	if !task.mentions(n.Text) {
		return false
	}

	held := task.Facets(now)

	for _, kind := range pickedKinds(n.Picked) {
		if !slices.ContainsFunc(held, func(facet Facet) bool {
			return facet.Kind == kind && slices.Contains(n.Picked, facet)
		}) {
			return false
		}
	}

	return true
}

// pickedKinds is each kind with a value picked, once.
func pickedKinds(picked []Facet) []FacetKind {
	var kinds []FacetKind

	for _, facet := range picked {
		if !slices.Contains(kinds, facet.Kind) {
			kinds = append(kinds, facet.Kind)
		}
	}

	return kinds
}

// mentions reports text, ignoring case, within one of the task's fields: its
// description, project, a tag written +tag, its issue key, or its id written
// #id. A match never spans two fields.
func (t Task) mentions(text string) bool {
	if text == "" {
		return true
	}

	needle := strings.ToLower(text)

	return slices.ContainsFunc(t.searchable(), func(field string) bool {
		return strings.Contains(strings.ToLower(field), needle)
	})
}

// searchable is every field typed text is matched against.
func (t Task) searchable() []string {
	fields := []string{t.Description, t.Project, t.IssueKey}

	for _, tag := range t.Tags {
		fields = append(fields, "+"+tag)
	}

	if t.ID > 0 {
		fields = append(fields, "#"+strconv.Itoa(t.ID))
	}

	return fields
}

// Facets is every value the task holds at now, one or more in each kind.
func (t Task) Facets(now time.Time) []Facet {
	facets := []Facet{
		{Kind: FacetState, Value: t.State(now).String()},
		{Kind: FacetPriority, Value: t.Priority},
		{Kind: FacetProject, Value: t.Project},
		{Kind: FacetIssue, Value: NoIssue},
	}

	if t.Linked() {
		facets[3].Value = WithIssue
	}

	if len(t.Tags) == 0 {
		return append(facets, Facet{Kind: FacetTag})
	}

	for _, tag := range t.Tags {
		facets = append(facets, Facet{Kind: FacetTag, Value: tag})
	}

	return facets
}

// Choices is every value the tasks hold, with how many hold each, in the order
// the list offers them, and every picked value none holds, at zero, so it can
// be unpicked. A count is over the tasks the list shows with nothing picked —
// the waiting tasks left out — but for the waiting state, which counts them.
func Choices(tasks []Task, picked []Facet, now time.Time) []FacetChoice {
	counts := map[Facet]int{}

	for _, task := range tasks {
		for _, facet := range task.Facets(now) {
			if !task.Waiting(now) || facet.Kind == FacetState {
				counts[facet]++
			}
		}
	}

	var choices []FacetChoice

	for _, offering := range offeredFacets(counts, picked) {
		if counts[offering] > 0 || slices.Contains(picked, offering) {
			choices = append(choices, FacetChoice{Facet: offering, Count: counts[offering]})
		}
	}

	return choices
}

// offeredFacets is every value the list could offer, in order: the states as
// ByState ranks them; priorities H, M, L, any other by name, then none;
// projects and tags by name, none last; with an issue, then without.
func offeredFacets(counts map[Facet]int, picked []Facet) []Facet {
	states := make([]Facet, 0, int(StateUnknown)+1)
	for state := StateStarted; state <= StateUnknown; state++ {
		states = append(states, Facet{Kind: FacetState, Value: state.String()})
	}

	return slices.Concat(
		states,
		ranked(FacetPriority, []string{"H", "M", "L"}, counts, picked),
		ranked(FacetProject, nil, counts, picked),
		ranked(FacetTag, nil, counts, picked),
		[]Facet{{Kind: FacetIssue, Value: WithIssue}, {Kind: FacetIssue, Value: NoIssue}},
	)
}

// ranked is a kind's values in order: those named first, in their order, then
// the rest held or picked, by name, then none.
func ranked(kind FacetKind, first []string, counts map[Facet]int, picked []Facet) []Facet {
	var rest []string

	for facet := range counts {
		if facet.Kind == kind && facet.Value != "" && !slices.Contains(first, facet.Value) {
			rest = append(rest, facet.Value)
		}
	}

	for _, facet := range picked {
		if facet.Kind == kind && facet.Value != "" && !slices.Contains(first, facet.Value) {
			rest = append(rest, facet.Value)
		}
	}

	slices.Sort(rest)

	values := slices.Concat(first, slices.Compact(rest), []string{""})
	facets := make([]Facet, 0, len(values))

	for _, value := range values {
		facets = append(facets, Facet{Kind: kind, Value: value})
	}

	return facets
}
