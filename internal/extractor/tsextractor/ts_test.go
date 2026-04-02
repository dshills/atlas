package tsextractor

import (
	"context"
	"strings"
	"testing"

	"github.com/dshills/atlas/internal/extractor"
)

func TestExtractTypeScriptBasic(t *testing.T) {
	ext := New()
	content := `import { useState } from 'react'
import axios from 'axios'

export interface UserProps {
  name: string
  age: number
}

export type ID = string | number

export class UserService {
  async getUser(id: string): Promise<User> {
    return await axios.get('/api/user/' + id)
  }

  deleteUser(id: string): void {
    axios.delete('/api/user/' + id)
  }
}

export function validateEmail(email: string): boolean {
  return email.includes('@')
}

export const MAX_RETRIES = 3

const helperFn = (x: number) => x * 2
`

	req := extractor.ExtractRequest{
		FilePath: "src/services/user.ts",
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
	if importCount != 2 {
		t.Errorf("expected 2 imports, got %d", importCount)
	}

	// Check symbols by kind
	kinds := make(map[string]int)
	for _, sym := range result.Symbols {
		kinds[sym.SymbolKind]++
	}

	if kinds["interface"] != 1 {
		t.Errorf("expected 1 interface, got %d", kinds["interface"])
	}
	if kinds["type"] != 1 {
		t.Errorf("expected 1 type, got %d", kinds["type"])
	}
	if kinds["class"] != 1 {
		t.Errorf("expected 1 class, got %d", kinds["class"])
	}
	if kinds["function"] < 1 {
		t.Errorf("expected at least 1 function, got %d", kinds["function"])
	}
	if kinds["const"] < 1 {
		t.Errorf("expected at least 1 const, got %d", kinds["const"])
	}

	// Check qualified name format
	for _, sym := range result.Symbols {
		if !strings.Contains(sym.QualifiedName, "src/services/user.") {
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
}

func TestExtractTestFile(t *testing.T) {
	ext := New()
	content := `import { validateEmail } from './user'

describe('validateEmail', () => {
  it('should return true for valid email', () => {
    expect(validateEmail('test@example.com')).toBe(true)
  })

  it('should return false for invalid email', () => {
    expect(validateEmail('invalid')).toBe(false)
  })
})

test('standalone test', () => {
  expect(1 + 1).toBe(2)
})
`

	req := extractor.ExtractRequest{
		FilePath: "src/services/user.test.ts",
		Content:  []byte(content),
	}

	result, err := ext.Extract(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}

	testCount := 0
	for _, sym := range result.Symbols {
		if sym.SymbolKind == "test" {
			testCount++
		}
	}
	if testCount < 3 {
		t.Errorf("expected at least 3 test symbols, got %d", testCount)
	}
}

func TestExtractJavaScript(t *testing.T) {
	ext := New()
	content := `const express = require('express')

function handleRequest(req, res) {
  res.json({ ok: true })
}

module.exports = { handleRequest }
`

	req := extractor.ExtractRequest{
		FilePath: "src/handler.js",
		Content:  []byte(content),
	}

	result, err := ext.Extract(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}

	if result.Package.Language != "javascript" {
		t.Errorf("expected javascript language, got %s", result.Package.Language)
	}

	foundFunc := false
	for _, sym := range result.Symbols {
		if sym.Name == "handleRequest" && sym.SymbolKind == "function" {
			foundFunc = true
		}
	}
	if !foundFunc {
		t.Error("expected handleRequest function symbol")
	}
}

func TestSupports(t *testing.T) {
	ext := New()
	cases := map[string]bool{
		"file.ts":   true,
		"file.tsx":  true,
		"file.js":   true,
		"file.jsx":  true,
		"file.go":   false,
		"file.py":   false,
		"file.d.ts": true,
		"FILE.TS":   true,
	}
	for path, want := range cases {
		if got := ext.Supports(path); got != want {
			t.Errorf("Supports(%q) = %v, want %v", path, got, want)
		}
	}
}
