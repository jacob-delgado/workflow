// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package forge

// CIState is where CI stands on a pull request.
type CIState int

const (
	// CINone means nothing reports CI for this change.
	CINone CIState = iota
	// CIRunning means something is still to finish.
	CIRunning
	// CIPassed means everything finished without failing.
	CIPassed
	// CIFailed means something failed, whatever else is still running.
	CIFailed
)

// CI is how CI stands, and how far through it is where the forge says.
type CI struct {
	State CIState
	// Total, Done and Failed count checks where the forge reports them one by
	// one; GitLab reports a pipeline as a whole, so they stay zero there.
	Total, Done, Failed int
}

// succeeded is what GitHub's statuses, its check runs and GitLab's pipelines all
// call a pass.
const succeeded = "success"

// ciTally counts checks toward a CI.
type ciTally struct {
	total, done, failed int
	running             bool
}

// status counts one status from GitHub's statuses API.
func (t *ciTally) status(state string) {
	t.total++

	switch state {
	case succeeded:
		t.done++
	case "error", "failure":
		t.done++
		t.failed++
	default:
		t.running = true
	}
}

// run counts one GitHub check run. Only a completed run has a conclusion.
//
// The conclusions that pass are named and every other one fails, including one
// GitHub adds later: announcing green on a conclusion nobody has heard of is
// the worse mistake.
func (t *ciTally) run(status, conclusion string) {
	t.total++

	if status != "completed" {
		t.running = true

		return
	}

	t.done++

	passing := map[string]bool{succeeded: true, "neutral": true, "skipped": true}
	if !passing[conclusion] {
		t.failed++
	}
}

// ci is the tally's verdict: any failure fails it, then anything unfinished
// keeps it running, and nothing at all is no CI rather than a pass.
func (t *ciTally) ci() CI {
	state := CIPassed

	switch {
	case t.failed > 0:
		state = CIFailed
	case t.running:
		state = CIRunning
	case t.total == 0:
		state = CINone
	}

	return CI{State: state, Total: t.total, Done: t.done, Failed: t.failed}
}
