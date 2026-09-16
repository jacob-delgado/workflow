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
		"a non-standard ssh port is kept on the host": {
			remote: "ssh://git@git.example.com:2222/acme/thing.git",
			want:   forge.Repo{Kind: forge.KindUnknown, Host: "git.example.com:2222", Path: acmePath},
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

			got, err := forge.ParseRemote(tt.remote)
			if err != nil {
				t.Fatalf("ParseRemote(%q) returned %v, want nil", tt.remote, err)
			}

			if got != tt.want {
				t.Errorf("ParseRemote(%q) = %+v, want %+v", tt.remote, got, tt.want)
			}
		})
	}
}

func TestParseRemoteDropsCredentials(t *testing.T) {
	t.Parallel()

	// A remote can carry userinfo. Keeping it would put a password into every
	// derived URL, and doctor prints those.
	repo, err := forge.ParseRemote("https://alice:sekret@github.com/owner/repo.git")
	if err != nil {
		t.Fatalf("ParseRemote returned %v, want nil", err)
	}

	if repo.Host != "github.com" {
		t.Errorf("Host = %q, want the host without its userinfo", repo.Host)
	}

	if repo.Path != "owner/repo" {
		t.Errorf("Path = %q, want %q", repo.Path, "owner/repo")
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

			_, err := forge.ParseRemote(remote)
			if !errors.Is(err, forge.ErrNotARemote) {
				t.Errorf("ParseRemote(%q) returned %v, want ErrNotARemote", remote, err)
			}
		})
	}
}

func TestKindString(t *testing.T) {
	t.Parallel()

	cases := map[forge.Kind]string{
		forge.KindUnknown: unknown,
		forge.KindGitHub:  "GitHub",
		forge.KindGitLab:  "GitLab",
		forge.Kind(99):    unknown,
	}

	for kind, want := range cases {
		if got := kind.String(); got != want {
			t.Errorf("Kind(%d).String() = %q, want %q", kind, got, want)
		}
	}
}

func TestRepoAPIBase(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		repo forge.Repo
		want string
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
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got, err := tt.repo.APIBase()
			if err != nil {
				t.Fatalf("APIBase() returned %v, want nil", err)
			}

			if got != tt.want {
				t.Errorf("APIBase() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRepoAPIBaseCannotGuessAnOnPremisesForge(t *testing.T) {
	t.Parallel()

	// A GitHub Enterprise Server and a self-managed GitLab look identical from
	// the remote URL alone, and their API paths differ. Guessing would send the
	// token to the wrong service.
	repo := forge.Repo{Kind: forge.KindUnknown, Host: onPremHost, Path: acmePath}

	_, err := repo.APIBase()
	if !errors.Is(err, forge.ErrUnknownForge) {
		t.Errorf("APIBase() returned %v, want ErrUnknownForge", err)
	}
}

func TestParseKind(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		name    string
		want    forge.Kind
		wantErr bool
	}{
		"github":            {name: "github", want: forge.KindGitHub},
		"gitlab":            {name: "gitlab", want: forge.KindGitLab},
		"case insensitive":  {name: "GitHub", want: forge.KindGitHub},
		"surrounding space": {name: "  gitlab  ", want: forge.KindGitLab},
		"empty is unset":    {name: "", want: forge.KindUnknown},
		"anything else":     {name: "bitbucket", want: forge.KindUnknown, wantErr: true},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got, err := forge.ParseKind(tt.name)
			if tt.wantErr != (err != nil) {
				t.Fatalf("ParseKind(%q) error = %v, wantErr %v", tt.name, err, tt.wantErr)
			}

			if got != tt.want {
				t.Errorf("ParseKind(%q) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}

func TestRepoWithConfiguredKind(t *testing.T) {
	t.Parallel()

	onPrem := forge.Repo{Kind: forge.KindUnknown, Host: onPremHost, Path: acmePath}

	// The configuration only ever fills a gap.
	filled, err := onPrem.WithConfiguredKind("gitlab")
	if err != nil {
		t.Fatalf("WithConfiguredKind returned %v, want nil", err)
	}

	if filled.Kind != forge.KindGitLab {
		t.Errorf("Kind = %v, want the configured one", filled.Kind)
	}

	// A host that already named itself is not overridden: the remote is the
	// better evidence, and disagreeing with it silently would be worse.
	hosted := forge.Repo{Kind: forge.KindGitHub, Host: githubHost, Path: ownerRepo}

	kept, err := hosted.WithConfiguredKind("gitlab")
	if err != nil {
		t.Fatalf("WithConfiguredKind returned %v, want nil", err)
	}

	if kept.Kind != forge.KindGitHub {
		t.Errorf("Kind = %v, want the host's own answer to win", kept.Kind)
	}

	_, err = onPrem.WithConfiguredKind("bitbucket")
	if !errors.Is(err, forge.ErrUnknownForge) {
		t.Errorf("WithConfiguredKind(bitbucket) returned %v, want ErrUnknownForge", err)
	}
}
