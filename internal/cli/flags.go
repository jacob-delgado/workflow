// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/jacob-delgado/workflow/internal/cli/console"
	"github.com/jacob-delgado/workflow/internal/config"
)

// errChannelNeedsUserToken refuses --channel where messaging posts through a
// webhook, which posts only to the channel it was made for.
var errChannelNeedsUserToken = errors.New("it takes effect only with a Slack user token; " +
	"a webhook posts to the channel it is bound to")

// jsonFlag names the flag every command that prints data takes to print it as
// JSON.
const jsonFlag = "json"

// The one-letter spellings of the flags the scriptable commands share, each
// the same on every command that takes its flag, so a script learns them
// once. No other flag is spelled short: a command's own flag taking one of
// these letters would clash with the root's -n or -q wherever cobra merges
// them. --verbose has none, since -v is --version.
const (
	dryRunShorthand = "n"
	quietShorthand  = "q"
	yesShorthand    = "y"
	jsonShorthand   = "j"
)

// The root's flags that say how what a command says on stderr is said, which
// every command inherits.
const (
	verboseFlag   = "verbose"
	quietFlag     = "quiet"
	logFormatFlag = "log-format"
)

// webFlag names the root's flag that serves the web interface instead of
// opening the terminal one.
const webFlag = "web"

// addJSONFlag declares --json, or -j, on cmd, held in asJSON, with help that
// says what it prints: the one way every command's data is asked for.
func addJSONFlag(cmd *cobra.Command, asJSON *bool, help string) {
	cmd.Flags().BoolVarP(asJSON, jsonFlag, jsonShorthand, false, help)
}

// postingIn is settings posting in channel, as --channel asks, in place of
// messaging.channel; a blank channel leaves them as they are. A webhook cannot
// post anywhere but where it is bound, so asking it to is refused as usage.
// With no messaging set up the channel is set all the same, for the command's
// own refusal to say what to set up.
func postingIn(settings config.Messaging, channel string) (config.Messaging, error) {
	chosen := strings.TrimSpace(channel)
	if chosen == "" {
		return settings, nil
	}

	if settings.Mode() == config.MessagingWebhook {
		return settings, fmt.Errorf("%w %q for \"--channel\" flag: %w", errUsage, chosen, errChannelNeedsUserToken)
	}

	posting := settings
	posting.Channel = chosen

	return posting, nil
}

// rootFlags are the root's flags: a dry run, which every command reads, and
// --web and --port, which choose what bare workflow opens.
type rootFlags struct {
	dryRun bool
	web    bool
	port   int
}

// declare declares the flags on root: --dry-run, --log and how stderr is
// said once, for every command, so a script passes them before the command's
// name or after it, and each command reads them back by name; --web and
// --port on the root alone.
func (f *rootFlags) declare(root *cobra.Command) {
	root.PersistentFlags().BoolVarP(&f.dryRun, dryRunFlag, dryRunShorthand, false,
		"hold back every write to Jira, the forge, the messaging service, Taskwarrior, git and files, "+
			"and say what it would have done")
	root.PersistentFlags().String(logFlag, "",
		"append a one-line outline of each request (method, path, status, duration) to `FILE`, for a bug report")
	root.PersistentFlags().Bool(verboseFlag, false,
		"also say each request's outline on stderr, as a debug line (not while the terminal interface draws)")
	root.PersistentFlags().BoolP(quietFlag, quietShorthand, false,
		"say only warnings, errors and questions on stderr, leaving every note out")
	root.PersistentFlags().Var(new(logFormatValue), logFormatFlag,
		"say what goes to stderr as `FORMAT`: text, or json or logfmt, an entry a line stamped with its time")
	root.Flags().BoolVar(&f.web, webFlag, false,
		"serve the web interface on http://127.0.0.1 instead of opening the terminal interface")
	root.Flags().IntVar(&f.port, portFlag, defaultWebPort, "the port --web serves on, from 1 to 65535")
}

// checkPort refuses, as a mistake in the call, --port without --web, where
// nothing listens on it, and a port no listener can take.
func checkPort(cmd *cobra.Command, web bool, port int) error {
	var why string

	switch {
	case !web && cmd.Flags().Changed(portFlag):
		why = "it takes effect only with --web"
	case port < 1 || port > math.MaxUint16:
		why = "it must be from 1 to 65535"
	default:
		return nil
	}

	return fmt.Errorf(`%w %q for "--port" flag: %s`, errUsage, strconv.Itoa(port), why)
}

// logFormatValue is --log-format's value: the format the notes are said in,
// refused as the flag is read when it names none.
type logFormatValue struct {
	format console.Format
}

var _ pflag.Value = (*logFormatValue)(nil)

// String is the format's name.
func (v *logFormatValue) String() string {
	return v.format.String()
}

// Set takes the format name names.
func (v *logFormatValue) Set(name string) error {
	format, err := console.ParseFormat(name)
	if err != nil {
		return err
	}

	v.format = format

	return nil
}

// Type is what --log-format's help calls its value when it names none.
func (v *logFormatValue) Type() string {
	return "format"
}

// errQuietAndVerbose is --quiet and --verbose given together, which ask for
// opposite things.
var errQuietAndVerbose = errors.New("--quiet and --verbose cannot be given together")

// checkVoice refuses, as a mistake in the call, --quiet and --verbose given
// to cmd together.
func checkVoice(cmd *cobra.Command) error {
	flags := cmd.Root().PersistentFlags()
	quiet, _ := flags.GetBool(quietFlag)
	verbose, _ := flags.GetBool(verboseFlag)

	if quiet && verbose {
		return asUsage(cmd, errQuietAndVerbose, nil)
	}

	return nil
}

// voiceOf is how the notes of a run of root are said, as its flags ask: in
// the --log-format given, leaving the notes out under --quiet and adding
// debug lines under --verbose, colored as getenv's variables allow.
func voiceOf(root *cobra.Command, getenv func(string) string) console.Options {
	flags := root.PersistentFlags()
	quiet, _ := flags.GetBool(quietFlag)
	verbose, _ := flags.GetBool(verboseFlag)
	format, _ := console.ParseFormat(flags.Lookup(logFormatFlag).Value.String())

	return console.Options{Format: format, Verbosity: verbosity(quiet, verbose), Getenv: getenv}
}

// verbosity is how much --quiet and --verbose ask the notes to say. Given
// together, which checkVoice refuses, they say as --quiet asks, which still
// says the refusal.
func verbosity(quiet, verbose bool) console.Verbosity {
	switch {
	case quiet:
		return console.Quiet
	case verbose:
		return console.Verbose
	default:
		return console.Normal
	}
}
