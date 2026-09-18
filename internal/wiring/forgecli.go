// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package wiring

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/proc"
)

// forgeTransport chooses how forge requests travel: through the forge's own CLI
// when forge.cli asks for it and the tool is installed, and otherwise over HTTP.
// The bool reports that the CLI was chosen, so the caller knows the request
// carries its own authentication and no token is required.
func forgeTransport(
	ctx context.Context, settings config.Forge, repo forge.Repo, base string,
	timeout time.Duration, available func(string) bool,
) (forge.Doer, bool) {
	if !settings.CLI {
		return forge.HTTPClient(timeout).Do, false
	}

	program, ok := forgeProgram(repo.Kind)
	if !ok || !available(program) {
		return forge.HTTPClient(timeout).Do, false
	}

	return forgeCLIDoer(ctx, proc.Capture, program, base, repo.Kind), true
}

// cliToken stands in for the forge client's token when the CLI carries the
// authentication itself. The client refuses to send a request with no token, and
// the CLI transport never reads this one, so a placeholder satisfies the guard
// without a real credential.
const cliToken = "cli"

// forgeCapture runs a program with a request body on standard input and returns
// its standard output, the seam the CLI transport is tested through.
type forgeCapture func(ctx context.Context, program proc.Command, input []byte) ([]byte, error)

// forgeProgram is the command-line tool that speaks a forge's API: gh for
// GitHub, glab for GitLab. An unknown forge has none.
func forgeProgram(kind forge.Kind) (string, bool) {
	switch kind {
	case forge.KindGitHub:
		return "gh", true
	case forge.KindGitLab:
		return "glab", true
	case forge.KindUnknown:
		return "", false
	default:
		return "", false
	}
}

// forgeCLIDoer routes a forge request through gh or glab's `api` command instead
// of net/http, so the login the shell already holds — which an SSO gateway may
// require — carries the request. The command is asked to include the response
// headers, which parse straight back into the HTTP response the forge client
// reads.
func forgeCLIDoer(
	ctx context.Context, capture forgeCapture, program, base string, kind forge.Kind,
) forge.Doer {
	return func(request *http.Request) (*http.Response, error) {
		body, err := requestBody(request)
		if err != nil {
			return nil, err
		}

		command := proc.Command{Name: program, Args: cliArgs(request, base, kind, len(body) > 0)}

		out, runErr := capture(ctx, command, body)

		response, parseErr := http.ReadResponse(bufio.NewReader(bytes.NewReader(out)), request)
		if parseErr != nil {
			return nil, cliFailure(program, runErr, parseErr)
		}

		return response, nil
	}
}

// requestBody reads a request's body, or nil when it has none.
func requestBody(request *http.Request) ([]byte, error) {
	if request.Body == nil {
		return nil, nil
	}

	body, err := io.ReadAll(request.Body)
	if err != nil {
		return nil, fmt.Errorf("reading the request body: %w", err)
	}

	return body, nil
}

// cliFailure explains a CLI transport that produced nothing to parse: the run's
// own error where there was one, otherwise the parse failure.
func cliFailure(program string, runErr, parseErr error) error {
	if runErr != nil {
		return fmt.Errorf("%s api: %w", program, runErr)
	}

	return fmt.Errorf("%s api: unreadable response: %w", program, parseErr)
}

// cliArgs builds the `api` invocation for a request: the method, the body on
// standard input when there is one, the response headers included so the status
// survives, and the endpoint the CLI understands.
func cliArgs(request *http.Request, base string, kind forge.Kind, hasBody bool) []string {
	args := []string{"api", "--include"}

	if request.Method != http.MethodGet {
		args = append(args, "-X", request.Method)
	}

	if hasBody {
		args = append(args, "--input", "-")
	}

	return append(args, cliEndpoint(request, base, kind))
}

// cliEndpoint is the endpoint each CLI takes: gh accepts the full URL, so a
// GitHub Enterprise host is reached with nothing more; glab takes the path under
// its own API root, so the base is trimmed off.
func cliEndpoint(request *http.Request, base string, kind forge.Kind) string {
	full := request.URL.String()
	if kind == forge.KindGitLab {
		return strings.TrimPrefix(full, base+"/")
	}

	return full
}
