// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cmdkit

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

// Connection is a command wired to where it runs: the configuration in effect
// and, when it did not load, why; the repository; the seams over both, and
// what finds their tokens ahead of first use, which only the interface and the
// web server call; and the close of the request log, which the caller defers.
type Connection struct {
	// Config is the configuration in effect.
	Config config.Config
	// LoadErr is why the configuration did not load — config.ErrNotFound
	// when there is no file — or nil.
	LoadErr error
	// Where is the repository the command runs in, or the directory when
	// there is none.
	Where wiring.Workspace
	// Deps are the seams over the configuration and the repository.
	Deps tui.Deps
	// Controls are the wiring's own: finding tokens ahead, and applying
	// settings the web's Settings saves.
	Controls wiring.Controls
	// RequestLog records each request, or is nil; a switch wires the next
	// directory to the same log.
	RequestLog *wiring.RequestLog
	// CloseLog closes the request log, which the caller defers.
	CloseLog func()
	// Keychain keeps a token typed into the first run's form in the OS
	// keychain; nil where none is wired, or under a dry run.
	Keychain func(ctx context.Context, service, secret string) error
	// Process is what the wiring read of the process: the home, variables,
	// store and programs the command runs with.
	Process wiring.Environment
}

// Connect wires a command to its working directory, recording each request in
// the --request-log file when one is named. A configuration file that is there but
// cannot be read is refused rather than replaced by the defaults, which would
// act on settings nobody chose; no file at all is not an error.
func Connect(cmd *cobra.Command) (Connection, error) {
	conn, err := ConnectLeniently(cmd)
	if err != nil {
		return Connection{}, err
	}

	err = conn.UnreadConfiguration()
	if err != nil {
		conn.CloseLog()

		return Connection{}, err
	}

	return conn, nil
}

// ConnectLeniently is Connect keeping a configuration that did not load, for
// the interface and the web server, which each show the user why.
func ConnectLeniently(cmd *cobra.Command) (Connection, error) {
	dir, err := WorkingDir(cmd)
	if err != nil {
		return Connection{}, err
	}

	requestLog, closeLog, err := RequestLogFor(cmd)
	if err != nil {
		return Connection{}, err
	}

	conn := ConnectAt(cmd.Context(), cmd, dir, requestLog)
	conn.CloseLog = closeLog

	return conn, nil
}

// ConnectAt wires the directory at dir, which reads its own configuration,
// recording each request in requestLog unless it is nil, and finding where dir
// is under ctx. Under --dry-run its store is only read, and only when it is
// already on disk. It opens nothing, so its close is a no-op.
func ConnectAt(ctx context.Context, cmd *cobra.Command, dir string, requestLog *wiring.RequestLog) Connection {
	process := EnvironmentOf(cmd).Process
	cfg, loadErr := config.Load(dir, process.Home)
	where := process.Locate(ctx, dir)
	deps, controls := process.Deps(cfg, where, requestLog)

	if DryRunRequested(cmd) {
		deps.Store = process.ReadOnlyStore(cfg, where)
	}

	return Connection{
		Config: cfg, LoadErr: loadErr, Where: where, Deps: deps, Controls: controls,
		RequestLog: requestLog, CloseLog: func() {}, Process: process,
	}
}

// WithSetup is the connection offering a first run where it works, with the
// token kept through keychain, which a dry run never stores in.
func (c Connection) WithSetup(
	cmd *cobra.Command, keychain func(ctx context.Context, service, secret string) error,
) Connection {
	if DryRunRequested(cmd) {
		keychain = nil
	}

	where := setup.Where{WorkDir: c.Where.Dir, HomeDir: ConfigHome(cmd)}
	c.Keychain = keychain
	c.Deps.Settings.Setup = wiring.SetupDeps(where, c.RequestLog, keychain)

	return c
}

// UnreadConfiguration is why the configuration file in effect could not be
// read, or nil when it was read or there is none.
func (c Connection) UnreadConfiguration() error {
	if errors.Is(c.LoadErr, config.ErrNotFound) {
		return nil
	}

	return c.LoadErr
}

// RequestTimeout is the configured request timeout, or the default when none
// is set: it bounds each of doctor's --online checks, so a hung service does
// not hang doctor, and how long a quit waits for the writes in flight.
func RequestTimeout(cfg config.Config) time.Duration {
	return cmp.Or(cfg.RequestTimeout(), config.DefaultRequestTimeout)
}

// OnlineDoer is the transport the tracker and messaging checks travel over: the
// redirect-refusing client, bounded by RequestTimeout.
func OnlineDoer(cfg config.Config) httpx.Doer {
	return httpx.Client(RequestTimeout(cfg)).Do
}

// ForgeNoun names a forge mid-sentence: by its own name, or as the forge when
// the remote does not say which.
func ForgeNoun(kind forge.Kind) string {
	if kind == forge.KindUnknown {
		return "the forge"
	}

	return kind.String()
}

// CompleteAssignedIssues completes an issue command's key with the keys of the
// issues assigned to you, for `workflow branch <tab>` and every command that
// names an issue first. Past the key it offers nothing and asks nothing, and a
// tracker that cannot be reached offers nothing rather than an error.
func CompleteAssignedIssues(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	dir, err := WorkingDir(cmd)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	// Completion runs on every <tab>, so it records nothing in a request log.
	conn := ConnectAt(cmd.Context(), cmd, dir, nil)

	result, err := conn.Deps.Jira.Search(cmd.Context(), jira.AssignedToMe, 0)
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
