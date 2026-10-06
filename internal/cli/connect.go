// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/jacob-delgado/workflow/internal/config"
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
	keychain func(secret string) (string, error)
}

// connect wires a command to its working directory, recording each request in
// the --log file when one is named. A configuration file that is there but
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
	// Trade-off TRADE-18: only Linux's tests reach this, since macOS still names
	// a working directory once it is removed.
	dir, err := os.Getwd()
	if err != nil {
		return connection{}, fmt.Errorf("determining the working directory: %w", err)
	}

	requestLog, closeLog, err := requestLogFor(cmd)
	if err != nil {
		return connection{}, err
	}

	conn := connectAt(cmd, dir, configHome(), requestLog)
	conn.closeLog = closeLog

	return conn, nil
}

// connectAt wires the directory at dir, which reads its own configuration,
// recording each request in requestLog unless it is nil. Under --dry-run its
// store is only read, and only when it is already on disk. It opens nothing, so
// its close is a no-op.
func connectAt(cmd *cobra.Command, dir, home string, requestLog *wiring.RequestLog) connection {
	ctx := cmd.Context()
	cfg, loadErr := config.Load(dir, home)
	where := wiring.Locate(ctx, dir)
	deps, controls := wiring.Deps(ctx, cfg, where, requestLog)

	if dryRunRequested(cmd) {
		deps.Store = wiring.ReadOnlyStore(ctx, cfg, where)
	}

	return connection{
		cfg: cfg, loadErr: loadErr, where: where, deps: deps, controls: controls,
		requestLog: requestLog, closeLog: func() {},
	}
}

// withSetup is the connection offering a first run where it works, with the
// token kept through keychain, which a dry run never stores in.
func (c connection) withSetup(cmd *cobra.Command, keychain func(secret string) (string, error)) connection {
	if dryRunRequested(cmd) {
		keychain = nil
	}

	where := setup.Where{WorkDir: c.where.Dir, HomeDir: configHome()}
	c.keychain = keychain
	c.deps.Settings.Setup = wiring.SetupDeps(cmd.Context(), where, c.requestLog, keychain)

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
