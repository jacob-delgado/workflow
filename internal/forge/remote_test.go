// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package forge_test

import (
	"errors"
	"testing"

	"github.com/jacob-delgado/workflow/internal/forge"
)

// Hosts and paths these tests repeat.
const (
	githubHost = "github.com"
	gitlabHost = "gitlab.com"
	onPremHost = "git.example.com"
	acmePath   = "acme/thing"
	ownerRepo  = "owner/repo"
	shortPath  = "o/r"
	unknown    = "unknown"
	github     = "github"
	gitlab     = "gitlab"
)

func TestParseRemote(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		remote string
		want   forge.Repo
	}{
		"scp-style ssh, the git default": {
			remote: "git@github.com:owner/repo.git",
			want:   forge.Repo{Kind: forge.KindGitHub, Host: githubHost, Path: ownerRepo},
		},
		"https with the .git suffix": {
			remote: "https://github.com/owner/repo.git",
			want:   forge.Repo{Kind: forge.KindGitHub, Host: githubHost, Path: ownerRepo},
		},
		"https without it": {
			remote: "https://github.com/owner/repo",
			want:   forge.Repo{Kind: forge.KindGitHub, Host: githubHost, Path: ownerRepo},
		},
		"explicit ssh scheme": {
			remote: "ssh://git@github.com/owner/repo.git",
			want:   forge.Repo{Kind: forge.KindGitHub, Host: githubHost, Path: ownerRepo},
		},
		"a trailing slash": {
			remote: "https://github.com/owner/repo/",
			want:   forge.Repo{Kind: forge.KindGitHub, Host: githubHost, Path: ownerRepo},
		},
		// A remote can carry userinfo. Keeping it would put a password into
		// every derived URL, and doctor prints those.
		"credentials are dropped": {
			remote: "https://alice:sekret@github.com/owner/repo.git",
			want:   forge.Repo{Kind: forge.KindGitHub, Host: githubHost, Path: ownerRepo},
		},
		// GitLab nests projects arbitrarily deep. Splitting into owner and name
		// would silently drop the middle of the path.
		"a gitlab subgroup": {
			remote: "git@gitlab.com:group/subgroup/project.git",
			want:   forge.Repo{Kind: forge.KindGitLab, Host: gitlabHost, Path: "group/subgroup/project"},
		},
		"a deeply nested gitlab subgroup": {
			remote: "https://gitlab.com/a/b/c/d/project.git",
			want:   forge.Repo{Kind: forge.KindGitLab, Host: gitlabHost, Path: "a/b/c/d/project"},
		},
		// An on-prem host names neither forge, so the kind is unknown until
		// something else says which it is.
		"an on-premises host": {
			remote: "git@git.example.com:acme/thing.git",
			want:   forge.Repo{Kind: forge.KindUnknown, Host: onPremHost, Path: acmePath},
		},
		// An SSH port has nothing to do with the HTTPS API, so it is dropped
		// from the host rather than leaking into every derived URL.
		"a non-standard ssh port is dropped from the host": {
			remote: "ssh://git@git.example.com:2222/acme/thing.git",
			want:   forge.Repo{Kind: forge.KindUnknown, Host: onPremHost, Path: acmePath},
		},
		// url.Parse ACCEPTS this one, with an empty host, so a parser that only
		// rewrites the user@ form rejects a perfectly good remote.
		"scp-style with no user": {
			remote: "git.example.com:acme/thing.git",
			want:   forge.Repo{Kind: forge.KindUnknown, Host: onPremHost, Path: acmePath},
		},
		"scp-style with no user on github": {
			remote: "github.com:owner/repo.git",
			want:   forge.Repo{Kind: forge.KindGitHub, Host: githubHost, Path: ownerRepo},
		},
		"a subdomain of github is not github.com": {
			remote: "https://pages.github.com/owner/repo.git",
			want:   forge.Repo{Kind: forge.KindUnknown, Host: "pages.github.com", Path: ownerRepo},
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			got, err := forge.ParseRemote(tt.remote)

			// Assert
			if err != nil || got != tt.want {
				t.Errorf("ParseRemote(%q) = %+v, %v; want %+v", tt.remote, got, err, tt.want)
			}
		})
	}
}

func TestParseRemoteRejectsWhatIsNotARepository(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"empty":          "",
		"no path at all": "https://github.com",
		"only an owner":  "https://github.com/owner",
		"no host":        "/owner/repo.git",
		"a control byte": "https://github.com/owner/\x7frepo",
	}

	for name, remote := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			_, err := forge.ParseRemote(remote)

			// Assert
			if !errors.Is(err, forge.ErrNotARemote) {
				t.Errorf("ParseRemote(%q) returned %v, want ErrNotARemote", remote, err)
			}
		})
	}
}

func TestKindString(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		kind forge.Kind
		want string
	}{
		"unknown":                  {kind: forge.KindUnknown, want: unknown},
		github:                     {kind: forge.KindGitHub, want: "GitHub"},
		gitlab:                     {kind: forge.KindGitLab, want: "GitLab"},
		"a value outside the enum": {kind: forge.Kind(99), want: unknown},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act & Assert
			if got := tt.kind.String(); got != tt.want {
				t.Errorf("Kind(%d).String() = %q, want %q", tt.kind, got, tt.want)
			}
		})
	}
}

func TestRepoAPIBase(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		repo    forge.Repo
		want    string
		wantErr error
	}{
		"github.com has its own API host": {
			repo: forge.Repo{Kind: forge.KindGitHub, Host: githubHost, Path: shortPath},
			want: "https://api.github.com",
		},
		// GitHub Enterprise Cloud with data residency keeps github.com's shape
		// rather than Enterprise Server's. gh special-cases it the same way.
		"enterprise cloud with data residency": {
			repo: forge.Repo{Kind: forge.KindGitHub, Host: "octocorp.ghe.com", Path: shortPath},
			want: "https://api.octocorp.ghe.com",
		},
		"enterprise server": {
			repo: forge.Repo{Kind: forge.KindGitHub, Host: "ghe.example.com", Path: shortPath},
			want: "https://ghe.example.com/api/v3",
		},
		"gitlab.com": {
			repo: forge.Repo{Kind: forge.KindGitLab, Host: gitlabHost, Path: shortPath},
			want: "https://gitlab.com/api/v4",
		},
		// Self-managed GitLab uses the same path as gitlab.com, with nothing in
		// the hostname to announce it.
		"self-managed gitlab": {
			repo: forge.Repo{Kind: forge.KindGitLab, Host: "salsa.debian.org", Path: shortPath},
			want: "https://salsa.debian.org/api/v4",
		},
		// A GitHub Enterprise Server and a self-managed GitLab look identical from
		// the remote URL alone, and their API paths differ. Guessing would send
		// the token to the wrong service.
		"an on-premises forge it cannot guess": {
			repo:    forge.Repo{Kind: forge.KindUnknown, Host: onPremHost, Path: acmePath},
			want:    "",
			wantErr: forge.ErrUnknownForge,
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			got, err := tt.repo.APIBase()

			// Assert
			if !errors.Is(err, tt.wantErr) || got != tt.want {
				t.Errorf("APIBase() = %q, %v; want %q, %v", got, err, tt.want, tt.wantErr)
			}
		})
	}
}

// An SSH remote on a non-standard port names the port for SSH, not for the
// HTTPS API. The port must not survive into the API's address.
func TestAnSSHPortDoesNotReachTheAPIBase(t *testing.T) {
	t.Parallel()

	// Arrange
	repo, parseErr := forge.ParseRemote("ssh://git@gitlab.com:2222/group/repo.git")
	if parseErr != nil {
		t.Fatalf("ParseRemote() = %v, want nil", parseErr)
	}

	// Act
	base, err := repo.APIBase()

	// Assert
	if err != nil || base != "https://gitlab.com/api/v4" {
		t.Errorf("APIBase() = %q, %v; want %q, nil", base, err, "https://gitlab.com/api/v4")
	}
}

func TestParseKind(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		name    string
		want    forge.Kind
		wantErr bool
	}{
		github:              {name: github, want: forge.KindGitHub},
		gitlab:              {name: gitlab, want: forge.KindGitLab},
		"case insensitive":  {name: "GitHub", want: forge.KindGitHub},
		"surrounding space": {name: "  gitlab  ", want: forge.KindGitLab},
		"empty is unset":    {name: "", want: forge.KindUnknown},
		"anything else":     {name: "bitbucket", want: forge.KindUnknown, wantErr: true},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			got, err := forge.ParseKind(tt.name)

			// Assert
			if tt.wantErr != (err != nil) || got != tt.want {
				t.Errorf("ParseKind(%q) = %v, %v; want %v (error: %v)", tt.name, got, err, tt.want, tt.wantErr)
			}
		})
	}
}

func TestRepoWithConfiguredKindOnlyFillsAGap(t *testing.T) {
	t.Parallel()

	onPrem := forge.Repo{Kind: forge.KindUnknown, Host: onPremHost, Path: acmePath}

	cases := map[string]struct {
		repo       forge.Repo
		configured string
		host       string
		want       forge.Kind
		wantErr    error
	}{
		"a host that names no forge takes the kind configured for it": {
			repo: onPrem, configured: gitlab, host: onPremHost, want: forge.KindGitLab,
		},
		"a host is the same host whatever its case or port": {
			repo:       forge.Repo{Kind: forge.KindUnknown, Host: "Git.Example.com:2222", Path: acmePath},
			configured: gitlab, host: onPremHost, want: forge.KindGitLab,
		},
		// forge.kind describes forge.host, and says nothing about any other.
		"a kind configured for another host is not taken": {
			repo: onPrem, configured: gitlab, host: "other.example.com", want: forge.KindUnknown,
		},
		"a kind configured for no host says it needs one": {
			repo: onPrem, configured: gitlab, host: "", want: forge.KindUnknown, wantErr: forge.ErrKindNeedsHost,
		},
		// A host that already named itself is not overridden: the remote is the
		// better evidence, and disagreeing with it silently would be worse.
		"a host that names its forge keeps it": {
			repo:       forge.Repo{Kind: forge.KindGitHub, Host: githubHost, Path: ownerRepo},
			configured: gitlab, host: githubHost,
			want: forge.KindGitHub,
		},
		"a configured kind that is not one": {
			repo: onPrem, configured: "bitbucket", host: onPremHost,
			want: forge.KindUnknown, wantErr: forge.ErrUnknownForge,
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			got, err := tt.repo.WithConfiguredKind(forge.Configured{Kind: tt.configured, Host: tt.host, Token: ""})

			// Assert
			if !errors.Is(err, tt.wantErr) || got.Kind != tt.want {
				t.Errorf("WithConfiguredKind(%q) = %v, %v; want %v, %v", tt.configured, got.Kind, err, tt.want, tt.wantErr)
			}
		})
	}
}
