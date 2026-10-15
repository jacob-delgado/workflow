// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webtest

import (
	"context"

	"github.com/jacob-delgado/workflow/internal/hooks"
	"github.com/jacob-delgado/workflow/internal/webserver"
)

// HookSetupAt is where the lefthook offer is read and taken, and HooksAt
// where the hooks lefthook configures are listed.
const (
	HookSetupAt = "/api/hooks/setup"
	HooksAt     = "/api/hooks"
)

// The hooks the tests' lefthook configures: pre-commit and pre-push, which a
// run by hand can run, and commit-msg, which only git runs, and why.
const (
	PreCommitHook   = "pre-commit"
	PrePushHook     = "pre-push"
	CommitMsgHook   = "commit-msg"
	CommitMsgReason = "git hands it the file holding the commit message"
)

// OldHooks is a repository's own pre-commit hook, which lefthook does not
// manage: two plain commands.
func OldHooks() []hooks.GitHook {
	return []hooks.GitHook{{Name: PreCommitHook, Script: "#!/bin/sh\ngo vet ./...\ngo test ./...\n"}}
}

// HookDeps is FilledDeps with the hooks directory holding found, configured
// or not, and the configuration written kept in written.
func HookDeps(found []hooks.GitHook, configured bool, written *[]hooks.Generated) webserver.Deps {
	deps := FilledDeps()
	deps.Hooks.Existing = func(context.Context) ([]hooks.GitHook, bool) { return found, configured }
	deps.Hooks.Write = func(_ context.Context, generated hooks.Generated) error {
		*written = append(*written, generated)

		return nil
	}

	return deps
}
