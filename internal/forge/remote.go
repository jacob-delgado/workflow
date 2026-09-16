// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

// Package forge talks to the Git forge a repository lives on — GitHub or
// GitLab, hosted or on-premises.
package forge

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
)

// Errors this package returns. Callers distinguish them with errors.Is.
var (
	// ErrNotARemote reports a remote URL that does not name a repository.
	ErrNotARemote = errors.New("not a repository remote")
	// ErrUnknownForge reports a host that names neither GitHub nor GitLab.
	ErrUnknownForge = errors.New("cannot tell which forge this host is")
)

// unknownLabel is what every enum in this package renders as when it holds a
// value outside the set it declares.
const unknownLabel = "unknown"

// Kind is which forge a repository lives on.
type Kind int

const (
	// KindUnknown means the host names neither forge, which is the normal case
	// for an on-premises instance.
	KindUnknown Kind = iota
	// KindGitHub is github.com or GitHub Enterprise Server.
	KindGitHub
	// KindGitLab is gitlab.com or a self-managed GitLab.
	KindGitLab
)

// String names the forge for humans.
func (k Kind) String() string {
	switch k {
	case KindUnknown:
		return unknownLabel
	case KindGitHub:
		return "GitHub"
	case KindGitLab:
		return "GitLab"
	default:
		return unknownLabel
	}
}

// Repo identifies a repository on a forge.
type Repo struct {
	// Kind is the forge, where the host says which one.
	Kind Kind
	// Host is the web host, including a port when the remote named one.
	Host string
	// Path is everything identifying the project on that host. It is not split
	// into an owner and a name because GitLab nests projects arbitrarily deep,
	// and splitting would quietly discard the middle.
	Path string
}

// minimumSegments is the shortest path that can name a project: an owner and a
// repository.
const minimumSegments = 2

// ParseRemote reads the repository a git remote URL points at.
func ParseRemote(remote string) (Repo, error) {
	address, err := url.Parse(normalize(remote))
	if err != nil {
		// Unwrapped: url.Parse quotes the whole URL, and a remote can carry a
		// password.
		return Repo{}, ErrNotARemote
	}

	if address.Host == "" {
		return Repo{}, ErrNotARemote
	}

	path := strings.Trim(address.Path, "/")
	path = strings.TrimSuffix(path, ".git")

	if len(strings.Split(path, "/")) < minimumSegments {
		return Repo{}, ErrNotARemote
	}

	// address.Host keeps any port; the kind is decided by the name alone.
	return Repo{Kind: kindOf(address.Hostname()), Host: address.Host, Path: path}, nil
}

// normalize rewrites git's scp-style remote into something url.Parse accepts.
//
// `git@github.com:owner/repo.git` is the shape git writes by default, and it is
// not a URL: the colon separates a host from a path rather than introducing a
// port.
func normalize(remote string) string {
	if strings.Contains(remote, "://") {
		return remote
	}

	colon := strings.Index(remote, ":")
	slash := strings.Index(remote, "/")

	// The shape, not the presence of a user, is what identifies it: a colon
	// with no slash before it. Requiring "user@" would miss host:path, which
	// url.Parse then accepts with an EMPTY host — parsing it as a scheme and an
	// opaque body rather than failing, so the mistake is silent.
	if colon < 0 || (slash >= 0 && slash < colon) {
		return remote
	}

	return "ssh://" + remote[:colon] + "/" + remote[colon+1:]
}

// kindOf reports which forge a hostname belongs to. An on-premises host names
// neither, and nothing about the URL can say which it is.
func kindOf(hostname string) Kind {
	switch strings.ToLower(hostname) {
	case "github.com":
		return KindGitHub
	case "gitlab.com":
		return KindGitLab
	default:
		return KindUnknown
	}
}

// APIBase is the root of the REST API serving this repository.
func (r Repo) APIBase() (string, error) {
	switch r.Kind {
	case KindGitHub:
		return githubAPIBase(r.Host), nil
	case KindGitLab:
		// The same path for gitlab.com and for every self-managed instance.
		return "https://" + r.Host + "/api/v4", nil
	case KindUnknown:
		return "", ErrUnknownForge
	default:
		return "", ErrUnknownForge
	}
}

// githubAPIBase picks between GitHub's three API shapes, which is the rule gh
// itself applies.
func githubAPIBase(host string) string {
	switch {
	case strings.EqualFold(host, "github.com"):
		return "https://api.github.com"
	// Enterprise Cloud with data residency keeps github.com's shape rather than
	// Enterprise Server's. Do not fold this into the case below: it would send
	// every request to a path that does not exist there.
	case strings.HasSuffix(strings.ToLower(host), ".ghe.com"):
		return "https://api." + host
	default:
		return "https://" + host + "/api/v3"
	}
}

// ParseKind reads a forge named in configuration.
func ParseKind(name string) (Kind, error) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "":
		return KindUnknown, nil
	case "github":
		return KindGitHub, nil
	case "gitlab":
		return KindGitLab, nil
	default:
		return KindUnknown, fmt.Errorf("%w: %q", ErrUnknownForge, name)
	}
}

// WithConfiguredKind fills in a forge the remote could not name.
//
// It only ever fills a gap. A host that names itself — github.com, gitlab.com —
// keeps its own answer, because the remote is the better evidence and silently
// disagreeing with it would be the worse failure.
func (r Repo) WithConfiguredKind(name string) (Repo, error) {
	if r.Kind != KindUnknown || name == "" {
		return r, nil
	}

	kind, err := ParseKind(name)
	if err != nil {
		return r, err
	}

	r.Kind = kind

	return r, nil
}
