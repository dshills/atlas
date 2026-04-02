package javaextractor

import (
	"context"
	"strings"
	"testing"

	"github.com/dshills/atlas/internal/extractor"
)

func TestExtractJavaBasic(t *testing.T) {
	ext := New()
	content := `package com.example.app;

import java.util.List;
import java.util.Map;
import static java.util.Collections.emptyList;

public class UserService {
    public static final int MAX_RETRIES = 3;
    public static final String DEFAULT_NAME = "unknown";

    public void createUser(String name) {
        // implementation
    }

    private boolean validateEmail(String email) {
        return email.contains("@");
    }
}

public interface Repository {
    void save(Object entity);
    Object findById(String id);
}

public enum Status {
    ACTIVE,
    INACTIVE,
    SUSPENDED;
}
`

	req := extractor.ExtractRequest{
		FilePath: "src/main/java/com/example/app/UserService.java",
		Content:  []byte(content),
	}

	result, err := ext.Extract(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}

	// Check package
	if result.Package.Name != "app" {
		t.Errorf("expected package name 'app', got %q", result.Package.Name)
	}
	if result.Package.ImportPath != "com.example.app" {
		t.Errorf("expected import path 'com.example.app', got %q", result.Package.ImportPath)
	}
	if result.Package.Language != "java" {
		t.Errorf("expected language 'java', got %q", result.Package.Language)
	}

	// Check imports (3: java.util.List, java.util.Map, java.util.Collections.emptyList)
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
	if kinds["interface"] != 1 {
		t.Errorf("expected 1 interface, got %d", kinds["interface"])
	}
	if kinds["enum"] != 1 {
		t.Errorf("expected 1 enum, got %d", kinds["enum"])
	}
	if kinds["method"] < 2 {
		t.Errorf("expected at least 2 methods, got %d", kinds["method"])
	}
	if kinds["const"] < 2 {
		t.Errorf("expected at least 2 constants, got %d", kinds["const"])
	}

	// Check stable ID format
	for _, sym := range result.Symbols {
		if !strings.HasPrefix(sym.StableID, "java:") {
			t.Errorf("expected stable ID to start with java:, got %s", sym.StableID)
		}
	}

	// Check file parse status
	if result.File.ParseStatus != "ok" {
		t.Errorf("expected parse status 'ok', got %q", result.File.ParseStatus)
	}
}

func TestExtractJavaTestDetection_JUnit(t *testing.T) {
	ext := New()
	content := `package com.example.test;

import org.junit.jupiter.api.Test;
import org.junit.jupiter.params.ParameterizedTest;

public class UserServiceTest {
    @Test
    public void shouldCreateUser() {
        // test code
    }

    @ParameterizedTest
    public void shouldValidateEmail() {
        // test code
    }

    public void helperMethod() {
        // not a test
    }
}
`

	req := extractor.ExtractRequest{
		FilePath: "src/test/java/com/example/test/UserServiceTest.java",
		Content:  []byte(content),
	}

	result, err := ext.Extract(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}

	testCount := 0
	methodCount := 0
	for _, sym := range result.Symbols {
		switch sym.SymbolKind {
		case "test":
			testCount++
		case "method":
			methodCount++
		}
	}

	if testCount != 2 {
		t.Errorf("expected 2 test symbols, got %d", testCount)
		for _, sym := range result.Symbols {
			t.Logf("  symbol: %s kind=%s line=%d", sym.Name, sym.SymbolKind, sym.StartLine)
		}
	}
	if methodCount != 1 {
		t.Errorf("expected 1 non-test method, got %d", methodCount)
	}
}

func TestExtractJavaTestDetection_TestCase(t *testing.T) {
	ext := New()
	content := `package com.example.test;

import junit.framework.TestCase;

public class LegacyTest extends TestCase {
    public void testAddition() {
        assertEquals(3, 1 + 2);
    }

    public void testSubtraction() {
        assertEquals(1, 3 - 2);
    }

    public void helperSetup() {
        // not a test - doesn't start with "test"
    }
}
`

	req := extractor.ExtractRequest{
		FilePath: "src/test/java/com/example/test/LegacyTest.java",
		Content:  []byte(content),
	}

	result, err := ext.Extract(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}

	testCount := 0
	methodCount := 0
	for _, sym := range result.Symbols {
		switch sym.SymbolKind {
		case "test":
			testCount++
		case "method":
			methodCount++
		}
	}

	if testCount != 2 {
		t.Errorf("expected 2 test symbols (testAddition, testSubtraction), got %d", testCount)
		for _, sym := range result.Symbols {
			t.Logf("  symbol: %s kind=%s line=%d", sym.Name, sym.SymbolKind, sym.StartLine)
		}
	}
	if methodCount != 1 {
		t.Errorf("expected 1 non-test method (helperSetup), got %d", methodCount)
	}
}

func TestExtractJavaVisibility(t *testing.T) {
	ext := New()
	content := `package com.example;

public class Example {
    public void publicMethod() {}
    private void privateMethod() {}
    protected void protectedMethod() {}
    void packageMethod() {}
}
`

	req := extractor.ExtractRequest{
		FilePath: "src/main/java/com/example/Example.java",
		Content:  []byte(content),
	}

	result, err := ext.Extract(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}

	exported := 0
	unexported := 0
	for _, sym := range result.Symbols {
		if sym.SymbolKind != "method" {
			continue
		}
		switch sym.Visibility {
		case "exported":
			exported++
		case "unexported":
			unexported++
		}
	}

	if exported != 1 {
		t.Errorf("expected 1 exported method, got %d", exported)
	}
	if unexported != 3 {
		t.Errorf("expected 3 unexported methods, got %d", unexported)
	}
}

func TestExtractJavaQualifiedNames(t *testing.T) {
	ext := New()
	content := `package com.example.service;

public class OrderService {
    public void placeOrder() {}
    private void validate() {}
}
`

	req := extractor.ExtractRequest{
		FilePath: "src/main/java/com/example/service/OrderService.java",
		Content:  []byte(content),
	}

	result, err := ext.Extract(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}

	expectedNames := map[string]bool{
		"com.example.service.OrderService":            false,
		"com.example.service.OrderService.placeOrder": false,
		"com.example.service.OrderService.validate":   false,
	}

	for _, sym := range result.Symbols {
		if _, ok := expectedNames[sym.QualifiedName]; ok {
			expectedNames[sym.QualifiedName] = true
		}
	}

	for name, found := range expectedNames {
		if !found {
			t.Errorf("expected qualified name %q not found", name)
		}
	}

	// All qualified names should contain the package prefix
	for _, sym := range result.Symbols {
		if !strings.HasPrefix(sym.QualifiedName, "com.example.service.") {
			t.Errorf("unexpected qualified name format: %s", sym.QualifiedName)
		}
	}
}

func TestExtractJavaAnnotationType(t *testing.T) {
	ext := New()
	content := `package com.example.annotations;

public @interface Cacheable {
    String value() default "";
    int ttl() default 300;
}

@interface InternalMarker {
}
`

	req := extractor.ExtractRequest{
		FilePath: "src/main/java/com/example/annotations/Cacheable.java",
		Content:  []byte(content),
	}

	result, err := ext.Extract(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}

	annotationCount := 0
	for _, sym := range result.Symbols {
		if sym.SymbolKind == "annotation" {
			annotationCount++
			if sym.Name != "Cacheable" && sym.Name != "InternalMarker" {
				t.Errorf("unexpected annotation name: %s", sym.Name)
			}
		}
	}

	if annotationCount != 2 {
		t.Errorf("expected 2 annotation types, got %d", annotationCount)
		for _, sym := range result.Symbols {
			t.Logf("  symbol: %s kind=%s", sym.Name, sym.SymbolKind)
		}
	}

	// Check visibility
	for _, sym := range result.Symbols {
		if sym.SymbolKind == "annotation" {
			if sym.Name == "Cacheable" && sym.Visibility != "exported" {
				t.Errorf("expected Cacheable to be exported, got %s", sym.Visibility)
			}
			if sym.Name == "InternalMarker" && sym.Visibility != "unexported" {
				t.Errorf("expected InternalMarker to be unexported, got %s", sym.Visibility)
			}
		}
	}
}

func TestExtractJavaGenericsInMethods(t *testing.T) {
	ext := New()
	content := `package com.example.util;

public class CollectionUtils {
    public <T> List<T> getItems() {
        return new ArrayList<>();
    }

    public <K, V> Map<K, V> createMap(List<K> keys, List<V> values) {
        return new HashMap<>();
    }

    public Optional<String> findByName(String name) {
        return Optional.empty();
    }

    public String[] toArray(List<String> list) {
        return list.toArray(new String[0]);
    }
}
`

	req := extractor.ExtractRequest{
		FilePath: "src/main/java/com/example/util/CollectionUtils.java",
		Content:  []byte(content),
	}

	result, err := ext.Extract(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}

	methodCount := 0
	methodNames := make(map[string]bool)
	for _, sym := range result.Symbols {
		if sym.SymbolKind == "method" {
			methodCount++
			methodNames[sym.Name] = true
		}
	}

	if methodCount != 4 {
		t.Errorf("expected 4 methods, got %d", methodCount)
		for _, sym := range result.Symbols {
			t.Logf("  symbol: %s kind=%s line=%d", sym.Name, sym.SymbolKind, sym.StartLine)
		}
	}

	for _, expected := range []string{"getItems", "createMap", "findByName", "toArray"} {
		if !methodNames[expected] {
			t.Errorf("expected method %q not found", expected)
		}
	}
}

func TestExtract_FullPipeline(t *testing.T) {
	ext := New()
	content := `package com.example.service;

import java.util.List;
import java.util.Optional;
import javax.persistence.Entity;
import static org.junit.jupiter.api.Assertions.assertEquals;

public class UserService {
    public static final int MAX_USERS = 1000;
    public static final String SERVICE_NAME = "user-service";

    public List<User> findAll() {
        return null;
    }

    public Optional<User> findById(String id) {
        return Optional.empty();
    }

    private void validateUser(User user) {
        // validation logic
    }
}

public interface UserRepository {
    void save(User user);
    User findById(String id);
    List<User> findAll();
}

public enum UserRole {
    ADMIN,
    USER,
    GUEST;
}

public @interface Secured {
    String[] roles() default {};
}
`

	req := extractor.ExtractRequest{
		FilePath: "src/main/java/com/example/service/UserService.java",
		Content:  []byte(content),
	}

	result, err := ext.Extract(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}

	// Verify file record
	if result.File == nil || result.File.ParseStatus != "ok" {
		t.Fatal("expected file record with parse status 'ok'")
	}

	// Verify package record
	if result.Package == nil {
		t.Fatal("expected package record")
	}
	if result.Package.ImportPath != "com.example.service" {
		t.Errorf("expected import path 'com.example.service', got %q", result.Package.ImportPath)
	}

	// Count imports
	importCount := 0
	for _, ref := range result.References {
		if ref.ReferenceKind == "imports" {
			importCount++
			if ref.Confidence != "exact" {
				t.Errorf("expected import confidence 'exact', got %q", ref.Confidence)
			}
			if ref.RawTargetText == "" {
				t.Errorf("expected non-empty RawTargetText for import at line %d", ref.Line)
			}
		}
	}
	if importCount != 4 {
		t.Errorf("expected 4 imports, got %d", importCount)
	}

	// Count symbols by kind
	kinds := make(map[string]int)
	for _, sym := range result.Symbols {
		kinds[sym.SymbolKind]++
	}

	if kinds["class"] != 1 {
		t.Errorf("expected 1 class, got %d", kinds["class"])
	}
	if kinds["interface"] != 1 {
		t.Errorf("expected 1 interface, got %d", kinds["interface"])
	}
	if kinds["enum"] != 1 {
		t.Errorf("expected 1 enum, got %d", kinds["enum"])
	}
	if kinds["annotation"] != 1 {
		t.Errorf("expected 1 annotation, got %d", kinds["annotation"])
	}
	if kinds["method"] < 3 {
		t.Errorf("expected at least 3 methods, got %d", kinds["method"])
	}
	if kinds["const"] < 2 {
		t.Errorf("expected at least 2 constants, got %d", kinds["const"])
	}

	// Verify all symbols have stable IDs starting with java:
	for _, sym := range result.Symbols {
		if !strings.HasPrefix(sym.StableID, "java:") {
			t.Errorf("expected stable ID to start with java:, got %s", sym.StableID)
		}
	}

	// Verify all symbols have qualified names with package prefix
	for _, sym := range result.Symbols {
		if !strings.HasPrefix(sym.QualifiedName, "com.example.service.") {
			t.Errorf("expected qualified name to start with com.example.service., got %s", sym.QualifiedName)
		}
	}

	// Verify methods have parent symbol IDs
	for _, sym := range result.Symbols {
		if sym.SymbolKind == "method" || sym.SymbolKind == "const" {
			if sym.ParentSymbolID == "" {
				t.Errorf("expected parent symbol ID for %s %q", sym.SymbolKind, sym.Name)
			}
		}
	}

	// Verify StartLine and EndLine are set
	for _, sym := range result.Symbols {
		if sym.StartLine == 0 {
			t.Errorf("symbol %q has StartLine 0", sym.Name)
		}
		if sym.EndLine == 0 {
			t.Errorf("symbol %q has EndLine 0", sym.Name)
		}
	}
}

func TestJavaSupports(t *testing.T) {
	ext := New()
	cases := map[string]bool{
		"File.java":       true,
		"Main.java":       true,
		"Test.JAVA":       true,
		"file.go":         false,
		"file.py":         false,
		"file.class":      false,
		"file.jar":        false,
		"file.ts":         false,
		"Makefile":        false,
		"pom.xml":         false,
		"MyClass.java.bk": false,
	}
	for path, want := range cases {
		if got := ext.Supports(path); got != want {
			t.Errorf("Supports(%q) = %v, want %v", path, got, want)
		}
	}
}

func TestJavaLanguage(t *testing.T) {
	ext := New()
	if ext.Language() != "java" {
		t.Errorf("expected language java, got %s", ext.Language())
	}
}

func TestJavaSupportedKinds(t *testing.T) {
	ext := New()
	kinds := ext.SupportedKinds()
	expected := map[string]bool{
		"package": true, "class": true, "interface": true, "enum": true,
		"method": true, "field": true, "const": true, "annotation": true, "test": true,
	}
	for _, k := range kinds {
		if !expected[k] {
			t.Errorf("unexpected kind: %s", k)
		}
		delete(expected, k)
	}
	for k := range expected {
		t.Errorf("missing expected kind: %s", k)
	}
}

func TestJavaPackageFallback(t *testing.T) {
	ext := New()
	// No package declaration — should fallback to file path
	content := `public class Simple {
    public void doSomething() {}
}
`

	req := extractor.ExtractRequest{
		FilePath: "src/Simple.java",
		Content:  []byte(content),
	}

	result, err := ext.Extract(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}

	if result.Package.ImportPath != "src.Simple" {
		t.Errorf("expected fallback import path 'src.Simple', got %q", result.Package.ImportPath)
	}
}
