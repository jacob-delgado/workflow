// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package testshape

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// Exit statuses, as the repository's other gates use them.
const (
	exitClean      = 0
	exitViolations = 1
	exitUnusable   = 2
)

var (
	// errUsage reports a command line with nothing to check.
	errUsage = errors.New("name the _test.go files to check")
	// errNotATest reports an argument that is not a Go test file.
	errNotATest = errors.New("not a _test.go file")
)

// Run checks the packages of the named test files, each package once and whole,
// since a helper in one file decides whether an Assert in another reaches a
// failure. It prints each violation to stderr and returns 1 when there are any,
// 0 when there are none, and 2 when it cannot check what it was given.
func Run(args []string, stdout, stderr io.Writer) int {
	violations, err := checkFiles(args)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "testshape: %v\n", err)

		return exitUnusable
	}

	if len(violations) == 0 {
		_, _ = fmt.Fprintln(stdout, "testshape: every test marks its Arrange, Act and Assert.")

		return exitClean
	}

	for _, violation := range violations {
		_, _ = fmt.Fprintln(stderr, violation)
	}

	return exitViolations
}

// checkFiles checks every package the files belong to.
func checkFiles(args []string) ([]Violation, error) {
	dirs, err := packageDirs(args)
	if err != nil {
		return nil, err
	}

	fset := token.NewFileSet()

	var violations []Violation

	for _, dir := range dirs {
		packages, err := parseTests(fset, dir)
		if err != nil {
			return nil, err
		}

		for _, name := range sortedKeys(packages) {
			violations = append(violations, Check(fset, packages[name])...)
		}
	}

	sortViolations(violations)

	return violations, nil
}

// packageDirs is each directory holding a named test file, once, in order.
func packageDirs(args []string) ([]string, error) {
	if len(args) == 0 {
		return nil, errUsage
	}

	var dirs []string

	for _, arg := range args {
		if !strings.HasSuffix(arg, "_test.go") {
			return nil, fmt.Errorf("%s: %w", arg, errNotATest)
		}

		_, err := os.Stat(arg)
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", arg, err)
		}

		dir := filepath.Dir(arg)
		if !slices.Contains(dirs, dir) {
			dirs = append(dirs, dir)
		}
	}

	slices.Sort(dirs)

	return dirs, nil
}

// parseTests parses every test file in a directory, grouped by package clause:
// a directory can hold both package x's tests and package x_test's.
func parseTests(fset *token.FileSet, dir string) (map[string][]*ast.File, error) {
	// Not filepath.Glob: it skips a directory it cannot read without saying so,
	// and a check that silently checks nothing passes.
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("listing %s: %w", dir, err)
	}

	packages := map[string][]*ast.File{}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}

		path := filepath.Join(dir, entry.Name())

		file, err := parser.ParseFile(fset, path, nil, parser.ParseComments|parser.SkipObjectResolution)
		if err != nil {
			return nil, fmt.Errorf("parsing: %w", err)
		}

		packages[file.Name.Name] = append(packages[file.Name.Name], file)
	}

	return packages, nil
}

// sortedKeys is a map's keys in order.
func sortedKeys(packages map[string][]*ast.File) []string {
	keys := make([]string, 0, len(packages))
	for key := range packages {
		keys = append(keys, key)
	}

	slices.Sort(keys)

	return keys
}
