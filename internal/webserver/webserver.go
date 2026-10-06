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
	"github.com/jacob-delgado/workflow/internal/codeowners"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/hooks"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/loop"
	"github.com/jacob-delgado/workflow/internal/proc"
	"github.com/jacob-delgado/workflow/internal/seams"
	"github.com/jacob-delgado/workflow/internal/store"
)

// Deps is what the server asks of the world, as plain functions over the domain
// clients — the same seams internal/seams declares, narrowed to what the API
// needs, and Taskwarrior's bundle whole, since the API uses every function in
// it. A nil function means the service is not configured. A list or
// snapshot read then answers empty; the issue read answers 422, as no issue
// tracker is configured; a write whose own seam is nil answers 422, as not
// available; and the announcement and pull request drafts, and the announce
// and open writes past that check, answer 409, as there is nothing to announce
// or open, when the branch read or the pull request find is missing.
type Deps struct {
	Search func(jql string, startAt int) (jira.SearchResult, error)
	// SearchLenient is Search for which of a branch list's issue keys are the
	// user's: a key the tracker does not know is skipped, not refused.
	SearchLenient func(jql string, startAt int) (jira.SearchResult, error)
	Issue         func(key jira.Key) (jira.IssueDetail, error)
	BrowseURL     func(key jira.Key) string
	Branch        func() (gitrepo.Branch, error)
	Branches      func() ([]string, error)
	Checkout      func(name string) error
	CreateBranch  func(name, start string) error
	// CreateWorktree creates a branch from start in a new worktree beside the
	// repository, and says where.
	CreateWorktree func(name, start string) (string, error)
	// Fetch updates origin's tracking refs before work starts, so a new branch
	// starts from what origin holds now. Nil starts from what is there.
	Fetch   func() error
	Commit  func(message string) (proc.Output, error)
	Push    func(branch string) (proc.Output, error)
	Changes func() ([]gitrepo.Change, error)
	// Diff reads a changed file's diff against HEAD, a change as Changes read
	// it — never a path a request names.
	Diff func(change gitrepo.Change) ([]string, error)
	// RunHook runs one git hook, and Rebase, Amend and Fixup rewrite the
	// branch's history, each streaming its output, as seams.Hooks and
	// seams.Git bind them; nil where there is no repository.
	RunHook func(hook string) (proc.Output, error)
	// HookExisting reports the hooks git would run that lefthook does not
	// manage, and whether lefthook is configured; HookWrite writes a
	// generated configuration and installs lefthook, as seams.Hooks binds
	// them. Nil where there is no repository.
	HookExisting func() ([]hooks.GitHook, bool)
	HookWrite    func(generated hooks.Generated) error
	Rebase       func(base string) (proc.Output, error)
	Amend        func() (proc.Output, error)
	Fixup        func(hash string) (proc.Output, error)
	FindPull     func(branch string) (forge.PullRequest, bool, error)
	CreatePull   func(request forge.NewPullRequest) (forge.PullRequest, error)
	// EditPull changes a pull request's title and description, as linking a
	// branch to its issue adds the line naming it.
	EditPull  func(pull forge.PullRequest, edit forge.PullRequestEdit) (forge.PullRequest, error)
	Templates func() []forge.Template
	// ChangedPaths and CodeOwnersAt read the code owners of the branch's
	// changes, proposed as the pull request draft's reviewers. Nil proposes
	// nobody.
	ChangedPaths func(base string) ([]string, error)
	CodeOwnersAt func(base string) (codeowners.File, bool, error)
	CheckCI      func(pull forge.PullRequest, head string) (forge.CI, error)
	// JobLog reads the end of a check's log, one the forge keeps a log for.
	JobLog func(check forge.Check) (forge.JobLog, error)
	// Rerun re-runs the failed CI on a pull request; Merge merges one by a
	// method MergeMethods says the repository permits; Finish finishes a
	// merged branch — switch to base, catch it up, delete it — as seams.Forge
	// and seams.Git bind them. Nil where there is no forge, or no repository.
	Rerun        func(pull forge.PullRequest, head string) (bool, error)
	Merge        func(pull forge.PullRequest, method forge.MergeMethod) error
	MergeMethods func() ([]forge.MergeMethod, error)
	Finish       func(branch, base string) error
	Author       func() (string, error)
	Post         func(channel, text string) error
	// RemoteBranches lists the branches on the remotes by name, without the
	// remote's prefix, so a branch only the remote has is in flight too.
	RemoteBranches func() ([]string, error)
	// IssueLinks is every branch linked to an issue by hand, by branch name,
	// so a branch whose name names none is still in flight for its issue.
	IssueLinks func() map[string]string
	// LinkIssue links a branch to an issue by hand, and UnlinkIssue forgets its
	// link: for work begun outside workflow on a branch whose name names none.
	LinkIssue   func(branch, issueKey string) error
	UnlinkIssue func(branch string) error
	// ReviewRequests lists the pull requests on the forge that ask for your
	// review, across repositories — the queue `workflow reviews` prints.
	ReviewRequests func() ([]forge.ReviewRequest, error)
	// LinkPullRequest records a pull request as a link on an issue; nil where
	// the tracker cannot take one — the forge's own issues.
	LinkPullRequest func(issueKey jira.Key, pullURL, title string) error
	Transitions     func(issueKey jira.Key) ([]jira.Transition, error)
	Transition      func(issueKey jira.Key, to jira.Transition, values []jira.FieldValue) error
	// Comment posts a comment on a Jira issue and answers it as Jira stored
	// it; nil where no Jira is configured.
	Comment func(issueKey jira.Key, text string) (jira.Comment, error)
	// Assign sets an issue's assignee by username, and AddWorklog logs time
	// spent on a Jira issue, as seams.Jira binds them; nil where no tracker
	// takes them.
	Assign     func(issueKey jira.Key, assignee string) error
	AddWorklog func(issueKey jira.Key, timeSpent, comment string) (jira.Worklog, error)
	// Stage and Unstage move one change into and out of the index: a change as
	// Changes read it, carrying a rename's original path — never a path a
	// request names.
	Stage   func(change gitrepo.Change) error
	Unstage func(change gitrepo.Change) error
	// Discard drops one change from the index and the work tree, which cannot
	// be undone: a change as Changes read it, never a path a request names.
	Discard func(change gitrepo.Change) error
	// LastScope is the commit scope last used in this repository, if one was,
	// and RecordScope remembers the one a commit just used: the store the
	// terminal's composer learns from. Nil where there is no store.
	LastScope   func() (string, bool)
	RecordScope func(scope string)
	// Announced is every pull request announced in this repository, from any
	// surface, and RecordAnnounce remembers one just made: the store the
	// terminal and workflow announce keep it in. Nil where there is no store.
	Announced      func() []loop.Announced
	RecordAnnounce func(made loop.Announced)
	// Tasks is what the server asks of Taskwarrior. A nil Install, or one that
	// fails, answers the task list and the snapshot's summary as not available —
	// never a 404 — and a write with a nil function is refused as unprocessable.
	Tasks seams.Tasks
	// HomeDir is your home directory, which Taskwarrior's words can name and an
	// answer shows as ~. Nil, or one that fails, leaves them naming it.
	HomeDir func() (string, error)
	// CheckKeys says why the terminal interface would refuse a ui.keys map,
	// or nil where it would start on it. Nil here means no map is checked.
	CheckKeys func(keys map[string]string) error
	// Unexpected hears each failure the server answers as an internal error —
	// one no class of failure explains, or an answer that could not be
	// written — whose cause the answer leaves out. Nil says nothing.
	Unexpected func(err error)
	// UseForgeSettings applies forge settings just saved to every forge call
	// after the save, and reports which forge the remote is on under them, so a
	// token, host or kind saved in Settings needs no restart. Nil leaves the
	// forge as it was started.
	UseForgeSettings func(settings config.Forge) forge.Kind

	// PlaceSlackCredentials keeps a Slack user token's secrets, typed into
	// Settings, where the configuration keeps them — refreshing the token once
	// and saving the pair to the macOS keychain, and answering the
	// configuration without them, or answering it unchanged where the file is
	// where they are kept. Nil writes them into the file as they came.
	PlaceSlackCredentials func(cfg config.Config) (config.Config, error)

	// UseMessagingSettings applies messaging settings just saved to every post
	// after the save. Nil leaves messaging as it was started.
	UseMessagingSettings func(settings config.Messaging)

	// LocalData is the store's directory and the database files in it, each
	// with its size and what it holds, read without writing. RemoveLocalData
	// removes the cache, or with store.CleanAll the kept associations too. Nil
	// answers the Local data area as not available.
	LocalData       func(ctx context.Context) (string, []store.DataFile, error)
	RemoveLocalData func(scope store.CleanScope) error

	// OwnerLinks, LinkOwner and ForgetOwner are whom each code owner on this
	// repository's forge host is on Slack, kept between sessions; RepoGroups
	// and SetRepoGroups the Slack user groups this repository may tag; and
	// LastGroups and RecordGroups the groups its last announcement chose —
	// each in the Slack workspace Workspace names, as seams.Store binds them.
	// A nil OwnerLinks or RepoGroups answers People and groups as not
	// available, and an announcement as tagging no one.
	OwnerLinks    func(workspace string) ([]loop.OwnerLink, error)
	LinkOwner     func(workspace string, decision loop.OwnerLink) error
	ForgetOwner   func(workspace, owner string) error
	RepoGroups    func(workspace string) ([]loop.SlackTarget, error)
	SetRepoGroups func(workspace string, groups []loop.SlackTarget) error
	LastGroups    func(workspace string) ([]string, bool)
	RecordGroups  func(workspace string, ids []string) error

	// IsGroup reports whether a bare CODEOWNERS name is a top-level GitLab
	// group, which is tagged as a team, as seams.Forge binds it. Nil, as on
	// GitHub, takes every bare name for a person.
	IsGroup func(name string) (bool, error)

	// Workspace is the ID of the Slack workspace the user token is for, as
	// seams.Messaging binds it. When it cannot be read, an announcement tags
	// no one and says why, and People and groups is refused with why. Nil
	// is no Slack user token.
	Workspace func() (string, error)

	// ChannelMembers and UserGroups read the Slack directory an owner is
	// linked from. Whether they are read at all follows the configuration in
	// effect, which a save in Settings changes: only a Slack user token tags,
	// and a read answered messaging.ErrNoCredential is no directory. Nil
	// means an announcement offers no tags.
	ChannelMembers func(channel string) ([]loop.SlackTarget, error)
	UserGroups     func() ([]loop.SlackTarget, error)

	// CommitsBetween, JiraActivity and ForgeActivity read back what you did
	// over a period, for the Summary, beside Tasks' Touched. A nil one is a
	// source the Summary does not ask.
	CommitsBetween func(start, end time.Time) []loop.RepositoryCommits
	JiraActivity   func(start, end time.Time) (jira.Activity, error)
	ForgeActivity  func(start, end time.Time) (forge.Activity, error)
	// Clock tells the time, for when the event stream last asked the forge.
	// Nil means the system clock.
	Clock func() time.Time

	// Repositories is where the server works and any other directory read the
	// same way. Favorites, Favor and Unfavor are the favorite directories the
	// store keeps; Favor and Unfavor are nil where it keeps nothing.
	Repositories seams.Repositories
	Favorites    func() ([]string, error)
	Favor        func(dir string) error
	Unfavor      func(dir string) error
	// Reach wires another directory as this one was wired, for a switch; nil
	// where switching is not offered.
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

	// assigned is which of the branches' issues the tracker last said are yours.
	assigned assignedCache

	// held is the announcement held until its pull request's CI passes.
	held heldAnnouncement

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

// authorCache is who the forge says a post would come from, kept from its first
// answer for the stream, GET /api/messaging and the announcement alike: the
// forge connection it comes through is kept once made, so the answer does not
// change while the server runs, and asking every frame spent a forge request
// per open page each interval. A failed read is not kept, so the next read asks
// again. Its lock is its own, since every open stream reads it.
type authorCache struct {
	mu    sync.Mutex
	name  string
	known bool
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
// refused. The server starts from the configuration file at cfg.Path
// as startingPoint reads it, not from cfg alone, which the process read a
// moment before. It fails when the embedded spec cannot be loaded or routed,
// which is a build defect, or when that file cannot be read.
func Handler(deps Deps, cfg config.Config, info Info, assets fs.FS) (http.Handler, error) {
	// Trade-off TRADE-14: the embedded spec loads in every build a test runs.
	validator, err := validate()
	if err != nil {
		return nil, err
	}

	held, err := newWorlds(World{Deps: deps, Config: cfg, Info: info}, validator)
	if err != nil {
		return nil, err
	}

	// The API is validated against the contract; the app is not, since its paths
	// are not in the spec, so only the /api subtree passes through the validator.
	root := http.NewServeMux()
	root.Handle("/api/", held)
	root.Handle("/", spaHandler(assets))

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

// Serve runs handler at addr until ctx is canceled, then drains in-flight
// requests within shutdownGrace and returns. A clean shutdown is not an error.
// Production passes a LoopbackAddr; a test passes a loopback address with port 0.
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
