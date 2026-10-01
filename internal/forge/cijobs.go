// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package forge

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"github.com/jacob-delgado/workflow/internal/sanitize"
)

// gitlabJob is one job of a pipeline: what it is called, the stage it ran in,
// why it failed, and its page.
type gitlabJob struct {
	ID            int64  `json:"id"`
	Name          string `json:"name"`
	Stage         string `json:"stage"`
	FailureReason string `json:"failure_reason"`
	WebURL        string `json:"web_url"`
}

// failedJobsPage is how many failed jobs one read asks for: more than a
// pipeline fails in practice, so the one page is all of them.
const failedJobsPage = "100"

// gitlabFailedJobs is a failed pipeline's failed jobs as its checks, each with
// its stage and why it failed, so the interface can say what broke rather than
// only that the pipeline did. A read that fails, or finds none, keeps
// fallback: the pipeline as the one check, still failed, still opening its
// page.
func gitlabFailedJobs(ctx context.Context, client Client, repo Repo, pipeline int64, fallback []Check) []Check {
	path := gitlabProjectPath(repo) + "/pipelines/" + strconv.FormatInt(pipeline, 10) +
		"/jobs?scope[]=failed&per_page=" + failedJobsPage

	jobs, err := repoCall[[]gitlabJob](ctx, client, repo, http.MethodGet, path, nil)
	if err != nil || len(jobs) == 0 {
		return fallback
	}

	checks := make([]Check, 0, len(jobs))
	for _, job := range jobs {
		checks = append(checks, Check{
			ID: strconv.FormatInt(job.ID, 10), Name: job.Name, Stage: job.Stage, State: CIFailed, URL: job.WebURL,
			Reason: sanitize.Line(strings.ReplaceAll(job.FailureReason, "_", " ")), LogAvailable: true,
		})
	}

	return checks
}
