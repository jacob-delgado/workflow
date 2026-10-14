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
// these letters would clash with the root's -n wherever cobra merges them.
const (
	dryRunShorthand = "n"
	yesShorthand    = "y"
	jsonShorthand   = "j"
)

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

// declare declares the flags on root: --dry-run and --log once, for every
// command, so a script passes them before the command's name or after it, and
// each command reads them back by name; --web and --port on the root alone.
func (f *rootFlags) declare(root *cobra.Command) {
	root.PersistentFlags().BoolVarP(&f.dryRun, dryRunFlag, dryRunShorthand, false,
		"hold back every write to Jira, the forge, the messaging service, Taskwarrior, git and files, "+
			"and say what it would have done")
	root.PersistentFlags().String(logFlag, "",
		"append a one-line outline of each request (method, path, status, duration) to `FILE`, for a bug report")
	root.Flags().BoolVar(&f.web, "web", false,
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
