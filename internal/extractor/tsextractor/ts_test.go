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

func TestExtract_FullPipeline(t *testing.T) {
	ext := New()
	content := `import { Router } from 'express';

const router = Router();

// This is a comment: app.get('/hidden', handler)

router.get('/users', listUsers);
router.post('/users', createUser);

const dbUrl = process.env.DATABASE_URL;
const apiKey = process.env['API_KEY'];

const query = ` + "`SELECT * FROM users WHERE active = true`" + `;

fetch('https://api.example.com/data');

new Worker('worker.js');

function listUsers() {
  processData(items);
  service.getData();
}

describe('listUsers', () => {
  it('should list users', () => {});
});
`

	req := extractor.ExtractRequest{
		FilePath: "src/routes/users.test.ts",
		Content:  []byte(content),
	}

	result, err := ext.Extract(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}

	// Count references by kind
	refKinds := make(map[string]int)
	for _, ref := range result.References {
		refKinds[ref.ReferenceKind]++
	}

	// Count artifacts by kind
	artKinds := make(map[string]int)
	for _, art := range result.Artifacts {
		artKinds[art.ArtifactKind]++
	}

	// Assert at least one reference of each expected kind
	expectedRefKinds := []string{
		"imports",
		"registers_route",
		"uses_config",
		"touches_table",
		"invokes_external_api",
		"calls",
		"tests",
	}
	for _, kind := range expectedRefKinds {
		if refKinds[kind] < 1 {
			t.Errorf("expected at least 1 reference of kind %q, got %d", kind, refKinds[kind])
		}
	}

	// Assert at least one artifact of each expected kind
	expectedArtKinds := []string{
		"route",
		"env_var",
		"sql_query",
		"external_service",
		"background_job",
	}
	for _, kind := range expectedArtKinds {
		if artKinds[kind] < 1 {
			t.Errorf("expected at least 1 artifact of kind %q, got %d", kind, artKinds[kind])
		}
	}

	// Assert comment line did NOT produce references
	// The comment is on line 5: "// This is a comment: app.get('/hidden', handler)"
	for _, ref := range result.References {
		if ref.ReferenceKind == "registers_route" && strings.Contains(ref.RawTargetText, "/hidden") {
			t.Error("comment line should not produce a registers_route reference for /hidden")
		}
	}
	for _, art := range result.Artifacts {
		if art.ArtifactKind == "route" && strings.Contains(art.Name, "/hidden") {
			t.Error("comment line should not produce a route artifact for /hidden")
		}
	}

	// Assert all artifacts have non-empty DataJSON
	for _, art := range result.Artifacts {
		if art.DataJSON == "" {
			t.Errorf("artifact %q (kind %s) has empty DataJSON", art.Name, art.ArtifactKind)
		}
	}

	// Assert all references have non-empty ReferenceKind and Confidence
	for _, ref := range result.References {
		if ref.ReferenceKind == "" {
			t.Errorf("reference at line %d has empty ReferenceKind", ref.Line)
		}
		if ref.Confidence == "" {
			t.Errorf("reference %q at line %d has empty Confidence", ref.ReferenceKind, ref.Line)
		}
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
