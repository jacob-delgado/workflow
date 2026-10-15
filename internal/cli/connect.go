// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"cmp"
	"context"
	"errors"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/httpx"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/setup"
	"github.com/jacob-delgado/workflow/internal/tui"
	"github.com/jacob-delgado/workflow/internal/wiring"
)

// connection is a command wired to where it runs: the configuration in effect
// and, when it did not load, why; the repository; the seams over both, and
// what finds their tokens ahead of first use, which only the interface and the
// web server call; and the close of the request log, which the caller defers.
type connection struct {
	cfg     config.Config
	loadErr error
	where   wiring.Workspace
	deps    tui.Deps
	// controls are the wiring's own: finding tokens ahead, and applying
	// settings the web's Settings saves.
	controls wiring.Controls
	// requestLog records each request, or is nil; a switch wires the next
	// directory to the same log.
	requestLog *wiring.RequestLog
	closeLog   func()
	// keychain keeps a token typed into the first run's form in the OS
	// keychain; nil where none is wired, or under a dry run.
	keychain func(ctx context.Context, service, secret string) error
	// process is what the wiring read of the process: the home, variables,
	// store and programs the command runs with.
	process wiring.Environment
}

// connect wires a command to its working directory, recording each request in
// the --request-log file when one is named. A configuration file that is there but
// cannot be read is refused rather than replaced by the defaults, which would
// act on settings nobody chose; no file at all is not an error.
func connect(cmd *cobra.Command) (connection, error) {
	conn, err := connectLeniently(cmd)
	if err != nil {
		return connection{}, err
	}

	err = conn.unreadConfiguration()
	if err != nil {
		conn.closeLog()

		return connection{}, err
	}

	return conn, nil
}

// connectLeniently is connect keeping a configuration that did not load, for
// the interface and the web server, which each show the user why.
func connectLeniently(cmd *cobra.Command) (connection, error) {
	dir, err := workingDir(cmd)
	if err != nil {
		return connection{}, err
	}

	requestLog, closeLog, err := requestLogFor(cmd)
	if err != nil {
		return connection{}, err
	}

	conn := connectAt(cmd.Context(), cmd, dir, requestLog)
	conn.closeLog = closeLog

	return conn, nil
}

// connectAt wires the directory at dir, which reads its own configuration,
// recording each request in requestLog unless it is nil, and finding where dir
// is under ctx. Under --dry-run its store is only read, and only when it is
// already on disk. It opens nothing, so its close is a no-op.
func connectAt(ctx context.Context, cmd *cobra.Command, dir string, requestLog *wiring.RequestLog) connection {
	process := environmentOf(cmd).Process
	cfg, loadErr := config.Load(dir, process.Home)
	where := process.Locate(ctx, dir)
	deps, controls := process.Deps(cfg, where, requestLog)

	if dryRunRequested(cmd) {
		deps.Store = process.ReadOnlyStore(cfg, where)
	}

	return connection{
		cfg: cfg, loadErr: loadErr, where: where, deps: deps, controls: controls,
		requestLog: requestLog, closeLog: func() {}, process: process,
	}
}

// withSetup is the connection offering a first run where it works, with the
// token kept through keychain, which a dry run never stores in.
func (c connection) withSetup(
	cmd *cobra.Command, keychain func(ctx context.Context, service, secret string) error,
) connection {
	if dryRunRequested(cmd) {
		keychain = nil
	}

	where := setup.Where{WorkDir: c.where.Dir, HomeDir: configHome(cmd)}
	c.keychain = keychain
	c.deps.Settings.Setup = wiring.SetupDeps(where, c.requestLog, keychain)

	return c
}

// unreadConfiguration is why the configuration file in effect could not be
// read, or nil when it was read or there is none.
func (c connection) unreadConfiguration() error {
	if errors.Is(c.loadErr, config.ErrNotFound) {
		return nil
	}

	return c.loadErr
}

// requestTimeout is the configured request timeout, or the default when none
// is set: it bounds each of doctor's --online checks, so a hung service does
// not hang doctor, and how long a quit waits for the writes in flight.
func requestTimeout(cfg config.Config) time.Duration {
	return cmp.Or(cfg.RequestTimeout(), config.DefaultRequestTimeout)
}

// onlineDoer is the transport the tracker and messaging checks travel over: the
// redirect-refusing client, bounded by requestTimeout.
func onlineDoer(cfg config.Config) httpx.Doer {
	return httpx.Client(requestTimeout(cfg)).Do
}

// forgeNoun names a forge mid-sentence: by its own name, or as the forge when
// the remote does not say which.
func forgeNoun(kind forge.Kind) string {
	if kind == forge.KindUnknown {
		return "the forge"
	}

	return kind.String()
}

// completeAssignedIssues completes an issue command's key with the keys of the
// issues assigned to you, for `workflow branch <tab>` and every command that
// names an issue first. Past the key it offers nothing and asks nothing, and a
// tracker that cannot be reached offers nothing rather than an error.
func completeAssignedIssues(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	dir, err := workingDir(cmd)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	// Completion runs on every <tab>, so it records nothing in a request log.
	conn := connectAt(cmd.Context(), cmd, dir, nil)

	result, err := conn.deps.Jira.Search(cmd.Context(), jira.AssignedToMe, 0)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	var keys []string

	for _, issue := range result.Issues {
		if key := string(issue.Key); strings.HasPrefix(key, toComplete) {
			keys = append(keys, key)
		}
	}

	return keys, cobra.ShellCompDirectiveNoFileComp
}
