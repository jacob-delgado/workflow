// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

//go:build unix

package fileowner

import (
	"io/fs"
	"syscall"
)

// Of is the owner of the file info describes, and false when info is not one
// the system described.
func Of(info fs.FileInfo) (Owner, bool) {
	stat, isStat := info.Sys().(*syscall.Stat_t)
	if !isStat {
		return Owner{}, false
	}

	return Owner{User: int(stat.Uid), Group: int(stat.Gid)}, true
}
