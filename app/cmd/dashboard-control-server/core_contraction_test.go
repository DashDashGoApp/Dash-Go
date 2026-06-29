package main

import (
	"bytes"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func parsedStructFields(t *testing.T, path, typeName string) map[string]string {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	fields := map[string]string{}
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.TYPE {
			continue
		}
		for _, spec := range gen.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if !ok || typeSpec.Name.Name != typeName {
				continue
			}
			structType, ok := typeSpec.Type.(*ast.StructType)
			if !ok {
				t.Fatalf("%s in %s is not a struct", typeName, path)
			}
			for _, field := range structType.Fields.List {
				var rendered bytes.Buffer
				if err := format.Node(&rendered, token.NewFileSet(), field.Type); err != nil {
					t.Fatal(err)
				}
				for _, name := range field.Names {
					fields[name.Name] = rendered.String()
				}
			}
			return fields
		}
	}
	t.Fatalf("could not find struct %s in %s", typeName, path)
	return nil
}

func parsedMethods(t *testing.T, paths ...string) map[string]bool {
	t.Helper()
	methods := map[string]bool{}
	for _, path := range paths {
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv == nil || len(fn.Recv.List) != 1 {
				continue
			}
			receiver := ""
			switch value := fn.Recv.List[0].Type.(type) {
			case *ast.StarExpr:
				if ident, ok := value.X.(*ast.Ident); ok {
					receiver = ident.Name
				}
			case *ast.Ident:
				receiver = value.Name
			}
			if receiver == "Service" {
				methods[fn.Name.Name] = true
			}
		}
	}
	return methods
}

// Beta.18 keeps only process orchestration, transport adapters, immutable paths,
// and lazy service references in app. Bounded domain/session state must not
// drift back into package main after the final contraction.
func TestCoreStateRetainsOnlyOrchestrationAndServiceReferences(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("could not locate core-contraction test")
	}
	serverRoot := filepath.Dir(thisFile)
	projectRoot := filepath.Clean(filepath.Join(serverRoot, "..", ".."))
	mainPath := filepath.Join(serverRoot, "main.go")
	mainFields := parsedStructFields(t, mainPath, "app")
	for _, retired := range []string{"sessions", "oneShots", "failTimes", "mu", "controlEnv"} {
		if _, ok := mainFields[retired]; ok {
			t.Fatalf("core app retained migrated auth state: %s", retired)
		}
	}
	for field, wantType := range map[string]string{
		"authInitMu":           "sync.Mutex",
		"auth":                 "*controlauth.Service",
		"updateMu":             "sync.Mutex",
		"updateAvailabilityMu": "sync.Mutex",
		"todoStreamMu":         "sync.Mutex",
		"todoStreams":          "map[chan []byte]bool",
	} {
		if got := mainFields[field]; got != wantType {
			t.Fatalf("core app lost required orchestration/transport field %s: got %q, want %q", field, got, wantType)
		}
	}
	authServicePath := filepath.Join(projectRoot, "internal", "auth", "service.go")
	authSessionsPath := filepath.Join(projectRoot, "internal", "auth", "sessions.go")
	authService, err := os.ReadFile(authServicePath)
	if err != nil {
		t.Fatal(err)
	}
	authSessions, err := os.ReadFile(authSessionsPath)
	if err != nil {
		t.Fatal(err)
	}
	authFacade, err := os.ReadFile(filepath.Join(serverRoot, "auth_facade.go"))
	if err != nil {
		t.Fatal(err)
	}
	for _, source := range []string{string(authService), string(authSessions)} {
		if strings.Contains(source, "package main") || strings.Contains(source, "cmd/dashboard-control-server") || strings.Contains(source, "*app") {
			t.Fatal("internal/auth must remain independent of core application state")
		}
	}
	authFields := parsedStructFields(t, authServicePath, "Service")
	for field, wantType := range map[string]string{
		"sessions":  "map[string]sessionMeta",
		"oneShots":  "map[string]oneShotMeta",
		"failTimes": "[]time.Time",
	} {
		if got := authFields[field]; got != wantType {
			t.Fatalf("auth service lost runtime ownership evidence for %s: got %q, want %q", field, got, wantType)
		}
	}
	methods := parsedMethods(t, authServicePath, authSessionsPath)
	for _, method := range []string{"IssueToken", "ConsumeOneShot"} {
		if !methods[method] {
			t.Fatalf("auth service lost runtime ownership method: %s", method)
		}
	}
	if !strings.Contains(string(authFacade), "func (a *app) authService() *controlauth.Service") {
		t.Fatal("core auth facade must construct the bounded service")
	}
	for _, retired := range []string{"auth_pin.go", "auth_session.go"} {
		if _, err := os.Stat(filepath.Join(serverRoot, retired)); !os.IsNotExist(err) {
			t.Fatalf("retired core auth implementation returned: %s", retired)
		}
	}
}
