// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/jacob-delgado/workflow/internal/buildinfo"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/tui"
	"github.com/jacob-delgado/workflow/internal/web"
	"github.com/jacob-delgado/workflow/internal/webserver"
)

// WebServerAt is the web server, built over the seams and served on addr until
// the context is canceled. Production serves only a webserver.LoopbackAddr,
// through NewRootCmd; a test hands it a port of its own. The handler reads the
// configuration file cfg came from once more, so it starts from an edit made
// since, with that edit's revision. Building it fails when the embedded spec
// cannot load, a build defect, or when that file cannot be read again. Each
// failure the server answers as internal goes to notes, a line each, its
// cause's own lines joined by "; " and every credential cfg holds masked.
func WebServerAt(addr string) RunWeb {
	return func(
		ctx context.Context, cfg config.Config, deps webserver.Deps, info webserver.Info, notes io.Writer,
	) error {
		deps.Unexpected = func(err error) {
			lines := strings.FieldsFunc(cfg.RedactText(err.Error()), func(r rune) bool { return r == '\n' || r == '\r' })
			fmt.Fprintf(notes, "workflow web: %s\n", strings.Join(lines, "; "))
		}

		// Trade-off TRADE-20: the embedded app always opens at its constant root.
		assets, err := web.Assets()
		if err != nil {
			return fmt.Errorf("building the web server: %w", err)
		}

		handler, err := webserver.Handler(deps, cfg, info, assets)
		if err != nil {
			return fmt.Errorf("building the web server: %w", err)
		}

		fmt.Fprintf(notes, "workflow web: serving http://%s — press Ctrl+C to stop\n", addr)

		return webserver.Serve(ctx, addr, handler)
	}
}

// WebDeps adapts the interface's dependency bundle to the web server's narrower
// one. They are the same seams, which is why the web server is another consumer
// of the wiring rather than a second implementation. A seam the interface wires
// is never dropped on the way: the server would answer that write as not
// available. The server also takes the interface's keymap check, so Settings
// never saves a ui.keys map the interface would refuse to start on.
func WebDeps(deps tui.Deps) webserver.Deps {
	return withRepositories(withSummarySources(webserver.Deps{
		Search:        deps.Jira.Search,
		SearchLenient: deps.Jira.SearchLenient,
		Issue:         deps.Jira.Issue,
		BrowseURL:     deps.Jira.BrowseURL,
		Branch:        deps.Git.Branch,
		Branches:      deps.Git.Branches,
		Checkout:      deps.Git.Checkout,
		CreateBranch:  deps.Git.CreateBranch,
		Commit:        deps.Git.Commit,
		Push:          deps.Git.Push,
		Changes:       deps.Git.Changes,
		FindPull:      deps.Forge.FindPullRequest,
		CreatePull:    deps.Forge.CreatePullRequest,
		Templates:     deps.Forge.Templates,
		ChangedPaths:  deps.Git.ChangedPaths,
		CodeOwnersAt:  deps.Git.CodeOwnersAt,
		CheckCI:       deps.Forge.CheckStatus,
		JobLog:        deps.Forge.JobLog,
		Author:        deps.Forge.Author,
		Post:          deps.Messaging.Post,
		IsGroup:       deps.Forge.IsGroup,

		ReviewRequests:  deps.Forge.ReviewRequests,
		LinkPullRequest: deps.Jira.LinkPullRequest,
		Transitions:     deps.Jira.Transitions,
		Transition:      deps.Jira.Transition,
		Comment:         deps.Jira.Comment,
		Stage:           deps.Git.Stage,
		Unstage:         deps.Git.Unstage,

		RemoteBranches: deps.Git.RemoteBranches,
		IssueLinks:     deps.Git.IssueLinks,
		LinkIssue:      deps.Git.LinkIssue,
		UnlinkIssue:    deps.Git.UnlinkIssue,
		EditPull:       deps.Forge.EditPullRequest,

		LastScope:   deps.Store.LastScope,
		RecordScope: deps.Store.RecordScope,
		Tasks:       deps.Tasks,
		HomeDir:     os.UserHomeDir,

		OwnerLinks:    deps.Store.OwnerLinks,
		LinkOwner:     deps.Store.LinkOwner,
		ForgetOwner:   deps.Store.ForgetOwner,
		RepoGroups:    deps.Store.RepoGroups,
		SetRepoGroups: deps.Store.SetRepoGroups,
		LastGroups:    deps.Store.LastGroups,
		RecordGroups:  deps.Store.RecordGroups,
		Workspace:     deps.Messaging.Workspace,

		ChannelMembers: deps.Messaging.ChannelMembers,
		UserGroups:     deps.Messaging.UserGroups,

		LocalData:      localData,
		CleanLocalData: cleanLocalData,

		CheckKeys: tui.CheckKeys,
		Clock:     deps.Clock,
	}, deps), deps)
}

// withRepositories gives the web server where it works, the directories it
// can switch to, and the favorites the store keeps.
func withRepositories(web webserver.Deps, deps tui.Deps) webserver.Deps {
	web.Repositories = deps.Repositories
	web.Favorites, web.Favor, web.Unfavor = deps.Store.Favorites, deps.Store.Favor, deps.Store.Unfavor

	return web
}

// withSummarySources gives the web server the reads the Summary asks of git,
// Jira and the forge, Taskwarrior's coming with its seams.
func withSummarySources(web webserver.Deps, deps tui.Deps) webserver.Deps {
	web.CommitsBetween = deps.Git.CommitsBetween
	web.JiraActivity = deps.Jira.Activity
	web.ForgeActivity = deps.Forge.Activity

	return web
}

// serveWeb serves the web interface over conn through serve, first saying when
// the configuration did not load cleanly, and hands the server the wiring's
// control over the forge settings a save in Settings changes, and a way to
// reach another directory, wired as this one was, for a switch.
func serveWeb(cmd *cobra.Command, conn connection, serve RunWeb, dryRun bool) error {
	if conn.loadErr != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "workflow web: configuration did not load cleanly: %v\n", conn.loadErr)
	}

	conn.controls.ResolveAhead()

	world := webWorld(cmd, conn, dryRun)

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
	deps.Reach = func(dir string) (webserver.World, error) {
		return reachFrom(cmd, conn, dir, dryRun)
	}

	return webserver.World{Deps: deps, Config: conn.cfg, Info: info}
}

// portFlag names the root's flag that picks the port --web serves on.
const portFlag = "port"
