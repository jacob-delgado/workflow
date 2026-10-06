// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/spf13/cobra"

	"github.com/jacob-delgado/workflow/internal/forge"
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
		Short: "List the pull requests that are waiting on your review",
		Long: "List the open pull or merge requests on your forge that request your\n" +
			"review, oldest first, with the author, how CI stands and how long each has\n" +
			"been waiting. The forge is the one your repository's remote points at.\n" +
			"--sort newest lists the latest first, and --sort repo groups them by\n" +
			"repository, oldest first within each.",
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

	seams := reviewsSeams{List: conn.deps.Forge.ReviewRequests, Now: time.Now}

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

	now := seams.Now()
	if opts.asJSON {
		return renderReviewsJSON(out.artifact, reviews, now)
	}

	renderReviews(out, reviews, now)

	return nil
}

// renderReviews writes one line per review, or says, as commentary, that the
// queue is empty: a script counting the lines counts none.
func renderReviews(out output, reviews []forge.ReviewRequest, now time.Time) {
	if len(reviews) == 0 {
		fmt.Fprintln(out.notes, "No pull requests are waiting on your review.")

		return
	}

	for _, review := range reviews {
		fmt.Fprintln(out.artifact, reviewLine(review, now))
	}
}

// reviewLine is one review on a single line: what it is, where, who wants it,
// how CI stands, how long it has waited, and where to open it. Every value from
// the forge arrives already neutralized by the forge client.
func reviewLine(review forge.ReviewRequest, now time.Time) string {
	repository := ""
	if review.Repository != "" {
		repository = "(" + review.Repository + ")  "
	}

	return fmt.Sprintf("#%d  %s  %sby %s  CI %s  %s  %s",
		review.Number, review.Title, repository, review.Author,
		ciWord(review.CI), humanizeAge(now, review.OpenedAt), review.URL)
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

// reviewReport is one review as JSON.
type reviewReport struct {
	Number     int    `json:"number"`
	Title      string `json:"title"`
	Author     string `json:"author"`
	Repository string `json:"repository,omitempty"`
	Draft      bool   `json:"draft"`
	CI         string `json:"ci"`
	Age        string `json:"age"`
	URL        string `json:"url"`
}

// renderReviewsJSON writes the reviews as a JSON array.
func renderReviewsJSON(out io.Writer, reviews []forge.ReviewRequest, now time.Time) error {
	reports := make([]reviewReport, 0, len(reviews))
	for _, review := range reviews {
		reports = append(reports, reviewReport{
			Number: review.Number, Title: review.Title, Author: review.Author,
			Repository: review.Repository, Draft: review.Draft,
			CI: ciWord(review.CI), Age: humanizeAge(now, review.OpenedAt), URL: review.URL,
		})
	}

	return encodeJSON(out, reports)
}
