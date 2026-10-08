// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package taskwarrior

import (
	"cmp"
	"slices"
	"strings"
	"time"
)

// State is where a task stands, as the Tasks list words and orders it. It is
// not Taskwarrior's status alone: a pending task that has been started is
// started, and a pending one with a wait still ahead waits.
type State int

// The states, in the order ByState lists them.
const (
	StateStarted State = iota
	StatePending
	StateWaiting
	StateRecurring
	StateCompleted
	StateDeleted
	StateUnknown
)

// String is the state as the list words it.
func (s State) String() string {
	words := [...]string{"started", "pending", "waiting", "recurring", "completed", "deleted", "unknown"}
	if s < 0 || int(s) >= len(words) {
		return "unknown"
	}

	return words[s]
}

// State is where the task stands at now.
func (t Task) State(now time.Time) State {
	switch t.Status {
	case Pending:
		return t.pendingState(now)
	case Waiting:
		return StateWaiting
	case Recurring:
		return StateRecurring
	case Completed:
		return StateCompleted
	case Deleted:
		return StateDeleted
	default:
		// A synced replica can carry a status this version does not know.
		return StateUnknown
	}
}

// pendingState is where a pending task stands at now: hidden until a date
// still ahead, started, or neither.
func (t Task) pendingState(now time.Time) State {
	switch {
	case t.Waiting(now):
		return StateWaiting
	case t.Active():
		return StateStarted
	default:
		return StatePending
	}
}

// The orders below are the Tasks list's.
//
// Trade-off TRADE-29: they are written again in
// web/src/features/tasks/taskOrder.ts, and twin-named tests pin the two.

// ByUrgency orders most urgent first, then by id, then by uuid, so a refresh
// keeps a stable order.
func ByUrgency(tasks []Task) []Task {
	return sortedBy(tasks, func(Task, Task) int { return 0 })
}

// ByState orders started tasks first, then pending, then waiting.
func ByState(tasks []Task, now time.Time) []Task {
	return sortedBy(tasks, func(first, second Task) int {
		return cmp.Compare(first.State(now), second.State(now))
	})
}

// ByID orders by working-set id, lowest first, with a task outside the working
// set, which has none, after every numbered one.
func ByID(tasks []Task) []Task {
	return sortedBy(tasks, func(first, second Task) int {
		return cmp.Or(compareLast(first.ID == 0, second.ID == 0), cmp.Compare(first.ID, second.ID))
	})
}

// ByTag orders by a task's tags taken in order, whatever order they were
// added in, so +ops comes before +ops +web, with untagged tasks last.
func ByTag(tasks []Task) []Task {
	return sortedBy(tasks, func(first, second Task) int {
		return cmp.Or(
			compareLast(len(first.Tags) == 0, len(second.Tags) == 0),
			slices.Compare(sortedTags(first), sortedTags(second)),
		)
	})
}

// ByIssue orders tasks linked to an issue first, by key in natural order, so
// PROJ-2 comes before PROJ-10, with unlinked tasks last.
func ByIssue(tasks []Task) []Task {
	return sortedBy(tasks, func(first, second Task) int {
		return cmp.Or(compareLast(!first.Linked(), !second.Linked()), naturalCompare(first.IssueKey, second.IssueKey))
	})
}

// ByPriority orders high, medium and low, then any other value Taskwarrior was
// configured with, then tasks with no priority.
func ByPriority(tasks []Task) []Task {
	return sortedBy(tasks, func(first, second Task) int {
		return cmp.Or(
			cmp.Compare(priorityRank(first.Priority), priorityRank(second.Priority)),
			strings.Compare(first.Priority, second.Priority),
		)
	})
}

// sortedBy is a copy of tasks in first's order, every tie falling back to most
// urgent first, so each order is stable across a refresh.
func sortedBy(tasks []Task, first func(Task, Task) int) []Task {
	ordered := slices.Clone(tasks)

	slices.SortStableFunc(ordered, func(a, b Task) int {
		return cmp.Or(
			first(a, b),
			cmp.Compare(b.Urgency, a.Urgency),
			cmp.Compare(a.ID, b.ID),
			strings.Compare(a.UUID, b.UUID),
		)
	})

	return ordered
}

// compareLast orders whatever a condition holds for after whatever it does
// not: the tasks with no id, no tag or no issue go last.
func compareLast(firstLast, secondLast bool) int {
	switch {
	case firstLast == secondLast:
		return 0
	case firstLast:
		return 1
	default:
		return -1
	}
}

// sortedTags is a task's tags in order, copied: a task's tags share their array
// with every copy of it, so sorting them in place would reorder the caller's.
func sortedTags(task Task) []string {
	tags := slices.Clone(task.Tags)
	slices.Sort(tags)

	return tags
}

// namedPriorities is Taskwarrior's own priorities, highest first.
func namedPriorities() []string {
	return []string{"H", "M", "L"}
}

// priorityRank places Taskwarrior's priorities: the named ones in their order,
// then any other value Taskwarrior was configured with, then none.
func priorityRank(priority string) int {
	named := namedPriorities()
	if rank := slices.Index(named, priority); rank >= 0 {
		return rank
	}

	otherRank := len(named)
	noneRank := otherRank + 1

	if priority == "" {
		return noneRank
	}

	return otherRank
}

// naturalCompare compares two keys reading each run of ASCII digits as a
// number, so PROJ-2 comes before PROJ-10, and anything else byte by byte.
func naturalCompare(first, second string) int {
	for first != "" && second != "" {
		firstRun, firstRest := leadingRun(first)
		secondRun, secondRest := leadingRun(second)

		if order := compareRuns(firstRun, secondRun); order != 0 {
			return order
		}

		first, second = firstRest, secondRest
	}

	return cmp.Compare(len(first), len(second))
}

// leadingRun splits off the leading run of digits, or of anything else.
func leadingRun(text string) (string, string) {
	digits := isDigit(text[0])
	end := 1

	for end < len(text) && isDigit(text[end]) == digits {
		end++
	}

	return text[:end], text[end:]
}

// compareRuns compares two runs as numbers when both are digits, the shorter
// once leading zeros are gone being the smaller, and byte by byte otherwise.
func compareRuns(first, second string) int {
	if !isDigit(first[0]) || !isDigit(second[0]) {
		return strings.Compare(first, second)
	}

	first, second = strings.TrimLeft(first, "0"), strings.TrimLeft(second, "0")

	return cmp.Or(cmp.Compare(len(first), len(second)), strings.Compare(first, second))
}

// isDigit reports an ASCII digit.
func isDigit(b byte) bool {
	return '0' <= b && b <= '9'
}
