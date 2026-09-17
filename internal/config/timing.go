// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"errors"
	"fmt"
	"time"
)

// ErrInvalidTiming reports a timing setting that is not a positive duration.
var ErrInvalidTiming = errors.New("invalid timing")

// Timing is how long the interface waits, for a network or a service that is
// slower or more rate-limited than the defaults assume. Each is a Go duration
// string, such as "20s" or "3m"; empty keeps the default.
type Timing struct {
	// RequestTimeout bounds each request to a service. The default is ten
	// seconds.
	RequestTimeout string `json:"request_timeout"`
	// CIInterval is how often CI is asked about while it runs. The default is
	// twenty seconds.
	CIInterval string `json:"ci_interval"`
}

// RequestTimeout is the configured per-request timeout, or zero when none is
// set, for the caller to fall back to its own default.
func (c Config) RequestTimeout() time.Duration {
	timeout, _ := parseDuration(c.Timing.RequestTimeout)

	return timeout
}

// CIInterval is the configured gap between CI checks, or zero when none is set.
func (c Config) CIInterval() time.Duration {
	interval, _ := parseDuration(c.Timing.CIInterval)

	return interval
}

// validateTiming refuses a timing setting that is not a positive duration, so a
// nonsense value fails at load rather than silently falling back.
func (c Config) validateTiming() error {
	for _, value := range []string{c.Timing.RequestTimeout, c.Timing.CIInterval} {
		_, err := parseDuration(value)
		if err != nil {
			return err
		}
	}

	return nil
}

// parseDuration reads a timing setting: empty is zero and no error, so a caller
// falls back to its default; anything else must be a positive Go duration.
func parseDuration(value string) (time.Duration, error) {
	if value == "" {
		return 0, nil
	}

	parsed, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("%w: not a duration such as \"20s\": %q", ErrInvalidTiming, value)
	}

	if parsed <= 0 {
		return 0, fmt.Errorf("%w: must be more than zero: %q", ErrInvalidTiming, value)
	}

	return parsed, nil
}
