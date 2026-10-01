// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

// nearest walks up from dir looking for the configuration file, so a session in
// a subdirectory of a repository finds the repository's own file rather than
// skipping it for the one at home. The search stops at the repository root — the
// directory holding .git — so it never escapes into an unrelated parent.
func nearest(dir string) (string, bool) {
	for dir != "" {
		if candidate, ok := fileIn(dir); ok {
			return candidate, true
		}

		if atRepoRoot(dir) {
			return "", false
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}

		dir = parent
	}

	return "", false
}

// RepoRoot returns the root of the repository dir sits in — the nearest
// ancestor holding a .git — so a file written there is found from any
// subdirectory below it. It returns dir unchanged when no repository encloses
// it, which is where a lone file still belongs.
func RepoRoot(dir string) string {
	for current := dir; current != ""; {
		if atRepoRoot(current) {
			return current
		}

		parent := filepath.Dir(current)
		if parent == current {
			break
		}

		current = parent
	}

	return dir
}

// fileIn is the configuration file in dir, when there is one.
func fileIn(dir string) (string, bool) {
	if dir == "" {
		return "", false
	}

	candidate := filepath.Join(dir, FileName)

	info, err := os.Stat(candidate)

	return candidate, err == nil && !info.IsDir()
}

// atRepoRoot reports whether dir holds a .git — a directory in a normal clone, a
// file in a worktree or submodule — which is where the walk upward stops.
func atRepoRoot(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, ".git"))

	return err == nil
}

// Load reads the configuration that applies: the files Locate finds, the
// repository's layered over the home directory's.
func Load(workDir, homeDir string) (Config, error) {
	files, err := Locate(workDir, homeDir)
	if err != nil {
		return Default(), err
	}

	cfg, _, err := LoadLayersAt(files)

	return cfg, err
}

// LoadFile reads the configuration from an exact path. Unknown keys are an
// error: a misspelled key that is silently ignored looks exactly like a
// credential that was never set.
func LoadFile(path string) (Config, error) {
	file, err := os.Open(path) //nolint:gosec // the path is the user's own config file, by design
	if err != nil {
		return Default(), fmt.Errorf("opening %s: %w", path, err)
	}
	defer file.Close() //nolint:errcheck // read-only file; a failed close is not actionable

	return parseFile(path, file)
}

// LoadFileAt reads the configuration from an exact path as LoadFile does,
// with the revision of the file it was read from. Both come from one read, so
// an edit landing between two reads cannot pair one state's revision with
// another's configuration. A file that does not exist, or an empty path, which
// names none, is the defaults at the no-file revision rather than an error.
func LoadFileAt(path string) (Config, Revision, error) {
	//nolint:gosec // the path is the user's own config file, by design
	contents, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return Default(), Revision{}, nil
	}

	if err != nil {
		return Default(), Revision{}, fmt.Errorf("reading %s: %w", path, err)
	}

	cfg, err := parseFile(path, bytes.NewReader(contents))
	if err != nil {
		return Default(), Revision{}, err
	}

	return cfg, revisionOfContents(contents), nil
}

// parseFile parses the configuration the file at path holds, as read from r.
func parseFile(path string, r io.Reader) (Config, error) {
	cfg, err := Parse(r)
	if err != nil {
		return Default(), fmt.Errorf("%s: %w", path, err)
	}

	cfg.Path = path

	return cfg, nil
}

// Parse decodes and validates a configuration from r, over the defaults. Unknown
// keys are an error, and the same validators a file read runs apply — so a
// configuration written over the web API is held to exactly the standard a file
// on disk is. It does not set Path; that belongs to the file it came from.
func Parse(r io.Reader) (Config, error) {
	contents, err := io.ReadAll(r)
	if err != nil {
		return Default(), fmt.Errorf("%w: %w", ErrInvalid, err)
	}

	cfg, err := decode(contents)
	if err != nil {
		return Default(), err
	}

	err = errors.Join(cfg.validateVersion(), cfg.validateTiming(),
		cfg.validateBranch(), cfg.validateViews(), cfg.validateCommit(), cfg.validatePullRequest(),
		cfg.validateMessaging(), cfg.validateUI(), cfg.validateTaskwarrior())
	if err != nil {
		return Default(), fmt.Errorf("%w: %w", ErrInvalid, err)
	}

	return cfg, nil
}

// decode reads contents over the defaults, refusing a key the configuration
// does not have.
func decode(contents []byte) (Config, error) {
	decoder := json.NewDecoder(bytes.NewReader(contents))
	decoder.DisallowUnknownFields()

	cfg := Default()

	err := decoder.Decode(&cfg)
	if err == nil {
		return cfg, nil
	}

	return Default(), fmt.Errorf("%w: %w", ErrInvalid, err)
}
