package goextractor

import (
	"encoding/json"
	"go/ast"
	"go/token"

	"github.com/dshills/atlas/internal/extractor"
)

// extractJobsAndServices detects background jobs and external service calls.
func extractJobsAndServices(fset *token.FileSet, file *ast.File) ([]extractor.ArtifactRecord, []extractor.ReferenceRecord) {
	var artifacts []extractor.ArtifactRecord
	var refs []extractor.ReferenceRecord

	ast.Inspect(file, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.GoStmt:
			// Goroutine launch → background_job
			name := goStmtName(node)
			dataJSON, _ := json.Marshal(map[string]string{
				"type": "goroutine",
				"name": name,
			})
			artifacts = append(artifacts, extractor.ArtifactRecord{
				ArtifactKind: "background_job",
				Name:         name,
				DataJSON:     string(dataJSON),
				Confidence:   "heuristic",
			})

		case *ast.CallExpr:
			sel, ok := node.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}

			ident, ok := sel.X.(*ast.Ident)
			if !ok {
				return true
			}

			pkg := ident.Name
			method := sel.Sel.Name

			// Background job patterns
			if isJobPattern(pkg, method) {
				name := pkg + "." + method
				if len(node.Args) > 0 {
					if s, c := extractStringArg(node.Args[0]); c == "exact" {
						name = s
					}
				}
				dataJSON, _ := json.Marshal(map[string]string{
					"type":   "scheduled",
					"source": pkg + "." + method,
				})
				artifacts = append(artifacts, extractor.ArtifactRecord{
					ArtifactKind: "background_job",
					Name:         name,
					DataJSON:     string(dataJSON),
					Confidence:   "heuristic",
				})
			}

			// External service patterns
			if isExternalServicePattern(pkg, method) {
				name := pkg + "." + method
				dataJSON, _ := json.Marshal(map[string]string{
					"type":   "http_client",
					"source": pkg + "." + method,
				})
				artifacts = append(artifacts, extractor.ArtifactRecord{
					ArtifactKind: "external_service",
					Name:         name,
					DataJSON:     string(dataJSON),
					Confidence:   "heuristic",
				})
				refs = append(refs, extractor.ReferenceRecord{
					ReferenceKind: "invokes_external_api",
					Confidence:    "heuristic",
					RawTargetText: name,
					Line:          fset.Position(node.Pos()).Line,
				})
			}
		}

		return true
	})

	return artifacts, refs
}

func goStmtName(g *ast.GoStmt) string {
	switch fn := g.Call.Fun.(type) {
	case *ast.Ident:
		return fn.Name
	case *ast.FuncLit:
		return "anonymous"
	case *ast.SelectorExpr:
		return exprToString(fn)
	default:
		return "goroutine"
	}
}

func isJobPattern(pkg, method string) bool {
	switch {
	case pkg == "cron" && method == "AddFunc":
		return true
	case pkg == "scheduler" && method == "Every":
		return true
	}
	return false
}

func isExternalServicePattern(pkg, method string) bool {
	switch {
	case pkg == "http" && (method == "Get" || method == "Post" || method == "NewRequest" || method == "Do"):
		return true
	case pkg == "rpc" && method == "Dial":
		return true
	case pkg == "grpc" && method == "Dial":
		return true
	}
	return false
}
