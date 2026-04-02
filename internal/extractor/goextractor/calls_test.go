package goextractor

import (
	"testing"
)

func TestExtractCalls(t *testing.T) {
	src := `package main

import "fmt"

func hello() string { return "hi" }

func main() {
	fmt.Println(hello())
}
`
	result := extract(t, "main.go", src)

	callCount := 0
	for _, ref := range result.References {
		if ref.ReferenceKind == "calls" {
			callCount++
		}
	}
	if callCount < 2 {
		t.Errorf("expected at least 2 call references, got %d", callCount)
	}
}

func TestExtractRoutes(t *testing.T) {
	src := `package main

import "net/http"

func handler(w http.ResponseWriter, r *http.Request) {}

func main() {
	http.HandleFunc("/api/users", handler)
}
`
	result := extract(t, "main.go", src)

	routeCount := 0
	for _, a := range result.Artifacts {
		if a.ArtifactKind == "route" {
			routeCount++
		}
	}
	if routeCount != 1 {
		t.Errorf("expected 1 route artifact, got %d", routeCount)
	}

	routeRefCount := 0
	for _, ref := range result.References {
		if ref.ReferenceKind == "registers_route" {
			routeRefCount++
		}
	}
	if routeRefCount != 1 {
		t.Errorf("expected 1 registers_route reference, got %d", routeRefCount)
	}
}

func TestExtractConfigAccess(t *testing.T) {
	src := `package main

import "os"

func main() {
	os.Getenv("DATABASE_URL")
}
`
	result := extract(t, "main.go", src)

	envCount := 0
	for _, a := range result.Artifacts {
		if a.ArtifactKind == "env_var" {
			envCount++
		}
	}
	if envCount != 1 {
		t.Errorf("expected 1 env_var artifact, got %d", envCount)
	}
}

func TestExtractImplements(t *testing.T) {
	src := `package svc

type Service interface {
	Run() error
	Stop()
}

type MyService struct{}

func (s *MyService) Run() error { return nil }
func (s *MyService) Stop() {}
`
	result := extract(t, "svc/service.go", src)

	implCount := 0
	for _, ref := range result.References {
		if ref.ReferenceKind == "implements" {
			implCount++
		}
	}
	if implCount != 1 {
		t.Errorf("expected 1 implements reference, got %d", implCount)
	}
}

func TestExtractSQLArtifacts(t *testing.T) {
	src := `package db

const createTable = "CREATE TABLE users (id INTEGER PRIMARY KEY)"
`
	result := extract(t, "db/db.go", src)

	sqlCount := 0
	for _, a := range result.Artifacts {
		if a.ArtifactKind == "sql_query" {
			sqlCount++
		}
	}
	if sqlCount != 1 {
		t.Errorf("expected 1 sql_query artifact, got %d", sqlCount)
	}
}

func TestExtractBackgroundJob(t *testing.T) {
	src := `package main

func worker() {}

func main() {
	go worker()
}
`
	result := extract(t, "main.go", src)

	jobCount := 0
	for _, a := range result.Artifacts {
		if a.ArtifactKind == "background_job" {
			jobCount++
		}
	}
	if jobCount != 1 {
		t.Errorf("expected 1 background_job artifact, got %d", jobCount)
	}
}

func TestExtractExternalService(t *testing.T) {
	src := `package main

import "net/http"

func fetchData() {
	http.Get("https://api.example.com/data")
}
`
	result := extract(t, "main.go", src)

	svcCount := 0
	for _, a := range result.Artifacts {
		if a.ArtifactKind == "external_service" {
			svcCount++
		}
	}
	if svcCount != 1 {
		t.Errorf("expected 1 external_service artifact, got %d", svcCount)
	}
}

func TestExtractTestReferences(t *testing.T) {
	src := `package pkg

import "testing"

func Foo() {}

func TestFoo(t *testing.T) {}
`
	result := extract(t, "pkg/foo_test.go", src)

	testRefCount := 0
	for _, ref := range result.References {
		if ref.ReferenceKind == "tests" {
			testRefCount++
		}
	}
	if testRefCount != 1 {
		t.Errorf("expected 1 tests reference, got %d", testRefCount)
	}
}
