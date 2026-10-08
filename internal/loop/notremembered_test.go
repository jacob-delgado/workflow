// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package loop_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/loop"
)

func TestNotRememberedReasonIsWhyTheStoreCouldNotRemember(t *testing.T) {
	t.Parallel()

	cases := map[string]func(error) error{
		"as Deliver answers it": func(err error) error { return err },
		"wrapped by its caller": func(err error) error { return fmt.Errorf("announcing to Slack: %w", err) },
	}

	for name, wrap := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			var sent deliveries

			forgetting := loop.AnnounceMemory{Record: func(loop.Announced) error { return errSeam }}
			err := wrap(loop.Deliver(sent.post(nil), forgetting, merged()))

			// Act
			why, notKept := loop.NotRememberedReason(err)

			// Assert
			if !notKept || why != errSeam.Error() {
				t.Errorf("NotRememberedReason = %q, %t; want %q, true", why, notKept, errSeam.Error())
			}
		})
	}
}

func TestNotRememberedReasonIsNoneForAnyOtherOutcome(t *testing.T) {
	t.Parallel()

	cases := map[string]error{
		"remembered":        nil,
		"a post that fails": errSeam,
		"no way to post":    loop.ErrAnnounceUnavailable,
	}

	for name, err := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			why, notKept := loop.NotRememberedReason(err)

			// Assert
			if notKept || why != "" {
				t.Errorf("NotRememberedReason = %q, %t; want nothing: the store was not what failed", why, notKept)
			}
		})
	}
}

func TestNotRememberedErrorSaysTheWarningsSentence(t *testing.T) {
	t.Parallel()

	// Arrange
	sentence := strings.TrimSuffix(loop.NotRememberedWarning, ".")
	asAnError := strings.ToLower(sentence[:1]) + sentence[1:]

	// Act
	said := loop.ErrNotRemembered.Error()

	// Assert
	if said != asAnError {
		t.Errorf("ErrNotRemembered says %q; want %q, the warning's sentence as an error says it", said, asAnError)
	}
}
