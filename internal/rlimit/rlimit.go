// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

//go:build unix

// Package rlimit lowers one of the process's resource limits around a single
// call in a test and puts it back afterward. A resource limit holds for every
// goroutine in the process, not the one test that lowered it, so a test that
// lowers one runs alone and restores it the moment its call returns. Only tests
// import this package.
package rlimit

import (
	"syscall"
	"testing"
)

// Lower sets the soft limit on resource, one of syscall's RLIMIT_ constants, to
// limit, and leaves the hard limit as it was. It returns what restores the limit
// it found, for the test to run straight after the call under test; the same
// restore is registered with t.Cleanup, so a test that stops early still leaves
// the limit as it found it. Failing to read, lower or restore the limit fails
// the test.
func Lower(t *testing.T, resource int, limit uint64) func() {
	t.Helper()

	var was syscall.Rlimit

	err := syscall.Getrlimit(resource, &was)
	if err != nil {
		t.Fatalf("reading resource limit %d: %v", resource, err)
	}

	restore := func() {
		err := syscall.Setrlimit(resource, &was)
		if err != nil {
			t.Fatalf("restoring resource limit %d: %v", resource, err)
		}
	}

	t.Cleanup(restore)

	lowered := was
	setCur(&lowered.Cur, limit)

	err = syscall.Setrlimit(resource, &lowered)
	if err != nil {
		t.Fatalf("lowering resource limit %d to %d: %v", resource, limit, err)
	}

	return restore
}

// setCur stores limit in an Rlimit's Cur field, which is uint64 on most Unix
// systems but int64 on FreeBSD and DragonFly.
func setCur[T ~int64 | ~uint64](cur *T, limit uint64) {
	*cur = T(limit)
}
