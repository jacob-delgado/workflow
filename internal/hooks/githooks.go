// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package hooks

import (
	"io/fs"
	"strings"
)

// GitHook is a hook script found in the hooks directory.
type GitHook struct {
	Name   string
	Script string
}

// executableBits are the permission bits git checks before running a hook.
const executableBits = 0o111

// gitHookNames are the hooks git runs, in the order git documents them.
func gitHookNames() []string {
	return []string{
		"applypatch-msg", "pre-applypatch", "post-applypatch", "pre-commit", "pre-merge-commit",
		"prepare-commit-msg", "commit-msg", "post-commit", "pre-rebase", "post-checkout", "post-merge",
		"pre-push", "pre-receive", "update", "proc-receive", "post-receive", "post-update",
		"reference-transaction", "push-to-checkout", "pre-auto-gc", "post-rewrite", "sendemail-validate",
		"fsmonitor-watchman", "p4-changelist", "p4-prepare-changelist", "p4-post-changelist",
		"p4-pre-submit", "post-index-change",
	}
}

// ExistingHooks finds the hooks git would run from a hooks directory, in git's
// order: files named for a hook, runnable as one, that are not lefthook's own
// shim. Whether a file is runnable is decided by goos, since Windows has no
// executable bit to check.
func ExistingHooks(hooksDir fs.FS, goos string) []GitHook {
	var found []GitHook

	for _, name := range gitHookNames() {
		info, err := fs.Stat(hooksDir, name)
		if err != nil || info.IsDir() || !runnableHook(info.Mode(), goos) {
			continue
		}

		script, err := fs.ReadFile(hooksDir, name)
		if err != nil || lefthooksShim(string(script)) {
			continue
		}

		found = append(found, GitHook{Name: name, Script: string(script)})
	}

	return found
}

// lefthooksShim reports a hook lefthook installed itself: one that carries its
// version header or calls its call_lefthook function. A hook that only mentions
// lefthook, in a comment or as a step, is the repository's own.
func lefthooksShim(script string) bool {
	for line := range strings.SplitSeq(script, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "# lefthook_version:") || strings.HasPrefix(line, "call_lefthook") {
			return true
		}
	}

	return false
}

// runnableHook reports whether git would run a file as a hook: executable on
// Unix, and any regular file on Windows, where Go reports no executable bit
// (0666 or 0444 for every file) so the bit cannot be the test.
func runnableHook(mode fs.FileMode, goos string) bool {
	if goos == "windows" {
		return true
	}

	return mode.Perm()&executableBits != 0
}
