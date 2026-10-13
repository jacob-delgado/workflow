// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/jacob-delgado/workflow/internal/convention"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/loop"
	"github.com/jacob-delgado/workflow/internal/sanitize"
)

// errBodyBesideTemplate refuses --body beside --template: the body is the
// whole description, so there is nothing for a template to start.
var errBodyBesideTemplate = errors.New("--body is the whole description and --template the one it starts " +
	"from: use one or the other")

// prOptions are pr's flags: what to open in place of what is composed, the
// JSON, and the dry run and --yes every scriptable write shares.
type prOptions struct {
	chosen pullChoices
	asJSON bool
	write  writeOptions
}

// pullChoices are what pr is told to open in place of what it composes, as
// the terminal's composer and the web's form let a person edit it: its title,
// its description or the template that starts it, the base it merges into,
// whether it is a draft, and whom it asks to review.
type pullChoices struct {
	title     string
	body      string
	template  string
	base      string
	draft     bool
	reviewers []string
}

// declare declares the choices' flags on cmd.
func (c *pullChoices) declare(cmd *cobra.Command) {
	cmd.Flags().StringVar(&c.title, "title", "", "the title, `TEXT`, in place of the one composed")
	cmd.Flags().StringVar(&c.body, "body", "",
		"the whole description, `TEXT`, in place of the one composed from the template and the commits")
	cmd.Flags().StringVar(&c.template, "template", "",
		"start the description from the repository's template `NAME` instead of its first")
	cmd.Flags().StringVar(&c.base, "base", "",
		"merge into `BRANCH` instead of the base the branch left; its code owners are asked to review")
	cmd.Flags().BoolVar(&c.draft, "draft", false, "open it as a draft")
	cmd.Flags().StringSliceVar(&c.reviewers, "reviewer", nil,
		"ask `NAME` to review instead of the code owners, org/team asking a team; "+
			"repeat it, or separate names with commas")
}

// check refuses, as a mistake in how pr was called, choices no pull request
// could be opened with: a description beside a template to start it from, or
// a base git would refuse as a branch's name.
func (c *pullChoices) check() error {
	if c.describes() && c.chosenTemplate() != "" {
		return fmt.Errorf("%w: %w", errUsage, errBodyBesideTemplate)
	}

	base := c.chosenBase()
	if base == "" {
		return nil
	}

	err := convention.ValidateBranchName(base)
	if err != nil {
		return fmt.Errorf("%w %q for \"--base\" flag: %w", errUsage, base, err)
	}

	return nil
}

// chosenBase is the base asked for, or empty for the one the branch left.
func (c *pullChoices) chosenBase() string {
	return strings.TrimSpace(c.base)
}

// chosenTemplate is the template asked for, or empty for the repository's
// first.
func (c *pullChoices) chosenTemplate() string {
	return strings.TrimSpace(c.template)
}

// describes reports whether a description was given in place of the one
// composed: a blank one is none.
func (c *pullChoices) describes() bool {
	return strings.TrimSpace(c.body) != ""
}

// namesReviewers reports whether reviewers were named, in place of the code
// owners.
func (c *pullChoices) namesReviewers() bool {
	return len(namesGiven(c.reviewers)) > 0
}

// over is request with the choices made in place of what was composed: the
// title and description given, a draft when asked, and the reviewers named,
// when any were, instead of the code owners. A blank title or description is
// none given.
func (c *pullChoices) over(request forge.NewPullRequest) forge.NewPullRequest {
	request.Title = cmp.Or(strings.TrimSpace(c.title), request.Title)
	request.Draft = c.draft

	if c.describes() {
		request.Body = c.body
	}

	if c.namesReviewers() {
		request.Reviewers, request.TeamReviewers = loop.SplitReviewers(namesGiven(c.reviewers))
	}

	return request
}

// reviewersNote says whom the reviewers previewed are: the code owners, unless
// they were named.
func (c *pullChoices) reviewersNote() string {
	if c.namesReviewers() {
		return ""
	}

	return " (code owners)"
}

// namesGiven are names trimmed, the blank ones a stray comma leaves dropped.
func namesGiven(names []string) []string {
	var given []string

	for _, name := range names {
		if trimmed := strings.TrimSpace(name); trimmed != "" {
			given = append(given, trimmed)
		}
	}

	return given
}

// composed is the pull request seams compose, with what pr was told to open in
// its place, and the branch it was composed for; a refusal is worded in the
// command line's terms.
func composed(ctx context.Context, seams prSeams) (forge.NewPullRequest, gitrepo.Branch, error) {
	request, branch, err := loop.ComposePull(ctx, seams.Compose, seams.Options)
	if errors.Is(err, loop.ErrNoSuchTemplate) {
		err = templateRefusal(seams.Options.Template, seams.Compose.Templates(), seams.Kind)
	}

	if err != nil {
		return forge.NewPullRequest{}, gitrepo.Branch{}, composeRefusal(err, seams.Kind)
	}

	return seams.Chosen.over(request), branch, nil
}

// templateRefusal words a template asked for by a name the repository has none
// of, in the forge's noun, naming the templates it has so the next run can
// name one of them.
func templateRefusal(name string, templates []forge.Template, kind forge.Kind) error {
	names := make([]string, 0, len(templates))
	for _, template := range templates {
		// A template's name is a file's in the repository, anyone's to write.
		names = append(names, sanitize.Line(template.Name))
	}

	return forgeWordedError{loop.ErrNoSuchTemplate, "the repository has no " + kind.Noun() + " template named " +
		strconv.Quote(name) + "; it has " + cmp.Or(strings.Join(names, ", "), "none")}
}

// draftNoun names the pull request about to be opened, in the forge's noun: a
// draft one when it is a draft, so a yes is never taken for more than was
// asked.
func draftNoun(request forge.NewPullRequest, kind forge.Kind) string {
	if request.Draft {
		return "draft " + kind.Noun()
	}

	return kind.Noun()
}
