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

// handedByGit is what git hands each hook it gives arguments or standard
// input, as githooks(5) documents them: a run by hand has none of it to hand
// on, so only git can run these. Every other hook, and a hook git does not
// know, a run by hand stands in for.
//
// Trade-off TRADE-39: pre-push is left out, so it is run by hand without the
// remote and the refs git hands it; lefthook reads what a push would carry
// from @{push} itself.
func handedByGit() map[string]string {
	return map[string]string{
		"applypatch-msg":        "the file holding the proposed commit message",
		"prepare-commit-msg":    "the file holding the commit message, and where the message came from",
		"commit-msg":            "the file holding the commit message",
		"pre-rebase":            "the upstream and the branch being rebased",
		"post-checkout":         "the commits checked out from and to, and whether a branch was",
		"post-merge":            "whether the merge was a squash",
		"pre-receive":           "the refs being pushed, on standard input",
		"update":                "the ref being updated, with its old and new object",
		"proc-receive":          "the commands to handle, through a protocol on standard input",
		"post-receive":          "the refs pushed, on standard input",
		"post-update":           "the refs updated",
		"reference-transaction": "the transaction's state, and its refs on standard input",
		"push-to-checkout":      "the commit the pushed branch now points at",
		"post-rewrite":          "the command that rewrote, and the commits it rewrote on standard input",
		"sendemail-validate":    "the patch about to be sent",
		"fsmonitor-watchman":    "the version of its interface, and the token to ask for changes since",
		"p4-changelist":         "the file holding the changelist text",
		"p4-prepare-changelist": "the file holding the changelist text",
		"post-index-change":     "whether the work tree and the index entries changed",
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
