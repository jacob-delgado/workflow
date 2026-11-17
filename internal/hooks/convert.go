// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package hooks

import (
	"iter"
	"regexp"
	"slices"
	"strings"
)

// convertibleRunner is the one interpreter whose scripts convert to jobs.
// lefthook runs a job with `sh -c`, so a script that already runs under sh keeps
// its meaning as jobs; a bash or zsh script might not, and stays a script.
const convertibleRunner = "sh"

// jobListSetLine matches a `set` line whose every option a piped job list keeps
// or does without: errexit, which the piped list does itself, and nounset and
// xtrace, which change nothing about plain commands. A piped job list honors
// no option, so any other `set` line — noglob, noexec, allexport, `-o
// pipefail` — would change what the commands do once they are jobs; it is not
// a plain command either, so a script that has one stays whole.
var jobListSetLine = regexp.MustCompile(`^set\s+(-[eux]+|-o\s+(errexit|nounset|xtrace))$`)

// errexitSetLine matches a jobListSetLine that turns errexit on: short flags
// including -e, or the long -o errexit.
var errexitSetLine = regexp.MustCompile(`^set\s+(-[ux]*e[eux]*|-o\s+errexit)$`)

// conversion is what reading a hook decided: that it runs as jobs, or that it
// stays a script, and when the reason is one its author would not guess, which.
type conversion int

const (
	// asJobs is a hook whose every line converts to a job.
	asJobs conversion = iota
	// asScript is a hook kept whole for what its lines plainly are: a branch, a
	// variable, another interpreter, no set -e at all.
	asScript
	// asScriptForLateErrexit is a hook of plain commands that turns set -e on
	// only after its first command, which ran on past a failure as the script
	// and would stop at it as jobs.
	asScriptForLateErrexit
)

// why is the comment a configuration carries above a hook kept as a script:
// for asScriptForLateErrexit, the reason; for any other conversion, none, since
// what keeps it whole is on its lines.
func (c conversion) why() string {
	if c == asScriptForLateErrexit {
		return "Kept as a script: it turns set -e on only after its first command, " +
			"and as jobs every command would stop the rest when it fails."
	}

	return ""
}

// plainCommands reads a script's commands, reporting whether it converts to a
// piped job list without changing what it does. It converts only a script that
// turns errexit on itself, in its shebang or before its first command, since the
// piped list stops at the first failure — a script that did not, or did only
// later, would run on past a failing command, and converting it would silently
// start stopping.
func plainCommands(script string) ([]string, conversion) {
	interpreter := strings.Fields(runner(script))
	if interpreter[0] != convertibleRunner {
		return nil, asScript
	}

	var commands []string

	errexit := shebangErrexit(interpreter[1:])

	for line := range saying(script) {
		switch {
		case errexitSetLine.MatchString(line):
			errexit = errexit.turnedOn(len(commands))
		case jobListSetLine.MatchString(line):
			// nounset or xtrace alone, which change nothing a job does.
		case !plainCommand(line):
			return nil, asScript
		default:
			commands = append(commands, line)
		}
	}

	if len(commands) == 0 {
		return nil, asScript
	}

	converted := errexit.convertsAs()
	if converted != asJobs {
		return nil, converted
	}

	return commands, asJobs
}

// errexit is how a script stands with errexit as it is read: off, on before
// any command ran, or turned on only after one had.
type errexit int

const (
	errexitOff errexit = iota
	errexitOn
	errexitLate
)

// shebangErrexit is how errexit stands once the shebang's options, as sh -e
// takes them, have been read: on when they turn it on, before any command.
func shebangErrexit(options []string) errexit {
	if errexitSetLine.MatchString(strings.Join(append([]string{"set"}, options...), " ")) {
		return errexitOn
	}

	return errexitOff
}

// turnedOn is errexit once a line turns it on after ran commands have run:
// late if it was off and a command has run, on if none has, and as it was if
// it was on already.
func (e errexit) turnedOn(ran int) errexit {
	if e != errexitOff {
		return e
	}

	if ran > 0 {
		return errexitLate
	}

	return errexitOn
}

// convertsAs is how a script of plain commands converts with errexit standing
// as e at its end: as jobs only when it was on before any command. A map, not a
// switch, so there is no last case gobco never sees false, and exhaustive keeps
// it complete.
func (e errexit) convertsAs() conversion {
	return map[errexit]conversion{
		errexitOff: asScript, errexitOn: asJobs, errexitLate: asScriptForLateErrexit,
	}[e]
}

// saying is each line of a script that says something, trimmed: neither blank
// nor a comment, which leaves the shebang out too.
func saying(script string) iter.Seq[string] {
	return func(yield func(string) bool) {
		for raw := range strings.SplitSeq(script, "\n") {
			line := strings.TrimSpace(raw)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}

			if !yield(line) {
				return
			}
		}
	}
}

// plainCommand reports a line that means the same run on its own as it did in
// its script: no variable or command substitution, no heredoc or process
// substitution, no pipe or background or continuation, no assignment, and not a
// keyword or builtin that only means something in sequence. Any `$` at all is
// out — a piped job list cannot carry the argument or variable it reads — and so
// is any `|` or `&`, since lefthook cannot reproduce a pipeline's pipefail or a
// job that runs in the background.
func plainCommand(line string) bool {
	for _, marker := range []string{"$", "`", "<<", "<(", ">(", "|", "&"} {
		if strings.Contains(line, marker) {
			return false
		}
	}

	first := strings.Fields(line)[0]

	return !strings.HasSuffix(line, "\\") && !strings.Contains(first, "=") && !strings.HasSuffix(first, "()") &&
		!slices.Contains(shellWords(), first)
}

// shellWords begin a line that is not a command by itself.
func shellWords() []string {
	return []string{
		"if", "then", "else", "elif", "fi", "for", "while", "until", "do", "done", "case", "esac",
		"function", "{", "}", "[", "[[", "(", ")", "exec", "exit", "return", "source", ".", "cd", "export",
		"trap", "shift", "read", "local", "declare", "readonly", "unset", "set",
		"pushd", "popd", "umask", "alias", "eval", "wait",
	}
}
