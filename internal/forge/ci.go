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

// Word names the state in the one word every surface writes it in — the
// review filter's value, the command line's status and the API's CI state. A
// map, not a switch, so there is no last-case arm gobco can never see;
// exhaustive keeps it complete.
func (s CIState) Word() string {
	return map[CIState]string{
		CINone: ciNoneWord, CIRunning: ciRunningWord, CIPassed: ciPassedWord, CIFailed: ciFailedWord,
	}[s]
}

// The words Word names each CI state in.
const (
	ciNoneWord    = "none"
	ciRunningWord = "running"
	ciPassedWord  = "passed"
	ciFailedWord  = "failed"
)

// Check is one reported check on a change: what it is called, where it stands,
// and the page that shows it in full — and, for one that failed, why, in the
// stage it ran in, and whether its log can be read.
type Check struct {
	// ID is the forge's own id for the check, a check run's or a job's, so its
	// log can be asked for; empty for a status, which has none.
	ID    string
	Name  string
	State CIState
	URL   string
	// Stage is the pipeline stage a GitLab job ran in; empty on GitHub.
	Stage string
	// Reason is why a failed check failed, in the forge's few words — a check
	// run's title, a job's failure reason — or empty when it gives none.
	Reason string
	// LogAvailable is a check whose log the forge serves: a GitHub Actions run
	// or a GitLab job.
	LogAvailable bool
}

// CI is how CI stands, and how far through it is where the forge says.
type CI struct {
	State CIState
	// Total, Done and Failed count checks where the forge reports them one by
	// one; GitLab reports a pipeline as a whole, so they stay zero there.
	Total, Done, Failed int
	// Checks are the reported checks one by one, so the interface can say which
	// failed rather than only how many.
	Checks []Check
}

// succeeded is what GitHub's statuses, its check runs and GitLab's pipelines all
// call a pass; skipped is another they share, a run or stage that never ran.
const (
	succeeded = "success"
	skipped   = "skipped"
)

// ciTally counts checks toward a CI, and keeps each one so the interface can
// list which is which.
type ciTally struct {
	total, done, failed int
	running             bool
	checks              []Check
}

// add records one check: its state toward the tally, and the check itself.
func (t *ciTally) add(check Check) {
	t.checks = append(t.checks, check)
	t.total++

	switch check.State {
	case CIPassed:
		t.done++
	case CIFailed:
		t.done++
		t.failed++
	case CINone, CIRunning:
		t.running = true
	}
}

// statusState reads one state from GitHub's statuses API.
func statusState(state string) CIState {
	switch state {
	case succeeded:
		return CIPassed
	case "error", "failure":
		return CIFailed
	default:
		return CIRunning
	}
}

// runState reads one GitHub check run. Only a completed run has a conclusion.
//
// The conclusions that pass are named and every other one fails, including one
// GitHub adds later: announcing green on a conclusion nobody has heard of is
// the worse mistake.
func runState(status, conclusion string) CIState {
	if status != "completed" {
		return CIRunning
	}

	passing := map[string]bool{succeeded: true, "neutral": true, skipped: true}
	if passing[conclusion] {
		return CIPassed
	}

	return CIFailed
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

	return CI{State: state, Total: t.total, Done: t.done, Failed: t.failed, Checks: t.checks}
}
