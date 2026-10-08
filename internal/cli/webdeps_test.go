// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli_test

import (
	"errors"
	"maps"
	"reflect"
	"testing"

	"github.com/jacob-delgado/workflow/internal/cli"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/tui"
)

func TestWebDepsHandsTheServerEverySeam(t *testing.T) {
	t.Parallel()

	// Arrange
	// Every seam the interface declares is wired, so a web seam left nil can
	// only be one the mapping dropped — which the server would answer as "not
	// available" while every webserver test, built on its own Deps, stays green.
	var deps tui.Deps

	wireEverySeam(reflect.ValueOf(&deps).Elem())

	// Act
	web := reflect.ValueOf(cli.WebDeps(deps))

	// Assert
	for _, name := range nilSeams(web, "webserver.Deps") {
		// Unexpected is where the server reports its own failures, the next
		// four are the wiring's own controls over the settings the server
		// saves, and Reach is the command line's way to wire another
		// directory; none is one of the interface's seams, so the interface
		// has none to hand it.
		if name == "webserver.Deps.Unexpected" || name == "webserver.Deps.UseForgeSettings" ||
			name == "webserver.Deps.UseMessagingSettings" || name == "webserver.Deps.PlaceSlackCredentials" ||
			name == "webserver.Deps.KeepJiraToken" || name == "webserver.Deps.Reach" {
			continue
		}

		t.Errorf("%s is nil though the interface wires every seam", name)
	}
}

func TestWebDepsChecksAKeymapAsTheInterfaceDoes(t *testing.T) {
	t.Parallel()

	// Arrange
	// Moving jump-to-pane is a refusal only the interface's own check makes.
	moved := map[string]string{"jump-to-pane": "f12"}

	// Act
	check := cli.WebDeps(tui.Deps{}).CheckKeys

	// Assert
	if check == nil {
		t.Fatal("webserver.Deps.CheckKeys is nil, so the server saves a keymap the interface refuses")
	}

	err := check(moved)
	if !errors.Is(err, config.ErrKeyNotRebindable) {
		t.Errorf("CheckKeys(%v) = %v, want the interface's %v", moved, err, config.ErrKeyNotRebindable)
	}
}

func TestWebDepsHandsTheServerEachSeamTheInterfaceHasInItsPlace(t *testing.T) {
	t.Parallel()

	// Arrange
	// Seams that share a type — Stage, Unstage and Discard; Search and
	// SearchLenient; Commit, Push, Rebase and the hook's run — would build
	// wired crosswise, so each stub says which seam it is when it is called.
	var (
		deps   tui.Deps
		called string
	)

	interfaceSeams := wireNamedSeams(reflect.ValueOf(&deps).Elem(), "", &called)

	// Act
	web := reflect.ValueOf(cli.WebDeps(deps))

	// Assert
	checked := 0

	for path, seam := range funcSeams(web, "") {
		if !interfaceSeams[path] || seam.IsNil() {
			continue
		}

		called = ""

		callWithZeroes(seam)

		checked++

		if called != path {
			t.Errorf("the server's %s calls the interface's %q, want its own", path, called)
		}
	}

	if checked == 0 {
		t.Error("no seam the server holds is one of the interface's")
	}
}

// wireNamedSeams sets every function in value, and in the structs it holds,
// to a stub that writes its own path, prefixed by prefix, to called and
// answers zero values, and returns every path it set.
func wireNamedSeams(value reflect.Value, prefix string, called *string) map[string]bool {
	wired := map[string]bool{}

	for seam, field := range value.Fields() {
		path := prefix + seam.Name

		switch {
		case !field.CanSet():
		case field.Kind() == reflect.Struct:
			maps.Copy(wired, wireNamedSeams(field, path+".", called))
		case field.Kind() == reflect.Func:
			answer := zeroResults(field.Type())
			field.Set(reflect.MakeFunc(field.Type(), func(args []reflect.Value) []reflect.Value {
				*called = path

				return answer(args)
			}))

			wired[path] = true
		}
	}

	return wired
}

// funcSeams is every function in value, and in the structs it holds, by its
// path, prefixed by prefix.
func funcSeams(value reflect.Value, prefix string) map[string]reflect.Value {
	found := map[string]reflect.Value{}

	for seam, field := range value.Fields() {
		path := prefix + seam.Name

		switch {
		case field.Kind() == reflect.Struct:
			maps.Copy(found, funcSeams(field, path+"."))
		case field.Kind() == reflect.Func:
			found[path] = field
		}
	}

	return found
}

// callWithZeroes calls seam with the zero value of each of its parameters.
func callWithZeroes(seam reflect.Value) {
	args := make([]reflect.Value, seam.Type().NumIn())
	for index := range args {
		args[index] = reflect.Zero(seam.Type().In(index))
	}

	seam.Call(args)
}

// nilSeams names every nil function in value, and in the structs it holds,
// under prefix.
func nilSeams(value reflect.Value, prefix string) []string {
	var names []string

	for seam, field := range value.Fields() {
		name := prefix + "." + seam.Name

		if field.Kind() == reflect.Struct {
			names = append(names, nilSeams(field, name)...)
		}

		if field.Kind() == reflect.Func && field.IsNil() {
			names = append(names, name)
		}
	}

	return names
}

// wireEverySeam sets every function in value, and in the structs it holds, to
// a stub answering zero values.
func wireEverySeam(value reflect.Value) {
	for _, field := range value.Fields() {
		if !field.CanSet() {
			continue
		}

		if field.Kind() == reflect.Struct {
			wireEverySeam(field)
		}

		if field.Kind() == reflect.Func {
			field.Set(reflect.MakeFunc(field.Type(), zeroResults(field.Type())))
		}
	}
}

// zeroResults is a function body that returns the zero value of each of a
// function type's results.
func zeroResults(kind reflect.Type) func([]reflect.Value) []reflect.Value {
	return func([]reflect.Value) []reflect.Value {
		results := make([]reflect.Value, kind.NumOut())
		for index := range results {
			results[index] = reflect.Zero(kind.Out(index))
		}

		return results
	}
}
