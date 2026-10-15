// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

// Package cmdkit is what every workflow command is built from, beneath the
// command tree internal/cli spells: the scriptable contract — the artifact on
// a checked stdout, what is said about it on stderr, --yes and --dry-run
// around a write, a mistake in the call as a usage error — and the connection
// to where a command runs, the environment it reads the process through, the
// flags every command shares and the prompt that asks the person at the
// terminal. It imports no command, and not the web server: the root hands it
// the port --web serves on unless told otherwise.
package cmdkit

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/jacob-delgado/workflow/internal/cli/console"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/httpx"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/messaging"
	"github.com/jacob-delgado/workflow/internal/slackauth"
	"github.com/jacob-delgado/workflow/internal/wiring"
)

// Output is where a command writes: the artifact — the preview, the JSON, what
// it created — on stdout, and what it says about it — a notice, a dry-run line,
// a warning — through the notes on stderr, so a script capturing stdout gets
// data alone.
type Output struct {
	// Artifact is where the artifact goes: stdout, or the notes' own stream
	// for an aside.
	Artifact io.Writer
	// Notes are what the command says about it, on stderr.
	Notes console.Notes
}

// OutputOf is the command's stdout and the notes it says on stderr.
func OutputOf(cmd *cobra.Command) Output {
	return Output{Artifact: cmd.OutOrStdout(), Notes: NotesOf(cmd)}
}

// Aside is out with its artifact said as notes too, for a command whose
// stdout carries another artifact: what it previews, printed beside the JSON
// a script reads.
func (o Output) Aside() Output {
	return Output{Artifact: o.Notes.Writer(), Notes: o.Notes}
}

// NotesOf is what the command cmd runs says on its stderr, as the root's
// flags ask, colored as the variables of the environment it runs in allow.
func NotesOf(cmd *cobra.Command) console.Notes {
	return console.New(cmd.ErrOrStderr(), VoiceOf(cmd.Root(), EnvironmentOf(cmd).Process.Getenv))
}

// ErrOutputUnwritten reports an artifact that could not be written to stdout —
// a pipe whose reader has gone, a full disk. It fails the command however its
// work went, since a script reading stdout would otherwise take an artifact cut
// short, or none, for a run that succeeded.
var ErrOutputUnwritten = errors.New("the output could not be written")

// CheckedOutput is stdout, remembering the first write it could not make and
// refusing every write after it, so every command's artifact — prose written
// line by line as much as JSON — fails the command when it went nowhere.
type CheckedOutput struct {
	// Out is the stdout the writes go to.
	Out io.Writer
	// Err is why the first write that failed could not be made, an
	// ErrOutputUnwritten, or nil while none has.
	Err error
}

var _ io.Writer = (*CheckedOutput)(nil)

// Write writes p, or refuses it once a write has failed.
func (o *CheckedOutput) Write(p []byte) (int, error) {
	if o.Err != nil {
		return 0, o.Err
	}

	written, err := o.Out.Write(p)
	if err != nil {
		o.Err = fmt.Errorf("%w: %w", ErrOutputUnwritten, err)
	}

	return written, o.Err
}

// Failed is a command's outcome err, joined with why its output could not be
// written when it could not and err does not already say so.
func (o *CheckedOutput) Failed(err error) error {
	if o.Err == nil || errors.Is(err, ErrOutputUnwritten) {
		return err
	}

	return errors.Join(o.Err, err)
}

// Stream is the checked stdout as a command writes it: over a file, the
// terminal it may be, so fang can tell whether to color its help.
func (o *CheckedOutput) Stream() io.Writer {
	file, isFile := o.Out.(*os.File)
	if !isFile {
		return o
	}

	return checkedFile{CheckedOutput: o, file: file}
}

// unchecked is the stream the check is over.
func (o *CheckedOutput) unchecked() io.Writer {
	return o.Out
}

// checkedFile is the checked stdout over a file: its writes checked, and the
// rest of it the file's, so it reads as the terminal the file may be.
type checkedFile struct {
	*CheckedOutput

	file *os.File
}

var _ interface {
	io.ReadWriteCloser
	Fd() uintptr
} = checkedFile{}

// Read reads the file.
func (f checkedFile) Read(p []byte) (int, error) {
	return f.file.Read(p) //nolint:wrapcheck // the file's own error, passed through as the file gave it
}

// Close closes the file.
func (f checkedFile) Close() error {
	return f.file.Close() //nolint:wrapcheck // the file's own error, passed through as the file gave it
}

// Fd is the file's descriptor, what tells a terminal.
func (f checkedFile) Fd() uintptr {
	return f.file.Fd()
}

// TerminalOut is the stdout cmd writes to as the stream it was handed, without
// the check over it: the interface draws on it itself, and Bubble Tea knows a
// terminal only as the file it is.
func TerminalOut(cmd *cobra.Command) io.Writer {
	if checked, ok := cmd.OutOrStdout().(interface{ unchecked() io.Writer }); ok {
		return checked.unchecked()
	}

	return cmd.OutOrStdout()
}

// WriteOptions are the flags every scriptable write shares: a dry run that
// changes nothing, and a yes that skips the confirmation for unattended use.
type WriteOptions struct {
	// DryRun is a run that changes nothing.
	DryRun bool
	// Yes skips the confirmation.
	Yes bool
}

// ConfirmationHelp is --yes's help on a write that asks one question.
const ConfirmationHelp = "go ahead without the confirmation"

// AddFlags declares --yes, or -y, on a scriptable write command, with help
// that says what it answers, and has it read the root's --dry-run — one flag
// every command inherits — before it runs.
func (o *WriteOptions) AddFlags(cmd *cobra.Command, yesHelp string) {
	cmd.Flags().BoolVarP(&o.Yes, "yes", yesShorthand, false, yesHelp)
	cmd.PreRun = func(cmd *cobra.Command, _ []string) { o.DryRun = DryRunRequested(cmd) }
}

// dryRunFlag names the root's flag that holds back every write.
const dryRunFlag = "dry-run"

// DryRunRequested reports whether --dry-run was given, before or after the
// command's name. The root declares it for every command, so it is always
// there to read.
func DryRunRequested(cmd *cobra.Command) bool {
	dryRun, _ := cmd.Flags().GetBool(dryRunFlag)

	return dryRun
}

// logFlag names the root's flag that turns on the request log.
const logFlag = "request-log"

// RequestLogFor opens the request log --request-log names and, under --verbose, says
// each outline as a debug line too, or keeps none when neither asks for one,
// warning on the command's stderr at close when the file was not fully
// written. The root declares both flags for every command, so they are
// always there to read.
func RequestLogFor(cmd *cobra.Command) (*wiring.RequestLog, func(), error) {
	path, _ := cmd.Flags().GetString(logFlag)
	logging := requestLogging{notes: NotesOf(cmd), echo: echoesRequests(cmd)}

	if path == "" {
		return logging.open(path)
	}

	found, err := FromWorkingDir(cmd, path)
	if err != nil {
		return nil, func() {}, fmt.Errorf("opening the request log: %w", err)
	}

	return logging.open(found)
}

// echoesRequests reports whether --verbose asks cmd to say each request's
// outline as a debug line: never while bare workflow opens the terminal
// interface, which draws on the terminal stderr shares.
func echoesRequests(cmd *cobra.Command) bool {
	web, _ := cmd.Flags().GetBool(webFlag)
	drawsInterface := !cmd.HasParent() && !web

	return VoiceOf(cmd.Root(), nil).Verbosity == console.Verbose && !drawsInterface
}

// requestLogging is how a command's request log is kept: notes warns when its
// file was not fully written, and, when echo is set, says each outline as a
// debug line.
type requestLogging struct {
	notes console.Notes
	echo  bool
}

// logFileMode is the permission a request log is created with. Like the
// configuration, it is the user's own file and nobody else's to read.
const logFileMode os.FileMode = 0o600

// open opens the request-outline log named by path, echoed as logging asks.
// An empty path means no file: echoed, the log writes to the debug lines
// alone, and otherwise it is nil; either way the returned close is a no-op,
// so the caller wires and closes it the same way.
func (logging requestLogging) open(path string) (*wiring.RequestLog, func(), error) {
	if path == "" {
		return logging.unfiled(), func() {}, nil
	}

	//nolint:gosec // the path is the user's own --request-log argument, by design.
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, logFileMode)
	if err != nil {
		return nil, func() {}, fmt.Errorf("opening the request log: %w", err)
	}

	var out io.Writer = file
	if logging.echo {
		out = io.MultiWriter(file, logging.notes.DebugWriter())
	}

	requestLog := wiring.NewRequestLog(out, nil)

	return requestLog, func() { closeRequestLog(requestLog, file, logging.notes) }, nil
}

// unfiled is the request log kept with no file: the debug lines alone when
// echoed, and none otherwise.
func (logging requestLogging) unfiled() *wiring.RequestLog {
	if !logging.echo {
		return nil
	}

	return wiring.NewRequestLog(logging.notes.DebugWriter(), nil)
}

// closeRequestLog closes the log's file and, when an outline was lost or the
// file did not close cleanly, warns so in one line. The command's own result
// stands: its work was done, and only the record of it is incomplete.
func closeRequestLog(requestLog *wiring.RequestLog, file *os.File, notes console.Notes) {
	// Trade-off TRADE-18: only Linux's tests reach this, through /dev/full.
	err := errors.Join(requestLog.Err(), file.Close())
	if err != nil {
		reason := strings.ReplaceAll(err.Error(), "\n", "; ")
		notes.Warn("The request log could not be fully written: " + reason)
	}
}

// WritePrompt is what a scriptable write says around its confirmation: the
// question it asks, what a dry run would do, and what it prints when declined.
type WritePrompt struct {
	// Question is what the confirmation asks.
	Question string
	// DryRun is what a dry run says the write would do.
	DryRun string
	// Declined is what the write prints when the answer is no.
	Declined string
}

// Proceed reports whether to go ahead with a write, the preview already printed.
// A dry run says what it would do and stops; --yes goes ahead without asking;
// otherwise it asks, and a declined answer says so and stops. What it says is
// commentary, written to notes.
func (o *WriteOptions) Proceed(notes console.Notes, confirm func(string) (bool, error), say WritePrompt) (bool, error) {
	if o.DryRun {
		notes.Note(say.DryRun)

		return false, nil
	}

	if o.Yes {
		return true, nil
	}

	ok, err := confirm(say.Question)
	if errors.Is(err, ErrNoTerminal) {
		return false, fmt.Errorf("%w; pass --yes to go ahead without asking", err)
	}

	if err != nil {
		return false, err
	}

	if !ok {
		notes.Note(say.Declined)
	}

	return ok, nil
}

// EncodeJSON writes value as indented JSON: the one encoder every command's
// data goes out through, so each one's output is shaped and worded the same.
func EncodeJSON(out io.Writer, value any) error {
	encoder := json.NewEncoder(out)
	encoder.SetIndent("", "  ")

	err := encoder.Encode(value)
	if err != nil {
		return fmt.Errorf("encoding the output: %w", err)
	}

	return nil
}

// ErrUsage marks a mistake in how a command was called — a flag or an argument
// it does not take — as distinct from a command that ran and failed. Its words
// are cobra's own for a flag value it refuses, so a value this tree refuses reads
// the same.
var ErrUsage = errors.New("invalid argument")

// usageError is a mistake in how a command was called, in cobra's own words,
// followed by the hint cobra would print were its own errors not silenced: the
// commands a mistyped name may have meant, and where to read how it is called.
type usageError struct {
	err  error
	hint string
}

var _ error = usageError{}

// Error is cobra's message, then the hint.
func (e usageError) Error() string {
	return e.err.Error() + "\n\n" + e.hint
}

// Unwrap lets errors.Is match both cobra's error and ErrUsage.
func (e usageError) Unwrap() []error {
	return []error{e.err, ErrUsage}
}

// asUsage marks err, when there is one, as a mistake in how cmd was called with
// args: the ones it refused, or those read before a flag it could not read.
func asUsage(cmd *cobra.Command, err error, args []string) error {
	if err == nil {
		return nil
	}

	return usageError{err: err, hint: usageHint(cmd, args)}
}

// typoDistance is how many edits away a mistyped command name may be and still
// be suggested: cobra's own default, which it applies only on a path this tree,
// whose commands all declare their arguments, never takes.
const typoDistance = 2

// usageHint points at cmd's help, after the subcommands the first of args may
// have been meant as, even when a flag was what failed: that flag is the meant
// command's. A command with no subcommands has no name to suggest.
func usageHint(cmd *cobra.Command, args []string) string {
	var hint strings.Builder

	if len(args) > 0 {
		if meant := cmd.SuggestionsFor(args[0]); len(meant) > 0 {
			hint.WriteString("Did you mean this?\n")

			for _, name := range meant {
				hint.WriteString("\t" + name + "\n")
			}

			hint.WriteString("\n")
		}
	}

	hint.WriteString("Run '" + cmd.CommandPath() + " --help' for usage.")

	return hint.String()
}

// MarkMisuse makes every command in the tree report its flag and argument
// errors as usage errors: a flag it does not know, a wrong argument count, and
// an unknown command, which is an argument to a command that takes none.
func MarkMisuse(root *cobra.Command) {
	root.SetFlagErrorFunc(func(cmd *cobra.Command, err error) error { return asUsage(cmd, err, cmd.Flags().Args()) })
	markArgsMisuse(root)
}

// markArgsMisuse wraps the argument check of cmd and everything under it. A
// command that only groups others, and so has no run of its own, is given one
// that prints its help, as bare `config` does: cobra would otherwise print the
// help for an unknown subcommand and succeed, before its check could refuse it.
func markArgsMisuse(cmd *cobra.Command) {
	if cmd.HasSubCommands() && !cmd.Runnable() {
		cmd.RunE = func(cmd *cobra.Command, _ []string) error { return cmd.Help() }
	}

	cmd.SuggestionsMinimumDistance = typoDistance

	// A command that declares no check, such as cobra's help, is left to
	// cobra's own.
	if check := cmd.Args; check != nil {
		cmd.Args = func(cmd *cobra.Command, args []string) error { return asUsage(cmd, check(cmd, args), args) }
	}

	for _, child := range cmd.Commands() {
		markArgsMisuse(child)
	}
}

// IsAny reports whether err is, or wraps, any of targets.
func IsAny(err error, targets ...error) bool {
	for _, target := range targets {
		if errors.Is(err, target) {
			return true
		}
	}

	return false
}

// ServicesUnreachable are the clients' errors for a service that never
// answered the request itself: it could not be reached, answered only to say
// wait, answered with a redirect the client refused to follow, or took too
// long to finish its answer. A script may try again later. The exit family
// adds doctor.ErrUnreachable, which says the same of a check doctor made.
func ServicesUnreachable() []error {
	return []error{
		jira.ErrUnreachable, forge.ErrUnreachable, messaging.ErrUnreachable, slackauth.ErrUnreachable,
		httpx.ErrRateLimited, httpx.ErrRedirected, httpx.ErrTimedOut,
	}
}

// Examples is a command's Example text: one invocation a line, indented as
// cobra indents its other sections, so --help and the reference page show
// what scripting.md shows.
func Examples(lines ...string) string {
	return "  " + strings.Join(lines, "\n  ")
}
