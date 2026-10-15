// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

import (
	"context"

	"github.com/jacob-delgado/workflow/internal/hooks"
	"github.com/jacob-delgado/workflow/internal/webserver"
)

// hookSetupAt is where the lefthook offer is read and taken, and hooksAt
// where the hooks lefthook configures are listed.
const (
	hookSetupAt = "/api/hooks/setup"
	hooksAt     = "/api/hooks"
)

// The hooks the tests' lefthook configures: pre-commit and pre-push, which a
// run by hand can run, and commit-msg, which only git runs, and why.
const (
	preCommitHook   = "pre-commit"
	prePushHook     = "pre-push"
	commitMsgHook   = "commit-msg"
	commitMsgReason = "git hands it the file holding the commit message"
)

// oldHooks is a repository's own pre-commit hook, which lefthook does not
// manage: two plain commands.
func oldHooks() []hooks.GitHook {
	return []hooks.GitHook{{Name: preCommitHook, Script: "#!/bin/sh\ngo vet ./...\ngo test ./...\n"}}
}

// hookDeps is filledDeps with the hooks directory holding found, configured
// or not, and the configuration written kept in written.
func hookDeps(found []hooks.GitHook, configured bool, written *[]hooks.Generated) webserver.Deps {
	deps := filledDeps()
	deps.Hooks.Existing = func(context.Context) ([]hooks.GitHook, bool) { return found, configured }
	deps.Hooks.Write = func(_ context.Context, generated hooks.Generated) error {
		*written = append(*written, generated)

		return nil
	}

	return deps
}
