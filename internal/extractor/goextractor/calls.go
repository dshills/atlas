package goextractor

import (
	"fmt"
	"go/ast"
	"go/token"

	"github.com/dshills/atlas/internal/extractor"
)

// extractCalls walks the AST to find function/method calls.
func extractCalls(fset *token.FileSet, file *ast.File, pkgName string) []extractor.ReferenceRecord {
	var refs []extractor.ReferenceRecord

	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		var targetText string
		var confidence string

		switch fn := call.Fun.(type) {
		case *ast.Ident:
			// Direct function call in same package
			targetText = fmt.Sprintf("%s.%s", pkgName, fn.Name)
			confidence = "exact"
		case *ast.SelectorExpr:
			// pkg.Func or receiver.Method
			if ident, ok := fn.X.(*ast.Ident); ok {
				targetText = fmt.Sprintf("%s.%s", ident.Name, fn.Sel.Name)
				confidence = "likely"
			}
		}

		if targetText != "" {
			refs = append(refs, extractor.ReferenceRecord{
				ReferenceKind: "calls",
				Confidence:    confidence,
				RawTargetText: targetText,
				Line:          fset.Position(call.Pos()).Line,
			})
		}

		return true
	})

	return refs
}
