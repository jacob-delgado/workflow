// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package forge_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/forge"
)

// Hosts the cases resolve a token for.
const (
	gheTenant   = "acme.ghe.com"
	anotherHost = "other.example.com"
	onPremSSH   = "Git.Example.com:2222"
	asGitLabURL = "https://git.example.com"
)

// The variables and configuration keys the cases set and look for.
const (
	enterpriseGH  = "GH_ENTERPRISE_TOKEN"
	hostForGH     = "GH_HOST"
	gitlabToken   = "GITLAB_TOKEN"
	hostForGitLab = "GITLAB_HOST"
	tokenKey      = "forge.token"
	hostKey       = "forge.host"
)

// envOf answers the named variables and no others.
func envOf(variables map[string]string) func(string) string {
	return func(asked string) string { return variables[asked] }
}

// onlyEnv is a resolver with nothing but an environment to draw on.
func onlyEnv(variables map[string]string) forge.Resolver {
	return forge.Resolver{Getenv: envOf(variables), Look: noProgram, Run: printing(""), Configured: inFile("")}
}

// onlyFile is a resolver with nothing but a configuration file to draw on.
func onlyFile(host string) forge.Resolver {
	return forge.Resolver{
		Getenv: noEnv, Look: noProgram, Run: printing(""),
		Configured: forge.Configured{Kind: "", Host: host, Token: secret},
	}
}

func TestResolveOffersATokenOnlyToTheHostItIsFor(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		resolver   forge.Resolver
		kind       forge.Kind
		host       string
		wantSource forge.Source
		wantErr    error
	}{
		// GitHub's own variables are for GitHub's own hosts, as gh reads them.
		"github's variable on github.com": {
			resolver: onlyEnv(map[string]string{"GITHUB_TOKEN": secret}),
			kind:     forge.KindGitHub, host: githubHost, wantSource: forge.SourceEnvironment,
		},
		"github's variable on a ghe.com tenant": {
			resolver: onlyEnv(map[string]string{"GH_TOKEN": secret}),
			kind:     forge.KindGitHub, host: gheTenant, wantSource: forge.SourceEnvironment,
		},
		"github's variable on any other host": {
			resolver: onlyEnv(map[string]string{"GITHUB_TOKEN": secret, "GH_TOKEN": secret}),
			kind:     forge.KindGitHub, host: onPremHost, wantErr: forge.ErrNoToken,
		},
		// The enterprise variables are for the one host GH_HOST names.
		"the enterprise variable on the host GH_HOST names": {
			resolver: onlyEnv(map[string]string{enterpriseGH: secret, hostForGH: onPremHost}),
			kind:     forge.KindGitHub, host: onPremHost, wantSource: forge.SourceEnvironment,
		},
		"the enterprise variable under its other name": {
			resolver: onlyEnv(map[string]string{"GITHUB_ENTERPRISE_TOKEN": secret, hostForGH: onPremHost}),
			kind:     forge.KindGitHub, host: onPremHost, wantSource: forge.SourceEnvironment,
		},
		"a host is the same host whatever its case or port": {
			resolver: onlyEnv(map[string]string{enterpriseGH: secret, hostForGH: onPremHost}),
			kind:     forge.KindGitHub, host: onPremSSH, wantSource: forge.SourceEnvironment,
		},
		"the enterprise variable when GH_HOST names another host": {
			resolver: onlyEnv(map[string]string{enterpriseGH: secret, hostForGH: anotherHost}),
			kind:     forge.KindGitHub, host: onPremHost, wantErr: forge.ErrNoToken,
		},
		"the enterprise variable with no GH_HOST": {
			resolver: onlyEnv(map[string]string{enterpriseGH: secret}),
			kind:     forge.KindGitHub, host: onPremHost, wantErr: forge.ErrNoToken,
		},
		// GitLab's variables are for the host GITLAB_HOST names, which is
		// gitlab.com when it names none, as glab reads them.
		"gitlab's variable on gitlab.com": {
			resolver: onlyEnv(map[string]string{gitlabToken: secret}),
			kind:     forge.KindGitLab, host: gitlabHost, wantSource: forge.SourceEnvironment,
		},
		"gitlab's variable on any other host": {
			resolver: onlyEnv(map[string]string{gitlabToken: secret, "GLAB_TOKEN": secret}),
			kind:     forge.KindGitLab, host: onPremHost, wantErr: forge.ErrNoToken,
		},
		"gitlab's variable on the host GITLAB_HOST names": {
			resolver: onlyEnv(map[string]string{"GLAB_TOKEN": secret, hostForGitLab: onPremHost}),
			kind:     forge.KindGitLab, host: onPremHost, wantSource: forge.SourceEnvironment,
		},
		"GITLAB_HOST written as an address": {
			resolver: onlyEnv(map[string]string{gitlabToken: secret, hostForGitLab: asGitLabURL}),
			kind:     forge.KindGitLab, host: onPremHost, wantSource: forge.SourceEnvironment,
		},
		"glab's other name for the host": {
			resolver: onlyEnv(map[string]string{gitlabToken: secret, "GL_HOST": onPremHost}),
			kind:     forge.KindGitLab, host: onPremHost, wantSource: forge.SourceEnvironment,
		},
		"gitlab's variable on gitlab.com when GITLAB_HOST names another host": {
			resolver: onlyEnv(map[string]string{gitlabToken: secret, hostForGitLab: onPremHost}),
			kind:     forge.KindGitLab, host: gitlabHost, wantErr: forge.ErrNoToken,
		},
		// forge.token is for forge.host, and with none named, for the forge a
		// remote names by itself.
		"forge.token on the host forge.host names": {
			resolver: onlyFile(onPremHost),
			kind:     forge.KindGitHub, host: onPremSSH, wantSource: forge.SourceConfiguration,
		},
		"forge.token on any other host": {
			resolver: onlyFile(onPremHost),
			kind:     forge.KindGitHub, host: anotherHost, wantErr: forge.ErrNoToken,
		},
		"forge.token with no forge.host, on github.com": {
			resolver: onlyFile(""),
			kind:     forge.KindGitHub, host: githubHost, wantSource: forge.SourceConfiguration,
		},
		"forge.token with no forge.host, on a host that names no forge": {
			resolver: onlyFile(""),
			kind:     forge.KindGitHub, host: onPremHost, wantErr: forge.ErrNoToken,
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			token, source, err := tt.resolver.Resolve(t.Context(), tt.kind, tt.host)

			// Assert
			if !errors.Is(err, tt.wantErr) || source != tt.wantSource {
				t.Fatalf("Resolve for %s = a token from %v, %v; want one from %v, %v",
					tt.host, source, err, tt.wantSource, tt.wantErr)
			}

			if offered := token.Secret() != ""; offered != (tt.wantErr == nil) {
				t.Errorf("Resolve for %s offered a token: %t", tt.host, offered)
			}
		})
	}
}

func TestSourcesNamesWhatWouldBeReadForTheHost(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		kind     forge.Kind
		host     string
		want     []string
		unwanted []string
	}{
		"GitHub's own host": {
			kind: forge.KindGitHub, host: githubHost,
			want:     []string{"$GITHUB_TOKEN", "gh auth login", tokenKey},
			unwanted: []string{hostForGH, hostKey},
		},
		"another GitHub host": {
			kind: forge.KindGitHub, host: onPremSSH,
			want: []string{
				"$" + enterpriseGH, "$" + hostForGH, "gh auth login --hostname " + onPremHost, tokenKey, hostKey,
			},
			unwanted: []string{"$GITHUB_TOKEN", "2222"},
		},
		"GitLab's own host": {
			kind: forge.KindGitLab, host: gitlabHost,
			want:     []string{"$" + gitlabToken, tokenKey},
			unwanted: []string{hostForGitLab, "gh auth", hostKey},
		},
		"another GitLab host": {
			kind: forge.KindGitLab, host: onPremHost,
			want:     []string{"$" + gitlabToken, "$" + hostForGitLab, onPremHost, tokenKey, hostKey},
			unwanted: []string{"gh auth"},
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			got := forge.Sources(tt.kind, tt.host)

			// Assert
			for _, want := range tt.want {
				if !strings.Contains(got, want) {
					t.Errorf("Sources = %q, want it to name %q", got, want)
				}
			}

			for _, unwanted := range tt.unwanted {
				if strings.Contains(got, unwanted) {
					t.Errorf("Sources = %q, want nothing about %q", got, unwanted)
				}
			}
		})
	}
}
