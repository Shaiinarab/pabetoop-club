package app

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strconv"
	"strings"
	"testing"
)

// httpMethods are the method verbs net/http's ServeMux accepts as a pattern
// prefix. PocketBase's router mirrors them, and a duplicated method+pattern
// makes ServeMux panic.
var httpMethods = map[string]bool{
	"GET": true, "POST": true, "PUT": true, "PATCH": true,
	"DELETE": true, "HEAD": true, "OPTIONS": true, "CONNECT": true, "TRACE": true,
}

// minExpectedRoutes guards the guard: if the routes ever move to a different
// registration shape, this test would silently match nothing and pass forever.
// Update the detector — and this floor — rather than deleting the check.
const minExpectedRoutes = 10

// TestNoDuplicateRouteRegistrations catches the one failure that neither the
// compiler nor any other test in this package can see: registering the same
// method and path twice.
//
// PocketBase builds its mux from these registrations, and net/http's ServeMux
// panics on a duplicate pattern. That panic happens while the mux is built,
// which is on the *first request* — health endpoint included. So a duplicate
// ships as a clean, fully green build and then the server fails on contact.
//
// This is not hypothetical: registerPublicRoutes used to register
// "/assets/manifest.webmanifest" twice, and `make run` was broken by it. The
// source is parsed rather than the router booted so the check stays fast,
// offline, and independent of PocketBase's internals.
func TestNoDuplicateRouteRegistrations(t *testing.T) {
	fset := token.NewFileSet()

	pkgs, err := parser.ParseDir(fset, ".", func(fi os.FileInfo) bool {
		return !strings.HasSuffix(fi.Name(), "_test.go")
	}, 0)
	if err != nil {
		t.Fatalf("parse package: %v", err)
	}

	type registration struct {
		position token.Position
	}
	seen := map[string]registration{} // "GET /path" -> first registration
	var duplicates []string

	for _, pkg := range pkgs {
		for _, file := range pkg.Files {
			ast.Inspect(file, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				sel, ok := call.Fun.(*ast.SelectorExpr)
				if !ok || !httpMethods[sel.Sel.Name] || len(call.Args) == 0 {
					return true
				}
				// Only match <recv>.Router.<METHOD>(...).
				recv, ok := sel.X.(*ast.SelectorExpr)
				if !ok || recv.Sel.Name != "Router" {
					return true
				}
				lit, ok := call.Args[0].(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					return true
				}
				path, err := strconv.Unquote(lit.Value)
				if err != nil {
					return true
				}

				key := sel.Sel.Name + " " + path
				position := fset.Position(call.Pos())
				if first, dup := seen[key]; dup {
					duplicates = append(duplicates,
						fmt.Sprintf("%s — first at %s, again at %s", key, first.position, position))
					return true
				}
				seen[key] = registration{position: position}
				return true
			})
		}
	}

	if len(duplicates) > 0 {
		t.Fatalf("duplicate route registration — net/http's ServeMux panics on these while the "+
			"mux is built, i.e. on the first request, so the server would die on contact:\n  %s",
			strings.Join(duplicates, "\n  "))
	}

	if len(seen) < minExpectedRoutes {
		t.Fatalf("found only %d route registrations (expected at least %d) — this guard is no "+
			"longer seeing the routes it exists to check", len(seen), minExpectedRoutes)
	}
}
