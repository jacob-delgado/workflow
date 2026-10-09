// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

//go:build !unix

package fileowner

import "io/fs"

// Owner is the user and group a file belongs to.
type Owner struct {
	User  int
	Group int
}

// Of reports no owner off Unix: Windows decides who may write a file by its
// access list, which no user and group id stand for.
//
// Trade-off TRADE-35: no CI job runs this; the Cross-compile job only builds
// it, for Windows.
func Of(_ fs.FileInfo) (Owner, bool) {
	return Owner{}, false
}
