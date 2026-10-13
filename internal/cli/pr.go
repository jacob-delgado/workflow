// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/convention"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/loop"
	"github.com/jacob-delgado/workflow/internal/proc"
)

// errNoCommitsToOpen refuses opening a pull request with nothing to propose.
// It is told through forgeWordedError, in the forge's own noun.
var errNoCommitsToOpen = errors.New("no branch with commits to open a pull request for")

// errPullAlreadyOpen refuses opening a second pull request for a branch that
// already has an open one. It is told through forgeWordedError, in the forge's own
// noun.
var errPullAlreadyOpen = errors.New("an open pull request already exists for this branch")

// forgeWordedError is a refusal told in the forge's own words — a merge request on
// GitLab — that still answers errors.Is for its sentinel, so its exit status
// is the same on every forge.
type forgeWordedError struct {
	sentinel error
	words    string
}

var _ error = forgeWordedError{}

// Error is the refusal in the forge's words.
func (e forgeWordedError) Error() string { return e.words }

// Unwrap lets errors.Is match the sentinel.
func (e forgeWordedError) Unwrap() error { return e.sentinel }

// errPushFailed reports a push that could not publish the branch.
var errPushFailed = errors.New("the branch could not be pushed")

// prSeams are what `workflow pr` reads and does, so a test can answer without a
// repository or a forge.
type prSeams struct {
	// Compose and Options are what the pull request is composed from, and under.
	Compose    loop.PullSeams
	Options    loop.PullOptions
	Push       func(ctx context.Context, branch string) (proc.Output, error)
	CreatePull func(context.Context, forge.NewPullRequest) (forge.PullRequest, error)
	// LinkPull adds a pull request's link to an issue, offered once it is open.
	// Nil — a tracker that cannot take a link — makes no offer.
	LinkPull    func(ctx context.Context, issueKey jira.Key, pullURL, title string) error
	Transitions func(context.Context, jira.Key) ([]jira.Transition, error)
	Transition  func(context.Context, jira.Key, jira.Transition, []jira.FieldValue) error
	// ReviewStatus is the status an issue moves to once its pull request is open,
	// offered after opening. Empty makes no offer.
	ReviewStatus string
	// Kind is the forge, whose own words — a merge request, marked "!" on
	// GitLab — the questions and the result use.
	Kind    forge.Kind
	Confirm func(question string) (bool, error)
	// MessagingSetUp ends an open with the step that follows it, announcing,
	// which only a configured messaging service can take.
	MessagingSetUp bool
	// Chosen is what the open was told to make in place of what is composed.
	Chosen pullChoices
}

// newPRCmd builds `workflow pr`.
func newPRCmd(prompt Prompt) *cobra.Command {
	var opts prOptions

	cmd := &cobra.Command{
		Use:   "pr",
		Short: "Open a pull or merge request for the current branch",
		Long: "Compose a pull request for the checked-out branch from its commits, the\n" +
			"issue and the repository's template — the same as the interface — pushing the\n" +
			"branch first when it is not yet on its remote. A preview — the title, the\n" +
			"branches, and the whole body — is confirmed first.\n\n" +
			"The code owners of the paths the branch changes, as CODEOWNERS on the base names\n" +
			"them, are asked to review it; the preview lists them.\n\n" +
			"--title and --body stand in for the title and description composed, and\n" +
			"--template names the repository's template the description starts from;\n" +
			"--reviewer asks the people and teams it names in place of the code owners;\n" +
			"--draft opens it as a draft; and --base merges it into another branch, drafting\n" +
			"the title and description from the commits past it and asking its code owners.\n\n" +
			"Once it is open, it offers — as the interface does — to link it on the branch's\n" +
			"issue, then to move the issue to the review status (jira.review_status). With\n" +
			"messaging set up, it ends by naming the next step, workflow announce, on stderr.\n\n" +
			"--json prints what was opened, and each offer and whether it was taken, as one\n" +
			"JSON object on stdout — under --dry-run, the pull request it would open — and\n" +
			"says everything else, the preview included, on stderr.",
		Example: examples(
			`workflow --dry-run pr                     # what pr would push and open`,
			`workflow pr --yes                         # push, open, link and move, without asking`,
			`workflow pr --yes --json | jq .pull.url   # the same, and the address it opened`,
			`workflow pr --draft --base release        # a draft into release, asking its code owners`,
		),
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runPRCommand(cmd, prompt, opts)
		},
	}

	opts.chosen.declare(cmd)
	addJSONFlag(cmd, &opts.asJSON, "print what was opened, and the offers that followed, as JSON")

	opts.write.addFlags(cmd, "go ahead without asking: push the branch when it needs it, open the pull request, "+
		"link it on the issue and move the issue to the review status")

	return cmd
}

// runPRCommand wires the real repository and forge to the pull-request flow.
func runPRCommand(cmd *cobra.Command, prompt Prompt, opts prOptions) error {
	err := opts.chosen.check()
	if err != nil {
		return err
	}

	conn, err := connect(cmd)
	if err != nil {
		return err
	}
	defer conn.closeLog()

	cfg, deps := conn.cfg, conn.deps
	seams := prSeams{
		Compose: loop.PullSeams{
			Branch:     deps.Git.Branch,
			BranchOnto: deps.Git.BranchOnto,
			FindPull:   deps.Forge.FindPullRequest,
			Templates:  deps.Forge.Templates,
			Issue:      deps.Jira.Issue,
			BrowseURL:  deps.Jira.BrowseURL,
			Owners: loop.OwnerSeams{
				ChangedPaths: deps.Git.ChangedPaths, CodeOwnersAt: deps.Git.CodeOwnersAt, Author: deps.Forge.Author,
			},
		},
		Options: loop.PullOptions{
			Project:     cfg.Jira.Project,
			TitleSource: convention.TitleSource(cfg.PullRequest.TitleSource),
			Template:    opts.chosen.chosenTemplate(),
			Base:        opts.chosen.chosenBase(),
		},
		Push:           deps.Git.Push,
		CreatePull:     deps.Forge.CreatePullRequest,
		LinkPull:       deps.Jira.LinkPullRequest,
		Transitions:    deps.Jira.Transitions,
		Transition:     deps.Jira.Transition,
		ReviewStatus:   cfg.Jira.ReviewStatus,
		Kind:           deps.Forge.Kind,
		Confirm:        func(question string) (bool, error) { return confirm(prompt, question) },
		MessagingSetUp: cfg.Messaging.Mode() != config.MessagingNone,
		Chosen:         opts.chosen,
	}

	out := outputOf(cmd)
	if !opts.asJSON {
		_, err = runPR(cmd.Context(), out, seams, opts.write)

		return err
	}

	// The report is the artifact, so everything else is said beside it.
	report, err := runPR(cmd.Context(), output{artifact: out.notes, notes: out.notes}, seams, opts.write)
	if report.ready {
		err = errors.Join(err, encodeJSON(out.artifact, report))
	}

	return err
}

// prReport is what `workflow pr --json` prints: the web's OpenedPullRequest
// shape — the pull request, a warning when it opened without everyone asked,
// and the offers that followed — with the branches it joins, and whether each
// offer was taken, since here they are answered rather than offered. ready is
// set once there is something to report: a pull request opened, or under a dry
// run one composed.
type prReport struct {
	Pull      pullReport       `json:"pull"`
	Warning   string           `json:"warning,omitempty"`
	FollowUps []followUpReport `json:"follow_ups"`
	ready     bool
}

// pullReport is a pull request as `pr --json` prints it: under a dry run, with
// no number or address yet.
type pullReport struct {
	Number int    `json:"number"`
	URL    string `json:"url"`
	Title  string `json:"title"`
	Draft  bool   `json:"draft"`
	Head   string `json:"head"`
	Base   string `json:"base"`
	Body   string `json:"body"`
}

// followUpReport is one offer made once the pull request was open: to link it
// on the issue, or to move the issue to status; and whether it was done.
type followUpReport struct {
	Action   string `json:"action"`
	IssueKey string `json:"issue_key"`
	Status   string `json:"status,omitempty"`
	Done     bool   `json:"done"`
}

// The offers a pull request's open is followed by, as the web names them.
const (
	followUpLink       = "link"
	followUpTransition = "transition"
)

// openedReport is the report of request opened as pull, its offers to follow.
func openedReport(request forge.NewPullRequest, pull forge.PullRequest) prReport {
	return prReport{
		Pull: pullReport{
			Number: pull.Number, URL: pull.URL, Title: cmp.Or(pull.Title, request.Title),
			Draft: request.Draft, Head: request.Head.Branch, Base: request.Base, Body: request.Body,
		},
		FollowUps: []followUpReport{},
		ready:     true,
	}
}

// runPR composes the pull request, previews it — the code owners it asks to
// review among what it shows — pushes the branch when needed,
// and opens it once confirmed, then follows up on the branch's issue, and
// reports what it opened and offered. A dry run says what it would do at each
// of those steps, the offers included, and reports the pull request it would
// open.
func runPR(ctx context.Context, out output, seams prSeams, opts writeOptions) (prReport, error) {
	request, branch, err := composed(ctx, seams)
	if err != nil {
		return prReport{}, err
	}

	previewPull(out.artifact, request, branch, seams.Chosen)

	issueKey, _ := loop.JiraIssue(branch, seams.Options.Project)

	proceed, err := opts.proceed(out.notes, seams.Confirm, writePrompt{
		question: openQuestion(branch, draftNoun(request, seams.Kind)),
		dryRun:   "dry run: would " + pushClause(branch) + "open " + request.Title,
		declined: "Not opened.",
	})
	if err != nil {
		return prReport{}, err
	}

	if opts.dryRun {
		// Nothing was opened, so each offer the open would lead to says so in its
		// own dry-run line.
		return followUp(
			ctx, out.notes, seams, openedPull{issueKey: issueKey}, opts, openedReport(request, forge.PullRequest{}),
		)
	}

	if !proceed {
		return prReport{}, nil
	}

	return openPull(ctx, out, seams, opts, request, branch)
}

// previewPull prints the pull request about to be opened: its title, the
// branches it joins and whether it is a draft, whom it asks to review — the
// code owners unless chosen named others — and, after a blank line, its whole
// body, so the question is asked about what was shown.
func previewPull(artifact io.Writer, request forge.NewPullRequest, branch gitrepo.Branch, chosen pullChoices) {
	fmt.Fprintln(artifact, "Open "+request.Title)

	branches := "  " + branch.Name + " → " + request.Base
	if request.Draft {
		branches += " (draft)"
	}

	fmt.Fprintln(artifact, branches)

	if reviewers := slices.Concat(request.Reviewers, request.TeamReviewers); len(reviewers) > 0 {
		fmt.Fprintln(artifact, "  reviewers "+strings.Join(reviewers, ", ")+chosen.reviewersNote())
	}

	fmt.Fprintln(artifact)
	fmt.Fprintln(artifact, strings.TrimRight(request.Body, "\n"))
}

// openPull pushes the branch when it needs it, opens the pull request, says
// so, and follows up on the branch's issue.
func openPull(
	ctx context.Context, out output, seams prSeams, opts writeOptions, request forge.NewPullRequest, branch gitrepo.Branch,
) (prReport, error) {
	err := loop.EnsurePushed(ctx, seams.Push, branch)
	if err != nil {
		return prReport{}, pushFailure(branch.Name, err)
	}

	pull, err := seams.CreatePull(ctx, request)
	if err != nil && !errors.Is(err, forge.ErrSomePeopleNotAdded) {
		return prReport{}, fmt.Errorf("opening the %s: %w", seams.Kind.Noun(), err)
	}

	fmt.Fprintln(out.artifact, "Opened "+seams.Kind.Sigil()+strconv.Itoa(pull.Number)+" "+pull.URL)

	report := openedReport(request, pull)

	if err != nil {
		// The pull request is open; who could not be added is a warning, as
		// the interface and the web give it, not a failure to open.
		fmt.Fprintf(out.notes, "Opened without everyone asked: %v\n", err)
		report.Warning = err.Error()
	}

	issueKey, _ := loop.JiraIssue(branch, seams.Options.Project)

	report, err = followUp(ctx, out.notes, seams, openedPull{issueKey: issueKey, pull: pull}, opts, report)
	if err == nil && seams.MessagingSetUp {
		fmt.Fprintln(out.notes, "Announce it with workflow announce.")
	}

	return report, err
}

// openedPull is a pull request just opened — under a dry run, the one that
// would be, with no number yet — and the Jira issue its branch names: empty
// when it names none, or only a forge issue number.
type openedPull struct {
	issueKey jira.Key
	pull     forge.PullRequest
}

// composeRefusal words a refusal to compose in the command line's own terms,
// pointing at the pull request that is already open. The shared layer's words
// stay the same on every surface; the address is added here, where it is shown
// to the person who asked.
func composeRefusal(err error, kind forge.Kind) error {
	if open, ok := errors.AsType[loop.PullAlreadyOpenError](err); ok {
		return forgeWordedError{errPullAlreadyOpen, "an open " + kind.Noun() + " already exists for this branch: " +
			open.Pull.URL}
	}

	if errors.Is(err, loop.ErrNothingToOpen) {
		return forgeWordedError{errNoCommitsToOpen, "no branch with commits to open a " + kind.Noun() + " for"}
	}

	return err
}

// pushFailure words a push that did not publish the branch: with the push's own
// output when it ran and failed, and with why it could not start otherwise.
func pushFailure(name string, err error) error {
	if failed, ok := errors.AsType[loop.PushFailedError](err); ok {
		return fmt.Errorf("%w:\n%s", errPushFailed, strings.Join(failed.Output, "\n"))
	}

	return fmt.Errorf("pushing %s: %w", name, err)
}

// openQuestion asks to open the pull request, named by the forge's noun, and
// names the push that comes first, and where it goes, when the branch is not
// yet on its push remote, so a yes is never taken for more than was asked.
func openQuestion(branch gitrepo.Branch, noun string) string {
	if branch.Pushed() {
		return "Open the " + noun + "?"
	}

	return "Push " + branch.Name + " to " + branch.PushRemote + " and open the " + noun + "?"
}

// pushClause names the push a not-yet-pushed branch needs first, and its
// remote, for the dry-run line, or nothing when the branch is already up.
func pushClause(branch gitrepo.Branch) string {
	if branch.Pushed() {
		return ""
	}

	return "push " + branch.Name + " to " + branch.PushRemote + " and "
}
