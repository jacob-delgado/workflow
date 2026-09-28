// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package taskwarrior_test

import (
	"context"
	"errors"
	"maps"
	"testing"

	"github.com/jacob-delgado/workflow/internal/taskwarrior"
)

func TestEveryReadAndWriteAnswersACanceledContextAsCanceled(t *testing.T) {
	t.Parallel()

	each := writes()
	maps.Copy(each, reads())

	each[addWord] = func(ctx context.Context, client taskwarrior.Client) error {
		_, err := client.Add(ctx, "Renew the cert")

		return err
	}
	each[undoWord] = func(ctx context.Context, client taskwarrior.Client) error {
		_, err := client.Undo(ctx)

		return err
	}
	each[syncWord] = func(ctx context.Context, client taskwarrior.Client) error {
		_, err := client.Sync(ctx)

		return err
	}

	for name, act := range each {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			fake := &fakeTask{}

			ctx, cancel := context.WithCancel(t.Context())
			cancel()

			// Act
			err := act(ctx, fake.client())

			// Assert
			if !errors.Is(err, context.Canceled) {
				t.Errorf("%s returned %v, want context.Canceled", name, err)
			}

			if live := liveRuns(fake.calls); live != 0 {
				t.Errorf("%s ran %+v, %d of them on a live context, want none", name, fake.calls, live)
			}
		})
	}
}
