// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package proc_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/proc"
)

func TestFailureReadsTheExitCodeAndStderr(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		fail       func(ctx context.Context) error
		failedAs   error
		wantCode   int
		wantStderr string
		wantExited bool
	}{
		{
			name: "exit 1 with its reason",
			fail: func(ctx context.Context) error {
				_, err := proc.Capture(ctx, script(`echo "No tasks specified." >&2; exit 1`), nil)

				return err
			},
			wantCode:   1,
			wantStderr: "No tasks specified.",
			wantExited: true,
		},
		{
			name: "exit 2 with its reason",
			fail: func(ctx context.Context) error {
				_, err := proc.Capture(ctx, script(`echo "Cannot proceed without rc file." >&2; exit 2`), nil)

				return err
			},
			wantCode:   2,
			wantStderr: "Cannot proceed without rc file.",
			wantExited: true,
		},
		{
			name: "a program not found",
			fail: func(ctx context.Context) error {
				_, err := proc.Capture(ctx, proc.Command{Name: missingProgram}, nil)

				return err
			},
			failedAs: proc.ErrNotFound,
		},
		{
			name: "a program stopped at its bound",
			fail: func(ctx context.Context) error {
				_, err := proc.CaptureWithin(ctx, 10*time.Millisecond, script("exec sleep 5"), nil)

				return err
			},
			failedAs: proc.ErrTimedOut,
		},
		{
			name: "a program the caller canceled",
			fail: func(ctx context.Context) error {
				ctx, cancel := context.WithCancel(ctx)
				defer cancel()

				time.AfterFunc(50*time.Millisecond, cancel)

				_, err := proc.Capture(ctx, script("exec sleep 5"), nil)

				return err
			},
		},
		{
			name: "a program killed by a signal",
			fail: func(ctx context.Context) error {
				_, err := proc.Capture(ctx, script("kill -KILL $$"), nil)

				return err
			},
		},
		{
			name: "a quick read the caller canceled",
			fail: func(ctx context.Context) error {
				ctx, cancel := context.WithCancel(ctx)
				defer cancel()

				time.AfterFunc(50*time.Millisecond, cancel)

				_, err := proc.RunWithin(ctx, time.Minute, "sh", "-c", "exec sleep 5")

				return err
			},
		},
		{
			name: "no error",
			fail: func(context.Context) error { return nil },
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			err := test.fail(t.Context())
			if test.failedAs != nil && !errors.Is(err, test.failedAs) {
				t.Fatalf("arranging %q returned %v, want %v", test.name, err, test.failedAs)
			}

			// Act
			code, stderr, exited := proc.Failure(err)

			// Assert
			if code != test.wantCode || stderr != test.wantStderr || exited != test.wantExited {
				t.Errorf("Failure(%v) = (%d, %q, %t), want (%d, %q, %t)",
					err, code, stderr, exited, test.wantCode, test.wantStderr, test.wantExited)
			}
		})
	}
}
