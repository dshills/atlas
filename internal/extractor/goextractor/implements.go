package goextractor

import (
	"fmt"
	"go/ast"
	"go/token"

	"github.com/dshills/atlas/internal/extractor"
)

// extractImplements detects interface implementations within the same file.
func extractImplements(fset *token.FileSet, file *ast.File, pkgName string) []extractor.ReferenceRecord {
	// Collect interfaces: name → method names
	interfaces := make(map[string][]string)
	// Collect struct method sets: name → method names
	structMethods := make(map[string][]string)

	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.GenDecl:
			for _, spec := range d.Specs {
				ts, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}
				if iface, ok := ts.Type.(*ast.InterfaceType); ok {
					var methods []string
					if iface.Methods != nil {
						for _, m := range iface.Methods.List {
							for _, name := range m.Names {
								methods = append(methods, name.Name)
							}
						}
					}
					if len(methods) > 0 {
						interfaces[ts.Name.Name] = methods
					}
				}
			}
		case *ast.FuncDecl:
			if d.Recv != nil && len(d.Recv.List) > 0 {
				recvType := receiverTypeName(d.Recv.List[0].Type)
				structMethods[recvType] = append(structMethods[recvType], d.Name.Name)
			}
		}
	}

	var refs []extractor.ReferenceRecord
	for structName, methods := range structMethods {
		methodSet := make(map[string]bool, len(methods))
		for _, m := range methods {
			methodSet[m] = true
		}

		for ifaceName, ifaceMethods := range interfaces {
			if implements(methodSet, ifaceMethods) {
				refs = append(refs, extractor.ReferenceRecord{
					FromSymbolName: fmt.Sprintf("%s.%s", pkgName, structName),
					ToSymbolName:   fmt.Sprintf("%s.%s", pkgName, ifaceName),
					ReferenceKind:  "implements",
					Confidence:     "exact",
					RawTargetText:  fmt.Sprintf("%s.%s", pkgName, ifaceName),
				})
			}
		}
	}

	return refs
}

func implements(methodSet map[string]bool, required []string) bool {
	for _, m := range required {
		if !methodSet[m] {
			return false
		}
	}
	return true
}
