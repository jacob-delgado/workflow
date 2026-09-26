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
	"net/http"
	"sync"
	"time"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/proc"
)

// Deps is what the server asks of the world, as plain functions over the domain
// clients — the same seams the interface declares, narrowed to what the API
// needs. A nil function means the service is not configured: a read answers
// with an empty result rather than an error, and a write as not available.
type Deps struct {
	Search       func(jql string, startAt int) (jira.SearchResult, error)
	Issue        func(key jira.Key) (jira.IssueDetail, error)
	BrowseURL    func(key jira.Key) string
	Branch       func() (gitrepo.Branch, error)
	Branches     func() ([]string, error)
	Checkout     func(name string) error
	CreateBranch func(name, start string) error
	Commit       func(message string) (proc.Output, error)
	Push         func(branch string) (proc.Output, error)
	Changes      func() ([]gitrepo.Change, error)
	FindPull     func(branch string) (forge.PullRequest, bool, error)
	CreatePull   func(request forge.NewPullRequest) (forge.PullRequest, error)
	Templates    func() []forge.Template
	CheckCI      func(pull forge.PullRequest, head string) (forge.CI, error)
	Author       func() (string, error)
	Post         func(channel, text string) error
	// ReviewRequests lists the pull requests on the forge that ask for your
	// review, across repositories — the queue `workflow reviews` prints.
	ReviewRequests func() ([]forge.ReviewRequest, error)
	// LinkPullRequest records a pull request as a link on an issue; nil where
	// the tracker cannot take one — the forge's own issues.
	LinkPullRequest func(issueKey jira.Key, pullURL, title string) error
	Transitions     func(issueKey jira.Key) ([]jira.Transition, error)
	Transition      func(issueKey jira.Key, to jira.Transition, values []jira.FieldValue) error
	// Stage and Unstage move one change into and out of the index: a change as
	// Changes read it, carrying a rename's original path — never a path a
	// request names.
	Stage   func(change gitrepo.Change) error
	Unstage func(change gitrepo.Change) error
	// LastScope is the commit scope last used in this repository, if one was,
	// and RecordScope remembers the one a commit just used: the store the
	// terminal's composer learns from. Nil where there is no store.
	LastScope   func() (string, bool)
	RecordScope func(scope string)
	// CheckKeys says why the terminal interface would refuse a ui.keys map,
	// or nil where it would start on it. Nil here means no map is checked.
	CheckKeys func(keys map[string]string) error
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

	// StreamInterval is how often the event stream re-pushes a snapshot. A zero
	// or negative value takes defaultStreamInterval.
	StreamInterval time.Duration
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

	// path is where the configuration file lives, fixed at construction from the
	// trusted location the process resolved. The write endpoint always saves here
	// and never to a path from the request body, so a client cannot redirect the
	// write — the file location is the server's to decide, not the caller's.
	path string

	mu  sync.RWMutex
	cfg config.Config

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

	// scope is the commit scope learned in this repository, held once read.
	scope scopeCache
}

// scopeCache is the store's last commit scope, read the first time a frame
// asks — opening the store's database on every frame is the cost it avoids —
// and read again once after a commit here records one, so it holds what the
// store kept: nothing, when the store is off. Its lock is its own, since the
// streams read it while a commit clears it.
type scopeCache struct {
	mu    sync.Mutex
	read  bool
	value string
	found bool
}

var _ api.StrictServerInterface = (*server)(nil)

// Handler builds the http.Handler that serves the web interface: the API under
// /api, checked against the contract by the request validator, and the embedded
// single-page app under every other path. The whole surface is behind the
// loopback guard, so a browser aimed at the server from a foreign origin is
// refused. A nil ui serves a notice instead of the app, for a build with no
// frontend embedded. The server starts from the configuration file at cfg.Path
// as startingPoint reads it, not from cfg alone, which the process read a
// moment before. It fails when the embedded spec cannot be loaded, which is a
// build defect, or when that file cannot be read.
func Handler(deps Deps, cfg config.Config, info Info, assets fs.FS) (http.Handler, error) {
	doc, err := loadSpec()
	if err != nil {
		return nil, err
	}

	inEffect, seen, err := startingPoint(cfg)
	if err != nil {
		return nil, fmt.Errorf("reading the configuration file: %w", err)
	}

	srv := &server{deps: deps, info: info, path: cfg.Path, cfg: inEffect, seen: seen}

	strict := api.NewStrictHandlerWithOptions(srv, nil, api.StrictHTTPServerOptions{
		RequestErrorHandlerFunc:  writeRequestError,
		ResponseErrorHandlerFunc: writeResponseError,
	})

	apiMux := http.NewServeMux()

	// The event stream is a streaming response the strict, one-response-object
	// interface cannot express, so it is registered by hand rather than generated.
	apiMux.HandleFunc("GET /api/events", srv.streamEvents)

	apiHandler := api.HandlerWithOptions(strict, api.StdHTTPServerOptions{
		BaseRouter:       apiMux,
		ErrorHandlerFunc: writeRequestError,
	})

	// The API is validated against the contract; the app is not, since its paths
	// are not in the spec, so only the /api subtree passes through the validator.
	root := http.NewServeMux()
	root.Handle("/api/", validate(doc)(apiHandler))
	root.Handle("/", uiHandler(assets))

	return guardLoopback(refuseWritesInDryRun(info.DryRun, root)), nil
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
	loaded, seen, err := config.LoadFileAt(cfg.Path)
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

// uiHandler serves the embedded app, or a notice when no assets are embedded.
func uiHandler(assets fs.FS) http.Handler {
	if assets == nil {
		return notEmbedded()
	}

	return spaHandler(assets)
}

// LoopbackAddr is where the server listens in production: the loopback
// interface only, so the API is reachable from this machine and nowhere else.
const LoopbackAddr = "127.0.0.1:7000"

// readHeaderTimeout bounds how long a client may take to send its headers, so a
// slow or stuck connection cannot tie up the server.
const readHeaderTimeout = 10 * time.Second

// shutdownGrace is how long in-flight requests are given to finish when the
// context is canceled before the listener is closed.
const shutdownGrace = 5 * time.Second

// Serve runs handler at addr until ctx is canceled, then drains in-flight
// requests within shutdownGrace and returns. A clean shutdown is not an error.
// Production passes LoopbackAddr; a test passes a loopback address with port 0.
func Serve(ctx context.Context, addr string, handler http.Handler) error {
	srv := &http.Server{
		Addr:              addr,
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

	err := srv.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}

	return fmt.Errorf("serving the web API: %w", err)
}
