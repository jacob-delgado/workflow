// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

// How Taskwarrior's tasks are drawn outside their own pane's list: the task
// mark on the Issues rows, the Tasks block in an issue's detail, the active task
// at the spine's end, and that none of it lets a style run on past a row.

import (
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/jacob-delgado/workflow/internal/taskwarrior"
	"github.com/jacob-delgado/workflow/internal/tui"
)

// longTask is a description too long for one row of the detail, even at 120
// columns.
const longTask = issueKey + ": Fix token redaction in every log line the forge client writes, " +
	"and in the request log the web server keeps"

// requireStylesClosed fails the test if a row of a raw screen ends with a style
// still open: the terminal would carry it on into the next row, over the rail's
// text and borders.
func requireStylesClosed(t *testing.T, view string) {
	t.Helper()

	style := regexp.MustCompile(`\x1b\[[0-9;:]*m`)

	for index, row := range strings.Split(view, "\n") {
		codes := style.FindAllString(row, -1)
		if len(codes) > 0 && !slices.Contains([]string{"\x1b[m", "\x1b[0m"}, codes[len(codes)-1]) {
			t.Errorf("row %d ends with a style still open: %q", index, row)
		}
	}
}

// detailText is the detail pane's text, each row cut from the column where the
// detail shows anchor, a line only it draws, with the border and padding after
// it trimmed away.
func detailText(t *testing.T, view, anchor string) string {
	t.Helper()

	rows := strings.Split(plain(view), "\n")

	for _, row := range rows {
		before, _, found := strings.Cut(row, anchor)
		if !found {
			continue
		}

		column := ansi.StringWidth(before)
		texts := make([]string, len(rows))

		for index, each := range rows {
			texts[index] = strings.TrimRight(ansi.Cut(each, column, ansi.StringWidth(each)), " │┃")
		}

		return strings.Join(texts, "\n")
	}

	t.Fatalf("the screen does not show %q:\n%s", anchor, plain(view))

	return ""
}

// startedHoldingTheTaskLoad runs Init's loads one at a time and finishes each,
// except Taskwarrior's, whose answer is never delivered: the interface is still
// asking. The load is found by the read it records, not by its place in the
// batch.
func startedHoldingTheTaskLoad(t *testing.T, repo *world, model tui.Model) tui.Model {
	t.Helper()

	loads, ok := model.Init()().(tea.BatchMsg)
	if !ok {
		t.Fatal("Init did not start its loads as a batch")
	}

	held := false

	for _, load := range loads {
		if load == nil {
			continue
		}

		readsBefore := len(repo.asked("tasks"))
		msg := load()

		if len(repo.asked("tasks")) > readsBefore {
			held = true

			continue
		}

		model = drain(t, model, func() tea.Msg { return msg })
	}

	if !held {
		t.Fatal("Init never asked Taskwarrior")
	}

	return model
}

func TestNoStyleRunsOnPastTheEndOfARow(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		width, height int
		description   string
		keys          []string
		want          string
	}{
		// At 80x24 the selected task's faint facts are wider than the detail.
		"the Tasks detail's facts": {width: 80, height: 24, keys: []string{tasksPane}, want: "urgency 14.2"},
		"the Tasks detail's description": {
			width: 120, height: 40, description: longTask, keys: []string{tasksPane}, want: "urgency 14.2",
		},
		"an issue's long task": {width: 80, height: 24, description: longTask, want: "started 1h12m ago"},
		// A task just short of a row, so its note wraps.
		"an issue's task whose note wraps": {
			width: 80, height: 24, description: issueKey + ": Fix token redaction now!", want: "started 1h12m ago",
		},
		// So narrow the untracked issue's one line of how to track it wraps; the
		// collapsed layout shows the issue once it is read.
		"an untracked issue's hint": {
			width: 24, height: 20, keys: []string{downAction, downAction, keyEnter}, want: "T tracks it in",
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			repo := withAnUntrackedIssue()
			if tt.description != "" {
				changeTask(repo, activeTaskUUID, func(task *taskwarrior.Task) { task.Description = tt.description })
			}

			// Act
			view := typing(t, repo.live(t, tt.width, tt.height), tt.keys...).View().Content

			// Assert
			requireScreen(t, view, tt.want)
			requireStylesClosed(t, view)
		})
	}
}

func TestTheIssuesRowsCarryATaskMark(t *testing.T) {
	t.Parallel()

	// With no column, an issue's status glyph is followed by its key.
	noColumn := []string{"◐◐", "○○", "◐·", "○·", "○●"}

	cases := map[string]struct {
		arrange func(*world)
		want    []string
		refuse  []string
	}{
		// An active task, a task not started, and no task, each after the issue's
		// status glyph.
		"with Taskwarrior": {want: []string{"◐◐ " + issueKey, "○○ " + secondIssue, "○· " + untrackedIssue}},
		"without Taskwarrior the column is not drawn": {
			arrange: func(repo *world) { repo.tasks.none = true },
			want:    []string{"◐ " + issueKey, "○ " + untrackedIssue}, refuse: noColumn,
		},
		// go-task answers to `task` on this repository's own machines.
		"with go-task on PATH the column is not drawn": {
			arrange: func(repo *world) { repo.tasks.installErr = taskwarrior.ErrNotTaskwarrior },
			want:    []string{"▸ ◐ " + issueKey}, refuse: noColumn,
		},
		"after a failed read the column is not drawn": {
			arrange: func(repo *world) { repo.tasks.err = taskwarrior.ErrNotConfigured },
			want:    []string{"▸ ◐ " + issueKey}, refuse: noColumn,
		},
		// Only a completed task is done; a waiting or recurring one is still to do.
		"a waiting task is tracked": {
			arrange: func(repo *world) {
				repo.tasks.linked = append(repo.tasks.linked, untrackedTask(taskwarrior.Waiting))
			},
			want: []string{"○○ " + untrackedIssue}, refuse: []string{"○● " + untrackedIssue},
		},
		"a recurring task is tracked": {
			arrange: func(repo *world) {
				repo.tasks.linked = append(repo.tasks.linked, untrackedTask(taskwarrior.Recurring))
			},
			want: []string{"○○ " + untrackedIssue}, refuse: []string{"○● " + untrackedIssue},
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			repo := withAnUntrackedIssue()
			if tt.arrange != nil {
				tt.arrange(repo)
			}

			// Act
			view := repo.live(t, 120, 40).View().Content

			// Assert
			requireScreen(t, view, tt.want...)
			refuseScreen(t, view, tt.refuse...)
		})
	}
}

// untrackedTask is a task linked to the untracked issue, in a status.
func untrackedTask(status taskwarrior.Status) taskwarrior.Task {
	return taskwarrior.Task{
		UUID: addedTaskUUID, ID: 21, Description: "Rotate them", Status: status, IssueKey: untrackedIssue,
	}
}

func TestWhileTaskwarriorIsAskedTheIssuesRowsHaveNoTaskColumn(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := withTasks()
	model := sized(t, tui.New(repo.cfg, nil, repo.deps()), 120, 40)

	// Act
	view := startedHoldingTheTaskLoad(t, repo, model).View().Content

	// Assert
	requireScreen(t, view, "▸ ◐ "+issueKey)
	refuseScreen(t, view, "◐·", "○·")
}

func TestAnIssueWhoseTasksAreDoneIsMarkedDoneWithoutAStaleID(t *testing.T) {
	t.Parallel()

	// Arrange
	// Reads skip Taskwarrior's garbage collection, so a completed task still
	// carries the working-set id it had; the id means nothing now, and is not
	// shown.
	repo := withAnUntrackedIssue()
	repo.tasks.linked = append(repo.tasks.linked, taskwarrior.Task{
		UUID: addedTaskUUID, ID: 7, Description: "Ship the rotation", Status: taskwarrior.Completed,
		IssueKey: untrackedIssue, End: testNow().Add(-time.Hour),
	})

	// Act
	view := typing(t, repo.live(t, 120, 40), downAction, downAction).View().Content

	// Assert
	requireScreen(t, view, "○● "+untrackedIssue, "● Ship the rotation")
	refuseScreen(t, view, "#7")
}

func TestTheIssueDetailListsItsTasks(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		arrange func(*world)
		keys    []string
		want    []string
	}{
		// Under the issue's facts and a heading of their own, before who reported
		// the issue.
		"a tracked issue lists its task": {
			want: []string{
				"Bug · In Progress", tasksTitle, "◐ #12 " + issueKey + ": " + issueSummary + "  started 1h12m ago",
				"reported by " + reporter,
			},
		},
		"a task not started says when it is due": {
			arrange: func(repo *world) {
				changeTask(repo, trackedTaskUUID, func(task *taskwarrior.Task) { task.Due = testNow().Add(26 * time.Hour) })
			},
			keys: []string{downAction}, want: []string{"○ #3 " + secondIssue + ": Add retries  due in 1d 2h"},
		},
		// Waiting, a task is still to do, and still in the working set.
		"a waiting task keeps its id": {
			arrange: func(repo *world) {
				repo.tasks.linked = append(repo.tasks.linked, untrackedTask(taskwarrior.Waiting))
			},
			keys: []string{downAction, downAction}, want: []string{"○ #21 Rotate them"},
		},
		"a waiting task says when it is due": {
			arrange: func(repo *world) {
				waiting := untrackedTask(taskwarrior.Waiting)
				waiting.Due = testNow().Add(26 * time.Hour)
				repo.tasks.linked = append(repo.tasks.linked, waiting)
			},
			keys: []string{downAction, downAction}, want: []string{"Rotate them  due in 1d 2h"},
		},
		"an untracked issue says how to track it": {
			keys: []string{downAction, downAction}, want: []string{"T tracks it in Taskwarrior"},
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			repo := withAnUntrackedIssue()
			if tt.arrange != nil {
				tt.arrange(repo)
			}

			// Act
			view := typing(t, repo.live(t, 120, 40), tt.keys...).View().Content

			// Assert
			requireInOrder(t, view, tt.want...)
		})
	}
}

func TestAFailedTaskReadLeavesTheIssueDetailWithoutATasksBlock(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := withAnUntrackedIssue()
	repo.tasks.err = taskwarrior.ErrNotConfigured

	// Act
	view := typing(t, repo.live(t, 120, 40), downAction, downAction).View().Content

	// Assert
	requireScreen(t, view, "Rotate the keys")
	refuseScreen(t, view, "T tracks it in Taskwarrior")
}

func TestTheIssueDetailSetsItsTasksApart(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		detailErr error
		want      string
	}{
		"from who reported the issue": {want: "started 1h12m ago\n\nreported by " + reporter},
		// The failure already opens with a blank row, and gets no second.
		"from a full read that failed": {detailErr: errNotVisible, want: "started 1h12m ago\n\n✗ "},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			repo := withTasks()
			repo.detailErr = tt.detailErr

			// Act
			view := repo.live(t, 120, 40).View().Content

			// Assert
			text := detailText(t, view, "Bug · In Progress")
			requireScreen(t, text, tt.want)
			refuseScreen(t, text, "started 1h12m ago\n\n\n")
		})
	}
}

func TestAnIssuesLongTaskWrapsUnderItsIndent(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := withTasks()
	changeTask(repo, activeTaskUUID, func(task *taskwarrior.Task) { task.Description = longTask })

	// Act
	view := repo.live(t, 120, 40).View().Content

	// Assert
	rows := strings.Split(detailText(t, view, "Bug · In Progress"), "\n")

	first := slices.IndexFunc(rows, func(row string) bool { return strings.HasPrefix(row, "  ◐ #12 "+issueKey) })
	if first < 0 || first+1 >= len(rows) {
		t.Fatalf("the detail does not list task 12 under its indent:\n%s", strings.Join(rows, "\n"))
	}

	if next := rows[first+1]; !strings.HasPrefix(next, "  ") || strings.HasPrefix(next, "   ") {
		t.Errorf("the task's second row is not under its two-cell indent: %q", next)
	}
}

func TestAnIssuesTaskWhoseFirstRowFillsTheDetailKeepsItsIndent(t *testing.T) {
	t.Parallel()

	// Arrange
	// At 120 columns the detail is 80 wide: the task's first row, under its
	// two-cell indent, ends at "writes," in the last column.
	repo := withTasks()
	changeTask(repo, activeTaskUUID, func(task *taskwarrior.Task) {
		task.Description = issueKey + ": Fix token redaction in every log line the forge client writes, a request log too"
	})

	// Act
	view := repo.live(t, 120, 40).View().Content

	// Assert
	requireScreen(t, detailText(t, view, "Bug · In Progress"),
		"\n  ◐ #12 "+issueKey+": Fix token redaction in every log line the forge client writes,\n"+
			"  a request log too  started 1h12m ago\n")
}

func TestTheSpineShowsTheActiveTaskAndItsElapsedTime(t *testing.T) {
	t.Parallel()

	fixTokenRedaction := issueKey + ": " + issueSummary

	// The stages take 52 columns; the room after them, less a column of space,
	// is what the tail has. Its description gets what the glyph, the separator
	// and the time leave, cut with the ellipsis, and at least eight columns of
	// it or none at all — or all of a shorter one.
	cases := map[string]struct {
		width, height int
		started       time.Duration
		description   string
		want          string
		refuse        []string
	}{
		"minutes": {width: 120, height: 40, started: 12 * time.Minute, want: fixTokenRedaction + " · 12m"},
		"an hour": {width: 120, height: 40, started: time.Hour, want: fixTokenRedaction + " · 1h0m"},
		"hours":   {width: 120, height: 40, started: 72 * time.Minute, want: "◐ " + fixTokenRedaction + " · 1h12m"},
		"days":    {width: 120, height: 40, started: 51 * time.Hour, want: fixTokenRedaction + " · 2d 3h"},
		"eighty columns cut the description": {
			width: 80, height: 24, started: 72 * time.Minute, want: "◐ " + issueKey + ": Fix to… · 1h12m",
		},
		"room for eight columns of description": {
			width: 71, height: 24, started: 72 * time.Minute, want: "◐ PROJ-41… · 1h12m",
		},
		"room for fewer than eight": {
			width: 70, height: 24, started: 72 * time.Minute, want: "◐ 1h12m", refuse: []string{"PROJ-41"},
		},
		"room for all of a short description": {
			width: 70, height: 24, started: 72 * time.Minute, description: "Deploy", want: "◐ Deploy · 1h12m",
		},
		// A compact spine has room for how long, not what.
		"a compact spine": {
			width: 100, height: 22, started: 72 * time.Minute, want: "◐ 1h12m", refuse: []string{issueKey},
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			repo := withTasks()
			changeTask(repo, activeTaskUUID, func(task *taskwarrior.Task) {
				task.Start = testNow().Add(-tt.started)
				if tt.description != "" {
					task.Description = tt.description
				}
			})

			// Act
			spine := spineLine(plain(repo.live(t, tt.width, tt.height).View().Content))

			// Assert
			// Right-aligned: the tail ends at the spine's last column.
			drawn := strings.TrimRight(spine, " ")
			if !strings.HasSuffix(drawn, tt.want) || ansi.StringWidth(drawn) != tt.width {
				t.Errorf("the spine does not end with %q at column %d:\n%q", tt.want, tt.width, spine)
			}

			refuseScreen(t, spine, tt.refuse...)
		})
	}
}

func TestTheSpineDrawsNoTaskItWouldHaveToCut(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		width, height int
		stages        string
	}{
		"a full spine":    {width: 58, height: 24, stages: "○ Slack"},
		"a compact spine": {width: 22, height: 20, stages: "S○"},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			repo := withTasks()

			// Act
			spine := spineLine(plain(repo.live(t, tt.width, tt.height).View().Content))

			// Assert
			if drawn := strings.TrimRight(spine, " "); !strings.HasSuffix(drawn, tt.stages) {
				t.Errorf("the spine draws past its last stage %q: %q", tt.stages, spine)
			}
		})
	}
}

func TestTheSpineFindsTheActiveTaskInEitherList(t *testing.T) {
	t.Parallel()

	cases := map[string]func(*world){
		// A context can hide the active task from the pending list; it is still
		// linked to its issue.
		"only linked": func(repo *world) {
			repo.tasks.pending = onlyTasks(repo.tasks.pending, trackedTaskUUID, looseTaskUUID)
		},
		"only pending": func(repo *world) { repo.tasks.linked = onlyTasks(repo.tasks.linked, trackedTaskUUID) },
	}

	for name, arrange := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			repo := withTasks()
			arrange(repo)

			// Act
			spine := spineLine(plain(repo.live(t, 120, 40).View().Content))

			// Assert
			requireScreen(t, spine, issueKey+": "+issueSummary+" · 1h12m")
		})
	}
}

func TestTheSpineSaysNothingWhenNoTaskIsActive(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := withTasks()
	changeTask(repo, activeTaskUUID, func(task *taskwarrior.Task) { task.Start = time.Time{} })

	// Act
	spine := spineLine(plain(repo.live(t, 120, 40).View().Content))

	// Assert
	refuseScreen(t, spine, issueSummary, "1h12m")
}

func TestTheSpineShowsNoTaskUntilTaskwarriorHasAnswered(t *testing.T) {
	t.Parallel()

	// Arrange
	// The pending list answers with the started task, but the linked read fails,
	// so Taskwarrior has not answered: the Tasks pane shows the failure, and the
	// spine shows no task it cannot vouch for.
	repo := withTasks()
	repo.tasks.linkedErr = fmt.Errorf("%w: unexpected end of JSON input", taskwarrior.ErrBadOutput)

	// Act
	spine := spineLine(plain(repo.live(t, 120, 40).View().Content))

	// Assert
	refuseScreen(t, spine, issueSummary, "1h12m")
}

func TestTaskLayoutStillFitsTheTerminal(t *testing.T) {
	t.Parallel()

	for _, size := range [][2]int{{120, 40}, {90, 24}, {80, 24}, {80, 30}, {200, 60}} {
		t.Run(strconv.Itoa(size[0])+"x"+strconv.Itoa(size[1]), func(t *testing.T) {
			t.Parallel()

			// Arrange
			repo := withTasks()

			// Act
			view := typing(t, repo.live(t, size[0], size[1]), tasksPane).View().Content

			// Assert
			rows := strings.Split(view, "\n")
			if len(rows) > size[1] {
				t.Errorf("rendered %d rows", len(rows))
			}

			for index, row := range rows {
				if width := lipgloss.Width(row); width > size[0] {
					t.Errorf("row %d is %d cells wide", index, width)
				}
			}
		})
	}
}
