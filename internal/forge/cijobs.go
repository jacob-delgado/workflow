// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package forge

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/jacob-delgado/workflow/internal/httpx"
	"github.com/jacob-delgado/workflow/internal/sanitize"
)

// Why a job's log was not read.
var (
	// ErrNoLog is a check the forge keeps no log for: a status, or a check run
	// from an app other than GitHub Actions.
	ErrNoLog = errors.New("the forge keeps no log for this check")
	// ErrInsecureLog is a log the forge redirected to an address that is not
	// https, which is not followed.
	ErrInsecureLog = errors.New("the forge sent the log to an address that is not https, so it was not read")
)

// How much of the end of a job's log is kept: a failure is at the end of a
// log, and a log can run to gigabytes.
const (
	logKeepBytes   = 64 << 10
	logBufferBytes = 2 * logKeepBytes
	logKeepLines   = 400
)

// JobLog is the end of a failed job's log, with every terminal control taken
// out, and whether more was there than is kept.
type JobLog struct {
	Text      string
	Truncated bool
}

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

// JobLog reads the end of the log of check, a GitHub Actions run or a GitLab
// job. GitHub answers with a redirect to signed storage elsewhere: it is
// followed once, over https only, in a request that carries no token, since
// the storage needs none and is no host the token belongs to.
func (c Client) JobLog(ctx context.Context, repo Repo, check Check) (JobLog, error) {
	if !check.LogAvailable || check.ID == "" {
		return JobLog{}, ErrNoLog
	}

	if c.token == "" {
		return JobLog{}, ErrNoToken
	}

	path, err := jobLogPath(repo, check.ID)
	if err != nil {
		return JobLog{}, err
	}

	response, err := c.logResponse(ctx, path)
	if err != nil {
		return JobLog{}, err
	}
	defer func() { _ = response.Body.Close() }()

	err = c.answerError(response)
	if err != nil {
		return JobLog{}, err
	}

	return tailOf(response.Body)
}

// jobLogPath is where the forge serves a job's log.
func jobLogPath(repo Repo, id string) (string, error) {
	switch repo.Kind {
	case KindGitHub:
		return githubRepoPath(repo) + "/actions/jobs/" + url.PathEscape(id) + "/logs", nil
	case KindGitLab:
		return gitlabProjectPath(repo) + "/jobs/" + url.PathEscape(id) + "/trace", nil
	case KindUnknown:
		return "", ErrUnknownForge
	}

	return "", ErrUnknownForge
}

// logResponse asks for the log at path, following the forge's one redirect to
// where it keeps it.
func (c Client) logResponse(ctx context.Context, path string) (*http.Response, error) {
	request, err := c.newRequest(httpx.ShowingRedirect(ctx), http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	response, err := c.do(request)
	if err != nil {
		return nil, httpx.Unreachable(ErrUnreachable, c.base, err)
	}

	if response.StatusCode < http.StatusMultipleChoices || response.StatusCode >= http.StatusBadRequest {
		return response, nil
	}

	location := response.Header.Get("Location")
	_ = response.Body.Close()

	return c.followLog(ctx, location)
}

// followLog asks the storage a log was redirected to for it, refusing an
// address that is not https, and sending no token.
func (c Client) followLog(ctx context.Context, location string) (*http.Response, error) {
	target, err := url.Parse(location)
	if err != nil || target.Scheme != "https" {
		return nil, ErrInsecureLog
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, target.String(), nil)
	if err != nil {
		return nil, ErrInsecureLog
	}

	request.Header.Set("User-Agent", userAgent)

	response, err := c.do(request)
	if err != nil {
		// The storage's address is a signed URL, so it stays out of the error.
		return nil, httpx.Unreachable(ErrUnreachable, "", err)
	}

	return response, nil
}

// tailOf reads log to its end and keeps its last logKeepLines lines, no more
// than logKeepBytes, with every terminal control taken out. The whole log is
// read rather than a capped prefix of it, since a failure is at the real end
// and a cap would show a middle; only the tail is ever held.
func tailOf(log io.Reader) (JobLog, error) {
	kept, truncated, err := lastBytes(log)
	if err != nil {
		return JobLog{}, err
	}

	lines := strings.Split(strings.TrimRight(sanitize.Text(string(kept)), "\n"), "\n")
	if len(lines) > logKeepLines {
		lines, truncated = lines[len(lines)-logKeepLines:], true
	}

	return JobLog{Text: strings.Join(lines, "\n"), Truncated: truncated}, nil
}

// lastBytes reads log to its end, keeping its last logKeepBytes, and whether
// any were dropped. It slides the kept tail back to the front of a buffer
// larger than it whenever the buffer fills, so memory stays fixed however
// long the log.
func lastBytes(log io.Reader) ([]byte, bool, error) {
	buffer := make([]byte, logBufferBytes)
	filled, truncated := 0, false

	for {
		if filled == len(buffer) {
			filled = copy(buffer, buffer[filled-logKeepBytes:])
			truncated = true
		}

		read, err := log.Read(buffer[filled:])
		filled += read

		if errors.Is(err, io.EOF) {
			return buffer[max(0, filled-logKeepBytes):filled], truncated || filled > logKeepBytes, nil
		}

		if err != nil {
			return nil, false, fmt.Errorf("reading the log: %w", err)
		}
	}
}
