// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package gitrepo

import (
	"context"
	"strings"
)

// CodeOwners reads the distinct user handles named in the repository's
// CODEOWNERS file, in the order they first appear, for suggesting a pull
// request's reviewers. Team handles (@org/team) and email owners are left out,
// since a reviewer is requested by username. A repository with no CODEOWNERS
// has no owners rather than an error — the suggestion is simply absent.
func (r Repository) CodeOwners(ctx context.Context) ([]string, error) {
	for _, path := range []string{"CODEOWNERS", ".github/CODEOWNERS", "docs/CODEOWNERS"} {
		content, err := r.run(ctx, "git", "-C", r.dir, "show", "HEAD:"+path)
		if err != nil {
			continue
		}

		return ownersIn(string(content)), nil
	}

	return nil, nil
}

// ownersIn reads the distinct user handles from CODEOWNERS content, keeping the
// order they first appear so the same file always suggests in the same order.
func ownersIn(content string) []string {
	seen := map[string]bool{}

	var owners []string

	for line := range strings.SplitSeq(content, "\n") {
		fields := ownerFields(line)

		for _, token := range fields {
			handle, ok := userHandle(token)
			if !ok || seen[handle] {
				continue
			}

			seen[handle] = true
			owners = append(owners, handle)
		}
	}

	return owners
}

// ownerFields is a CODEOWNERS line's owner tokens: the fields after the path
// pattern, or none for a blank or comment line. A non-empty line always has at
// least the pattern, so dropping it never indexes out of range.
func ownerFields(line string) []string {
	line = strings.TrimSpace(line)
	if line == "" || strings.HasPrefix(line, "#") {
		return nil
	}

	return strings.Fields(line)[1:]
}

// userHandle reads a CODEOWNERS owner token as a username: it must start with @
// and name a person, not a team (@org/team) or an email address.
func userHandle(token string) (string, bool) {
	if !strings.HasPrefix(token, "@") {
		return "", false
	}

	handle := strings.TrimPrefix(token, "@")
	if handle == "" || strings.Contains(handle, "/") {
		return "", false
	}

	return handle, true
}
