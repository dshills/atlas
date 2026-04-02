package swiftextractor

import (
	"context"
	"testing"

	"github.com/dshills/atlas/internal/extractor"
)

func TestSwiftExtractor_Language(t *testing.T) {
	e := New()
	if e.Language() != "swift" {
		t.Errorf("expected language 'swift', got %q", e.Language())
	}
}

func TestSwiftExtractor_Supports(t *testing.T) {
	e := New()
	if !e.Supports("main.swift") {
		t.Error("expected Supports('main.swift') = true")
	}
	if !e.Supports("Sources/App.Swift") {
		t.Error("expected Supports('Sources/App.Swift') = true")
	}
	if e.Supports("main.go") {
		t.Error("expected Supports('main.go') = false")
	}
}

func TestSwiftExtractor_Extract_Basic(t *testing.T) {
	e := New()
	content := `import Foundation
import UIKit

class UserService {
    func fetchUser(id: Int) -> User {
        return User(id: id)
    }

    func testFetchUser() {
        let u = fetchUser(id: 1)
    }
}

struct User {
    let id: Int
    let name: String
}

protocol Fetchable {
    func fetch()
}

func globalHelper() -> Int {
    return 42
}
`
	result, err := e.Extract(context.Background(), extractor.ExtractRequest{
		FilePath: "Sources/UserService.swift",
		Content:  []byte(content),
	})
	if err != nil {
		t.Fatalf("Extract() error: %v", err)
	}

	if result.File.ParseStatus != "ok" {
		t.Errorf("expected parse status 'ok', got %q", result.File.ParseStatus)
	}
	if result.Package.Language != "swift" {
		t.Errorf("expected language 'swift', got %q", result.Package.Language)
	}

	// Check imports
	importCount := 0
	for _, r := range result.References {
		if r.ReferenceKind == "imports" {
			importCount++
		}
	}
	if importCount != 2 {
		t.Errorf("expected 2 imports, got %d", importCount)
	}

	// Check symbols
	symbolKinds := make(map[string]int)
	for _, s := range result.Symbols {
		symbolKinds[s.SymbolKind]++
	}
	if symbolKinds["class"] != 1 {
		t.Errorf("expected 1 class symbol, got %d", symbolKinds["class"])
	}
	if symbolKinds["struct"] != 1 {
		t.Errorf("expected 1 struct symbol, got %d", symbolKinds["struct"])
	}
	if symbolKinds["protocol"] != 1 {
		t.Errorf("expected 1 protocol symbol, got %d", symbolKinds["protocol"])
	}
	if symbolKinds["function"] != 1 {
		t.Errorf("expected 1 function symbol, got %d", symbolKinds["function"])
	}
}

func TestSwiftExtractor_Extract_WithRoutes(t *testing.T) {
	e := New()
	content := `import Vapor

func routes(_ app: Application) throws {
    app.get("users") { req in
        return "ok"
    }
    app.post("users") { req in
        return "created"
    }
}
`
	result, err := e.Extract(context.Background(), extractor.ExtractRequest{
		FilePath: "Sources/routes.swift",
		Content:  []byte(content),
	})
	if err != nil {
		t.Fatalf("Extract() error: %v", err)
	}

	routeCount := 0
	for _, a := range result.Artifacts {
		if a.ArtifactKind == "route" {
			routeCount++
		}
	}
	if routeCount != 2 {
		t.Errorf("expected 2 route artifacts, got %d", routeCount)
	}
}
