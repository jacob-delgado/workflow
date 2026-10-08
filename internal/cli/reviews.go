// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/spf13/cobra"

	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/report"
)

// errUnknownSort is a --sort naming no order the queue can be listed in.
var errUnknownSort = errors.New("sort by oldest, newest or repo")

// day is the window past which an age is counted in days rather than hours.
const day = 24 * time.Hour

// reviewsSeams are what `workflow reviews` reads. They are seams so a test can
// answer without a forge or a clock.
type reviewsSeams struct {
	List func() ([]forge.ReviewRequest, error)
	Now  func() time.Time
	// Kind is the forge, whose own words — merge requests, each marked "!" on
	// GitLab — the queue is told in.
	Kind forge.Kind
}

// reviewsOptions are how `workflow reviews` prints the queue: as JSON or as
// lines, and in which order.
type reviewsOptions struct {
	asJSON bool
	sort   string
}

// newReviewsCmd builds `workflow reviews`.
func newReviewsCmd() *cobra.Command {
	var opts reviewsOptions

	cmd := &cobra.Command{
		Use:   "reviews",
		Short: "List the pull or merge requests that are waiting on your review",
		Long: "List the open pull or merge requests on your forge that request your\n" +
			"review, oldest first, with the author, how CI stands and how long each has\n" +
			"been waiting. The forge is the one your repository's remote points at.\n" +
			"--sort newest lists the latest first, and --sort repo groups them by\n" +
			"repository, oldest first within each. --json prints the web API's review\n" +
			"requests, each with when it was opened.",
		Example: examples(
			`workflow reviews | wc -l      # how many pull requests wait on you`,
			`workflow reviews --json       # the same, oldest first, as data`,
		),
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runReviewsCommand(cmd, opts)
		},
	}

	cmd.Flags().BoolVar(&opts.asJSON, "json", false, "print the reviews as JSON")
	cmd.Flags().StringVar(&opts.sort, "sort", "oldest", "the order to list them in: oldest, newest or repo")

	return cmd
}

// runReviewsCommand wires the real forge to the reviews listing.
func runReviewsCommand(cmd *cobra.Command, opts reviewsOptions) error {
	conn, err := connect(cmd)
	if err != nil {
		return err
	}
	defer conn.closeLog()

	seams := reviewsSeams{List: conn.deps.Forge.ReviewRequests, Now: time.Now, Kind: conn.deps.Forge.Kind}

	return runReviews(outputOf(cmd), seams, opts)
}

// runReviews lists the reviews waiting on you, in the order opts asks for.
func runReviews(out output, seams reviewsSeams, opts reviewsOptions) error {
	order, known := map[string]func([]forge.ReviewRequest) []forge.ReviewRequest{
		"oldest": forge.OldestFirst, "newest": forge.NewestFirst, "repo": forge.ByRepository,
	}[opts.sort]
	if !known {
		return fmt.Errorf(`%w %q for "--sort" flag: %w`, errUsage, opts.sort, errUnknownSort)
	}

	answered, err := seams.List()
	if err != nil {
		return fmt.Errorf("reading review requests: %w", err)
	}

	reviews := order(answered)
	if opts.asJSON {
		return encodeJSON(out.artifact, report.ReviewRequests(reviews))
	}

	renderReviews(out, reviews, seams.Now(), seams.Kind)

	return nil
}

// renderReviews writes one line per review, or says, as commentary, that the
// queue is empty: a script counting the lines counts none.
func renderReviews(out output, reviews []forge.ReviewRequest, now time.Time, kind forge.Kind) {
	if len(reviews) == 0 {
		fmt.Fprintln(out.notes, "No "+kind.Noun()+"s are waiting on your review.")

		return
	}

	for _, review := range reviews {
		fmt.Fprintln(out.artifact, reviewLine(review, now, kind))
	}
}

// reviewLine is one review on a single line: what it is, where, who wants it,
// how CI stands, how long it has waited, and where to open it. Every value from
// the forge arrives already neutralized by the forge client.
func reviewLine(review forge.ReviewRequest, now time.Time, kind forge.Kind) string {
	repository := ""
	if review.Repository != "" {
		repository = "(" + review.Repository + ")  "
	}

	return fmt.Sprintf("%s%d  %s  %sby %s  CI %s  %s  %s",
		kind.Sigil(), review.Number, review.Title, repository, review.Author,
		review.CI.Word(), humanizeAge(now, review.OpenedAt), review.URL)
}

// humanizeAge is how long ago then was, rounded down to minutes, hours or days.
func humanizeAge(now, then time.Time) string {
	elapsed := max(now.Sub(then), 0)

	switch {
	case elapsed < time.Hour:
		return strconv.Itoa(int(elapsed.Minutes())) + "m"
	case elapsed < day:
		return strconv.Itoa(int(elapsed.Hours())) + "h"
	default:
		return strconv.Itoa(int(elapsed/day)) + "d"
	}
}
