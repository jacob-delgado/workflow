// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package loop_test

import (
	"context"
	"testing"

	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/loop"
	"github.com/jacob-delgado/workflow/internal/messaging"
)

// A post that has gone out is remembered whatever becomes of its asker: a
// record a canceled context stopped would offer the announcement again, and
// a later session or page would post it twice.
func TestAPostThatWentOutIsRememberedThoughItsAskerLeft(t *testing.T) {
	t.Parallel()

	// Arrange
	ctx, leave := context.WithCancel(t.Context())
	post := func(context.Context, string, string) error {
		leave()

		return nil
	}

	var recordedUnder, groupsUnder []error

	memory := loop.AnnounceMemory{
		Recorded: nil,
		Record: func(ctx context.Context, _ loop.Announced) error {
			recordedUnder = append(recordedUnder, ctx.Err())

			return nil
		},
		RecordGroups: func(ctx context.Context, _ []string) error {
			groupsUnder = append(groupsUnder, ctx.Err())

			return nil
		},
	}
	delivery := loop.Delivery{
		Channel: devChannel, Text: readyText, Pull: forge.PullRequest{Number: 9}, Moment: messaging.MomentReady,
	}

	// Act
	err := loop.Deliver(ctx, passing(post), memory, delivery)
	// Assert
	if err != nil {
		t.Fatalf("Deliver = %v, want the post delivered", err)
	}

	if len(recordedUnder) != 1 || recordedUnder[0] != nil || len(groupsUnder) != 1 || groupsUnder[0] != nil {
		t.Errorf("recorded under contexts ended %v and the groups under %v, want each once under a live one",
			recordedUnder, groupsUnder)
	}
}
