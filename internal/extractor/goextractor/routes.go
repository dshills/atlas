package goextractor

import (
	"encoding/json"
	"go/ast"
	"go/token"
	"strings"

	"github.com/dshills/atlas/internal/extractor"
)

// extractRoutes detects HTTP route registrations.
func extractRoutes(fset *token.FileSet, file *ast.File) ([]extractor.ArtifactRecord, []extractor.ReferenceRecord) {
	var artifacts []extractor.ArtifactRecord
	var refs []extractor.ReferenceRecord

	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}

		methodName := sel.Sel.Name
		var method, path, handler string
		var confidence string

		switch {
		case methodName == "HandleFunc" || methodName == "Handle":
			// http.HandleFunc(path, handler) or mux.HandleFunc(path, handler)
			if len(call.Args) >= 2 {
				path, confidence = extractStringArg(call.Args[0])
				handler = exprToString(call.Args[1])
				method = "ANY"
			}
		case isHTTPMethod(methodName):
			// router.GET(path, handler), router.POST(path, handler), etc.
			if len(call.Args) >= 2 {
				path, confidence = extractStringArg(call.Args[0])
				handler = exprToString(call.Args[1])
				method = strings.ToUpper(methodName)
			}
		default:
			return true
		}

		if path != "" {
			dataJSON, _ := json.Marshal(map[string]string{
				"method":  method,
				"path":    path,
				"handler": handler,
			})

			artifacts = append(artifacts, extractor.ArtifactRecord{
				ArtifactKind: "route",
				Name:         method + " " + path,
				DataJSON:     string(dataJSON),
				Confidence:   confidence,
			})

			refs = append(refs, extractor.ReferenceRecord{
				ReferenceKind: "registers_route",
				Confidence:    confidence,
				RawTargetText: path,
				Line:          fset.Position(call.Pos()).Line,
			})
		}

		return true
	})

	return artifacts, refs
}

func isHTTPMethod(name string) bool {
	switch strings.ToUpper(name) {
	case "GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS":
		return true
	}
	return false
}

func extractStringArg(expr ast.Expr) (string, string) {
	if lit, ok := expr.(*ast.BasicLit); ok && lit.Kind == token.STRING {
		s := strings.Trim(lit.Value, `"`)
		return s, "exact"
	}
	return exprToString(expr), "heuristic"
}

func exprToString(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.SelectorExpr:
		return exprToString(e.X) + "." + e.Sel.Name
	case *ast.BasicLit:
		return strings.Trim(e.Value, `"`)
	default:
		return ""
	}
}
