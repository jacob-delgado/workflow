// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package report_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/httpx"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/messaging"
	"github.com/jacob-delgado/workflow/internal/report"
)

// errUnforeseen is a failure no class explains.
var errUnforeseen = errors.New("something no class explains")

func TestAFaultsDetailIsTheProblemsWordsWithoutTheHost(t *testing.T) {
	t.Parallel()

	// Arrange
	unreachable := fmt.Errorf("%w: https://jira.internal.example", jira.ErrUnreachable)

	// Act
	detail := report.FaultDetail(unreachable)

	// Assert
	if strings.Contains(detail, "internal.example") || !strings.Contains(detail, "could not be reached") {
		t.Errorf("FaultDetail = %q, want the problem's words for an unreachable service, no host", detail)
	}
}

func TestEverySharedFaultIsToldByItsOwnClass(t *testing.T) {
	t.Parallel()

	for _, class := range report.SharedFaults().All() {
		for _, cause := range class.Causes {
			t.Run(cause.Error(), func(t *testing.T) {
				t.Parallel()

				// Arrange
				// The cause's own words carry an address, which the detail never does.
				failed := fmt.Errorf("asking https://internal.example: %w", cause)

				// Act
				told, classified := report.Classify(failed, report.SharedFaults().All())

				// Assert
				if !classified || told.Detail != class.Detail {
					t.Errorf("Classify(%v) = %q, %v; want its class's %q", failed, told.Detail, classified, class.Detail)
				}

				if strings.Contains(told.Detail, "internal.example") {
					t.Errorf("Classify(%v) names the address: %q", failed, told.Detail)
				}
			})
		}
	}
}

func TestAFaultNothingSetUpIsToldAsNotSetUp(t *testing.T) {
	t.Parallel()

	// Act
	told, _ := report.Classify(jira.ErrNoCredential, report.SharedFaults().All())

	// Assert
	if told.Code != api.ProblemCodeNotSetUp {
		t.Errorf("Classify(%v) code = %q, want %q", jira.ErrNoCredential, told.Code, api.ProblemCodeNotSetUp)
	}
}

func TestARefusalKeepsItsClassesCode(t *testing.T) {
	t.Parallel()

	// Act
	told, _ := report.Classify(jira.ErrRejected, report.SharedFaults().All())

	// Assert
	if told.Code != api.ProblemCodeUnprocessable {
		t.Errorf("Classify(%v) code = %q, want %q", jira.ErrRejected, told.Code, api.ProblemCodeUnprocessable)
	}
}

func TestARateLimitSaysHowLongItAskedToWait(t *testing.T) {
	t.Parallel()

	// Arrange
	limited := fmt.Errorf("asking the forge: %w", &httpx.RateLimitError{Wait: 30 * time.Second})

	// Act
	told, _ := report.Classify(limited, report.SharedFaults().All())

	// Assert
	if told.RetryAfter == nil || *told.RetryAfter != 30 {
		t.Errorf("Classify(%v) RetryAfter = %v, want 30 seconds", limited, told.RetryAfter)
	}
}

func TestAFailureThatIsNoRateLimitAsksNoWait(t *testing.T) {
	t.Parallel()

	// Act
	told, _ := report.Classify(jira.ErrUnreachable, report.SharedFaults().All())

	// Assert
	if told.RetryAfter != nil {
		t.Errorf("Classify(%v) RetryAfter = %d, want none", jira.ErrUnreachable, *told.RetryAfter)
	}
}

func TestATokenTheForgeTurnedDownIsToldInTheForgesAdvice(t *testing.T) {
	t.Parallel()

	// Arrange
	refused := &forge.RefusalError{Kind: forge.KindGitHub, Status: forge.ErrRefused, Reason: "Resource not accessible"}

	// Act
	told, classified := report.Classify(refused, nil)

	// Assert
	if !classified || !strings.Contains(told.Detail, "Resource not accessible") {
		t.Errorf("Classify(%v) = %q, %v; want the forge's advice with its reason", refused, told.Detail, classified)
	}
}

func TestAMessageSlackWouldNotDeliverIsToldWithItsReason(t *testing.T) {
	t.Parallel()

	// Arrange
	refused := messaging.PostRefusedError{Reason: "invite the app to #general"}

	// Act
	told, classified := report.Classify(refused, nil)

	// Assert
	if !classified || !strings.Contains(told.Detail, "invite the app to #general") {
		t.Errorf("Classify(%v) = %q, %v; want Slack's refusal and its fix", refused, told.Detail, classified)
	}
}

func TestAFailureNoClassExplainsIsAnInternalOneWithoutItsWords(t *testing.T) {
	t.Parallel()

	// Act
	told, classified := report.Classify(errUnforeseen, report.SharedFaults().All())

	// Assert
	if classified || told.Code != api.ProblemCodeInternal {
		t.Errorf("Classify(%v) = %q, %v; want an internal failure, unclassified", errUnforeseen, told.Code, classified)
	}

	if strings.Contains(told.Detail, errUnforeseen.Error()) || !strings.Contains(told.Detail, report.TryAgain) {
		t.Errorf("Classify(%v) detail = %q, want what to do and none of its words", errUnforeseen, told.Detail)
	}
}

func TestAClassMatchesOnlyItsOwnCauses(t *testing.T) {
	t.Parallel()

	// Arrange
	class := report.Class{Causes: []error{jira.ErrNotFound}, Code: api.ProblemCodeNotFound, Detail: "not found"}

	// Act
	matched := class.Matches(jira.ErrUnreachable)

	// Assert
	if matched {
		t.Errorf("a class of %v matches %v", jira.ErrNotFound, jira.ErrUnreachable)
	}
}

func TestTheTaskListAndAWriteShareTaskwarriorsReasons(t *testing.T) {
	t.Parallel()

	for _, reason := range report.TaskwarriorReasons() {
		t.Run(reason.Cause.Error(), func(t *testing.T) {
			t.Parallel()

			// Act
			told, _ := report.Classify(reason.Cause, report.SharedFaults().Taskwarrior)

			// Assert
			if told.Detail != reason.Text {
				t.Errorf("a write refused for %v says %q, want the task list's %q", reason.Cause, told.Detail, reason.Text)
			}
		})
	}
}
