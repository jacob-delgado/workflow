// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package forge

import (
	"net"
	"strings"
)

// sameHost reports two spellings of one host. A host that was never named is
// the same as nothing.
func sameHost(named, host string) bool {
	return named != "" && hostname(named) == hostname(host)
}

// hostname is a host as it is compared: lowercased, and without the scheme,
// path or port it may have been written with. A remote over SSH carries SSH's
// port, and GITLAB_HOST has been written both bare and as an address.
func hostname(host string) string {
	if _, rest, found := strings.Cut(host, "://"); found {
		host = rest
	}

	host, _, _ = strings.Cut(host, "/")

	name, _, err := net.SplitHostPort(host)
	if err == nil {
		host = name
	}

	return strings.ToLower(host)
}

// githubsOwn reports a host GitHub itself runs: github.com, or an Enterprise
// Cloud tenant under ghe.com.
func githubsOwn(host string) bool {
	name := hostname(host)

	return name == "github.com" || strings.HasSuffix(name, ".ghe.com")
}
