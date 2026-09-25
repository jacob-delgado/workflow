// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package forge

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// ErrNoToken reports that no source supplied a credential.
var ErrNoToken = errors.New("no forge token found")

// masked is what a Token renders as. It is deliberately not the real value, and
// not a prefix of it.
const masked = "****"

// Token is a forge credential.
//
// It is a named type with a String and a GoString rather than a bare string,
// and that is a guard rather than a decoration: every verb a string takes — %v,
// %s, %q, %x, %X and %#v — goes through one of them and yields the mask, as does
// a Token in an exported field printed with %v, %+v or %#v. fmt calls neither
// method for a verb a string rejects (%d, %t, ...), which go vet's printf check
// flags, nor for a Token in an unexported field. Reading the real value takes
// an explicit Secret call, which is easy to find in review and impossible to do
// by accident.
type Token string

var (
	_ fmt.Stringer   = Token("")
	_ fmt.GoStringer = Token("")
)

// String masks the token.
func (t Token) String() string {
	if t == "" {
		return ""
	}

	return masked
}

// GoString masks the token under %#v, quoted so the output still reads as Go
// syntax: "****", or "" for an empty Token.
func (t Token) GoString() string {
	return strconv.Quote(t.String())
}

// Secret returns the real value. Call it only where the credential is being
// sent, never where it might be printed.
func (t Token) Secret() string {
	return string(t)
}

// Source is where a token came from, for reporting.
type Source int

const (
	// SourceNone means nothing supplied one.
	SourceNone Source = iota
	// SourceEnvironment is an environment variable.
	SourceEnvironment
	// SourceCLI is the forge's own command line tool.
	SourceCLI
	// SourceConfiguration is forge.token in the configuration file.
	SourceConfiguration
)

var _ fmt.Stringer = Source(0)

// String names the source for humans.
func (s Source) String() string {
	switch s {
	case SourceNone:
		return "none"
	case SourceEnvironment:
		return "the environment"
	case SourceCLI:
		return "the forge CLI"
	case SourceConfiguration:
		return "forge.token"
	default:
		return unknownLabel
	}
}

// Look reports where a program is, or an error if it is not on PATH.
// exec.LookPath satisfies it.
type Look func(name string) (string, error)

// Run executes a program and returns its standard output. proc.Run satisfies it.
type Run func(ctx context.Context, name string, args ...string) ([]byte, error)

// Configured is what the configuration file says about the forge: forge.kind,
// forge.host and forge.token.
type Configured struct {
	// Kind is "github" or "gitlab", for a host whose name says neither.
	Kind string
	// Host is the host Kind and Token were written for, and the only one
	// either applies to.
	Host string
	// Token is consulted last, after the environment and the forge's own CLI.
	Token Token
}

// tokenFor is forge.token where host is the one it was written for: forge.host,
// or with none named, a host that names its own forge.
func (c Configured) tokenFor(host string) (Token, bool) {
	if c.Token == "" {
		return "", false
	}

	if c.Host == "" {
		return c.Token, kindOf(hostname(host)) != KindUnknown
	}

	return c.Token, sameHost(c.Host, host)
}

// Resolver finds a forge credential. Every source it consults is a field, so
// its tests never read the real environment or spawn anything.
type Resolver struct {
	// Getenv reads an environment variable. os.Getenv satisfies it.
	Getenv func(string) string
	// Look and Run reach the forge's own command line tool, if it is installed.
	Look Look
	Run  Run
	// Configured is the configuration file's say, consulted last.
	Configured Configured
}

// Resolve finds the credential for a forge, and says where it came from.
//
// The order is deliberate. An environment variable is the most explicit thing a
// person can do and wins; the forge's own CLI is next, so anyone already signed
// in with it needs no configuration at all; forge.token comes last, because
// storing a credential we do not have to store is the option worth avoiding.
//
// Every source answers for a host, as the forges' own tools have theirs do, and
// a token is offered to the host it is for and to no other.
func (r Resolver) Resolve(ctx context.Context, kind Kind, host string) (Token, Source, error) {
	for _, name := range r.environmentNames(kind, host) {
		value := r.Getenv(name)
		if value != "" {
			return Token(value), SourceEnvironment, nil
		}
	}

	token, ok := r.fromCLI(ctx, kind, host)
	if ok {
		return token, SourceCLI, nil
	}

	if configured, ok := r.Configured.tokenFor(host); ok {
		return configured, SourceConfiguration, nil
	}

	return "", SourceNone, ErrNoToken
}

// fromCLI asks the forge's own tool. Any failure is a miss rather than an
// error: the tool exits non-zero when it simply holds no credential for the
// host, which is not a problem, only a reason to try the next source.
func (r Resolver) fromCLI(ctx context.Context, kind Kind, host string) (Token, bool) {
	program, args, ok := cliCommand(kind, host)
	if !ok {
		return "", false
	}

	_, err := r.Look(program)
	if err != nil {
		return "", false
	}

	output, err := r.Run(ctx, program, args...)
	if err != nil {
		return "", false
	}

	token := strings.TrimSpace(string(output))
	if token == "" {
		return "", false
	}

	return Token(token), true
}

// environmentNames lists the variables that may hold a token for a host, in
// order.
func (r Resolver) environmentNames(kind Kind, host string) []string {
	switch kind {
	case KindGitHub:
		return r.githubNames(host)
	case KindGitLab:
		return r.gitlabNames(host)
	case KindUnknown:
		return nil
	default:
		return nil
	}
}

// githubNames reads GitHub's variables as gh does: its own two are for the
// hosts GitHub runs, and the enterprise two are for the host GH_HOST names.
func (r Resolver) githubNames(host string) []string {
	switch {
	case githubsOwn(host):
		return []string{"GITHUB_TOKEN", "GH_TOKEN"}
	case sameHost(r.Getenv("GH_HOST"), host):
		return []string{"GH_ENTERPRISE_TOKEN", "GITHUB_ENTERPRISE_TOKEN"}
	default:
		return nil
	}
}

// gitlabCom is the host GitLab's variables are for when nothing names another.
const gitlabCom = "gitlab.com"

// gitlabNames reads GitLab's variables as glab does: they are for the host
// GITLAB_HOST names, or GL_HOST after it, which is gitlab.com when neither
// names one.
func (r Resolver) gitlabNames(host string) []string {
	named := r.Getenv("GITLAB_HOST")
	if named == "" {
		named = r.Getenv("GL_HOST")
	}

	if named == "" {
		named = gitlabCom
	}

	if !sameHost(named, host) {
		return nil
	}

	return []string{"GITLAB_TOKEN", "GLAB_TOKEN"}
}

// Sources says where a token for a host would be read from, for telling someone
// who has none what to set. It names only what Resolve would read for that host.
func Sources(kind Kind, host string) string {
	name := hostname(host)

	switch {
	case kind == KindGitLab && name == gitlabCom:
		return "set $GITLAB_TOKEN, or set forge.token"
	case kind == KindGitLab:
		return "set $GITLAB_TOKEN with $GITLAB_HOST naming " + name + ", or set forge.token with forge.host"
	case githubsOwn(host):
		return "set $GITHUB_TOKEN, run `gh auth login`, or set forge.token"
	default:
		return "set $GH_ENTERPRISE_TOKEN with $GH_HOST naming " + name + ", run `gh auth login --hostname " + name +
			"`, or set forge.token with forge.host"
	}
}

// cliCommand is how to ask a forge's own tool for the token.
//
// GitHub only. `gh auth token` prints the credential on standard output and
// nothing else, which is a contract worth relying on. glab has no equivalent:
// it reports its token through `auth status`, whose output is prose on standard
// error, and parsing prose is not something to put a credential behind.
func cliCommand(kind Kind, host string) (string, []string, bool) {
	if kind != KindGitHub {
		return "", nil, false
	}

	return "gh", []string{"auth", "token", "--hostname", host}, true
}
