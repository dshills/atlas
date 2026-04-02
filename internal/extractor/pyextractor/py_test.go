package pyextractor

import (
	"context"
	"strings"
	"testing"

	"github.com/dshills/atlas/internal/extractor"
)

func TestExtractPythonBasic(t *testing.T) {
	ext := New()
	content := `import os
from pathlib import Path
from typing import Optional

MAX_RETRIES = 3
DEFAULT_TIMEOUT = 30

class UserService:
    def __init__(self, db):
        self.db = db

    def get_user(self, user_id: int) -> Optional[dict]:
        return self.db.find(user_id)

    async def delete_user(self, user_id: int) -> None:
        await self.db.delete(user_id)

    def _internal_method(self):
        pass

def validate_email(email: str) -> bool:
    return "@" in email

async def fetch_data(url: str) -> dict:
    pass

_private_helper = lambda x: x * 2
`

	req := extractor.ExtractRequest{
		FilePath: "src/services/user.py",
		Content:  []byte(content),
	}

	result, err := ext.Extract(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}

	// Check imports
	importCount := 0
	for _, ref := range result.References {
		if ref.ReferenceKind == "imports" {
			importCount++
		}
	}
	if importCount != 3 {
		t.Errorf("expected 3 imports, got %d", importCount)
	}

	// Check symbols by kind
	kinds := make(map[string]int)
	for _, sym := range result.Symbols {
		kinds[sym.SymbolKind]++
	}

	if kinds["class"] != 1 {
		t.Errorf("expected 1 class, got %d", kinds["class"])
	}
	if kinds["function"] < 2 {
		t.Errorf("expected at least 2 functions, got %d", kinds["function"])
	}
	if kinds["method"] < 3 {
		t.Errorf("expected at least 3 methods, got %d", kinds["method"])
	}
	if kinds["const"] < 2 {
		t.Errorf("expected at least 2 consts, got %d", kinds["const"])
	}

	// Check qualified name format
	for _, sym := range result.Symbols {
		if sym.SymbolKind == "decorator" {
			continue
		}
		if !strings.Contains(sym.QualifiedName, "src.services.user.") {
			t.Errorf("unexpected qualified name format: %s", sym.QualifiedName)
		}
	}

	// Check visibility
	foundExported := false
	foundUnexported := false
	for _, sym := range result.Symbols {
		if sym.Visibility == "exported" {
			foundExported = true
		} else {
			foundUnexported = true
		}
	}
	if !foundExported {
		t.Error("expected exported symbols")
	}
	if !foundUnexported {
		t.Error("expected unexported symbols")
	}

	// Check stable ID format
	for _, sym := range result.Symbols {
		if !strings.HasPrefix(sym.StableID, "python:") {
			t.Errorf("expected stable ID to start with python:, got %s", sym.StableID)
		}
	}
}

func TestExtractPythonTestFile(t *testing.T) {
	ext := New()
	content := `import pytest
from user import validate_email

def test_validate_email_valid():
    assert validate_email("test@example.com")

def test_validate_email_invalid():
    assert not validate_email("invalid")

class TestUserService:
    def test_get_user(self):
        pass

    def test_delete_user(self):
        pass

def helper_function():
    pass
`

	req := extractor.ExtractRequest{
		FilePath: "tests/test_user.py",
		Content:  []byte(content),
	}

	result, err := ext.Extract(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}

	testCount := 0
	funcCount := 0
	for _, sym := range result.Symbols {
		if sym.SymbolKind == "test" {
			testCount++
		}
		if sym.SymbolKind == "function" {
			funcCount++
		}
	}
	if testCount < 2 {
		t.Errorf("expected at least 2 test symbols, got %d", testCount)
	}
	if funcCount < 1 {
		t.Errorf("expected at least 1 non-test function, got %d", funcCount)
	}
}

func TestExtractPythonDecorators(t *testing.T) {
	ext := New()
	content := `from flask import Flask

app = Flask(__name__)

@app.route("/api/users")
def list_users():
    return []

@app.route("/api/users/<id>")
def get_user(id):
    return {}
`

	req := extractor.ExtractRequest{
		FilePath: "routes.py",
		Content:  []byte(content),
	}

	result, err := ext.Extract(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}

	decoratorCount := 0
	for _, sym := range result.Symbols {
		if sym.SymbolKind == "decorator" {
			decoratorCount++
		}
	}
	if decoratorCount < 2 {
		t.Errorf("expected at least 2 decorators, got %d", decoratorCount)
	}
}

func TestPythonSupports(t *testing.T) {
	ext := New()
	cases := map[string]bool{
		"file.py":     true,
		"file.pyi":    true,
		"file.go":     false,
		"file.ts":     false,
		"file.rs":     false,
		"FILE.PY":     true,
		"__init__.py": true,
	}
	for path, want := range cases {
		if got := ext.Supports(path); got != want {
			t.Errorf("Supports(%q) = %v, want %v", path, got, want)
		}
	}
}

func TestPythonLanguage(t *testing.T) {
	ext := New()
	if ext.Language() != "python" {
		t.Errorf("expected language python, got %s", ext.Language())
	}
}

func TestPythonPackageInfo(t *testing.T) {
	ext := New()
	req := extractor.ExtractRequest{
		FilePath: "mypackage/utils.py",
		Content:  []byte("x = 1\n"),
	}

	result, err := ext.Extract(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}

	if result.Package.Language != "python" {
		t.Errorf("expected python language, got %s", result.Package.Language)
	}
	if result.Package.Name != "utils" {
		t.Errorf("expected package name utils, got %s", result.Package.Name)
	}
}
