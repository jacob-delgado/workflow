// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package activity_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/activity"
)

var errUnreachable = errors.New("unreachable")

func TestTheCopyableTextNestsTheGroupingAndNamesWhatIsMissing(t *testing.T) {
	t.Parallel()

	// Arrange
	period, err := activity.NewPeriod(day(t, "2026-10-02"), day(t, "2026-10-02"))
	if err != nil {
		t.Fatalf("NewPeriod: %v", err)
	}

	summary := activity.Summary{
		Period: period,
		Reads: []activity.Read{
			{Source: activity.SourceGit, Items: []activity.Item{
				{At: at(t, "2026-10-02T09:15:00Z"), Kind: activity.Committed, Ref: "abc1234", Title: "Fix the token leak"},
			}},
			{Source: activity.SourceJira, Failed: errUnreachable},
			{Source: activity.SourceForge, Truncated: true, Items: []activity.Item{
				{At: at(t, "2026-10-02T15:40:00Z"), Kind: activity.PullOpened, Ref: "#42", Title: "Redact tokens"},
			}},
		},
	}

	// Act
	text := summary.Text(time.UTC)

	// Assert
	want := strings.Join([]string{
		"# 2026-10-02",
		"",
		"## 2026",
		"",
		"### 2026-10 October",
		"",
		"#### 2026-10-02 Friday",
		"",
		"##### 09:00",
		"",
		"- committed abc1234 Fix the token leak",
		"",
		"##### 15:00",
		"",
		"- opened #42 Redact tokens",
		"",
		"Jira could not be read.",
		"The forge had more than this shows.",
		"",
	}, "\n")
	if text != want {
		t.Errorf("Text =\n%s\nwant\n%s", text, want)
	}
}

func TestAPeriodOfSeveralDaysIsNamedByItsEnds(t *testing.T) {
	t.Parallel()

	// Arrange
	period, err := activity.NewPeriod(day(t, "2026-10-02"), day(t, "2026-10-04"))
	if err != nil {
		t.Fatalf("NewPeriod: %v", err)
	}

	// Act
	text := activity.Summary{Period: period}.Text(time.UTC)

	// Assert
	if !strings.HasPrefix(text, "# 2026-10-02 to 2026-10-04\n\nNothing was done") {
		t.Errorf("Text = %q, want the period named by its ends and nothing done said", text)
	}
}

func TestTheCopyableTextEscapesATitlesMarkdown(t *testing.T) {
	t.Parallel()

	// Arrange
	period, err := activity.NewPeriod(day(t, "2026-10-02"), day(t, "2026-10-02"))
	if err != nil {
		t.Fatalf("NewPeriod: %v", err)
	}

	summary := activity.Summary{Period: period, Reads: []activity.Read{{Source: activity.SourceGit, Items: []activity.Item{
		{At: at(t, "2026-10-02T09:15:00Z"), Kind: activity.Committed, Ref: "def5678", Title: "[Click](https://evil) *now*"},
	}}}}

	// Act
	text := summary.Text(time.UTC)

	// Assert
	if want := `- committed def5678 \[Click\]\(https://evil\) \*now\*`; !strings.Contains(text, want+"\n") {
		t.Errorf("Text =\n%s\nwant the title's Markdown escaped: %s", text, want)
	}
}
