// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package notice_test

import (
	"testing"

	"github.com/jacob-delgado/workflow/internal/tui/notice"
)

const (
	uncovered = false
	covered   = true

	// news is the report a test makes, and lost the report no pane repeats
	// it makes, shown in a view's after as lostAfter.
	news      = "pulled"
	lost      = "lost"
	lostAfter = "[" + lost + "]"
)

// none is a Footer that reports nothing.
func none(*testing.T) notice.Footer {
	return notice.Footer{}
}

// plain is a Footer reporting "staged", plainly.
func plain(t *testing.T) notice.Footer {
	t.Helper()

	return none(t).Reported("staged")
}

// failure is a Footer reporting "push failed" as a failure.
func failure(t *testing.T) notice.Footer {
	t.Helper()

	return none(t).Failed("push failed")
}

// unrepeated is a Footer showing "not kept", which no pane repeats.
func unrepeated(t *testing.T) notice.Footer {
	t.Helper()

	return none(t).Unrepeated("not kept", uncovered)
}

// unrepeatedAndWaiting is a Footer showing "not kept" with "also not kept"
// waiting behind it.
func unrepeatedAndWaiting(t *testing.T) notice.Footer {
	t.Helper()

	return unrepeated(t).Unrepeated("also not kept", uncovered)
}

// oneWaiting is a Footer reporting nothing, with "also not kept" waiting.
func oneWaiting(t *testing.T) notice.Footer {
	t.Helper()

	return unrepeatedAndWaiting(t).Cleared()
}

// view is everything the notice row can be seen to report of a Footer: what
// it shows, and what shows after it, one key at a time, until nothing is
// left waiting.
type view struct {
	stage notice.Stage
	text  string
	after string
}

func viewOf(footer notice.Footer) view {
	seen := view{stage: footer.Stage(), text: footer.Text(), after: ""}

	for next := footer.Cleared().Next(uncovered); next.Stage() != notice.StageNone; next = next.Cleared().Next(uncovered) {
		seen.after += "[" + next.Text() + "]"
	}

	return seen
}
