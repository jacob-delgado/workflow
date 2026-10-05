// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package wiring

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/loop"
	"github.com/jacob-delgado/workflow/internal/sanitize"
	"github.com/jacob-delgado/workflow/internal/seams"
	"github.com/jacob-delgado/workflow/internal/workdirs"
)

// Workspace is where the interface runs: the repository's root, or the directory
// it was started in when there is no repository, and origin's URL.
type Workspace struct {
	Root   string
	Remote string
	// Dir is the directory the session was started in, Root or one within it,
	// with every link resolved.
	Dir string
	// Repository reports Root a repository's root rather than a directory
	// outside one.
	Repository bool
}

// Locate reads the repository dir is in. Outside one, every git action fails on
// its own and says why, so this is not an error.
func Locate(ctx context.Context, dir string) Workspace {
	// git names the root with every link resolved; the directory is read the
	// same way, so the path from one to the other never climbs out.
	resolved, err := filepath.EvalSymlinks(dir)
	if err == nil {
		dir = resolved
	}

	repo, err := gitrepo.At(gitRunner, dir).Describe(ctx)
	if err != nil {
		return Workspace{Root: dir, Remote: "", Dir: dir, Repository: false}
	}

	return Workspace{Root: repo.Root, Remote: repo.Remote, Dir: dir, Repository: true}
}

// Name names the repository as a person reads it: its forge path, which
// carries no credential, or else its directory's name.
func (w Workspace) Name() string {
	repo, parsed := parsedRemote(w)
	if !parsed {
		return filepath.Base(w.Root)
	}

	return repo.Path
}

// repoKey names the repository the store keys its state by: the origin remote's
// host and path where there is one, so the same repository shares it across
// clones, and the working tree's root otherwise. The remote is parsed to its
// host and path, never used raw, because an HTTPS remote can carry a credential
// in its userinfo and the store must never hold a secret.
func repoKey(where Workspace) string {
	repo, parsed := parsedRemote(where)
	if !parsed {
		return where.Root
	}

	return repo.Host + "/" + repo.Path
}

// parsedRemote is the origin remote parsed to its host and path, and whether
// there is one that parses.
func parsedRemote(where Workspace) (forge.Repo, bool) {
	if where.Remote == "" {
		return forge.Repo{}, false
	}

	repo, err := forge.ParseRemote(where.Remote)
	if err != nil {
		return forge.Repo{}, false
	}

	return repo, true
}

// repositoriesDeps is what a surface asks of the directories it can work in:
// where this session works, and any other directory read the same way, with
// the configuration files that would apply there.
func repositoriesDeps(ctx context.Context, cfg config.Config, where Workspace) seams.Repositories {
	home, _ := os.UserHomeDir()

	return seams.Repositories{
		Here: placeOf(where, cfg.Layers()),
		Home: home,
		Look: func(dir string) (seams.Place, error) {
			err := workdirs.Check(dir)
			if err != nil {
				return seams.Place{}, err
			}

			// No configuration file there or above is not a failure: the
			// defaults apply, as they would on switching.
			files, _ := config.Locate(dir, home)

			return placeOf(Locate(ctx, dir), files), nil
		},
		Subdirectories: workdirs.List,
	}
}

// placeOf is a workspace as a surface reads it, with the configuration files
// that apply there.
func placeOf(where Workspace, files config.Files) seams.Place {
	root := ""
	if where.Repository {
		root = where.Root
	}

	remote, _ := parsedRemote(where)

	return seams.Place{Dir: where.Dir, Root: root, Remote: remote, Config: files}
}

// yourCommits reads the commits you wrote in the repository where is in and in
// every favorite that is a repository, each repository once, however many of
// its directories are favorites. A repository is named only when more than one
// is read. With none, where is read regardless, so git says why there is
// nothing to read.
func yourCommits(
	ctx context.Context, where Workspace, favorites func() ([]string, error),
) func(start, end time.Time) []loop.RepositoryCommits {
	return func(start, end time.Time) []loop.RepositoryCommits {
		repositories := commitRepositories(ctx, where, favorites)
		if len(repositories) == 0 {
			repositories = []Workspace{where}
		}

		read := make([]loop.RepositoryCommits, 0, len(repositories))
		names := distinctNames(repositories)

		for at, repository := range repositories {
			commits, err := gitrepo.At(gitRunner, repository.Root).CommitsBetween(ctx, start, end)
			read = append(read, loop.RepositoryCommits{Repository: names[at], Commits: commits, Failed: err})
		}

		return read
	}
}

// commitRepositories is the repository where is in, when it is in one, then
// each favorite's that is not already listed. Repositories are told apart by
// the git directory their worktrees share, since each worktree has a root of
// its own. A favorites list that cannot be read reads as none: the
// Repositories pane says why, and the commits here are still yours to see.
func commitRepositories(ctx context.Context, where Workspace, favorites func() ([]string, error)) []Workspace {
	var repositories []Workspace

	seen := map[string]bool{}
	add := func(candidate Workspace) {
		if !candidate.Repository {
			return
		}

		shared, err := gitrepo.At(gitRunner, candidate.Root).SharedDir(ctx)
		if err != nil {
			shared = candidate.Root
		}

		if !seen[shared] {
			seen[shared] = true

			repositories = append(repositories, candidate)
		}
	}

	add(where)

	dirs, _ := favorites()
	for _, dir := range dirs {
		add(Locate(ctx, dir))
	}

	return repositories
}

// distinctNames names each repository read so no two read alike: by Name,
// unless that is another's too, when each that clashes is named by as much of
// the end of its root as tells them apart, marked as a directory so it never
// passes for a forge repository — …/work/api beside …/oss/api. Every name is
// sanitized before it is compared. With one repository, none is named.
func distinctNames(repositories []Workspace) []string {
	names := make([]string, len(repositories))
	if len(repositories) <= 1 {
		return names
	}

	ends := make([][]string, len(repositories))
	deepest := 0

	for at, repository := range repositories {
		names[at] = sanitize.Line(repository.Name())
		ends[at] = sanitizedElements(repository.Root)
		deepest = max(deepest, len(ends[at]))
	}

	for count := 2; count <= deepest; count++ {
		for _, at := range clashing(names) {
			names[at] = "…/" + strings.Join(ends[at][max(0, len(ends[at])-count):], "/")
		}
	}

	return names
}

// sanitizedElements is root's path elements, each sanitized.
func sanitizedElements(root string) []string {
	elements := strings.FieldsFunc(filepath.ToSlash(root), func(r rune) bool { return r == '/' })
	for at, element := range elements {
		elements[at] = sanitize.Line(element)
	}

	return elements
}

// clashing is the index of every name another shares.
func clashing(names []string) []int {
	counts := map[string]int{}
	for _, name := range names {
		counts[name]++
	}

	var clashes []int

	for at, name := range names {
		if counts[name] > 1 {
			clashes = append(clashes, at)
		}
	}

	return clashes
}
