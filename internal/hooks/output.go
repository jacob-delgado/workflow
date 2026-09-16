// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

// Package hooks works with lefthook: reading what a run of it printed, reading
// its configuration, and writing a configuration for a repository whose hooks
// predate it.
package hooks

import (
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// JobState is how a job in a hook run stands.
type JobState int

const (
	// JobRunning has started and not yet been reported on.
	JobRunning JobState = iota
	// JobPassed finished successfully.
	JobPassed
	// JobFailed finished and failed the hook.
	JobFailed
	// JobSkipped did not run: no files matched it, or its skip rule applied.
	JobSkipped
)

// Job is one job in a hook run.
type Job struct {
	Name  string
	State JobState
	// Duration is lefthook's own words for how long it took, once reported.
	Duration string
}

// Location is a place in a file a tool pointed at.
type Location struct {
	File string
	Line int
	// Column is zero where the tool gave none.
	Column  int
	Message string
}

// Jobs reads which jobs a lefthook run has reached and how each stands, from
// its output so far. It works on partial output, a line at a time: a job is
// running from its header until the summary reports it.
func Jobs(lines []string) []Job {
	var jobs []Job

	summary := false

	for _, raw := range lines {
		line := strings.TrimSpace(raw)

		switch {
		case strings.HasPrefix(line, "summary:"):
			summary = true
		case summary:
			jobs = reported(jobs, line)
		default:
			jobs = started(jobs, line)
		}
	}

	return jobs
}

// started records a job's header, or a job lefthook skipped.
func started(jobs []Job, line string) []Job {
	const header, skip, skipMark = "┃", "│", " (skip)"

	switch {
	case strings.HasPrefix(line, header) && strings.HasSuffix(line, "❯"):
		name := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(line, header), "❯"))

		return upsert(jobs, Job{Name: name, State: JobRunning, Duration: ""})
	case strings.HasPrefix(line, skip) && strings.Contains(line, skipMark):
		name, _, _ := strings.Cut(strings.TrimSpace(strings.TrimPrefix(line, skip)), skipMark)

		return upsert(jobs, Job{Name: name, State: JobSkipped, Duration: ""})
	default:
		return jobs
	}
}

// summaryLine matches one job in lefthook's summary: a glyph, the name, and how
// long it took.
func summaryLine() *regexp.Regexp {
	return regexp.MustCompile(`^(\S+)\s+(.+?)\s+\((\d+(?:\.\d+)? seconds?)\)$`)
}

// reported records a job's outcome from the summary. A check mark in either of
// lefthook's styles is a pass; its other marks are failures.
func reported(jobs []Job, line string) []Job {
	match := summaryLine().FindStringSubmatch(line)
	if match == nil {
		return jobs
	}

	state := JobFailed
	if slices.Contains([]string{"✔️", "✔", "✓"}, match[1]) {
		state = JobPassed
	}

	return upsert(jobs, Job{Name: match[2], State: state, Duration: match[3]})
}

// upsert replaces a job of the same name, or appends it, keeping the order jobs
// first appeared in.
func upsert(jobs []Job, job Job) []Job {
	index := slices.IndexFunc(jobs, func(each Job) bool { return each.Name == job.Name })
	if index < 0 {
		return append(jobs, job)
	}

	jobs[index] = job

	return jobs
}

// locationPatterns match the ways tools point at a place in a file, each with
// the file, line, column and message in groups 1 to 4 where the tool gives them.
// A file must have an extension, which is what keeps a time of day or a URL with
// a port from reading as one.
func locationPatterns() []*regexp.Regexp {
	return []*regexp.Regexp{
		// file:line:column: message, and file:line: message — Go, most linters.
		regexp.MustCompile(`^([^\s:]+\.[A-Za-z0-9]+):(\d+)(?::(\d+))?:?\s*(.*)$`),
		// shellcheck: "In file line 12:".
		regexp.MustCompile(`^In (\S+\.[A-Za-z0-9]+) line (\d+)():()$`),
		// typos and others that point with an arrow.
		regexp.MustCompile(`^(?:╭▸|-->)\s*(\S+\.[A-Za-z0-9]+):(\d+)(?::(\d+))?()$`),
	}
}

// Failures finds every place in a hook's output that a tool pointed at, once
// each, in the order they appeared.
func Failures(lines []string) []Location {
	var found []Location

	patterns := locationPatterns()

	for _, raw := range lines {
		location, ok := locate(patterns, strings.TrimSpace(raw))
		if ok && !slices.ContainsFunc(found, location.samePlace) {
			found = append(found, location)
		}
	}

	return found
}

// locate reads a place from one line, if the line names one.
func locate(patterns []*regexp.Regexp, line string) (Location, bool) {
	for _, pattern := range patterns {
		match := pattern.FindStringSubmatch(line)
		if match == nil {
			continue
		}

		lineNumber, _ := strconv.Atoi(match[2])
		column, _ := strconv.Atoi(match[3])

		return Location{File: match[1], Line: lineNumber, Column: column, Message: match[4]}, true
	}

	return Location{}, false
}

// samePlace reports whether two locations point at the same place.
func (l Location) samePlace(other Location) bool {
	return l.File == other.File && l.Line == other.Line && l.Column == other.Column
}
