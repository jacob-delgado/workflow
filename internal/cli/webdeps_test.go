// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli_test

import (
	"reflect"
	"testing"

	"github.com/jacob-delgado/workflow/internal/cli"
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
	for seam, field := range web.Fields() {
		if field.Kind() == reflect.Func && field.IsNil() {
			t.Errorf("webserver.Deps.%s is nil though the interface wires every seam", seam.Name)
		}
	}
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
