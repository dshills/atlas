package goextractor

import (
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/dshills/atlas/internal/extractor"
)

// GoExtractor implements extractor.Extractor for Go source files.
type GoExtractor struct{}

// New creates a new GoExtractor.
func New() *GoExtractor {
	return &GoExtractor{}
}

func (g *GoExtractor) Language() string { return "go" }

func (g *GoExtractor) Supports(path string) bool {
	return strings.HasSuffix(path, ".go")
}

func (g *GoExtractor) SupportedKinds() []string {
	return []string{
		"package", "function", "method", "struct", "interface",
		"type", "const", "var", "field", "test", "benchmark", "entrypoint",
	}
}

func (g *GoExtractor) Extract(ctx context.Context, req extractor.ExtractRequest) (*extractor.ExtractResult, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, req.FilePath, req.Content, parser.ParseComments)
	if err != nil {
		// Return partial results on parse error
		return &extractor.ExtractResult{
			File: &extractor.FileRecord{ParseStatus: "error"},
			Diagnostics: []extractor.DiagnosticRecord{
				{Severity: "error", Code: "PARSE_ERROR", Message: fmt.Sprintf("parse error: %v", err)},
			},
		}, nil
	}

	pkgName := file.Name.Name
	dirPath := filepath.Dir(req.FilePath)
	if dirPath == "." {
		dirPath = ""
	}

	result := &extractor.ExtractResult{
		File: &extractor.FileRecord{ParseStatus: "ok"},
		Package: &extractor.PackageRecord{
			Name:          pkgName,
			ImportPath:    buildImportPath(req.ModulePath, dirPath),
			DirectoryPath: dirPath,
			Language:      "go",
		},
	}

	// Extract imports
	for _, imp := range file.Imports {
		importPath := strings.Trim(imp.Path.Value, `"`)
		result.References = append(result.References, extractor.ReferenceRecord{
			ReferenceKind: "imports",
			Confidence:    "exact",
			RawTargetText: importPath,
			Line:          fset.Position(imp.Pos()).Line,
		})
	}

	// Check if this is an entrypoint
	isCmd := strings.Contains(req.FilePath, "cmd/")

	// Extract declarations
	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			g.extractFunc(fset, d, pkgName, isCmd, result)
		case *ast.GenDecl:
			g.extractGenDecl(fset, d, pkgName, result)
		}
	}

	// Extract calls
	callRefs := extractCalls(fset, file, pkgName)
	result.References = append(result.References, callRefs...)

	// Extract interface implementations
	implRefs := extractImplements(fset, file, pkgName)
	result.References = append(result.References, implRefs...)

	// Extract routes
	routeArtifacts, routeRefs := extractRoutes(fset, file)
	result.Artifacts = append(result.Artifacts, routeArtifacts...)
	result.References = append(result.References, routeRefs...)

	// Extract config/env access
	configArtifacts, configRefs := extractConfigAccess(fset, file)
	result.Artifacts = append(result.Artifacts, configArtifacts...)
	result.References = append(result.References, configRefs...)

	// Extract SQL artifacts
	sqlArtifacts, sqlRefs := extractSQLArtifacts(fset, file, req.FilePath)
	result.Artifacts = append(result.Artifacts, sqlArtifacts...)
	result.References = append(result.References, sqlRefs...)

	// Extract background jobs and external services
	jobArtifacts, jobRefs := extractJobsAndServices(fset, file)
	result.Artifacts = append(result.Artifacts, jobArtifacts...)
	result.References = append(result.References, jobRefs...)

	// Extract test references
	testRefs := extractTestReferences(result.Symbols, pkgName)
	result.References = append(result.References, testRefs...)

	return result, nil
}

func (g *GoExtractor) extractFunc(fset *token.FileSet, d *ast.FuncDecl, pkgName string, isCmd bool, result *extractor.ExtractResult) {
	name := d.Name.Name
	startLine := fset.Position(d.Pos()).Line
	endLine := fset.Position(d.End()).Line

	var kind, qualName, sig string
	if d.Recv != nil && len(d.Recv.List) > 0 {
		// Method
		kind = "method"
		recvType := receiverTypeName(d.Recv.List[0].Type)
		qualName = fmt.Sprintf("%s.%s.%s", pkgName, recvType, name)
		sig = formatFuncSignature(d, recvType)
	} else {
		kind = "function"
		qualName = fmt.Sprintf("%s.%s", pkgName, name)
		sig = formatFuncSignature(d, "")

		// Check for test/benchmark/entrypoint
		if strings.HasPrefix(name, "Test") && hasTestParam(d, "T") {
			kind = "test"
		} else if strings.HasPrefix(name, "Benchmark") && hasTestParam(d, "B") {
			kind = "benchmark"
		} else if name == "main" && pkgName == "main" && isCmd {
			kind = "entrypoint"
		}
	}

	stableID := fmt.Sprintf("go:%s:%s", qualName, kind)
	vis := visibility(name)

	doc := ""
	if d.Doc != nil {
		doc = d.Doc.Text()
	}

	result.Symbols = append(result.Symbols, extractor.SymbolRecord{
		Name:          name,
		QualifiedName: qualName,
		SymbolKind:    kind,
		Visibility:    vis,
		Signature:     sig,
		DocComment:    doc,
		StartLine:     startLine,
		EndLine:       endLine,
		StableID:      stableID,
	})
}

func (g *GoExtractor) extractGenDecl(fset *token.FileSet, d *ast.GenDecl, pkgName string, result *extractor.ExtractResult) {
	for _, spec := range d.Specs {
		switch s := spec.(type) {
		case *ast.TypeSpec:
			g.extractTypeSpec(fset, d, s, pkgName, result)
		case *ast.ValueSpec:
			g.extractValueSpec(fset, d, s, pkgName, result)
		}
	}
}

func (g *GoExtractor) extractTypeSpec(fset *token.FileSet, d *ast.GenDecl, s *ast.TypeSpec, pkgName string, result *extractor.ExtractResult) {
	name := s.Name.Name
	qualName := fmt.Sprintf("%s.%s", pkgName, name)
	startLine := fset.Position(s.Pos()).Line
	endLine := fset.Position(s.End()).Line

	var kind string
	switch st := s.Type.(type) {
	case *ast.StructType:
		kind = "struct"
		// Extract fields
		if st.Fields != nil {
			for _, field := range st.Fields.List {
				for _, fieldName := range field.Names {
					fieldQN := fmt.Sprintf("%s.%s.%s", pkgName, name, fieldName.Name)
					fieldStableID := fmt.Sprintf("go:%s:field", fieldQN)
					result.Symbols = append(result.Symbols, extractor.SymbolRecord{
						Name:           fieldName.Name,
						QualifiedName:  fieldQN,
						SymbolKind:     "field",
						Visibility:     visibility(fieldName.Name),
						ParentSymbolID: qualName,
						StartLine:      fset.Position(field.Pos()).Line,
						EndLine:        fset.Position(field.End()).Line,
						StableID:       fieldStableID,
					})
				}
			}
		}
	case *ast.InterfaceType:
		kind = "interface"
	default:
		kind = "type"
	}

	stableID := fmt.Sprintf("go:%s:%s", qualName, kind)
	vis := visibility(name)

	doc := ""
	if s.Doc != nil {
		doc = s.Doc.Text()
	} else if d.Doc != nil && len(d.Specs) == 1 {
		doc = d.Doc.Text()
	}

	result.Symbols = append(result.Symbols, extractor.SymbolRecord{
		Name:          name,
		QualifiedName: qualName,
		SymbolKind:    kind,
		Visibility:    vis,
		DocComment:    doc,
		StartLine:     startLine,
		EndLine:       endLine,
		StableID:      stableID,
	})
}

func (g *GoExtractor) extractValueSpec(fset *token.FileSet, d *ast.GenDecl, s *ast.ValueSpec, pkgName string, result *extractor.ExtractResult) {
	var kind string
	switch d.Tok {
	case token.CONST:
		kind = "const"
	case token.VAR:
		kind = "var"
	default:
		return
	}

	for _, name := range s.Names {
		if name.Name == "_" {
			continue
		}
		qualName := fmt.Sprintf("%s.%s", pkgName, name.Name)
		stableID := fmt.Sprintf("go:%s:%s", qualName, kind)

		doc := ""
		if s.Doc != nil {
			doc = s.Doc.Text()
		} else if d.Doc != nil && len(d.Specs) == 1 {
			doc = d.Doc.Text()
		}

		result.Symbols = append(result.Symbols, extractor.SymbolRecord{
			Name:          name.Name,
			QualifiedName: qualName,
			SymbolKind:    kind,
			Visibility:    visibility(name.Name),
			DocComment:    doc,
			StartLine:     fset.Position(s.Pos()).Line,
			EndLine:       fset.Position(s.End()).Line,
			StableID:      stableID,
		})
	}
}

func visibility(name string) string {
	if len(name) > 0 && unicode.IsUpper(rune(name[0])) {
		return "exported"
	}
	return "unexported"
}

func receiverTypeName(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.StarExpr:
		return receiverTypeName(t.X)
	case *ast.Ident:
		return t.Name
	default:
		return "unknown"
	}
}

func hasTestParam(d *ast.FuncDecl, typeSuffix string) bool {
	if d.Type.Params == nil || len(d.Type.Params.List) == 0 {
		return false
	}
	for _, p := range d.Type.Params.List {
		if sel, ok := p.Type.(*ast.StarExpr); ok {
			if s, ok := sel.X.(*ast.SelectorExpr); ok {
				if s.Sel.Name == typeSuffix {
					return true
				}
			}
		}
	}
	return false
}

func formatFuncSignature(d *ast.FuncDecl, recvType string) string {
	var b strings.Builder
	b.WriteString("func ")
	if recvType != "" {
		b.WriteString("(")
		b.WriteString(recvType)
		b.WriteString(") ")
	}
	b.WriteString(d.Name.Name)
	b.WriteString("(")
	if d.Type.Params != nil {
		var params []string
		for _, p := range d.Type.Params.List {
			typeStr := typeExprStr(p.Type)
			if len(p.Names) == 0 {
				params = append(params, typeStr)
			} else {
				for _, n := range p.Names {
					params = append(params, n.Name+" "+typeStr)
				}
			}
		}
		b.WriteString(strings.Join(params, ", "))
	}
	b.WriteString(")")
	if d.Type.Results != nil && len(d.Type.Results.List) > 0 {
		var results []string
		for _, r := range d.Type.Results.List {
			typeStr := typeExprStr(r.Type)
			if len(r.Names) > 0 {
				for _, n := range r.Names {
					results = append(results, n.Name+" "+typeStr)
				}
			} else {
				results = append(results, typeStr)
			}
		}
		if len(results) == 1 {
			b.WriteString(" " + results[0])
		} else {
			b.WriteString(" (" + strings.Join(results, ", ") + ")")
		}
	}
	return b.String()
}

func typeExprStr(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return "*" + typeExprStr(t.X)
	case *ast.SelectorExpr:
		return typeExprStr(t.X) + "." + t.Sel.Name
	case *ast.ArrayType:
		return "[]" + typeExprStr(t.Elt)
	case *ast.MapType:
		return "map[" + typeExprStr(t.Key) + "]" + typeExprStr(t.Value)
	case *ast.InterfaceType:
		return "interface{}"
	case *ast.Ellipsis:
		return "..." + typeExprStr(t.Elt)
	case *ast.FuncType:
		return "func(...)"
	case *ast.ChanType:
		return "chan " + typeExprStr(t.Value)
	default:
		return "unknown"
	}
}

func buildImportPath(modulePath, dirPath string) string {
	if modulePath == "" {
		return dirPath
	}
	if dirPath == "" {
		return modulePath
	}
	return modulePath + "/" + filepath.ToSlash(dirPath)
}
