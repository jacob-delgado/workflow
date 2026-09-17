// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package hooks_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/hooks"
)

// Hook names the tests share.
const (
	preCommit = "pre-commit"
	commitMsg = "commit-msg"
	prePush   = "pre-push"
)

// lefthookRun is real lefthook 2.1 output with no terminal, once the colors are
// stripped: a skipped job, one that passed, one that failed, and the summary.
func lefthookRun() []string {
	return strings.Split(`╭──────────────────────────────────────╮
│ 🥊 lefthook  v2.1.14   hook:  pre-commit │
╰──────────────────────────────────────╯
│  skipped (skip) no matching staged files
┃  passes ❯ 
fine

┃  fails ❯ 
main.go:3:1: something is wrong (fakelint)

exit status 1
  ────────────────────────────────────
summary: (done in 0.06 seconds)

✔️ passes (0.01 seconds)
🥊 fails (0.01 seconds)`, "\n")
}

func TestJobsReadHowEachJobEnded(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		lines []string
		want  []hooks.Job
	}{
		"a finished run": {
			lines: lefthookRun(),
			want: []hooks.Job{
				{Name: "skipped", State: hooks.JobSkipped, Duration: ""},
				{Name: "passes", State: hooks.JobPassed, Duration: "0.01 seconds"},
				{Name: "fails", State: hooks.JobFailed, Duration: "0.01 seconds"},
			},
		},
		// Streamed output arrives a line at a time; a job has started when its
		// header has, and has not finished until the summary says so.
		"a run before its summary": {
			lines: lefthookRun()[:8],
			want: []hooks.Job{
				{Name: "skipped", State: hooks.JobSkipped, Duration: ""},
				{Name: "passes", State: hooks.JobRunning, Duration: ""},
				{Name: "fails", State: hooks.JobRunning, Duration: ""},
			},
		},
		// With --colors=off lefthook marks the summary with plain check marks.
		"the colorless glyphs": {
			lines: []string{"┃  lint ❯ ", "summary: (done in 1.2 seconds)", "✓ lint (1.1 seconds)", "✗ test (0.1 second)"},
			want: []hooks.Job{
				{Name: "lint", State: hooks.JobPassed, Duration: "1.1 seconds"},
				{Name: "test", State: hooks.JobFailed, Duration: "0.1 second"},
			},
		},
		"output that is not lefthook's": {lines: []string{"On branch main", "nothing to commit"}, want: nil},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			got := hooks.Jobs(tt.lines)

			// Assert
			if !slices.Equal(got, tt.want) {
				t.Errorf("Jobs = %+v\nwant   %+v", got, tt.want)
			}
		})
	}
}

func TestFailuresFindWhereEachToolPoints(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		lines []string
		want  []hooks.Location
	}{
		"each tool's way of naming a place": {
			lines: []string{
				// golangci-lint and go vet: file:line:column: message.
				"internal/tui/pane.go:64:1: cyclomatic complexity 12 of func `Update` is high (> 10) (gocyclo)",
				// markdownlint: file:line:column and a rule.
				"README.md:12:81 MD013/line-length Line length [Expected: 80; Actual: 96]",
				// a compiler: file:line: message, indented.
				"  cmd/workflow/main.go:9: undefined: run",
				// shellcheck's default format names the file and line in a sentence.
				"In scripts/check.sh line 17:",
				// typos points with an arrow.
				"  ╭▸ docs/usage.md:4:10",
				// The same place twice is one failure.
				"internal/tui/pane.go:64:1: cyclomatic complexity 12 of func `Update` is high (> 10) (gocyclo)",
			},
			want: []hooks.Location{
				{
					File: "internal/tui/pane.go", Line: 64, Column: 1,
					Message: "cyclomatic complexity 12 of func `Update` is high (> 10) (gocyclo)",
				},
				{File: "README.md", Line: 12, Column: 81, Message: "MD013/line-length Line length [Expected: 80; Actual: 96]"},
				{File: "cmd/workflow/main.go", Line: 9, Column: 0, Message: "undefined: run"},
				{File: "scripts/check.sh", Line: 17, Column: 0, Message: ""},
				{File: "docs/usage.md", Line: 4, Column: 10, Message: ""},
			},
		},
		"what only looks like a place": {
			lines: []string{
				"summary: (done in 0.06 seconds)",
				"started at 15:52:01",
				"see https://example.com:8080/docs for help",
				"exit status 1",
				"fine",
				"no extension:12:3: nope",
			},
			want: nil,
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			got := hooks.Failures(tt.lines)

			// Assert
			if !slices.Equal(got, tt.want) {
				t.Errorf("Failures =\n%+v\nwant\n%+v", got, tt.want)
			}
		})
	}
}
