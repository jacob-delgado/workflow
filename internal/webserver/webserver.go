// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

// Package webserver serves the local web API behind `workflow --web`: the same
// information the terminal interface shows, and the configuration file, over the
// REST surface described by api/openapi.yaml. It reuses the domain seams the TUI
// and the CLI already use — it is another consumer of the wiring, not a second
// implementation — so each write it makes, to the repository, the forge, the
// tracker, the messaging service or the configuration file, goes through the
// seam the terminal's own goes through.
package webserver

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/seams"
)

// Deps is what the server asks of the world: the seam groups internal/seams
// declares, as the terminal interface takes them, and the few the server
// alone asks for. A nil function means the service is not configured. A list
// or snapshot read then answers empty; the issue read answers 422, as no issue
// tracker is configured; a write whose own seam is nil answers 422, as not
// available; and the announcement and pull request drafts, and the announce
// and open writes past that check, answer 409, as there is nothing to announce
// or open, when the branch read or the pull request find is missing.
type Deps struct {
	Jira      seams.Jira
	Git       seams.Git
	Forge     seams.Forge
	Messaging seams.Messaging
	Hooks     seams.Hooks
	Store     seams.Store
	// Tasks is what the server asks of Taskwarrior. A nil Install, or one that
	// fails, answers the task list and the snapshot's summary as not available —
	// never a 404 — and a write with a nil function is refused as unprocessable.
	Tasks seams.Tasks
	// Repositories is where the server works and any other directory read the
	// same way, and your home directory, which Taskwarrior's words can name
	// and an answer shows as ~; none leaves them naming it.
	Repositories seams.Repositories
	// Settings lists and removes the local data, and sets up a first
	// configuration file where none applies; Settings offers that only where
	// its Setup.Write is wired. The server keeps its own read of the
	// configuration, for its ETag, so it never calls Read or Save.
	Settings seams.Settings
	// Clock tells the time, for when the event stream last asked the forge.
	// Nil means the system clock.
	Clock func() time.Time

	// CheckKeys says why the terminal interface would refuse a ui.keys map,
	// or nil where it would start on it. Nil here means no map is checked.
	CheckKeys func(keys map[string]string) error
	// KeyActions is every action the terminal interface's help lists, with a
	// ui.keys map applied, its review noun and its messaging service naming
	// what the help names for them: the keys a page binds, read from the
	// terminal's own bindings. Nil lists none.
	KeyActions func(reviewNoun, messagingService string, overrides map[string]string) []seams.KeyAction
	// Unexpected hears each failure the server answers as an internal error —
	// one no class of failure explains, or an answer that could not be
	// written — whose cause the answer leaves out, with the configuration in
	// effect when it failed: a credential taken up since the server started,
	// from a switch or a save in Settings, is that configuration's. Nil says
	// nothing.
	Unexpected func(inEffect config.Config, err error)
	// UseForgeSettings applies forge settings just saved to every forge call
	// after the save, and reports which forge the remote is on under them, so a
	// token, host or kind saved in Settings needs no restart. Nil leaves the
	// forge as it was started.
	UseForgeSettings func(settings config.Forge) forge.Kind
	// UseMessagingSettings applies messaging settings just saved to every post
	// after the save. Nil leaves messaging as it was started.
	UseMessagingSettings func(settings config.Messaging)
	// PlaceSlackCredentials keeps a Slack user token's secrets, typed into
	// Settings, where the configuration keeps them — refreshing the token once
	// and saving the pair to the macOS keychain, and answering the
	// configuration without them, or answering it unchanged where the file is
	// where they are kept. Nil writes them into the file as they came.
	PlaceSlackCredentials func(cfg config.Config) (config.Config, error)
	// KeepJiraToken keeps a Jira token typed into Settings in the macOS
	// keychain, under the item for its address, so the file saved reads it
	// from there. Nil writes it into the file as it came.
	KeepJiraToken func(service, secret string) error
	// Reach wires another directory as this one was wired, for a switch; nil
	// where switching is not offered. A setup takes the file up through it,
	// where the server works.
	Reach func(dir string) (World, error)
}

// Info is the build and run facts the API reports and the server needs.
type Info struct {
	Version string
	DryRun  bool

	// ForgeKind is the forge the remote points at, resolved from the git remote
	// as the interface resolves it, so the announcement and the browser name a
	// "merge request" on GitLab and a "pull request" elsewhere — the health read
	// carries the words and the sigil to the page. The raw config kind is unset
	// for self-identifying hosts (gitlab.com), so it cannot answer this.
	ForgeKind forge.Kind

	// Repository names the repository the server runs in, as its forge path
	// or its directory, for the groups Settings keeps for it.
	Repository string

	// StreamInterval is how often the event stream re-pushes a snapshot. A zero
	// or negative value takes defaultStreamInterval.
	StreamInterval time.Duration

	// Taskwarrior is the taskwarrior settings Deps.Tasks was bound with at
	// start. A change saved since applies at the next start, so while the
	// settings in effect differ the task list says so rather than a reason
	// the new settings would not give.
	Taskwarrior config.Taskwarrior
}

// streamInterval is the configured stream cadence, or the default when unset.
func (i Info) streamInterval() time.Duration {
	if i.StreamInterval <= 0 {
		return defaultStreamInterval
	}

	return i.StreamInterval
}

// server implements api.StrictServerInterface over the seams and configuration.
// The configuration is held behind a lock because the write endpoint replaces it
// while read endpoints are serving concurrent requests.
type server struct {
	deps Deps
	info Info

	// files are where the configuration files live, fixed at construction from
	// the trusted locations the process resolved. The write endpoint always
	// saves here and never to a path from the request body, so a client cannot
	// redirect the write — the file location is the server's to decide, not the
	// caller's.
	files config.Files

	mu  sync.RWMutex
	cfg config.Config

	// forgeKind is the forge the remote is on under the settings in effect,
	// under mu with cfg: info's until a save of the forge settings changes it.
	forgeKind forge.Kind

	// seen is the revision of the file cfg was last read from or written as,
	// under mu with it: a read of the configuration that finds the file at
	// another revision takes the file up as the configuration in effect. A read
	// that finds the file gone leaves both standing and sets gone, so the ETag
	// of that read names the configuration it served, not merely no file.
	seen config.Revision
	gone bool

	// indexWrites queues the page's index writes (a stage, a commit, a new
	// branch): git lets one process write the index at a time, and one that
	// finds it taken fails rather than waits. A checkout needs no place in the
	// queue: it refuses the dirty tree any stage leaves.
	indexWrites sync.Mutex

	// keptWrites queues every write to the kept associations and every
	// removal of the local data: a removal sets the kept file aside, and a
	// write that lands between would either be lost with it or remake the file
	// the removal was taking away.
	keptWrites sync.Mutex

	// scope is the commit scope learned in this repository, held once read.
	scope scopeCache

	// author is who a post would come from, held once the forge answers.
	author authorCache

	// forgeAnswer is the forge's part of the stream's frames, held for an
	// interval.
	forgeAnswer forgeCache

	// issuesHeld is each view's first page of issues, held for an interval.
	issuesHeld issuesCache

	// assigned is which of the branches' issues the tracker last said are yours.
	assigned assignedCache

	// held is the announcement held until its pull request's CI passes.
	held heldAnnouncement

	// delivering is held across an announcement's check that it was not made
	// already, its post and its record, so two asks at once post it once.
	delivering sync.Mutex

	// announced is what the store remembers announcing, held for an interval.
	announced announcedCache

	// run is the git run going, if any.
	run runSlot

	// detection is the stream's last search for Taskwarrior, when it found none.
	detection detectionCache

	// worlds holds this server, and replaces it on a switch, closing retired
	// so its event streams end and the page reconnects to the next.
	worlds  *worlds
	retired chan struct{}
}

var _ api.StrictServerInterface = (*server)(nil)

// Handler builds the http.Handler that serves the web interface for first,
// the directory it starts in: the API under /api, checked against the contract
// by the request validator, which admits only a request presenting session,
// and the embedded single-page app, assets, under every other path. The whole
// surface is behind the loopback guard, so a browser aimed at the server from
// a foreign origin is refused, and every answer carries the content policy, so
// no other page can frame the app. The server starts from the configuration
// file at first.Config.Path as startingPoint reads it, not from that
// configuration alone, which the process read a moment before. It fails when
// the embedded spec cannot be loaded or routed, which is a build defect, or
// when that file cannot be read.
func Handler(first World, assets fs.FS, session Session) (http.Handler, error) {
	// Trade-off TRADE-14: the embedded spec loads in every build a test runs.
	spec, err := loadContract(session)
	if err != nil {
		return nil, err
	}

	held, err := newWorlds(first)
	if err != nil {
		return nil, err
	}

	// The API is validated against the contract, before a write takes the gate
	// a switch waits on or is held to the directory its page shows, so a
	// request presenting no session learns nothing of either; the app is not,
	// since its paths are not in the spec, so only the /api subtree passes
	// through the validator.
	root := http.NewServeMux()
	root.Handle("/api/", spec.admit(held.serve))
	root.Handle("/", spaHandler(assets))

	return withPolicyHeaders(guardLoopback(refuseWritesInDryRun(first.Info.DryRun, root))), nil
}

// startingPoint is the configuration the server starts from, with the revision
// of the file it stands for, both from one read of the file at cfg.Path: an
// edit made since the process read cfg is taken up, never paired with the
// revision of a configuration it replaced, or a save over the first read would
// overwrite it. With no file there, cfg stands at the no-file revision. With a
// file that has turned invalid since the process read it, cfg, which the
// process read, also stands at the no-file revision, rather than at a revision
// learned by reading the file again: reads answer 422 until the file is fixed,
// and the first read that finds it valid takes it up.
func startingPoint(cfg config.Config) (config.Config, config.Revision, error) {
	loaded, seen, err := config.LoadLayersAt(cfg.Layers())
	if errors.Is(err, config.ErrInvalid) {
		return cfg, config.Revision{}, nil
	}

	if err != nil {
		return config.Config{}, config.Revision{}, err
	}

	if !seen.Exists() {
		return cfg, seen, nil
	}

	return loaded, seen, nil
}

// DefaultPort is the port the server listens on when `workflow --web` is given
// no --port. IANA assigns 13579 to no service, and it sits below the range
// Linux, macOS and Windows hand out to outgoing connections, so another
// program is unlikely to hold it.
const DefaultPort = 13579

// LoopbackAddr is where the server listens in production, on port: the
// loopback interface only, so the API is reachable from this machine and
// nowhere else.
func LoopbackAddr(port int) string {
	return net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
}

// readHeaderTimeout bounds how long a client may take to send its headers, so a
// slow or stuck connection cannot tie up the server.
const readHeaderTimeout = 10 * time.Second

// shutdownGrace is how long in-flight requests are given to finish when the
// context is canceled before the listener is closed.
const shutdownGrace = 5 * time.Second

// Serve runs handler on listener until ctx is canceled, then drains in-flight
// requests within shutdownGrace and returns. A clean shutdown is not an error.
// The caller binds the listener, so it can say where the server is only once
// the port is truly its own.
func Serve(ctx context.Context, listener net.Listener, handler http.Handler) error {
	srv := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: readHeaderTimeout,
	}

	// context.AfterFunc runs the shutdown on the context package's own goroutine
	// when ctx is done, so the server drains without this package starting one.
	// The shutdown uses a fresh context on purpose: ctx is already canceled —
	// that is why we are shutting down — so reusing it would abandon the drain.
	//nolint:contextcheck // the shutdown intentionally detaches from the canceled ctx
	stop := context.AfterFunc(ctx, func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownGrace)
		defer cancel()

		_ = srv.Shutdown(shutdownCtx)
	})
	defer stop()

	err := srv.Serve(listener)
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}

	return fmt.Errorf("serving the web API: %w", err)
}
