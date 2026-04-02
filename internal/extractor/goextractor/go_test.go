package goextractor

import (
	"context"
	"testing"

	"github.com/dshills/atlas/internal/extractor"
)

func TestExtractFunction(t *testing.T) {
	src := `package main

// Hello greets someone.
func Hello(name string) string {
	return "Hello, " + name
}
`
	result := extract(t, "main.go", src)
	assertSymbol(t, result, "Hello", "function", "exported", "main.Hello")
}

func TestExtractMethod(t *testing.T) {
	src := `package pkg

type Server struct{}

func (s *Server) Start() error {
	return nil
}
`
	result := extract(t, "pkg/server.go", src)
	assertSymbol(t, result, "Start", "method", "exported", "pkg.Server.Start")
	assertSymbol(t, result, "Server", "struct", "exported", "pkg.Server")
}

func TestExtractStruct(t *testing.T) {
	src := `package model

type User struct {
	Name  string
	Email string
}
`
	result := extract(t, "model/user.go", src)
	assertSymbol(t, result, "User", "struct", "exported", "model.User")
	assertSymbol(t, result, "Name", "field", "exported", "model.User.Name")
	assertSymbol(t, result, "Email", "field", "exported", "model.User.Email")
}

func TestExtractInterface(t *testing.T) {
	src := `package svc

type Service interface {
	Run() error
}
`
	result := extract(t, "svc/service.go", src)
	assertSymbol(t, result, "Service", "interface", "exported", "svc.Service")
}

func TestExtractConst(t *testing.T) {
	src := `package config

const MaxRetries = 3
`
	result := extract(t, "config/config.go", src)
	assertSymbol(t, result, "MaxRetries", "const", "exported", "config.MaxRetries")
}

func TestExtractVar(t *testing.T) {
	src := `package main

var defaultTimeout = 30
`
	result := extract(t, "main.go", src)
	assertSymbol(t, result, "defaultTimeout", "var", "unexported", "main.defaultTimeout")
}

func TestExtractTest(t *testing.T) {
	src := `package pkg

import "testing"

func TestFoo(t *testing.T) {}
`
	result := extract(t, "pkg/foo_test.go", src)
	assertSymbol(t, result, "TestFoo", "test", "exported", "pkg.TestFoo")
}

func TestExtractBenchmark(t *testing.T) {
	src := `package pkg

import "testing"

func BenchmarkFoo(b *testing.B) {}
`
	result := extract(t, "pkg/foo_test.go", src)
	assertSymbol(t, result, "BenchmarkFoo", "benchmark", "exported", "pkg.BenchmarkFoo")
}

func TestExtractEntrypoint(t *testing.T) {
	src := `package main

func main() {}
`
	result := extract(t, "cmd/server/main.go", src)
	assertSymbol(t, result, "main", "entrypoint", "unexported", "main.main")
}

func TestExtractImports(t *testing.T) {
	src := `package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Println(os.Args)
}
`
	result := extract(t, "main.go", src)
	if len(result.References) != 2 {
		t.Errorf("expected 2 import references, got %d", len(result.References))
	}
	for _, ref := range result.References {
		if ref.ReferenceKind != "imports" {
			t.Errorf("expected reference kind 'imports', got %q", ref.ReferenceKind)
		}
	}
}

func TestExtractParseError(t *testing.T) {
	src := `package main

func broken( {
}
`
	result := extract(t, "main.go", src)
	if result.File.ParseStatus != "error" {
		t.Errorf("expected parse status 'error', got %q", result.File.ParseStatus)
	}
	if len(result.Diagnostics) == 0 {
		t.Error("expected diagnostics for parse error")
	}
}

func TestStableID(t *testing.T) {
	src := `package main

func Hello() {}
`
	result := extract(t, "main.go", src)
	found := false
	for _, s := range result.Symbols {
		if s.Name == "Hello" {
			if s.StableID != "go:main.Hello:function" {
				t.Errorf("expected stable_id 'go:main.Hello:function', got %q", s.StableID)
			}
			found = true
		}
	}
	if !found {
		t.Error("Hello symbol not found")
	}
}

func TestVisibility(t *testing.T) {
	src := `package pkg

func Exported() {}
func unexported() {}
`
	result := extract(t, "pkg/file.go", src)
	for _, s := range result.Symbols {
		if s.Name == "Exported" && s.Visibility != "exported" {
			t.Errorf("expected Exported to be exported")
		}
		if s.Name == "unexported" && s.Visibility != "unexported" {
			t.Errorf("expected unexported to be unexported")
		}
	}
}

func TestTypeAlias(t *testing.T) {
	src := `package main

type Duration int64
`
	result := extract(t, "main.go", src)
	assertSymbol(t, result, "Duration", "type", "exported", "main.Duration")
}

func TestPackageRecord(t *testing.T) {
	src := `package handler`
	result := extract(t, "internal/handler/handler.go", src)
	if result.Package == nil {
		t.Fatal("expected package record")
	}
	if result.Package.Name != "handler" {
		t.Errorf("expected package name 'handler', got %q", result.Package.Name)
	}
	if result.Package.DirectoryPath != "internal/handler" {
		t.Errorf("expected dir path 'internal/handler', got %q", result.Package.DirectoryPath)
	}
}

func extract(t *testing.T, path string, src string) *extractor.ExtractResult {
	t.Helper()
	ext := New()
	req := extractor.ExtractRequest{
		FilePath: path,
		Content:  []byte(src),
	}
	result, err := ext.Extract(context.Background(), req)
	if err != nil {
		t.Fatalf("extraction failed: %v", err)
	}
	return result
}

func assertSymbol(t *testing.T, result *extractor.ExtractResult, name, kind, vis, qualName string) {
	t.Helper()
	for _, s := range result.Symbols {
		if s.Name == name {
			if s.SymbolKind != kind {
				t.Errorf("symbol %q: expected kind %q, got %q", name, kind, s.SymbolKind)
			}
			if s.Visibility != vis {
				t.Errorf("symbol %q: expected visibility %q, got %q", name, vis, s.Visibility)
			}
			if s.QualifiedName != qualName {
				t.Errorf("symbol %q: expected qualified name %q, got %q", name, qualName, s.QualifiedName)
			}
			return
		}
	}
	t.Errorf("symbol %q not found in results (have %d symbols)", name, len(result.Symbols))
}
