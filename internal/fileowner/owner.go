// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

// Package fileowner says who owns a file, where the operating system keeps
// that as a user and a group id. It is the platform glue alone, in a package
// of its own, so the code deciding what an owner means is measured where it
// lives.
package fileowner

// Owner is the user and group a file belongs to.
type Owner struct {
	User  int
	Group int
}
