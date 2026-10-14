// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// nearest is the configuration file that applies in dir: the nearest one
// walking up from dir to the root of the repository dir is in — the directory
// holding .git — so a session in a subdirectory finds the repository's own
// file rather than skipping it for the one at home. Outside a repository it is
// dir's own file alone: a parent directory that is no repository's, a shared
// /tmp among them, is somewhere anyone may have put one.
func nearest(dir string) (string, bool) {
	root, inRepository := enclosingRepository(dir)
	if !inRepository {
		return fileIn(dir)
	}

	for current := dir; ; current = filepath.Dir(current) {
		if candidate, ok := fileIn(current); ok {
			return candidate, true
		}

		if current == root {
			return "", false
		}
	}
}

// RepoRoot returns the root of the repository dir sits in — the nearest
// ancestor holding a .git — so a file written there is found from any
// subdirectory below it. It returns dir unchanged when no repository encloses
// it, which is where a lone file still belongs.
func RepoRoot(dir string) string {
	root, inRepository := enclosingRepository(dir)
	if !inRepository {
		return dir
	}

	return root
}

// enclosingRepository is the nearest of dir and its ancestors holding a .git,
// and whether there is one.
func enclosingRepository(dir string) (string, bool) {
	for current := dir; current != ""; {
		if atRepoRoot(current) {
			return current, true
		}

		parent := filepath.Dir(current)
		if parent == current {
			break
		}

		current = parent
	}

	return "", false
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

	err = errors.Join(cfg.validateVersion(), cfg.validateJira(), cfg.validateForge(), cfg.validateTiming(),
		cfg.validateBranch(), cfg.validateViews(), cfg.validateHeaders(), cfg.validateCommit(), cfg.validatePullRequest(),
		cfg.validateMessaging(), cfg.validateUI(), cfg.validateTaskwarrior())
	if err != nil {
		return Default(), fmt.Errorf("%w: %w", ErrInvalid, err)
	}

	return cfg, nil
}

// FieldError is a validation's refusal of one setting, which it names by its
// dotted key as the file spells it: jira.base_url, branch.slug_limit.
type FieldError struct {
	Key string
	Err error
}

var _ error = FieldError{}

// Error is the refusal, which already says which setting it refuses and why.
func (e FieldError) Error() string {
	return e.Err.Error()
}

// Unwrap is the refusal, so errors.Is still finds its kind.
func (e FieldError) Unwrap() error {
	return e.Err
}

// FieldErrors is each setting err refuses, in the order Parse validates
// them, through any wrapping; none for an error that refuses no one setting,
// such as a key the configuration does not have.
func FieldErrors(err error) []FieldError {
	//nolint:errorlint // each error in the tree is looked at itself: errors.As would find only the first refusal
	switch refusal := err.(type) {
	case FieldError:
		return []FieldError{refusal}
	case interface{ Unwrap() []error }:
		var fields []FieldError
		for _, joined := range refusal.Unwrap() {
			fields = append(fields, FieldErrors(joined)...)
		}

		return fields
	case interface{ Unwrap() error }:
		return FieldErrors(refusal.Unwrap())
	default:
		return nil
	}
}

// refusing is err as a refusal of the setting key.
func refusing(key string, err error) FieldError {
	return FieldError{Key: key, Err: err}
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
