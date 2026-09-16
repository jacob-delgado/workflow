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

// printing is a Run that answers with output, as a CLI writing to stdout would.
func printing(output string) forge.Run {
	return func(context.Context, string, ...string) ([]byte, error) {
		return []byte(output), nil
	}
}

func TestResolveTokenPrefersTheEnvironment(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		kind forge.Kind
		name string
	}{
		"github's own name":  {kind: forge.KindGitHub, name: "GITHUB_TOKEN"},
		"github's gh name":   {kind: forge.KindGitHub, name: "GH_TOKEN"},
		"gitlab's own name":  {kind: forge.KindGitLab, name: "GITLAB_TOKEN"},
		"gitlab's glab name": {kind: forge.KindGitLab, name: "GLAB_TOKEN"},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			resolver := forge.Resolver{
				Getenv:     envWith(tt.name, secret),
				Look:       programAt("gh"),
				Run:        printing("a-different-token\n"),
				Configured: "a-third-token",
			}

			token, source, err := resolver.Resolve(t.Context(), tt.kind, "example.com")
			if err != nil {
				t.Fatalf("Resolve returned %v, want nil", err)
			}

			if string(token) != secret {
				t.Errorf("token came from somewhere else, source %v", source)
			}

			if source != forge.SourceEnvironment {
				t.Errorf("source = %v, want SourceEnvironment", source)
			}
		})
	}
}

func TestResolveTokenFallsBackToTheForgeCLI(t *testing.T) {
	t.Parallel()

	resolver := forge.Resolver{
		Getenv:     noEnv,
		Look:       programAt("gh"),
		Run:        printing(secret + "\n"),
		Configured: "",
	}

	token, source, err := resolver.Resolve(t.Context(), forge.KindGitHub, "github.com")
	if err != nil {
		t.Fatalf("Resolve returned %v, want nil", err)
	}

	// gh prints the token and a newline, and nothing else.
	if string(token) != secret {
		t.Errorf("token = %q, want the trimmed output of gh auth token", token)
	}

	if source != forge.SourceCLI {
		t.Errorf("source = %v, want SourceCLI", source)
	}
}

func TestResolveTokenFallsBackToTheConfiguration(t *testing.T) {
	t.Parallel()

	resolver := forge.Resolver{
		Getenv:     noEnv,
		Look:       noProgram,
		Run:        printing("never reached"),
		Configured: secret,
	}

	token, source, err := resolver.Resolve(t.Context(), forge.KindGitHub, "github.com")
	if err != nil {
		t.Fatalf("Resolve returned %v, want nil", err)
	}

	if string(token) != secret {
		t.Errorf("token = %q, want the configured one", token)
	}

	if source != forge.SourceConfiguration {
		t.Errorf("source = %v, want SourceConfiguration", source)
	}
}

func TestResolveTokenReportsWhenThereIsNone(t *testing.T) {
	t.Parallel()

	resolver := forge.Resolver{Getenv: noEnv, Look: noProgram, Run: printing(""), Configured: ""}

	_, source, err := resolver.Resolve(t.Context(), forge.KindGitHub, "github.com")
	if !errors.Is(err, forge.ErrNoToken) {
		t.Errorf("Resolve returned %v, want ErrNoToken", err)
	}

	if source != forge.SourceNone {
		t.Errorf("source = %v, want SourceNone", source)
	}
}

func TestResolveTokenIgnoresAFailingCLI(t *testing.T) {
	t.Parallel()

	// gh exits non-zero when it holds no credential for the host. That is not an
	// error worth reporting — it just means the next source gets a turn.
	resolver := forge.Resolver{
		Getenv: noEnv,
		Look:   programAt("gh"),
		Run: func(context.Context, string, ...string) ([]byte, error) {
			return nil, errors.New("exit status 1") //nolint:err113 // a stand-in for whatever gh returns
		},
		Configured: secret,
	}

	token, source, err := resolver.Resolve(t.Context(), forge.KindGitHub, "github.com")
	if err != nil {
		t.Fatalf("Resolve returned %v, want it to fall through to the configuration", err)
	}

	if source != forge.SourceConfiguration || string(token) != secret {
		t.Errorf("source = %v token = %q, want the configured token", source, token)
	}
}

func TestResolveTokenHasNoCLIForGitLab(t *testing.T) {
	t.Parallel()

	// glab reports its token through `auth status`, whose output is prose on
	// standard error. Parsing that is too fragile to put a credential behind, so
	// GitLab users set the environment variable or the configuration instead.
	resolver := forge.Resolver{
		Getenv:     noEnv,
		Look:       programAt("glab"),
		Run:        printing("should not be run"),
		Configured: secret,
	}

	_, source, err := resolver.Resolve(t.Context(), forge.KindGitLab, "gitlab.com")
	if err != nil {
		t.Fatalf("Resolve returned %v, want nil", err)
	}

	if source != forge.SourceConfiguration {
		t.Errorf("source = %v, want the configuration rather than a CLI", source)
	}
}

func TestTokenNeverPrintsItself(t *testing.T) {
	t.Parallel()

	token := forge.Token(secret)

	// The guard is the type, not a convention: every formatting verb goes
	// through String(), including one nested in a struct.
	wrapped := struct{ Token forge.Token }{Token: token}

	for _, verb := range []string{"%v", "%s", "%q", "%+v"} {
		for _, subject := range []any{token, wrapped} {
			rendered := fmt.Sprintf(verb, subject)
			if strings.Contains(rendered, secret) {
				t.Errorf("a Token printed itself with %s: %s", verb, rendered)
			}
		}
	}

	if token.String() != "****" {
		t.Errorf("String() = %q, want the mask", token.String())
	}

	// An empty token has nothing to hide, and masking it would invent a
	// credential where there is none.
	if forge.Token("").String() != "" {
		t.Errorf("an empty Token rendered as %q, want empty", forge.Token("").String())
	}

	// Reading it back is explicit, and that is the only way out.
	if token.Secret() != secret {
		t.Errorf("Secret() = %q, want the token", token.Secret())
	}
}

func TestSourceString(t *testing.T) {
	t.Parallel()

	cases := map[forge.Source]string{
		forge.SourceNone:          "none",
		forge.SourceEnvironment:   "the environment",
		forge.SourceCLI:           "the forge CLI",
		forge.SourceConfiguration: "forge.token",
		forge.Source(99):          unknown,
	}

	for source, want := range cases {
		if got := source.String(); got != want {
			t.Errorf("Source(%d).String() = %q, want %q", source, got, want)
		}
	}
}

func TestResolveTokenHasNoEnvironmentNamesForAnUnknownForge(t *testing.T) {
	t.Parallel()

	// An on-premises host names neither forge, so there is no variable to read
	// and no CLI to ask — only what the configuration says.
	resolver := forge.Resolver{
		Getenv:     envWith("GITHUB_TOKEN", "should not be read"),
		Look:       programAt("gh"),
		Run:        printing("should not be run"),
		Configured: secret,
	}

	token, source, err := resolver.Resolve(t.Context(), forge.KindUnknown, "git.example.com")
	if err != nil {
		t.Fatalf("Resolve returned %v, want nil", err)
	}

	if source != forge.SourceConfiguration || string(token) != secret {
		t.Errorf("source = %v token = %q, want the configured token", source, token)
	}
}

func TestResolveTokenIgnoresACLIThatPrintsNothing(t *testing.T) {
	t.Parallel()

	// gh can exit zero and print only a newline. That is not a token.
	resolver := forge.Resolver{
		Getenv:     noEnv,
		Look:       programAt("gh"),
		Run:        printing("  \n"),
		Configured: secret,
	}

	_, source, err := resolver.Resolve(t.Context(), forge.KindGitHub, "github.com")
	if err != nil {
		t.Fatalf("Resolve returned %v, want nil", err)
	}

	if source != forge.SourceConfiguration {
		t.Errorf("source = %v, want the configuration", source)
	}
}
