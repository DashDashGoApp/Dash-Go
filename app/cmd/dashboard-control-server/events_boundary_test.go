package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestEventDomainBoundaryUsesServiceAndCalendarPolicyFacade(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("could not locate test source")
	}
	serverRoot := filepath.Dir(thisFile)
	projectRoot := filepath.Clean(filepath.Join(serverRoot, "..", ".."))
	facadePath := filepath.Join(serverRoot, "events_facade.go")
	facade, err := os.ReadFile(facadePath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(facade), `return a.eventService().Refresh(force, daysPast, daysFuture)`) {
		t.Fatal("event facade lost required Refresh delegation")
	}

	fileSet := token.NewFileSet()
	parsed, err := parser.ParseFile(fileSet, facadePath, facade, 0)
	if err != nil {
		t.Fatalf("parse event facade: %v", err)
	}
	fields, ok := eventServiceConfigFields(parsed)
	if !ok {
		t.Fatal("event facade no longer constructs eventspkg.ServiceConfig")
	}
	for key, want := range map[string]string{
		"OutputEnabled":  "a.calendarOutputEnabledForURL",
		"SourceIdentity": "calendarSourceIdentity",
		"OwnedSource":    "ownedCalendarSource",
	} {
		got, present := fields[key]
		if !present {
			t.Fatalf("event facade lost required ServiceConfig seam %q", key)
		}
		if got != want {
			t.Fatalf("event facade ServiceConfig seam %q = %q; want %q", key, got, want)
		}
	}

	for _, retired := range []string{
		"events_ics_parse.go", "events_recurrence.go", "events_sources.go", "events_cache_pipeline.go", "events_utils.go",
	} {
		if _, err := os.Stat(filepath.Join(serverRoot, retired)); !os.IsNotExist(err) {
			t.Fatalf("retired main-package event implementation survived: %s (%v)", retired, err)
		}
	}
	serviceRoot := filepath.Join(projectRoot, "internal", "calendar", "events")
	seen := 0
	err = filepath.WalkDir(serviceRoot, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		seen++
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if strings.Contains(string(body), "cmd/dashboard-control-server") || strings.Contains(string(body), "*app") {
			t.Fatalf("event service leaked a core dependency in %s", path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if seen == 0 {
		t.Fatal("event service source is missing")
	}
}

// eventServiceConfigFields returns the ServiceConfig fields from the unique
// eventspkg.New(eventspkg.ServiceConfig{...}) construction in events_facade.go.
// It deliberately inspects Go syntax instead of formatted text so gofmt's
// alignment changes cannot weaken or spuriously fail this boundary contract.
func eventServiceConfigFields(file *ast.File) (map[string]string, bool) {
	var config *ast.CompositeLit
	ast.Inspect(file, func(node ast.Node) bool {
		if config != nil {
			return false
		}
		call, ok := node.(*ast.CallExpr)
		if !ok || !matchesSelector(call.Fun, "eventspkg", "New") || len(call.Args) != 1 {
			return true
		}
		lit, ok := call.Args[0].(*ast.CompositeLit)
		if !ok || !matchesSelector(lit.Type, "eventspkg", "ServiceConfig") {
			return true
		}
		config = lit
		return false
	})
	if config == nil {
		return nil, false
	}
	fields := make(map[string]string, len(config.Elts))
	for _, raw := range config.Elts {
		field, ok := raw.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		key, ok := field.Key.(*ast.Ident)
		if !ok {
			continue
		}
		fields[key.Name] = exprName(field.Value)
	}
	return fields, true
}

func matchesSelector(expr ast.Expr, qualifier, member string) bool {
	selector, ok := expr.(*ast.SelectorExpr)
	if !ok || selector.Sel == nil || selector.Sel.Name != member {
		return false
	}
	ident, ok := selector.X.(*ast.Ident)
	return ok && ident.Name == qualifier
}

func exprName(expr ast.Expr) string {
	switch value := expr.(type) {
	case *ast.Ident:
		return value.Name
	case *ast.SelectorExpr:
		if qualifier, ok := value.X.(*ast.Ident); ok && value.Sel != nil {
			return qualifier.Name + "." + value.Sel.Name
		}
	}
	return ""
}
