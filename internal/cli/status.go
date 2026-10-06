// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/loop"
	"github.com/jacob-delgado/workflow/internal/progress"
	"github.com/jacob-delgado/workflow/internal/sanitize"
	"github.com/jacob-delgado/workflow/internal/wiring"
)

// statusSeams are what `workflow status` reads to build its line. They are
// seams so a test can answer without a repository or a network.
type statusSeams struct {
	Branch      func() (gitrepo.Branch, error)
	Changes     func() ([]gitrepo.Change, error)
	FindPull    func(branch string) (forge.PullRequest, bool, error)
	CheckStatus func(pull forge.PullRequest, head string) (forge.CI, error)
	Issue       func(issueKey jira.Key) (jira.IssueDetail, error)
	// Memory is what the store remembers of the announcements made here.
	Memory loop.AnnounceMemory
	// Project is the configured Jira project; empty falls back to the shape guard.
	Project string
	// Service is the configured messaging service, which names the last stage.
	Service string
	// Forge is the forge the remote points at, which names it in a note.
	Forge forge.Kind
	// Tracker names the issue tracker in a note: Jira, or the forge when
	// its issues stand in for Jira's.
	Tracker string
}

// newStatusCmd builds `workflow status [directory...]`.
func newStatusCmd() *cobra.Command {
	var asJSON bool

	cmd := &cobra.Command{
		Use:   "status [directory...]",
		Short: "Print the current work's issue, stage and CI on one line",
		Long: "Print, on one line, the issue the branch is for, how far along the loop\n" +
			"the work has got, and how CI stands — the same progress the interface's\n" +
			"top row shows, for a shell prompt or a status bar. --json prints it as data.\n" +
			"A service that refuses to answer leaves its stage as though there were nothing\n" +
			"to say — CI none — and is named on standard error, one line each.\n\n" +
			"Given one or more directories, it prints a labeled line for each, so\n" +
			"`workflow status ~/src/*` reports every repository at once. Each reads its\n" +
			"own configuration. A directory that cannot be read still gets its line,\n" +
			"saying why, and the command then fails, as it does outside a repository.",
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runStatusCommand(cmd, asJSON, args)
		},
	}

	cmd.Flags().BoolVar(&asJSON, "json", false, "print the status as JSON")

	return cmd
}

// runStatusCommand prints the status of the current repository, or of each
// named directory.
func runStatusCommand(cmd *cobra.Command, asJSON bool, dirs []string) error {
	if len(dirs) == 0 {
		return statusHere(cmd, asJSON)
	}

	return statusAcross(cmd, dirs, asJSON)
}

// statusHere prints the status of the current directory: a bare line, and a
// directory that is no repository is an error.
func statusHere(cmd *cobra.Command, asJSON bool) error {
	// A missing configuration is not fatal: the repository stages still read,
	// and the service stages simply stay not-started.
	conn, err := connect(cmd)
	if err != nil {
		return err
	}
	defer conn.closeLog()

	return runStatus(outputOf(cmd), seamsFor(conn), conn.cfg.UI.ASCII, asJSON)
}

// statusAcross prints one labeled status per named directory. A directory that
// cannot be read is noted in its row rather than stopping the rest, and then
// fails the command, as bare status fails outside a repository.
func statusAcross(cmd *cobra.Command, dirs []string, asJSON bool) error {
	requestLog, closeLog, err := requestLogFor(cmd)
	if err != nil {
		return err
	}
	defer closeLog()

	out := outputOf(cmd)
	statuses := statusesOf(cmd, dirs, requestLog)

	if asJSON {
		err = statusesJSON(out.artifact, statuses)
	} else {
		statusLines(out.artifact, statuses)
	}

	for _, status := range statuses {
		writeUnread(out.notes, status.label+": ", status.facts.unread)
	}

	return errors.Join(err, unreadDirectories(statuses))
}

// statusLines prints each directory's labeled line, or why it has none.
func statusLines(out io.Writer, statuses []directoryStatus) {
	for _, status := range statuses {
		fmt.Fprintf(out, "%s  ", status.label)

		if status.err != nil {
			fmt.Fprintln(out, unreadReason(status.err))

			continue
		}

		renderStatusLine(out, status.facts, status.ascii)
	}
}

// unreadReason says why a directory has no status: that it is no repository,
// or, for a repository that could not be read, the error itself.
func unreadReason(err error) string {
	if errors.Is(err, gitrepo.ErrNotARepository) {
		return gitrepo.ErrNotARepository.Error()
	}

	return err.Error()
}

// unreadDirectories joins the error of every directory that could not be read,
// each named, so the exit status tells a script one of them failed.
func unreadDirectories(statuses []directoryStatus) error {
	var failures []error

	for _, status := range statuses {
		if status.err != nil {
			failures = append(failures, fmt.Errorf("%s: %w", status.label, status.err))
		}
	}

	return errors.Join(failures...)
}

// directoryStatus is one named directory's status, or why it has none.
type directoryStatus struct {
	label string
	facts statusFacts
	ascii bool
	err   error
}

// statusesOf gathers the status of the repository at each directory, which
// reads its own configuration, recording every request in one log.
func statusesOf(cmd *cobra.Command, dirs []string, requestLog *wiring.RequestLog) []directoryStatus {
	home := configHome()
	statuses := make([]directoryStatus, 0, len(dirs))

	for _, dir := range dirs {
		conn := connectAt(cmd, dir, home, requestLog)
		facts, err := statusOf(conn)

		statuses = append(statuses, directoryStatus{
			label: repoLabel(dir), facts: facts, ascii: conn.cfg.UI.ASCII, err: err,
		})
	}

	return statuses
}

// statusOf gathers the status of the repository a connection wired, refusing
// one whose configuration file could not be read.
func statusOf(conn connection) (statusFacts, error) {
	err := conn.unreadConfiguration()
	if err != nil {
		return statusFacts{}, err
	}

	return statusFromSeams(seamsFor(conn))
}

// seamsFor reads the repository, forge, Jira and store a connection wired.
func seamsFor(conn connection) statusSeams {
	return statusSeams{
		Branch:      conn.deps.Git.Branch,
		Changes:     conn.deps.Git.Changes,
		FindPull:    conn.deps.Forge.FindPullRequest,
		CheckStatus: conn.deps.Forge.CheckStatus,
		Issue:       conn.deps.Jira.Issue,
		Memory:      loop.AnnounceMemory{Recorded: conn.deps.Store.Announced},
		Project:     conn.cfg.Jira.Project,
		Service:     conn.cfg.Messaging.Service(),
		Forge:       conn.deps.Forge.Kind,
		Tracker:     trackerName(conn),
	}
}

// trackerName names the tracker the issue is read from: Jira when it is
// configured, and otherwise the forge, whose issues stand in for it.
func trackerName(conn connection) string {
	if conn.cfg.Jira.Configured() {
		return "Jira"
	}

	return forgeName(conn.deps.Forge.Kind)
}

// repoLabel names a directory in the output: its base name, or the path itself
// when the base name would not say which repository it is.
func repoLabel(dir string) string {
	base := filepath.Base(dir)
	if base == "." {
		return dir
	}

	return base
}

// statusFacts is everything the line and the JSON are built from, and why
// each service that refused to answer did not.
type statusFacts struct {
	issue   string
	summary string
	stages  []progress.Stage
	ci      forge.CIState
	unread  []error
}

// runStatus gathers the current state and prints it, as a line or as JSON,
// with a note on stderr for each service that refused to answer.
func runStatus(out output, seams statusSeams, ascii, asJSON bool) error {
	facts, err := statusFromSeams(seams)
	if err != nil {
		return err
	}

	writeUnread(out.notes, "", facts.unread)

	if asJSON {
		return renderStatusJSON(out.artifact, facts)
	}

	renderStatusLine(out.artifact, facts, ascii)

	return nil
}

// writeUnread notes each service that refused to answer, one line apiece
// after label. Its words are the service's, so none may drive the terminal.
func writeUnread(notes io.Writer, label string, unread []error) {
	for _, err := range unread {
		fmt.Fprintln(notes, label+sanitize.Line(err.Error()))
	}
}

// statusFromSeams reads the branch and derives the facts, or fails when the
// branch cannot be read at all.
func statusFromSeams(seams statusSeams) (statusFacts, error) {
	branch, err := seams.Branch()
	if err != nil {
		return statusFacts{}, fmt.Errorf("reading the branch: %w", err)
	}

	return gather(seams, branch), nil
}

// gather reads the issue, the pull request, its CI and whether it was
// announced, and derives the stages.
// A service that will not answer leaves its stage not-started rather than
// failing the whole line, and is named among the facts' unread.
func gather(seams statusSeams, branch gitrepo.Branch) statusFacts {
	issueRef, named := loop.IssueOf(branch, seams.Project)
	facts := statusFacts{issue: issueRef.Key}

	var issueErr error
	if named {
		facts.summary, issueErr = issueSummary(seams, issueRef.Key)
	}

	onFeature := branch.Name != "" && branch.Name != branch.BaseName()
	review := gatherReview(seams, branch, onFeature)
	changes, changesErr := seams.Changes()
	facts.ci = review.ci
	facts.unread = unreadServices([]serviceRead{
		{name: seams.Tracker, err: issueErr},
		{name: forgeName(seams.Forge), err: review.err},
		{name: "The working tree", err: changesErr},
	})

	facts.stages = progress.Stages(progress.Work{
		OnFeatureBranch:    onFeature,
		IssueNamed:         named,
		Commits:            len(branch.Commits),
		UncommittedChanges: len(changes),
		PullRequest:        review.state,
		CI:                 review.ci,
		ChangesRequested:   review.pull.ChangesRequested,
		Announced:          review.state != progress.NoPullRequest && announcedNow(seams.Memory, review.pull, review.ci),
	}, seams.Service)

	return facts
}

// serviceRead is one service's read, by the name a note gives it, and why it
// failed, or nil.
type serviceRead struct {
	name string
	err  error
}

// unreadServices is why each service that refused to answer did not, in its
// name. A service there was nothing to ask — no credential set, no forge the
// origin names — answered nothing because nothing was asked, and one that
// says the branch's issue does not exist has answered; both are left out.
func unreadServices(reads []serviceRead) []error {
	var unread []error

	for _, read := range reads {
		if read.err != nil && !answeredOrUnasked(read.err) {
			unread = append(unread, fmt.Errorf("%s could not be read: %w", read.name, read.err))
		}
	}

	return unread
}

// answeredOrUnasked reports an error that says a service was never asked —
// Jira or the forge has no credential, or the origin names no forge workflow
// reads — or that the tracker answered it holds no such issue.
func answeredOrUnasked(err error) bool {
	for _, said := range []error{
		jira.ErrNoCredential, forge.ErrNoToken, forge.ErrNotARemote, forge.ErrUnknownForge, jira.ErrNotFound,
	} {
		if errors.Is(err, said) {
			return true
		}
	}

	return false
}

// forgeName names the forge as the subject of a note: by its name when the
// remote says which, and as the forge otherwise.
func forgeName(kind forge.Kind) string {
	if kind == forge.KindUnknown {
		return "The forge"
	}

	return kind.String()
}

// issueSummary is the named issue's summary, or none, with why, when Jira
// will not answer.
func issueSummary(seams statusSeams, issueKey string) (string, error) {
	detail, err := seams.Issue(jira.Key(issueKey))
	if err != nil {
		return "", err
	}

	return detail.Issue.Summary, nil
}

// announcedNow reports that the store remembers the pull request announced at
// the moment it is at now, which is what the interface's top row reads too.
func announcedNow(memory loop.AnnounceMemory, pull forge.PullRequest, ciState forge.CIState) bool {
	return memory.Holds(loop.Announced{Pull: pull.Number, Moment: loop.AnnounceMoment(pull, forge.CI{State: ciState})})
}

// reviewRead is what the forge said of the branch's pull request: the pull,
// where its review stands, its CI, and why the forge would not say, or nil.
type reviewRead struct {
	pull  forge.PullRequest
	state progress.PullState
	ci    forge.CIState
	err   error
}

// gatherReview looks for the branch's pull request, where it stands and its
// CI, on a feature branch with a forge to ask. A read that fails reads as no
// pull request, or no CI, with its error beside.
func gatherReview(seams statusSeams, branch gitrepo.Branch, onFeature bool) reviewRead {
	none := reviewRead{state: progress.NoPullRequest, ci: forge.CINone}
	if !onFeature {
		return none
	}

	pull, found, err := seams.FindPull(branch.Name)
	if err != nil || !found {
		none.err = err

		return none
	}

	if !pull.IsOpen() {
		// A merged pull request has no live CI to poll: its review is over, as
		// the interface's spine and rail say too.
		return reviewRead{pull: pull, state: progress.PullStateOf(pull.State), ci: forge.CINone}
	}

	status, err := seams.CheckStatus(pull, branch.Head)
	if err != nil {
		return reviewRead{pull: pull, state: progress.PullRequestOpen, ci: forge.CINone, err: err}
	}

	return reviewRead{pull: pull, state: progress.PullRequestOpen, ci: status.State}
}

// renderStatusLine prints the issue, the stage glyphs and the CI state.
func renderStatusLine(out io.Writer, facts statusFacts, ascii bool) {
	var line strings.Builder

	if facts.issue != "" {
		line.WriteString(facts.issue)

		if facts.summary != "" {
			line.WriteString(" " + facts.summary)
		}

		line.WriteString("  ")
	}

	marks := make([]string, len(facts.stages))
	for index, stage := range facts.stages {
		marks[index] = statusGlyph(stage.State, ascii) + " " + stage.Name
	}

	line.WriteString(strings.Join(marks, "  "))
	line.WriteString("  CI " + ciWord(facts.ci))

	fmt.Fprintln(out, line.String())
}

// statusReport is the JSON shape of the status.
type statusReport struct {
	Issue   string        `json:"issue,omitempty"`
	Summary string        `json:"summary,omitempty"`
	Stages  []stageReport `json:"stages"`
	CI      string        `json:"ci"`
}

// stageReport is one stage as data.
type stageReport struct {
	Name  string `json:"name"`
	State string `json:"state"`
}

// renderStatusJSON prints the same facts as indented JSON.
func renderStatusJSON(out io.Writer, facts statusFacts) error {
	return encodeJSON(out, statusReport{
		Issue:   facts.issue,
		Summary: facts.summary,
		Stages:  stageReports(facts.stages),
		CI:      ciWord(facts.ci),
	})
}

// repoStatus is one repository's status in the array `status DIR...` prints.
type repoStatus struct {
	Repository string        `json:"repository"`
	Issue      string        `json:"issue,omitempty"`
	Summary    string        `json:"summary,omitempty"`
	Stages     []stageReport `json:"stages,omitempty"`
	CI         string        `json:"ci,omitempty"`
	Error      string        `json:"error,omitempty"`
}

// statusesJSON prints the status of each directory as a JSON array, one object
// per repository, so a directory that cannot be read is a row with an error
// rather than a gap in the array.
func statusesJSON(out io.Writer, statuses []directoryStatus) error {
	reports := make([]repoStatus, 0, len(statuses))

	for _, status := range statuses {
		report := repoStatus{Repository: status.label}

		if status.err != nil {
			report.Error = unreadReason(status.err)
		} else {
			facts := status.facts
			report.Issue, report.Summary = facts.issue, facts.summary
			report.Stages, report.CI = stageReports(facts.stages), ciWord(facts.ci)
		}

		reports = append(reports, report)
	}

	return encodeJSON(out, reports)
}

// stageReports turns the stages into their JSON shape.
func stageReports(stages []progress.Stage) []stageReport {
	reports := make([]stageReport, len(stages))
	for index, stage := range stages {
		reports[index] = stageReport{Name: stage.Name, State: stateWord(stage.State)}
	}

	return reports
}

// statusGlyph is the mark for a stage state, plain by default or ASCII. The
// marks are a map, not a switch, so there is no last-case arm gobco can never
// see; exhaustive keeps it complete, as it does stateWord's and ciWord's.
func statusGlyph(state progress.State, ascii bool) string {
	marks := map[progress.State]struct{ unicode, plain string }{
		progress.Done:       {unicode: "●", plain: "#"},
		progress.InFlight:   {unicode: "◐", plain: "*"},
		progress.Failed:     {unicode: "✗", plain: "x"},
		progress.NotStarted: {unicode: "○", plain: "o"},
	}[state]

	if ascii {
		return marks.plain
	}

	return marks.unicode
}

// stateWord names a stage state for JSON.
func stateWord(state progress.State) string {
	return map[progress.State]string{
		progress.Done:       "done",
		progress.InFlight:   "in_flight",
		progress.Failed:     "failed",
		progress.NotStarted: "not_started",
	}[state]
}

// ciWord names how CI stands for the line and the JSON.
func ciWord(state forge.CIState) string {
	return map[forge.CIState]string{
		forge.CIRunning: "running",
		forge.CIPassed:  "passed",
		forge.CIFailed:  "failed",
		forge.CINone:    "none",
	}[state]
}
