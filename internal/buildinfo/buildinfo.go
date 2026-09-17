// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

// Package buildinfo reports which build of workflow is running, from the module
// version when it was installed from a release, or the commit it was built from
// otherwise. It reads what the Go toolchain stamps in; it needs no linker flags.
package buildinfo

import (
	"regexp"
	"runtime/debug"
)

// shortCommit is how many hex characters of a commit identify it in the output.
const shortCommit = 12

// Current is the running build's version.
func Current() string {
	return Version(debug.ReadBuildInfo())
}

// Version reports a build's version from its build info: the module version when
// installed from a release, otherwise the short commit, marked when the tree was
// dirty. It takes the info so a test can supply its own.
func Version(info *debug.BuildInfo, ok bool) string {
	if !ok || info == nil {
		return "unknown"
	}

	if released := info.Main.Version; isRelease(released) {
		return released
	}

	return fromCommit(info.Settings)
}

// isRelease reports a module version that names an official release, as opposed
// to a source build's "(devel)" or the pseudo-version the toolchain synthesizes
// for a commit that no tag names — for which the user asked to see the commit.
func isRelease(version string) bool {
	return version != "" && version != "(devel)" && !pseudoVersion().MatchString(version)
}

// pseudoVersion matches the timestamp-and-commit tail the toolchain builds a
// pseudo-version from, such as v0.0.0-20260917173147-ddbb935d6c04.
func pseudoVersion() *regexp.Regexp {
	return regexp.MustCompile(`\d{14}-[0-9a-f]{12}`)
}

// fromCommit builds a version from the vcs settings the toolchain records.
func fromCommit(settings []debug.BuildSetting) string {
	revision, modified := "", false

	for _, setting := range settings {
		switch setting.Key {
		case "vcs.revision":
			revision = setting.Value
		case "vcs.modified":
			modified = setting.Value == "true"
		}
	}

	if revision == "" {
		return "unknown"
	}

	short := revision
	if len(short) > shortCommit {
		short = short[:shortCommit]
	}

	if modified {
		short += "-dirty"
	}

	return short
}
