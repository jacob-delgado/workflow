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

// NextJob folds one more output line into the jobs seen so far, returning the
// updated jobs and whether the summary has begun: a job is running from its
// header until the summary reports it. It does not modify the jobs it is given,
// so a caller streaming output can keep the running set on its own value and
// never re-read the whole output — a chatty hook would otherwise cost a full
// re-parse on every line, and again on every redraw.
func NextJob(jobs []Job, inSummary bool, raw string) ([]Job, bool) {
	line := strings.TrimSpace(raw)
	next := slices.Clone(jobs)

	switch {
	case strings.HasPrefix(line, "summary:"):
		return next, true
	case inSummary:
		return reported(next, line), true
	default:
		return started(next, line), inSummary
	}
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

// knownBasenames are files a tool points at that carry no extension —
// hadolint's Dockerfile, make's Makefile, and the task-runner and language
// files a linter can flag by name. They join the file:line pattern as a literal
// allowlist rather than "any extensionless word", so a time of day ("12:30") or
// a host and port ("localhost:8080") still cannot read as a place.
const knownBasenames = `Dockerfile|Containerfile|Makefile|Justfile|Rakefile|` +
	`Gemfile|Vagrantfile|Jenkinsfile|Procfile|Brewfile`

// filePatterns returns the drive prefix and the file sub-pattern shared by the
// location and ESLint patterns: a leading drive letter is allowed only on
// Windows, and a file part must carry an extension or be a known extensionless
// name (anchored as the last path segment). It returns the drive, then the file.
func filePatterns(goos string) (string, string) {
	drive := ""
	if goos == "windows" {
		drive = `(?:[A-Za-z]:)?`
	}

	file := `(?:[^\s:]+\.[A-Za-z0-9]+|(?:[^\s:]*[/\\])?(?:` + knownBasenames + `))`

	return drive, file
}

// locationPatterns match the ways tools point at a place in a file, each with
// the file, line, column and message in groups 1 to 4 where the tool gives them.
// A file part must either carry an extension or be a known extensionless name,
// which is what keeps a time of day or a URL with a port from reading as one.
//
// The Go/linter and parenthesized formats split the file from the line at a
// colon or a parenthesis, so their file part stops at the first colon. On
// Windows a path opens with a drive letter — "C:\src\main.go" — whose colon
// would be read as that separator, so a single drive prefix is allowed there,
// and only there: on Unix an "a:b.go" would look the same and is far likelier
// to be a false positive than a real drive. The other formats span the drive
// already, their file part being \S+ or a quoted path.
func locationPatterns(goos string) []*regexp.Regexp {
	drive, file := filePatterns(goos)

	return []*regexp.Regexp{
		// file:line:column: message, and file:line: message — Go, most linters.
		regexp.MustCompile(`^(` + drive + file + `):(\d+)(?::(\d+))?:?\s*(.*)$`),
		// file(line,col): message, and file(line): message — MSVC, and the
		// TypeScript compiler the web build runs.
		regexp.MustCompile(`^(` + drive + file + `)\((\d+)(?:,(\d+))?\):?\s*(.*)$`),
		// shellcheck: "In file line 12:".
		regexp.MustCompile(`^In (\S+\.[A-Za-z0-9]+) line (\d+)():()$`),
		// typos and others that point with an arrow.
		regexp.MustCompile(`^(?:╭▸|-->)\s*(\S+\.[A-Za-z0-9]+):(\d+)(?::(\d+))?()$`),
		// a Python traceback frame: `File "path/to/x.py", line 10, in <module>`.
		// The path must end in .py so a quoted name of the same shape from another
		// tool is not read as one; the trailing ", in …" is dropped, not kept.
		regexp.MustCompile(`^File "([^"]+\.py)", line (\d+)()(?:,.*)?()$`),
	}
}

// eslintPatterns match ESLint's "stylish" reporter, which names the file once on
// its own line and then lists each place under it as "line:col severity message"
// with no filename — so the file has to be carried across lines, which the
// per-line patterns cannot do. The row keeps the error/warning word so a bare
// "12:5 …" elsewhere is not mistaken for one. The header reuses the file rule so
// only a plausible path opens a block. It returns the header pattern, then the
// row. The header's extension must start with a letter, so a version banner
// ("v1.2.3") or a decimal ("3.14") on its own line is not read as a file.
func eslintPatterns(goos string) (*regexp.Regexp, *regexp.Regexp) {
	drive, _ := filePatterns(goos)

	header := `(?:[^\s:]+\.[A-Za-z][A-Za-z0-9]*|(?:[^\s:]*[/\\])?(?:` + knownBasenames + `))`

	return regexp.MustCompile(`^(` + drive + header + `)$`),
		regexp.MustCompile(`^(\d+):(\d+)\s+(?:error|warning)\s+(.*)$`)
}

// MaxPlaces is the most places a FailureScan keeps. A run folds every line of
// its output into the scan as the line arrives, so an output naming ever more
// places would otherwise grow the scan, and the work of keeping each place
// once, without limit. Past it the scan keeps the first places, which are
// usually the causes of the rest.
const MaxPlaces = 1000

// FailureScan reads where a hook's tools pointed one output line at a time, as
// NextJob reads its jobs, so a run can keep the places as its output streams
// in: by the time a long run ends, its first lines are gone. Start one with
// NewFailureScan.
type FailureScan struct {
	patterns     []*regexp.Regexp
	eslintHeader *regexp.Regexp
	eslintRow    *regexp.Regexp
	found        []Location
	// eslintFile is the file ESLint named above the rows now arriving.
	eslintFile string
	// held is a line naming a file on its own, which opens an ESLint block only
	// if the next line is one of its rows, so it waits for that line.
	held string
}

// NewFailureScan starts reading a hook's output. goos is the running platform,
// which decides whether a Windows drive letter is read as part of a path.
func NewFailureScan(goos string) FailureScan {
	header, row := eslintPatterns(goos)

	return FailureScan{
		patterns: locationPatterns(goos), eslintHeader: header, eslintRow: row,
		found: nil, eslintFile: "", held: "",
	}
}

// Next folds one more output line in. It does not modify the scan it is given.
//
// The ESLint file is carried only while its rows follow it: a header opens a
// block when the next line is one of its rows, and any line that is neither a
// row nor a new header ends the block — so a blank line, or a later tool's
// output, is never read against a stale filename.
func (s FailureScan) Next(raw string) FailureScan {
	line := strings.TrimSpace(raw)
	row := s.eslintRow.FindStringSubmatch(line)

	if held := s.held; held != "" {
		s.held = ""

		if row != nil {
			s.eslintFile = held
		} else {
			s = s.located(held)
		}
	}

	switch {
	case row != nil && s.eslintFile != "":
		lineNumber, _ := strconv.Atoi(row[1])
		column, _ := strconv.Atoi(row[2])

		return s.with(Location{
			File: s.eslintFile, Line: lineNumber, Column: column,
			Message: strings.Join(strings.Fields(row[3]), " "),
		})
	case s.eslintHeader.MatchString(line):
		s.held = line

		return s
	default:
		return s.located(line)
	}
}

// Places is every place found so far, once each, in the order they appeared. A
// held line that nothing followed is read as any other line is.
func (s FailureScan) Places() []Location {
	if s.held != "" {
		return s.located(s.held).found
	}

	return s.found
}

// located ends any ESLint block, and keeps the place a line names if it names
// one.
func (s FailureScan) located(line string) FailureScan {
	s.eslintFile = ""

	location, ok := locate(s.patterns, line)
	if !ok {
		return s
	}

	return s.with(location)
}

// with keeps a place once, while fewer than MaxPlaces are kept. It appends to a
// clipped copy of the places, so the scan it came from keeps its own.
func (s FailureScan) with(location Location) FailureScan {
	if len(s.found) >= MaxPlaces {
		return s
	}

	if !slices.ContainsFunc(s.found, location.samePlace) {
		s.found = append(slices.Clip(s.found), location)
	}

	return s
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
