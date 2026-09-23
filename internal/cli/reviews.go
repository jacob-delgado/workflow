// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"fmt"
	"io"
	"slices"
	"strconv"
	"time"

	"github.com/spf13/cobra"

	"github.com/jacob-delgado/workflow/internal/forge"
)

// day is the window past which an age is counted in days rather than hours.
const day = 24 * time.Hour

// reviewsSeams are what `workflow reviews` reads. They are seams so a test can
// answer without a forge or a clock.
type reviewsSeams struct {
	List func() ([]forge.ReviewRequest, error)
	Now  func() time.Time
}

// newReviewsCmd builds `workflow reviews`.
func newReviewsCmd() *cobra.Command {
	var asJSON bool

	cmd := &cobra.Command{
		Use:   "reviews",
		Short: "List the pull requests that are waiting on your review",
		Long: "List the open pull or merge requests on your forge that request your\n" +
			"review, oldest first, with the author, how CI stands and how long each has\n" +
			"been waiting. The forge is the one your repository's remote points at.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runReviewsCommand(cmd, asJSON)
		},
	}

	cmd.Flags().BoolVar(&asJSON, "json", false, "print the reviews as JSON")

	return cmd
}

// runReviewsCommand wires the real forge to the reviews listing.
func runReviewsCommand(cmd *cobra.Command, asJSON bool) error {
	conn, err := connect(cmd)
	if err != nil {
		return err
	}
	defer conn.closeLog()

	seams := reviewsSeams{List: conn.deps.Forge.ReviewRequests, Now: time.Now}

	return runReviews(cmd.OutOrStdout(), seams, asJSON)
}

// runReviews lists the reviews waiting on you, the longest-waiting first.
func runReviews(out io.Writer, seams reviewsSeams, asJSON bool) error {
	reviews, err := seams.List()
	if err != nil {
		return fmt.Errorf("reading review requests: %w", err)
	}

	slices.SortStableFunc(reviews, func(left, right forge.ReviewRequest) int {
		return left.OpenedAt.Compare(right.OpenedAt)
	})

	now := seams.Now()
	if asJSON {
		return renderReviewsJSON(out, reviews, now)
	}

	renderReviews(out, reviews, now)

	return nil
}

// renderReviews writes one line per review, or says the queue is empty.
func renderReviews(out io.Writer, reviews []forge.ReviewRequest, now time.Time) {
	if len(reviews) == 0 {
		fmt.Fprintln(out, "No pull requests are waiting on your review.")

		return
	}

	for _, review := range reviews {
		fmt.Fprintln(out, reviewLine(review, now))
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
