// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver

import (
	"errors"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/proc"
	"github.com/jacob-delgado/workflow/internal/taskwarrior"
)

// taskFault is fault for what a Taskwarrior seam answered. A refusal carries
// Taskwarrior's own words, which name what in the line it could not take, and a
// bound that ran out is Taskwarrior's: no class can say so, since git's reads
// time out with the same sentinel.
func (s *server) taskFault(err error) api.Problem {
	switch {
	case errors.Is(err, taskwarrior.ErrRefused):
		return problem(api.ProblemCodeUnprocessable, s.refusalDetail(err))
	case errors.Is(err, proc.ErrTimedOut):
		return problem(api.ProblemCodeUnreachable, "Taskwarrior did not answer in time")
	default:
		return s.fault(err)
	}
}

// malformedEntry opens Taskwarrior's words for a taskrc line it cannot read,
// which quote the line, secret and all. internal/taskwarrior words that refusal
// anew; a line still carrying it is dropped all the same.
const malformedEntry = "Malformed entry '"

// refusalDetail words a command Taskwarrior refused: its own words but what
// they can carry that no answer may — its data directory and your home, which
// read as fixed words, and each line naming a URL or a host and port, since a
// hook's feedback can name the sync server, or quoting a taskrc line, dropped
// whole — or, with no words left, the refusal alone.
func (s *server) refusalDetail(err error) string {
	places := s.knownPlaces()
	address, clock := networkAddress(), clockTime()

	var kept []string

	for line := range strings.Lines(refusalWords(err)) {
		if !address.MatchString(clock.ReplaceAllString(line, "$1")) && !strings.Contains(line, malformedEntry) {
			kept = append(kept, places.Replace(strings.TrimRight(line, "\r\n")))
		}
	}

	words := strings.TrimSpace(strings.Join(kept, "\n"))
	if words == "" {
		return "Taskwarrior refused the command"
	}

	return "Taskwarrior refused the command: " + words
}

// knownPlaces puts fixed words in place of the directories Taskwarrior's words
// can name: its data directory, then your home, which most often holds it, so
// a path under both reads as the data directory. One that names no place of
// its own is left out — see namesAPlace.
func (s *server) knownPlaces() *strings.Replacer {
	var pairs []string

	install, err := s.installed()
	if err == nil && namesAPlace(install.DataDir) {
		pairs = append(pairs, install.DataDir, "the data directory")
	}

	if home := s.homeDir(); namesAPlace(home) {
		pairs = append(pairs, home, "~")
	}

	return strings.NewReplacer(pairs...)
}

// namesAPlace reports whether dir is an absolute path below the root, the only
// kind worth replacing: an empty one would match between every letter, a
// relative one ("task") ordinary words, and the root every path.
func namesAPlace(dir string) bool {
	return filepath.IsAbs(dir) && filepath.Dir(dir) != dir
}

// homeDir is your home directory, or "" when none is known.
func (s *server) homeDir() string {
	return s.deps.Repositories.Home
}

// networkAddress matches what names a machine on the network: a URL, or a host
// and port — a name, an IPv4 address or a bracketed IPv6 one before a colon
// and a port. A clock time is neither: its hour holds no letter and no dot.
func networkAddress() *regexp.Regexp {
	return regexp.MustCompile(`[A-Za-z][A-Za-z0-9+.-]*://|` +
		`(?:[A-Za-z0-9.-]*[A-Za-z.][A-Za-z0-9.-]*|\[[0-9A-Fa-f:.]+\]):[0-9]{1,5}\b`)
}

// clockTime matches a date and a time of day, as 2026-02-30T08:00 or
// tomorrowT10:00 write them, and the character before it, which is not part of
// a host name: the T before its hour would read as a host's letter, so it is
// taken out before networkAddress looks.
func clockTime() *regexp.Regexp {
	return regexp.MustCompile(`(^|[^A-Za-z0-9.-])(?:[0-9]{4}-[0-9]{2}-[0-9]{2}|[a-z]+)` +
		`T[0-9]{1,2}:[0-9]{2}(?::[0-9]{2})?\b`)
}

// refusalWords is what Taskwarrior said in refusing a command: the error's text
// after the refusal's own.
func refusalWords(err error) string {
	_, words, _ := strings.Cut(err.Error(), taskwarrior.ErrRefused.Error()+": ")

	return words
}
