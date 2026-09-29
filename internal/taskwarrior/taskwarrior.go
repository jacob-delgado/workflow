// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

// Package taskwarrior drives the task program: finds it, reads its tasks as
// the JSON its export writes, and changes them through its own commands.
package taskwarrior

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/jacob-delgado/workflow/internal/proc"
	"github.com/jacob-delgado/workflow/internal/sanitize"
)

// What workflow asks of Taskwarrior, and the names it links a task to an issue
// by.
const (
	// MinimumVersion is the oldest Taskwarrior workflow drives.
	MinimumVersion = "3.5.0"
	// LinkUDA holds the Jira issue key on a task; LinkURLUDA its page. bugwarrior
	// names them the same, so a task it made is recognized and one made here is
	// recognized by it.
	LinkUDA    = "jiraid"
	LinkURLUDA = "jiraurl"
	// LinkTag marks every task created from an issue.
	LinkTag = "jira"
)

var (
	// ErrNotInstalled reports no task program to try, or none there to run.
	ErrNotInstalled = errors.New("taskwarrior is not installed")
	// ErrNotTaskwarrior reports task programs that did not answer _version as
	// Taskwarrior does; go-task, the Taskfile runner, is also called task.
	ErrNotTaskwarrior = errors.New("the task program is not Taskwarrior")
	// ErrTooOld reports a Taskwarrior older than MinimumVersion.
	ErrTooOld = errors.New("taskwarrior is older than workflow needs")
	// ErrNotConfigured reports a Taskwarrior with no taskrc, which it will not
	// create unless asked at a terminal.
	ErrNotConfigured = errors.New("taskwarrior has never been run: it has no configuration file")
	// ErrNothingChanged reports a write Taskwarrior declined: the task is not
	// pending, is already started, or is not there, or there is nothing to undo.
	ErrNothingChanged = errors.New("taskwarrior changed nothing")
	// ErrRefused reports a command Taskwarrior refused, in its own words, or
	// in fixed words when the taskrc has a line it cannot parse.
	ErrRefused = errors.New("taskwarrior refused the command")
	// ErrBadOutput reports an answer that is not what the command writes.
	ErrBadOutput = errors.New("taskwarrior answered with something other than its JSON")
	// ErrNoSync reports a sync asked of a Taskwarrior with nowhere to sync.
	ErrNoSync = errors.New("no sync backend is configured in taskrc")
	// ErrAnnotateFailed reports a task added whose annotation was not.
	ErrAnnotateFailed = errors.New("the task was created but could not be annotated")
)

// NeverRunError is ErrNotConfigured naming the Taskwarrior that answered so:
// the program to run once at a terminal, by its path, since go-task can come
// first on PATH and answer to task.
type NeverRunError struct {
	Program string
}

var _ error = NeverRunError{}

// Error says Taskwarrior has never been run, and which.
func (e NeverRunError) Error() string {
	return ErrNotConfigured.Error() + ": " + e.Program
}

// Unwrap is ErrNotConfigured, so errors.Is finds it.
func (NeverRunError) Unwrap() error {
	return ErrNotConfigured
}

// Runner runs the task program with input on its standard input, bounded by
// timeout, and returns its standard output. A non-zero exit is an error that
// proc.Failure reads. In production it is proc.CaptureWithin.
type Runner func(ctx context.Context, timeout time.Duration, program proc.Command, input []byte) ([]byte, error)

// Install is the Taskwarrior that answered, read once from _version and _show.
type Install struct {
	Program        string // the path or name that answered
	Version        string // "3.5.0"
	DataDir        string // rc.data.location, for doctor
	SyncConfigured bool   // any sync.* backend named in taskrc
	LinkUDADefined bool   // taskrc already defines uda.jiraid.type
}

// Candidates is every task on pathList, in PATH order: on Windows task.exe
// beside task, elsewhere a regular file with an execute bit. It walks pathList
// as exec.LookPath does but keeps every hit rather than the first, since
// go-task, also called task, can come before Taskwarrior on PATH. It passes
// over an empty or relative entry, which names the working directory or one
// under it: LookPath refuses a program found through one with ErrDot, where
// Candidates goes on to the absolute entries.
func Candidates(pathList, goos string) []string {
	names := []string{"task"}
	if goos == "windows" {
		names = []string{"task.exe", "task"}
	}

	var found []string

	for dir := range strings.SplitSeq(pathList, string(os.PathListSeparator)) {
		if !filepath.IsAbs(dir) {
			continue
		}

		for _, name := range names {
			path := filepath.Join(dir, name)
			if runnable(path, goos) {
				found = append(found, path)

				break
			}
		}
	}

	return found
}

// runnable reports a regular file at path that goos would run: on Windows any,
// elsewhere one with an execute bit.
func runnable(path, goos string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}

	return info.Mode().IsRegular() && (goos == "windows" || info.Mode()&0o111 != 0)
}

// Detect finds Taskwarrior. With program set only it is tried; otherwise each
// candidate in order, each asked from the filesystem root. The first that
// answers _version with MinimumVersion or newer wins; a candidate that fails
// _version is not Taskwarrior, one that answers with an older version is too
// old, one that is not there is neither, and once every candidate has been
// tried Detect reports the kindest: too old, then not Taskwarrior, then
// ErrNotInstalled when none was there. Three failures are Taskwarrior's own
// and end the walk at once. A Taskwarrior never run fails _version, or failing
// that _show, with "Cannot proceed without rc file": that is a NeverRunError,
// which is ErrNotConfigured. One whose taskrc has a line it cannot parse fails
// _version, or failing that _show, quoting the line: that is ErrRefused in
// fixed words, since the line may hold a secret, and is told apart first. One
// that cannot start for another reason — an include it cannot read, a database
// it cannot open — exits 2 with words not in go-task's "task: …" form: that is
// ErrRefused in Taskwarrior's words. An exit 2 with nothing on standard error
// is not Taskwarrior refusing, and the walk goes on past it. A context done
// stops the walk with an error that is the context's.
func Detect(ctx context.Context, program string, candidates []string, run Runner) (Install, error) {
	tried := candidates
	if program != "" {
		tried = []string{program}
	}

	if len(tried) == 0 {
		return Install{}, ErrNotInstalled
	}

	var misses []error

	for _, candidate := range tried {
		version, err := askVersion(ctx, run, candidate)
		if err == nil {
			return readShow(ctx, run, candidate, version)
		}

		if ctx.Err() != nil {
			return Install{}, fmt.Errorf("%w: looking for taskwarrior at %s", ctx.Err(), candidate)
		}

		if errors.Is(err, ErrNotConfigured) || errors.Is(err, ErrRefused) {
			return Install{}, err
		}

		misses = append(misses, err)
	}

	return Install{}, kindestMiss(misses, tried)
}

// kindestMiss is why no candidate in tried was Taskwarrior, the kindest first:
// the first too old, then one that answered as some other program, and
// ErrNotInstalled when not one of them was there to run.
func kindestMiss(misses []error, tried []string) error {
	tooOld := slices.IndexFunc(misses, func(miss error) bool { return errors.Is(miss, ErrTooOld) })
	if tooOld >= 0 {
		return misses[tooOld]
	}

	if slices.ContainsFunc(misses, func(miss error) bool { return !errors.Is(miss, proc.ErrNotFound) }) {
		return fmt.Errorf("%w: tried %s", ErrNotTaskwarrior, strings.Join(tried, ", "))
	}

	return fmt.Errorf("%w: tried %s", ErrNotInstalled, strings.Join(tried, ", "))
}

// askVersion asks candidate for its version, refusing one older than
// MinimumVersion. It turns hooks off, since _version reads the taskrc and runs
// its on-launch hooks.
func askVersion(ctx context.Context, run Runner, candidate string) (string, error) {
	out, err := run(ctx, readTimeout, probe(candidate, "rc.hooks=off", "_version"), nil)
	if err != nil {
		return "", versionFailure(candidate, err)
	}

	parts, version, err := parseVersion(out)
	if err != nil {
		return "", err
	}

	minimum, _, _ := parseVersion([]byte(MinimumVersion))
	if slices.Compare(parts[:], minimum[:]) < 0 {
		return "", fmt.Errorf("%w: %s at %s; %s or newer is needed", ErrTooOld, version, candidate, MinimumVersion)
	}

	return version, nil
}

// probe is a detection run of program from the filesystem root, so a Taskfile
// in the working directory never runs; a relative path is made absolute
// first, since it names a program from the working directory, not the root.
func probe(program string, args ...string) proc.Command {
	name := program
	if !filepath.IsAbs(program) && filepath.Base(program) != program {
		absolute, err := filepath.Abs(program)
		if err == nil {
			name = absolute
		}
	}

	return proc.Command{Dir: filepath.VolumeName(name) + string(filepath.Separator), Name: name, Args: args}
}

// versionFailure words why candidate did not answer _version: only
// Taskwarrior quotes a taskrc line it cannot parse, which is refused in fixed
// words, says it cannot proceed without an rc file, which is all one never run
// answers, or cannot start in words of its own, and a program that is not there
// is not some other task.
func versionFailure(candidate string, err error) error {
	code, stderr, exited := proc.Failure(err)

	switch {
	case exited && malformedTaskrc(stderr):
		return fmt.Errorf("%w: taskrc has a malformed line", ErrRefused)
	case exited && lacksTaskrc(stderr):
		return NeverRunError{Program: candidate}
	case exited && cannotStart(code, stderr):
		return fmt.Errorf("%w: %s", ErrRefused, stderr)
	case errors.Is(err, proc.ErrNotFound):
		return err
	default:
		return fmt.Errorf("%w: %w", ErrNotTaskwarrior, err)
	}
}

// startFailed is the code Taskwarrior exits with when it cannot run a command
// at all.
const startFailed = 2

// cannotStart reports Taskwarrior failing to start in its own words, told apart
// from go-task, which exits otherwise or words its failures "task: …".
func cannotStart(code int, stderr string) bool {
	return code == startFailed && stderr != "" && !strings.HasPrefix(stderr, "task: ")
}

// lacksTaskrc reports stderr carrying the one line a Taskwarrior never run
// prints.
func lacksTaskrc(stderr string) bool {
	return strings.Contains(stderr, "Cannot proceed without rc file")
}

// versionPattern is the version _version prints first: "3.5.0", perhaps
// followed by the commit it was built from.
func versionPattern() *regexp.Regexp {
	return regexp.MustCompile(`^(\d+)\.(\d+)\.(\d+)`)
}

// parseVersion reads "3.5.0" or "3.5.0 (abc123)".
func parseVersion(out []byte) ([3]int, string, error) {
	firstLine, _, _ := strings.Cut(strings.TrimSpace(string(out)), "\n")

	match := versionPattern().FindStringSubmatch(firstLine)
	if match == nil {
		return [3]int{}, "", ErrNotTaskwarrior
	}

	var parts [3]int

	for index, digits := range match[1:] {
		number, err := strconv.Atoi(digits)
		if err != nil {
			return [3]int{}, "", fmt.Errorf("%w: %w", ErrNotTaskwarrior, err)
		}

		parts[index] = number
	}

	return parts, match[0], nil
}

// readShow reads the Taskwarrior at program's settings from _show, which runs
// without the link UDAs: it echoes an override back as though the taskrc set
// it, and LinkUDADefined is about the taskrc. A _show that fails once the
// context is done is the context's error, as a _version is in Detect's walk.
func readShow(ctx context.Context, run Runner, program, version string) (Install, error) {
	out, err := run(ctx, readTimeout, probe(program, append(baseReadOverrides(), "_show")...), nil)
	if err != nil {
		if ctx.Err() != nil {
			return Install{}, fmt.Errorf("%w: looking for taskwarrior at %s", ctx.Err(), program)
		}

		return Install{}, readFailure(program, err)
	}

	install := showFacts(out)
	install.Program, install.Version = program, version

	return install, nil
}

// showFacts keeps the three facts Install holds from _show's name=value lines
// and nothing else: _show prints every setting, secrets included.
func showFacts(out []byte) Install {
	var install Install

	for line := range strings.Lines(string(out)) {
		name, value, _ := strings.Cut(strings.TrimSpace(line), "=")
		if value == "" {
			continue
		}

		switch {
		case name == "data.location":
			install.DataDir = sanitize.Line(value)
		case name == "uda."+LinkUDA+".type":
			install.LinkUDADefined = true
		case slices.Contains(syncBackends(), name):
			install.SyncConfigured = true
		}
	}

	return install
}

// syncBackends are the settings that each name somewhere to sync with.
func syncBackends() []string {
	return []string{
		"sync.server.url", "sync.server.origin", "sync.local.server_dir",
		"sync.gcp.bucket", "sync.aws.bucket", "sync.git.local_path",
	}
}
