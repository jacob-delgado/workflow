// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package armed

import (
	"time"

	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/loop"
)

// HeadWatch is a CIWatch over a pull request's head: how long the reads heard
// on that head have reported no check, which a read of another head, as a
// push brings, starts again. A merge with no CI reported is offered only once
// it has heard NoCIPatience of none, as long as an announcement armed to go
// once CI passes waits: before that, no check reads as checks not reported
// yet, which a forge lists a moment after a push.
//
// It counts only the reads its surface heard, so a head quiet since long
// before the surface started is offered the merge once it has been quiet for
// NoCIPatience of them. The zero HeadWatch has heard nothing.
type HeadWatch struct {
	head    string
	watch   CIWatch
	verdict Verdict
}

// Read is the watch after a read of pull's CI, heard at readAt, reported run.
func (w HeadWatch) Read(pull forge.PullRequest, run forge.CI, readAt time.Time) HeadWatch {
	watch := w.watch
	if pull.HeadCommit != w.head {
		watch = WatchFrom(time.Time{})
	}

	watch, verdict := watch.Read(run, readAt)

	return HeadWatch{head: pull.HeadCommit, watch: watch, verdict: verdict}
}

// Unheard is the watch after a read that failed: the quiet heard so far is
// kept, and the time until the next read heard is not counted.
func (w HeadWatch) Unheard() HeadWatch {
	return HeadWatch{head: w.head, watch: w.watch.Unheard(), verdict: w.verdict}
}

// NoCI reports that pull's head has reported no check for NoCIPatience of
// reads heard.
func (w HeadWatch) NoCI(pull forge.PullRequest) bool {
	return w.head == pull.HeadCommit && w.verdict == VerdictNoCI
}

// CanMerge is loop.CanMerge, taking checks that report none only once the
// head is NoCI.
func (w HeadWatch) CanMerge(pull forge.PullRequest, checks forge.CI) bool {
	return loop.CanMerge(pull, checks) && (checks.State != forge.CINone || w.NoCI(pull))
}
