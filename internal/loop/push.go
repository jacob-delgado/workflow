// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package loop

import (
	"errors"
	"strings"

	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/proc"
)

var (
	// ErrPushFailed is a push that ran and did not publish the branch. The error
	// returned is a PushFailedError, which carries the push's own output.
	ErrPushFailed = errors.New("the push failed")
	// ErrPushUnavailable refuses a push with no way to make one.
	ErrPushUnavailable = errors.New("pushing is not available")
)

// PushFailedError is ErrPushFailed with what the push printed, which is where
// the reason — a rejected ref, a missing remote — is.
type PushFailedError struct {
	Output []string
}

var _ error = PushFailedError{}

// Error says the push failed, followed by its output.
func (e PushFailedError) Error() string {
	return ErrPushFailed.Error() + ":\n" + strings.Join(e.Output, "\n")
}

// Unwrap lets errors.Is match ErrPushFailed.
func (PushFailedError) Unwrap() error {
	return ErrPushFailed
}

// Push publishes branch through push, draining its output so the push runs to
// completion. A push that ran and failed is a PushFailedError; one that could
// not start returns the seam's own error, for the caller to word.
func Push(push func(branch string) (proc.Output, error), branch string) error {
	if push == nil {
		return ErrPushUnavailable
	}

	output, err := push(branch)
	if err != nil {
		return err
	}

	var lines []string
	for line := range output.Lines {
		lines = append(lines, line)
	}

	err = output.Wait()
	if err != nil {
		return PushFailedError{Output: lines}
	}

	return nil
}

// EnsurePushed publishes the branch when its remote does not have it yet, since
// a pull request cannot open from an unpushed branch. A pushed branch is left
// alone.
func EnsurePushed(push func(branch string) (proc.Output, error), branch gitrepo.Branch) error {
	if branch.Pushed() {
		return nil
	}

	return Push(push, branch.Name)
}
