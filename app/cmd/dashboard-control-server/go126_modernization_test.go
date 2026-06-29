package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// Static expressions must compile once at startup rather than once per
// calendar/event/diagnostic request. Dynamic patterns remain intentionally
// allowed when they are constructed from runtime keys or values.
func TestStaticRegexpsStayOutsideFunctionBodies(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("could not locate test source")
	}
	root := filepath.Dir(thisFile)
	fset := token.NewFileSet()
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return err
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			ast.Inspect(fn.Body, func(node ast.Node) bool {
				call, ok := node.(*ast.CallExpr)
				if !ok || len(call.Args) != 1 {
					return true
				}
				selector, ok := call.Fun.(*ast.SelectorExpr)
				if !ok || selector.Sel.Name != "MustCompile" {
					return true
				}
				pkg, ok := selector.X.(*ast.Ident)
				if !ok || pkg.Name != "regexp" {
					return true
				}
				if _, literal := call.Args[0].(*ast.BasicLit); literal {
					t.Errorf("static regexp.MustCompile remains inside %s (%s)", fn.Name.Name, filepath.Base(path))
				}
				return true
			})
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestMethodRoutesPreserveDashGoAPIResponses(t *testing.T) {
	a := &app{}
	mux := a.httpRoutes()

	for _, method := range []string{http.MethodHead, http.MethodPut, http.MethodPatch, http.MethodDelete} {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/api/status", nil)
			req.RemoteAddr = "127.0.0.1:12345"
			res := httptest.NewRecorder()
			mux.ServeHTTP(res, req)
			if res.Code != http.StatusMethodNotAllowed {
				t.Fatalf("status=%d want %d", res.Code, http.StatusMethodNotAllowed)
			}
			if got := res.Header().Get("Cache-Control"); got != "no-store, no-cache, must-revalidate, max-age=0" {
				t.Fatalf("Cache-Control=%q", got)
			}
			if !strings.Contains(res.Body.String(), `"method not allowed"`) {
				t.Fatalf("response did not keep JSON error: %s", res.Body.String())
			}
		})
	}

	remote := httptest.NewRequest(http.MethodPut, "/api/status", nil)
	remote.RemoteAddr = "203.0.113.9:12345"
	blocked := httptest.NewRecorder()
	mux.ServeHTTP(blocked, remote)
	if blocked.Code != http.StatusForbidden || !strings.Contains(blocked.Body.String(), `"loopback only"`) {
		t.Fatalf("non-loopback response=%d %s", blocked.Code, blocked.Body.String())
	}
}

func TestModernMapCopiesAreAllocatedAndIndependent(t *testing.T) {
	source := map[string]any{"state": "fresh"}
	for name, copyFn := range map[string]func(map[string]any) map[string]any{
		"settings": cloneStringAnyMap,
		"status":   copyStatusMap,
		"update":   copyUpdateAvailability,
		"todo":     todoCloneGraphRow,
	} {
		t.Run(name, func(t *testing.T) {
			copy := copyFn(source)
			if copy == nil {
				t.Fatal("copy is nil")
			}
			copy["state"] = "changed"
			if got := source["state"]; got != "fresh" {
				t.Fatalf("copy mutated source: %v", got)
			}
		})
	}
	if copied := cloneStringAnyMap(nil); copied == nil {
		t.Fatal("nil input must preserve the previous allocated-map contract")
	}
}
