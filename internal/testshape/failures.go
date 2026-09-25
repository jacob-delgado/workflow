// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package testshape

import (
	"go/ast"
	"go/token"
)

// outcome is what a call, a helper or an Assert can do to the test, weakest
// first.
type outcome int

const (
	silent outcome = iota
	// ambiguous can fail the test only through a method whose receiver's type
	// syntax does not show, where methods of its name disagree.
	ambiguous
	failing
)

// pkg is one package's test files, with what every check needs to know about
// them: its helpers, what each can do to the test, and the rules' fixed tables.
type pkg struct {
	fset  *token.FileSet
	files []*ast.File
	// funcs are the package's non-test functions by name, methods its methods
	// by receiver type and name, methodsNamed the same by name alone, and
	// fileOf where each is declared.
	funcs        map[string][]*ast.FuncDecl
	methods      map[methodKey]*ast.FuncDecl
	methodsNamed map[string][]*ast.FuncDecl
	fileOf       map[*ast.FuncDecl]*ast.File
	outcomes     map[*ast.FuncDecl]outcome

	failureMethods map[string]bool
	keywords       map[string]markerKind
	loose          map[string]bool
	transitions    map[state]map[markerKind]transition
	unfinished     map[state]string
	messages       map[Rule]string
}

// newPackage indexes a package's test files and works out what each helper
// can do to the test.
func newPackage(fset *token.FileSet, files []*ast.File) *pkg {
	indexed := &pkg{
		fset: fset, files: files,
		funcs: map[string][]*ast.FuncDecl{}, methods: map[methodKey]*ast.FuncDecl{},
		methodsNamed: map[string][]*ast.FuncDecl{},
		fileOf:       map[*ast.FuncDecl]*ast.File{}, outcomes: map[*ast.FuncDecl]outcome{},
		failureMethods: map[string]bool{
			"Error": true, "Errorf": true, "Fatal": true, "Fatalf": true, "Fail": true, "FailNow": true,
		},
		keywords: markerKeywords(), loose: looseKeywords(),
		transitions: transitions(), unfinished: unfinished(), messages: messages(),
	}

	for _, file := range files {
		for _, decl := range file.Decls {
			function, ok := decl.(*ast.FuncDecl)
			if !ok || function.Body == nil || indexed.isTest(file, function) {
				continue
			}

			indexed.index(file, function)
		}
	}

	indexed.findHelperOutcomes()

	return indexed
}

// index records a helper under its name.
func (p *pkg) index(file *ast.File, function *ast.FuncDecl) {
	p.fileOf[function] = file

	if function.Recv == nil {
		p.funcs[function.Name.Name] = append(p.funcs[function.Name.Name], function)

		return
	}

	p.indexMethod(function)
}

// findHelperOutcomes works out what every helper can do to the test, directly
// or through another helper, repeating until nothing changes so a chain of any
// length, or a cycle, settles. An outcome only ever rises, so it ends.
func (p *pkg) findHelperOutcomes() {
	for changed := true; changed; {
		changed = false

		for function, file := range p.fileOf {
			found := newScope(p, file, function).reaches(function.Body.List).outcome
			if found > p.outcomes[function] {
				p.outcomes[function], changed = found, true
			}
		}
	}
}

// strongest is the most any of the helpers can do to the test.
func (p *pkg) strongest(helpers []*ast.FuncDecl) outcome {
	found := silent

	for _, helper := range helpers {
		found = max(found, p.outcomes[helper])
	}

	return found
}

// reach is what a walk found: the most anything in it can do to the test, and
// the first call that does that much.
type reach struct {
	outcome outcome
	at      token.Pos
}

// raisedTo is the reach once a call at pos turns out to do found.
func (r reach) raisedTo(found outcome, pos token.Pos) reach {
	if found <= r.outcome {
		return r
	}

	return reach{outcome: found, at: pos}
}

// reaches is what the nodes can do to the test.
func (s scope) reaches(nodes []ast.Stmt) reach {
	return s.reachesVisiting(nodes, map[*ast.FuncLit]bool{})
}

// reachesVisiting is reaches, remembering the closures already followed so a
// closure that calls itself ends. A stored literal is not walked where it is
// stored: it counts where its name is called, or handed to a call or set in a
// composite literal, from where it may run.
func (s scope) reachesVisiting(nodes []ast.Stmt, visited map[*ast.FuncLit]bool) reach {
	var found reach

	visit := func(node ast.Node) bool {
		if found.outcome == failing {
			return false
		}

		switch node := node.(type) {
		case *ast.FuncLit:
			return !s.stored[node]
		case *ast.CallExpr:
			found = found.raisedTo(max(s.fails(node, visited), s.handsOnClosure(node.Args, visited)), node.Pos())
		case *ast.CompositeLit:
			found = found.raisedTo(s.handsOnClosure(fieldValues(node.Elts), visited), node.Pos())
		}

		return true
	}

	for _, node := range nodes {
		ast.Inspect(node, visit)
	}

	return found
}

// handsOnClosure is the most the stored closures among the values can do to
// the test.
func (s scope) handsOnClosure(values []ast.Expr, visited map[*ast.FuncLit]bool) outcome {
	found := silent

	for _, value := range values {
		reached, _ := s.follow(value, visited)
		found = max(found, reached)
	}

	return found
}

// follow is what the closure an expression names can do to the test, and
// whether it names one at all. A closure already followed does nothing more,
// so one that calls itself ends.
func (s scope) follow(expr ast.Expr, visited map[*ast.FuncLit]bool) (outcome, bool) {
	path, ok := pathOf(expr)
	if !ok {
		return silent, false
	}

	literal, isClosure := s.closures[path]
	if !isClosure || visited[literal] {
		return silent, isClosure
	}

	visited[literal] = true

	return s.reachesVisiting(literal.Body.List, visited).outcome, true
}

// fieldValues are the values a composite literal sets, keyed or not.
func fieldValues(elements []ast.Expr) []ast.Expr {
	values := make([]ast.Expr, len(elements))

	for index, element := range elements {
		values[index] = element
		if pair, ok := element.(*ast.KeyValueExpr); ok {
			values[index] = pair.Value
		}
	}

	return values
}

// fails is what a call can do to the test: a failure method on a test value
// or a closure that reaches one fails it, and a helper handed the test does
// what the helper does.
func (s scope) fails(call *ast.CallExpr, visited map[*ast.FuncLit]bool) outcome {
	if found, isClosure := s.follow(call.Fun, visited); isClosure {
		return found
	}

	switch function := call.Fun.(type) {
	case *ast.Ident:
		return s.handedTest(call, s.pkg.strongest(s.pkg.funcs[function.Name]))
	case *ast.SelectorExpr:
		return s.selectorFails(call, function)
	default:
		return silent
	}
}

// selectorFails is what x.name(...) can do to the test. A failure method on
// *testing.F does not count: inside a fuzz target it panics rather than
// failing the input.
func (s scope) selectorFails(call *ast.CallExpr, function *ast.SelectorExpr) outcome {
	if receiver, ok := function.X.(*ast.Ident); ok {
		if kind, isValue := s.values[receiver.Name]; isValue {
			return failingIf(kind != kindF && s.pkg.failureMethods[function.Sel.Name])
		}

		if s.imports[receiver.Name] {
			return silent
		}
	}

	return s.handedTest(call, s.methodOutcome(function))
}

// failingIf is failing when a call fails the test, and silent otherwise.
func failingIf(fails bool) outcome {
	if fails {
		return failing
	}

	return silent
}

// handedTest is what a helper that does found does from a call: nothing, unless
// the call hands it the test.
func (s scope) handedTest(call *ast.CallExpr, found outcome) outcome {
	if !s.passesTest(call) {
		return silent
	}

	return found
}

// passesTest reports whether a call hands a helper the test, without which the
// helper cannot fail it.
func (s scope) passesTest(call *ast.CallExpr) bool {
	for _, arg := range call.Args {
		ident, ok := arg.(*ast.Ident)
		if !ok {
			continue
		}

		if kind, isValue := s.values[ident.Name]; isValue && kind != kindF {
			return true
		}
	}

	return false
}
