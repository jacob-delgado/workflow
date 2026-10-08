// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

//go:build !unix

package fileowner

import "io/fs"

// Of reports no owner off Unix: Windows decides who may write a file by its
// access list, which no user and group id stand for.
func Of(_ fs.FileInfo) (Owner, bool) {
	return Owner{}, false
}
