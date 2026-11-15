// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package notice_test

import (
	"testing"

	"github.com/jacob-delgado/workflow/internal/tui/notice"
)

func TestTheZeroFooterReportsNothing(t *testing.T) {
	t.Parallel()

	// Arrange
	var footer notice.Footer

	// Act
	got := viewOf(footer)

	// Assert
	if got != (view{stage: notice.StageNone, text: "", after: ""}) || footer.Waiting() != 0 {
		t.Errorf("the zero Footer is %+v with %d waiting; want nothing reported or waiting", got, footer.Waiting())
	}
}

func TestAReportTakesTheRowItIsOwed(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		from   func(*testing.T) notice.Footer
		report func(notice.Footer, string) notice.Footer
		want   view
	}{
		"a plain report takes an empty row": {
			from: none, report: notice.Footer.Reported, want: view{stage: notice.StagePlain, text: news, after: ""},
		},
		"a plain report replaces a plain report": {
			from: plain, report: notice.Footer.Reported, want: view{stage: notice.StagePlain, text: news, after: ""},
		},
		"a plain report is dropped under a failure": {
			from: failure, report: notice.Footer.Reported,
			want: view{stage: notice.StageFailure, text: "push failed", after: ""},
		},
		"a plain report puts what no pane repeats first in line": {
			from: unrepeatedAndWaiting, report: notice.Footer.Reported,
			want: view{stage: notice.StagePlain, text: news, after: "[not kept][also not kept]"},
		},
		"a failure takes an empty row": {
			from: none, report: failed, want: view{stage: notice.StageFailure, text: news, after: ""},
		},
		"a failure replaces a plain report": {
			from: plain, report: failed, want: view{stage: notice.StageFailure, text: news, after: ""},
		},
		"a failure replaces an earlier failure": {
			from: failure, report: failed, want: view{stage: notice.StageFailure, text: news, after: ""},
		},
		"a failure puts what no pane repeats first in line": {
			from: unrepeated, report: failed,
			want: view{stage: notice.StageFailure, text: news, after: "[not kept]"},
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			footer := tt.from(t)

			// Act
			reported := tt.report(footer, news)

			// Assert
			if got := viewOf(reported); got != tt.want {
				t.Errorf("the report is %+v; want %+v", got, tt.want)
			}
		})
	}
}

func TestAReportNoPaneRepeatsShowsOnARowFreeToBeRead(t *testing.T) {
	t.Parallel()

	cases := map[string]func(*testing.T) notice.Footer{
		"an empty row": none, "a plain report's": plain,
	}

	for name, from := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			footer := from(t)

			// Act
			shown := footer.Unrepeated(lost, uncovered)

			// Assert
			if got := viewOf(shown); got != (view{stage: notice.StageUnrepeated, text: lost, after: ""}) {
				t.Errorf("Unrepeated is %+v; want lost shown, nothing after", got)
			}
		})
	}
}

func TestAReportNoPaneRepeatsWaitsLastInLineWhenItCannotBeRead(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		from    func(*testing.T) notice.Footer
		covered bool
		want    view
	}{
		"over a failure": {
			from: failure, covered: uncovered,
			want: view{stage: notice.StageFailure, text: "push failed", after: lostAfter},
		},
		"over another such report": {
			from: unrepeated, covered: uncovered,
			want: view{stage: notice.StageUnrepeated, text: "not kept", after: lostAfter},
		},
		"behind others waiting": {
			from: oneWaiting, covered: uncovered,
			want: view{stage: notice.StageNone, text: "", after: alsoLostAfter + lostAfter},
		},
		"under a search": {
			from: none, covered: covered,
			want: view{stage: notice.StageNone, text: "", after: lostAfter},
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			footer := tt.from(t)

			// Act
			waiting := footer.Unrepeated(lost, tt.covered)

			// Assert
			if got := viewOf(waiting); got != tt.want {
				t.Errorf("Unrepeated is %+v; want %+v", got, tt.want)
			}
		})
	}
}

func TestClearingDropsTheReportShownAndKeepsTheLine(t *testing.T) {
	t.Parallel()

	// Arrange
	footer := unrepeatedAndWaiting(t)

	// Act
	cleared := footer.Cleared()

	// Assert
	if got := viewOf(cleared); got != (view{stage: notice.StageNone, text: "", after: alsoLostAfter}) {
		t.Errorf("Cleared is %+v; want nothing shown, also not kept after", got)
	}
}

func TestTheNextReportWaitingTakesAFreeRow(t *testing.T) {
	t.Parallel()

	// Arrange
	footer := oneWaiting(t)

	// Act
	next := footer.Next(uncovered)

	// Assert
	if got := viewOf(next); got != (view{stage: notice.StageUnrepeated, text: alsoLost, after: ""}) {
		t.Errorf("Next is %+v; want also not kept shown, nothing after", got)
	}
}

func TestTheNextReportWaitsWhileTheRowIsNotFree(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		from    func(*testing.T) notice.Footer
		covered bool
	}{
		"a report shown":  {from: unrepeatedAndWaiting, covered: uncovered},
		"under a search":  {from: oneWaiting, covered: covered},
		"nothing waiting": {from: none, covered: uncovered},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			footer := tt.from(t)

			// Act
			next := footer.Next(tt.covered)

			// Assert
			if got, want := viewOf(next), viewOf(footer); got != want || next.Waiting() != footer.Waiting() {
				t.Errorf("Next is %+v with %d waiting; want %+v with %d unchanged",
					got, next.Waiting(), want, footer.Waiting())
			}
		})
	}
}

func TestATransitionLeavesTheFooterItCameFromAsItWas(t *testing.T) {
	t.Parallel()

	// Arrange
	// Three waiting leave room in the array they are held in for a fourth.
	footer := failure(t).Unrepeated("one", uncovered).Unrepeated("two", uncovered).Unrepeated("three", uncovered)
	first := footer.Unrepeated("four", uncovered)

	// Act
	second := footer.Unrepeated("five", uncovered)

	// Assert
	if got := viewOf(first).after; got != "[one][two][three][four]" {
		t.Errorf("the first Footer has %s waiting after another was made from its source; want [one][two][three][four]",
			got)
	}

	if got := viewOf(second).after; got != "[one][two][three][five]" {
		t.Errorf("the second Footer has %s waiting; want [one][two][three][five]", got)
	}
}

func TestAFailureUnderASearchWaitsFirstInLine(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		from func(*testing.T) notice.Footer
		want view
	}{
		"behind nothing": {
			from: none, want: view{stage: notice.StageNone, text: "", after: "[" + news + "]"},
		},
		"ahead of what no pane repeats": {
			from: oneWaiting, want: view{stage: notice.StageNone, text: "", after: "[" + news + "]" + alsoLostAfter},
		},
		"under the report shown": {
			from: plain, want: view{stage: notice.StagePlain, text: "staged", after: "[" + news + "]"},
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			footer := tt.from(t)

			// Act
			waiting := footer.Failed(news, covered)

			// Assert
			if got := viewOf(waiting); got != tt.want {
				t.Errorf("Failed under a search is %+v; want %+v", got, tt.want)
			}
		})
	}
}

func TestAFailureWaitingTakesTheRowOnceTheSearchEnds(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		from func(*testing.T) notice.Footer
		want view
	}{
		"from nothing": {
			from: none, want: view{stage: notice.StageFailure, text: news, after: ""},
		},
		"from a plain report, which is dropped": {
			from: plain, want: view{stage: notice.StageFailure, text: news, after: ""},
		},
		"from an earlier failure, which is dropped": {
			from: failure, want: view{stage: notice.StageFailure, text: news, after: ""},
		},
		"from what no pane repeats, which waits first in line": {
			from: unrepeated, want: view{stage: notice.StageFailure, text: news, after: "[not kept]"},
		},
		"ahead of what no pane repeats waiting before it": {
			from: oneWaiting, want: view{stage: notice.StageFailure, text: news, after: alsoLostAfter},
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			footer := tt.from(t).Failed(news, covered)

			// Act
			next := footer.Next(uncovered)

			// Assert
			if got := viewOf(next); got != tt.want {
				t.Errorf("Next is %+v; want %+v", got, tt.want)
			}
		})
	}
}

func TestALaterFailureDropsTheOneWaiting(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		covered bool
		want    view
	}{
		"under the search": {
			covered: covered, want: view{stage: notice.StageNone, text: "", after: "[pull failed]" + alsoLostAfter},
		},
		"once it closed": {
			covered: uncovered, want: view{stage: notice.StageFailure, text: "pull failed", after: alsoLostAfter},
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			footer := oneWaiting(t).Failed("push failed", covered)

			// Act
			later := footer.Failed("pull failed", tt.covered)

			// Assert
			if got := viewOf(later); got != tt.want {
				t.Errorf("the later failure is %+v; want %+v", got, tt.want)
			}
		})
	}
}
