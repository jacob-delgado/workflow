// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package httpx_test

import (
	"errors"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/jacob-delgado/workflow/internal/httpx"
)

// readLimit is the most every read here takes.
const readLimit = 16

func TestReadRefusesAnAnswerPastItsLimit(t *testing.T) {
	t.Parallel()

	// Arrange
	// Valid JSON one byte past the limit: cut at the limit, it would decode as
	// broken JSON rather than as an answer too large.
	answer := `{"a":"` + strings.Repeat("x", readLimit+1-len(`{"a":""}`)) + `"}`

	// Act
	read, err := httpx.Read(strings.NewReader(answer), readLimit)

	// Assert
	if !errors.Is(err, httpx.ErrAnswerTooLarge) || read != nil {
		t.Errorf("Read = %q, %v; want nothing and ErrAnswerTooLarge", read, err)
	}
}

func TestReadReadsAnAnswerAtItsLimitWhole(t *testing.T) {
	t.Parallel()

	// Arrange
	answer := strings.Repeat("x", readLimit)

	// Act
	read, err := httpx.Read(strings.NewReader(answer), readLimit)

	// Assert
	if err != nil || string(read) != answer {
		t.Errorf("Read = %q, %v; want the whole answer", read, err)
	}
}

func TestReadKeepsWhyAnAnswerBrokeOff(t *testing.T) {
	t.Parallel()

	// Act
	_, err := httpx.Read(iotest.ErrReader(errUnderlying), readLimit)

	// Assert
	if !errors.Is(err, errUnderlying) || errors.Is(err, httpx.ErrAnswerTooLarge) {
		t.Errorf("Read = %v, want the read's own failure", err)
	}
}
