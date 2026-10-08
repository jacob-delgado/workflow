// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/jacob-delgado/workflow/internal/buildinfo"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/tui"
	"github.com/jacob-delgado/workflow/internal/web"
	"github.com/jacob-delgado/workflow/internal/webserver"
)

// RunWeb starts the local web server and blocks until the context is canceled.
// What it says about the server goes to notes, stderr: the server has no
// artifact for stdout to carry.
type RunWeb func(
	ctx context.Context, cfg config.Config, deps webserver.Deps, info webserver.Info, notes io.Writer,
) error

// RunWebAt is the web server that serves on addr. It is WebServerAt in
// production and a fake in tests, so the --web and --port flags' wiring can be
// exercised without binding a port.
type RunWebAt func(addr string) RunWeb

// defaultWebPort is the port --web serves on when --port is not given.
const defaultWebPort = webserver.DefaultPort

// WebServerAt is the web server, built over the seams and served on addr until
// the context is canceled, under a session it makes as it starts. It says
// where it serves only once it holds the port, naming the port it bound, so a
// port another program holds is a failure rather than a claim, and port 0
// names the port the system chose; the address it prints carries the session,
// which only a request that presents it gets past. Production serves only a
// webserver.LoopbackAddr, through NewRootCmd; a test hands it port 0. The
// handler reads the configuration file cfg came from once more, so it starts
// from an edit made since, with that edit's revision. Building it fails when
// the embedded spec cannot load, a build defect, or when that file cannot be
// read again. Each failure the server answers as internal goes to notes, a
// line each, its cause's own lines joined by "; " and every credential masked
// that the configuration in effect when it failed holds.
func WebServerAt(addr string) RunWeb {
	return func(
		ctx context.Context, cfg config.Config, deps webserver.Deps, info webserver.Info, notes io.Writer,
	) error {
		deps.Unexpected = func(inEffect config.Config, err error) {
			lines := strings.FieldsFunc(inEffect.RedactText(err.Error()), func(r rune) bool { return r == '\n' || r == '\r' })
			fmt.Fprintf(notes, "workflow web: %s\n", strings.Join(lines, "; "))
		}

		session := webserver.NewSession()

		handler, err := webserver.Handler(webserver.World{Deps: deps, Config: cfg, Info: info}, web.Assets(), session)
		if err != nil {
			return fmt.Errorf("building the web server: %w", err)
		}

		listener, err := new(net.ListenConfig).Listen(ctx, "tcp", addr)
		if err != nil {
			return fmt.Errorf("serving the web API: %w", err)
		}

		fmt.Fprintf(notes, "workflow web: serving %s — press Ctrl+C to stop\n", session.Address(listener.Addr().String()))

		return webserver.Serve(ctx, listener, handler)
	}
}

// WebDeps hands the web server the interface's seam groups whole, with the
// keymap the interface checks and binds, so a seam a group gains reaches the
// server with no edit here. They are the same seams, which is why the web
// server is another consumer of the wiring rather than a second
// implementation.
func WebDeps(deps tui.Deps) webserver.Deps {
	return webserver.Deps{
		Jira: deps.Jira, Git: deps.Git, Forge: deps.Forge, Messaging: deps.Messaging, Hooks: deps.Hooks,
		Store: deps.Store, Tasks: deps.Tasks, Repositories: deps.Repositories, Settings: deps.Settings,
		Clock: deps.Clock, HomeDir: os.UserHomeDir, CheckKeys: tui.CheckKeys, KeyActions: tui.KeyActions,
	}
}

// webSetupStep names the page's own way to set up a first file, beside
// config init.
const webSetupStep = "Set one up in Settings, on the page served below."

// serveWeb serves the web interface over conn through the server serveAt
// makes on the loopback port flags name, first saying when the configuration
// did not load cleanly — or, with no file, the ways to set one up, as every
// surface names them — and hands the server the wiring's control over the
// forge settings a save in Settings changes, and a way to reach another
// directory, wired as this one was, for a switch.
func serveWeb(cmd *cobra.Command, conn connection, serveAt RunWebAt, flags rootFlags) error {
	switch {
	case errors.Is(conn.loadErr, config.ErrNotFound):
		fmt.Fprintf(cmd.ErrOrStderr(), "%s\n\n%s\n%s\n%s\n\n",
			config.NoConfigHeadline, webSetupStep, config.InitStep, config.DoctorStep)
	case conn.loadErr != nil:
		fmt.Fprintf(cmd.ErrOrStderr(), "workflow web: configuration did not load cleanly: %v\n", conn.loadErr)
	}

	conn.controls.ResolveAhead()

	world := webWorld(cmd, conn, flags.dryRun)
	serve := serveAt(webserver.LoopbackAddr(flags.port))

	return serve(cmd.Context(), world.Config, world.Deps, world.Info, cmd.ErrOrStderr())
}

// reachFrom wires dir as conn was wired, for the web server's switch, and
// moves the process there only once the server can serve it: its keys are
// ones the interface accepts and its configuration loaded cleanly.
func reachFrom(cmd *cobra.Command, conn connection, dir string, dryRun bool) (webserver.World, error) {
	wired, err := wireAt(cmd, conn, dir)
	if errors.Is(err, errKeysRefused) {
		return webserver.World{}, fmt.Errorf("%w: %w", webserver.ErrConfigurationRefused, err)
	}

	if err != nil {
		return webserver.World{}, err
	}

	// A configuration that did not load would leave the server on the
	// defaults there, posting and pushing with settings nobody chose.
	err = wired.unreadConfiguration()
	if err != nil {
		return webserver.World{}, fmt.Errorf("%w: %w", webserver.ErrConfigurationUnreadable, err)
	}

	wired.controls.ResolveAhead()

	err = moveTo(dir)
	if err != nil {
		return webserver.World{}, err
	}

	return webWorld(cmd, wired, dryRun), nil
}

// webWorld is the web server's view of the directory conn is wired to.
func webWorld(cmd *cobra.Command, conn connection, dryRun bool) webserver.World {
	info := webserver.Info{
		Version: buildinfo.Current(), DryRun: dryRun, ForgeKind: conn.deps.Forge.Kind,
		Taskwarrior: conn.cfg.Taskwarrior, Repository: conn.where.Name(),
	}

	deps := WebDeps(conn.deps)
	deps.UseForgeSettings = conn.controls.UseForgeSettings
	deps.UseMessagingSettings = conn.controls.UseMessagingSettings
	deps.PlaceSlackCredentials = conn.controls.PlaceSlackCredentials
	deps.KeepJiraToken = conn.controls.KeepJiraToken
	deps.Reach = func(dir string) (webserver.World, error) {
		return reachFrom(cmd, conn, dir, dryRun)
	}

	return webserver.World{Deps: deps, Config: conn.cfg, Info: info}
}

// portFlag names the root's flag that picks the port --web serves on.
const portFlag = "port"
