// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package forge_test

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/forge"
)

// secret is the token these fakes hand back.
const secret = "forge-token-for-tests"

// Tokens the environment cases offer from sources that should lose.
const (
	fromCLI           = "a-different-token\n"
	fromConfiguration = "a-third-token"
)

// Compile-time proof that the real seams satisfy what Resolver takes, so the
// production wiring cannot drift from what the tests exercise.
var (
	_ forge.Look = exec.LookPath
	_ forge.Run  = func(_ context.Context, _ string, _ ...string) ([]byte, error) { return nil, nil }
)

// noEnv answers every environment lookup with nothing.
func noEnv(string) string { return "" }

// envWith answers only the named variable.
func envWith(name, value string) func(string) string {
	return func(asked string) string {
		if asked == name {
			return value
		}

		return ""
	}
}

// noProgram reports every program as absent.
func noProgram(name string) (string, error) {
	return "", fmt.Errorf("%s: %w", name, exec.ErrNotFound)
}

// programAt reports one program as present.
func programAt(want string) forge.Look {
	return func(name string) (string, error) {
		if name == want {
			return "/usr/bin/" + name, nil
		}

		return "", fmt.Errorf("%s: %w", name, exec.ErrNotFound)
	}
}

// errCLIFailed stands in for whatever a CLI exits with when it holds no
// credential.
var errCLIFailed = errors.New("exit status 1")

// failing is a Run whose program exits non-zero.
func failing(context.Context, string, ...string) ([]byte, error) {
	return nil, errCLIFailed
}

// printing is a Run that answers with output, as a CLI writing to stdout would.
func printing(output string) forge.Run {
	return func(context.Context, string, ...string) ([]byte, error) {
		return []byte(output), nil
	}
}

func TestResolveTokenTakesTheFirstSourceThatHasOne(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		resolver   forge.Resolver
		kind       forge.Kind
		wantToken  string
		wantSource forge.Source
		wantErr    error
	}{
		// The environment wins over a CLI and the configuration, under either
		// name each forge's tools read.
		"github's own variable": {
			resolver: forge.Resolver{
				Getenv: envWith("GITHUB_TOKEN", secret), Look: programAt("gh"),
				Run: printing(fromCLI), Configured: fromConfiguration,
			},
			kind: forge.KindGitHub, wantToken: secret, wantSource: forge.SourceEnvironment,
		},
		"gh's variable": {
			resolver: forge.Resolver{
				Getenv: envWith("GH_TOKEN", secret), Look: programAt("gh"),
				Run: printing(fromCLI), Configured: fromConfiguration,
			},
			kind: forge.KindGitHub, wantToken: secret, wantSource: forge.SourceEnvironment,
		},
		"gitlab's own variable": {
			resolver: forge.Resolver{
				Getenv: envWith("GITLAB_TOKEN", secret), Look: programAt("gh"),
				Run: printing(fromCLI), Configured: fromConfiguration,
			},
			kind: forge.KindGitLab, wantToken: secret, wantSource: forge.SourceEnvironment,
		},
		"glab's variable": {
			resolver: forge.Resolver{
				Getenv: envWith("GLAB_TOKEN", secret), Look: programAt("gh"),
				Run: printing(fromCLI), Configured: fromConfiguration,
			},
			kind: forge.KindGitLab, wantToken: secret, wantSource: forge.SourceEnvironment,
		},
		// gh prints the token and a newline, and nothing else.
		"the forge CLI, trimmed": {
			resolver: forge.Resolver{Getenv: noEnv, Look: programAt("gh"), Run: printing(secret + "\n"), Configured: ""},
			kind:     forge.KindGitHub, wantToken: secret, wantSource: forge.SourceCLI,
		},
		"the configuration with no CLI": {
			resolver: forge.Resolver{Getenv: noEnv, Look: noProgram, Run: printing("never reached"), Configured: secret},
			kind:     forge.KindGitHub, wantToken: secret, wantSource: forge.SourceConfiguration,
		},
		// gh exits non-zero when it holds no credential for the host. That is not
		// an error worth reporting — it just means the next source gets a turn.
		"the configuration after a failing CLI": {
			resolver: forge.Resolver{Getenv: noEnv, Look: programAt("gh"), Run: failing, Configured: secret},
			kind:     forge.KindGitHub, wantToken: secret, wantSource: forge.SourceConfiguration,
		},
		// gh can exit zero and print only a newline. That is not a token.
		"the configuration after a CLI that prints nothing": {
			resolver: forge.Resolver{Getenv: noEnv, Look: programAt("gh"), Run: printing("  \n"), Configured: secret},
			kind:     forge.KindGitHub, wantToken: secret, wantSource: forge.SourceConfiguration,
		},
		// glab reports its token through `auth status`, whose output is prose on
		// standard error. Parsing that is too fragile to put a credential behind,
		// so GitLab users set the environment variable or the configuration.
		"the configuration for gitlab, which has no CLI step": {
			resolver: forge.Resolver{
				Getenv: noEnv, Look: programAt("glab"), Run: printing("should not be run"), Configured: secret,
			},
			kind: forge.KindGitLab, wantToken: secret, wantSource: forge.SourceConfiguration,
		},
		// An on-premises host names neither forge, so there is no variable to read
		// and no CLI to ask — only what the configuration says.
		"the configuration for an unknown forge": {
			resolver: forge.Resolver{
				Getenv: envWith("GITHUB_TOKEN", "should not be read"), Look: programAt("gh"),
				Run: printing("should not be run"), Configured: secret,
			},
			kind: forge.KindUnknown, wantToken: secret, wantSource: forge.SourceConfiguration,
		},
		"nothing anywhere": {
			resolver: forge.Resolver{Getenv: noEnv, Look: noProgram, Run: printing(""), Configured: ""},
			kind:     forge.KindGitHub, wantToken: "", wantSource: forge.SourceNone, wantErr: forge.ErrNoToken,
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			token, source, err := tt.resolver.Resolve(t.Context(), tt.kind, "example.com")

			// Assert
			if !errors.Is(err, tt.wantErr) || token.Secret() != tt.wantToken || source != tt.wantSource {
				t.Errorf("Resolve = %v from %v, %v; want the token from %v, %v", token, source, err, tt.wantSource, tt.wantErr)
			}
		})
	}
}

func TestTokenNeverPrintsItself(t *testing.T) {
	t.Parallel()

	// The guard is the type, not a convention: every formatting verb goes
	// through String(), including one nested in a struct.
	token := forge.Token(secret)
	subjects := map[string]any{"a token": token, "a token in a struct": struct{ Token forge.Token }{Token: token}}

	for _, verb := range []string{"%v", "%s", "%q", "%+v"} {
		for subject, value := range subjects {
			t.Run(subject+" printed with "+verb, func(t *testing.T) {
				t.Parallel()

				// Act
				rendered := fmt.Sprintf(verb, value)

				// Assert
				if strings.Contains(rendered, secret) || !strings.Contains(rendered, "****") {
					t.Errorf("a Token printed as %s, want the mask and never the token", rendered)
				}
			})
		}
	}
}

func TestTokenStringIsAMask(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		token forge.Token
		want  string
	}{
		"a token": {token: forge.Token(secret), want: "****"},
		// An empty token has nothing to hide, and masking it would invent a
		// credential where there is none.
		"no token": {token: forge.Token(""), want: ""},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act & Assert
			if got := tt.token.String(); got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestTokenSecretIsTheOnlyWayOut(t *testing.T) {
	t.Parallel()

	// Act & Assert
	if got := forge.Token(secret).Secret(); got != secret {
		t.Errorf("Secret() = %q, want the token", got)
	}
}

func TestSourceString(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		source forge.Source
		want   string
	}{
		"none":                     {source: forge.SourceNone, want: "none"},
		"the environment":          {source: forge.SourceEnvironment, want: "the environment"},
		"the forge CLI":            {source: forge.SourceCLI, want: "the forge CLI"},
		"the configuration":        {source: forge.SourceConfiguration, want: "forge.token"},
		"a value outside the enum": {source: forge.Source(99), want: unknown},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act & Assert
			if got := tt.source.String(); got != tt.want {
				t.Errorf("Source(%d).String() = %q, want %q", tt.source, got, tt.want)
			}
		})
	}
}
