package rustextractor

import (
	"context"
	"strings"
	"testing"

	"github.com/dshills/atlas/internal/extractor"
)

func TestExtractRustBasic(t *testing.T) {
	ext := New()
	content := `use std::collections::HashMap;
use serde::{Deserialize, Serialize};

pub const MAX_RETRIES: u32 = 3;
static GLOBAL_COUNT: u32 = 0;

pub struct User {
    pub name: String,
    age: u32,
}

pub enum Status {
    Active,
    Inactive,
    Suspended,
}

pub trait Repository {
    fn find(&self, id: u64) -> Option<User>;
    fn save(&mut self, user: User) -> Result<(), Error>;
}

impl User {
    pub fn new(name: String, age: u32) -> Self {
        Self { name, age }
    }

    pub fn greet(&self) -> String {
        format!("Hello, {}", self.name)
    }
}

impl Repository for UserRepo {
    fn find(&self, id: u64) -> Option<User> {
        None
    }

    fn save(&mut self, user: User) -> Result<(), Error> {
        Ok(())
    }
}

pub type UserId = u64;

pub fn validate_email(email: &str) -> bool {
    email.contains('@')
}

async fn fetch_data(url: &str) -> Result<String, Error> {
    Ok(String::new())
}

pub mod models;

macro_rules! debug_print {
    ($val:expr) => {
        println!("{:?}", $val);
    };
}
`

	req := extractor.ExtractRequest{
		FilePath: "src/user.rs",
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

	if kinds["struct"] != 1 {
		t.Errorf("expected 1 struct, got %d", kinds["struct"])
	}
	if kinds["enum"] != 1 {
		t.Errorf("expected 1 enum, got %d", kinds["enum"])
	}
	if kinds["trait"] != 1 {
		t.Errorf("expected 1 trait, got %d", kinds["trait"])
	}
	if kinds["function"] < 2 {
		t.Errorf("expected at least 2 functions, got %d", kinds["function"])
	}
	if kinds["method"] < 2 {
		t.Errorf("expected at least 2 methods, got %d", kinds["method"])
	}
	if kinds["const"] < 2 {
		t.Errorf("expected at least 2 consts, got %d", kinds["const"])
	}
	if kinds["type"] != 1 {
		t.Errorf("expected 1 type alias, got %d", kinds["type"])
	}
	if kinds["module"] != 1 {
		t.Errorf("expected 1 module, got %d", kinds["module"])
	}
	if kinds["macro"] != 1 {
		t.Errorf("expected 1 macro, got %d", kinds["macro"])
	}

	// Check qualified name format uses ::
	for _, sym := range result.Symbols {
		if !strings.Contains(sym.QualifiedName, "src::user::") {
			t.Errorf("unexpected qualified name format: %s", sym.QualifiedName)
		}
	}

	// Check stable ID format
	for _, sym := range result.Symbols {
		if !strings.HasPrefix(sym.StableID, "rust:") {
			t.Errorf("expected stable ID to start with rust:, got %s", sym.StableID)
		}
	}
}

func TestExtractRustTests(t *testing.T) {
	ext := New()
	content := `pub fn add(a: i32, b: i32) -> i32 {
    a + b
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_add() {
        assert_eq!(add(1, 2), 3);
    }

    #[test]
    fn test_add_negative() {
        assert_eq!(add(-1, 1), 0);
    }
}
`

	req := extractor.ExtractRequest{
		FilePath: "src/math.rs",
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
	if testCount < 2 {
		t.Errorf("expected at least 2 test symbols, got %d", testCount)
	}
}

func TestExtractRustVisibility(t *testing.T) {
	ext := New()
	content := `pub fn public_fn() {}
fn private_fn() {}
pub(crate) fn crate_fn() {}
pub struct PublicStruct {}
struct PrivateStruct {}
`

	req := extractor.ExtractRequest{
		FilePath: "src/lib.rs",
		Content:  []byte(content),
	}

	result, err := ext.Extract(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}

	exported := 0
	unexported := 0
	for _, sym := range result.Symbols {
		if sym.SymbolKind == "method" || sym.SymbolKind == "module" {
			continue
		}
		if sym.Visibility == "exported" {
			exported++
		} else {
			unexported++
		}
	}
	if exported < 2 {
		t.Errorf("expected at least 2 exported symbols, got %d", exported)
	}
	if unexported < 1 {
		t.Errorf("expected at least 1 unexported symbol, got %d", unexported)
	}
}

func TestExtract_FullPipeline(t *testing.T) {
	ext := New()
	content := `use reqwest;
use std::env;

// #[get("/hidden")]

#[get("/users")]
async fn list_users() -> impl Responder {
    let db_url = env::var("DATABASE_URL").unwrap();
    let query = r#"SELECT * FROM users WHERE active = true"#;
    let resp = reqwest::get("https://api.example.com").await?;
    tokio::spawn(async { process().await });
    process_data(items);
    service.get_data();
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_list_users() {
        assert_eq!(1, 1);
    }
}
`

	req := extractor.ExtractRequest{
		FilePath: "src/handlers.rs",
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
	// The comment is on line 4: "// #[get("/hidden")]"
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

func TestRustSupports(t *testing.T) {
	ext := New()
	cases := map[string]bool{
		"file.rs": true,
		"lib.rs":  true,
		"mod.rs":  true,
		"main.rs": true,
		"file.go": false,
		"file.py": false,
		"file.ts": false,
		"FILE.RS": true,
	}
	for path, want := range cases {
		if got := ext.Supports(path); got != want {
			t.Errorf("Supports(%q) = %v, want %v", path, got, want)
		}
	}
}

func TestRustLanguage(t *testing.T) {
	ext := New()
	if ext.Language() != "rust" {
		t.Errorf("expected language rust, got %s", ext.Language())
	}
}

func TestRustModuleName(t *testing.T) {
	cases := map[string]string{
		"src/lib.rs":         "src",
		"src/mod.rs":         "src",
		"src/main.rs":        "src",
		"src/models/user.rs": "src::models::user",
		"lib.rs":             "lib",
	}
	for path, want := range cases {
		got := deriveModuleName(path)
		if got != want {
			t.Errorf("deriveModuleName(%q) = %q, want %q", path, got, want)
		}
	}
}

func TestRustImplMethods(t *testing.T) {
	ext := New()
	content := `pub struct Point {
    x: f64,
    y: f64,
}

impl Point {
    pub fn new(x: f64, y: f64) -> Self {
        Self { x, y }
    }

    pub fn distance(&self, other: &Point) -> f64 {
        ((self.x - other.x).powi(2) + (self.y - other.y).powi(2)).sqrt()
    }
}
`

	req := extractor.ExtractRequest{
		FilePath: "src/point.rs",
		Content:  []byte(content),
	}

	result, err := ext.Extract(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}

	methodCount := 0
	for _, sym := range result.Symbols {
		if sym.SymbolKind == "method" {
			methodCount++
			if sym.ParentSymbolID != "src::point::Point" {
				t.Errorf("expected parent src::point::Point, got %s", sym.ParentSymbolID)
			}
		}
	}
	if methodCount < 2 {
		t.Errorf("expected at least 2 methods, got %d", methodCount)
	}
}
