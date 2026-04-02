package goextractor

import (
	"encoding/json"
	"go/ast"
	"go/token"
	"strings"

	"github.com/dshills/atlas/internal/extractor"
)

// extractConfigAccess detects os.Getenv, os.LookupEnv, and viper.Get* calls.
func extractConfigAccess(fset *token.FileSet, file *ast.File) ([]extractor.ArtifactRecord, []extractor.ReferenceRecord) {
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

		ident, ok := sel.X.(*ast.Ident)
		if !ok {
			return true
		}

		pkg := ident.Name
		method := sel.Sel.Name

		var kind, name string
		switch {
		case pkg == "os" && (method == "Getenv" || method == "LookupEnv"):
			kind = "env_var"
			if len(call.Args) > 0 {
				name, _ = extractStringArg(call.Args[0])
			}
		case pkg == "viper" && strings.HasPrefix(method, "Get"):
			kind = "config_key"
			if len(call.Args) > 0 {
				name, _ = extractStringArg(call.Args[0])
			}
		default:
			return true
		}

		if name != "" {
			dataJSON, _ := json.Marshal(map[string]string{
				"key":    name,
				"source": pkg + "." + method,
			})

			artifacts = append(artifacts, extractor.ArtifactRecord{
				ArtifactKind: kind,
				Name:         name,
				DataJSON:     string(dataJSON),
				Confidence:   "exact",
			})

			refs = append(refs, extractor.ReferenceRecord{
				ReferenceKind: "uses_config",
				Confidence:    "exact",
				RawTargetText: name,
				Line:          fset.Position(call.Pos()).Line,
			})
		}

		return true
	})

	return artifacts, refs
}
