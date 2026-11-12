// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package armed_test

import (
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/armed"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/loop"
	"github.com/jacob-delgado/workflow/internal/messaging"
)

// heldHead is the commit the held pull request is at on the forge.
const heldHead = "a1b2c3d4"

// heldPull is the open pull request #9 these tests hold an announcement for,
// at heldHead on the forge.
func heldPull() forge.PullRequest {
	pull := openPull()
	pull.HeadCommit = heldHead

	return pull
}

// otherPull is another open pull request, #12.
func otherPull() forge.PullRequest {
	return forge.PullRequest{
		Number: 12, URL: "https://github.com/acme/widgets/pull/12", Title: "Rotate the key", HeadCommit: "e5f6",
	}
}

// deliveryOf is the ready-for-review announcement of pull, to #dev.
func deliveryOf(pull forge.PullRequest) loop.Delivery {
	return loop.Delivery{
		Channel: "#dev", Text: pull.Title + " is ready for review", Pull: pull, Head: "local-head",
		Moment: messaging.MomentReady,
	}
}

// arming is the announcement of heldPull asked for, found by branchName.
func arming() armed.Arming {
	return armed.Arming{Delivery: deliveryOf(heldPull()), Branch: branchName}
}

// opening is arming for a pull request just opened, armed on its own.
func opening() armed.Arming {
	arm := arming()
	arm.Opened, arm.Automatic, arm.Unasked = true, true, true

	return arm
}

// after is the time after has passed since armedAt.
func after(elapsed time.Duration) time.Time {
	return armedAt().Add(elapsed)
}

// armedWith is an idle hold armed with arm at armedAt.
func armedWith(t *testing.T, arm armed.Arming) armed.Hold {
	t.Helper()

	got, err := armed.Hold{}.Arm(armedAt(), arm)
	if err != nil {
		t.Fatalf("arming an idle hold: %v", err)
	}

	return got.Hold
}

// heardAs is hold stepped on a read, after elapsed, that found pull with its
// CI at state.
func heardAs(hold armed.Hold, pull forge.PullRequest, state forge.CIState, elapsed time.Duration) armed.Hold {
	stepped, _ := hold.Heard(armed.Reading{
		Round: hold.Round(), Pull: pull, Found: true, CI: forge.CI{State: state}, At: after(elapsed),
	})

	return stepped
}

// unheardAt is hold stepped on a read, after elapsed, that failed with errSeam.
func unheardAt(hold armed.Hold, elapsed time.Duration) armed.Hold {
	stepped, _ := hold.Unheard(armed.Failure{Round: hold.Round(), Err: errSeam, At: after(elapsed)})

	return stepped
}

// deliveredAs is hold stepped on its post's outcome, err, after elapsed.
func deliveredAs(hold armed.Hold, err error, elapsed time.Duration) armed.Hold {
	stepped, _ := hold.Delivered(armed.Outcome{Round: hold.Round(), Err: err, At: after(elapsed)})

	return stepped
}

// The holds the tables start from, each reached through the hold's own
// events, as a surface reaches it.

func idle(*testing.T) armed.Hold { return armed.Hold{} }

func listing(t *testing.T) armed.Hold {
	t.Helper()

	return armedWith(t, opening())
}

func awaitingChecks(t *testing.T) armed.Hold {
	t.Helper()

	return armedWith(t, arming())
}

func running(t *testing.T) armed.Hold {
	t.Helper()

	return heardAs(armedWith(t, arming()), heldPull(), forge.CIRunning, time.Minute)
}

func awaitingFix(t *testing.T) armed.Hold {
	t.Helper()

	return heardAs(armedWith(t, arming()), heldPull(), forge.CIFailed, time.Minute)
}

func unheard(t *testing.T) armed.Hold {
	t.Helper()

	return unheardAt(running(t), 2*time.Minute)
}

func due(t *testing.T) armed.Hold {
	t.Helper()

	return heardAs(armedWith(t, arming()), heldPull(), forge.CIPassed, time.Minute)
}

func posted(t *testing.T) armed.Hold {
	t.Helper()

	return deliveredAs(due(t), nil, 2*time.Minute)
}

func dropped(t *testing.T) armed.Hold {
	t.Helper()

	return heardAs(running(t), otherPull(), forge.CIRunning, 2*time.Minute)
}

func stopped(t *testing.T) armed.Hold {
	t.Helper()

	hold, _ := running(t).Stop()

	return hold
}

// stageName names each stage, for a failure's message.
func stageName(stage armed.Stage) string {
	return map[armed.Stage]string{
		armed.StageIdle: "idle", armed.StageListing: "listing", armed.StageAwaitingChecks: "awaiting checks",
		armed.StageRunning: "running", armed.StageAwaitingFix: "awaiting a fix", armed.StageUnheard: "unheard",
		armed.StageDue: "due", armed.StagePosted: "posted", armed.StageElsewhere: "elsewhere",
		armed.StageDropped: "dropped", armed.StageStopped: "stopped",
	}[stage]
}

// nextName names each effect, for a failure's message.
func nextName(next armed.Next) string {
	return map[armed.Next]string{
		armed.NextNothing: "nothing", armed.NextRead: "read", armed.NextReadLater: "read later",
		armed.NextDeliver: "deliver",
	}[next]
}
