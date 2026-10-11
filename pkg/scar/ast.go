package scar

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"path/filepath"
	"strings"
)

// ExtractGoSymbolHash parses a Go source file and computes a deterministic SHA-256 hash
// of the AST node corresponding to the given symbol name.
func ExtractGoSymbolHash(fullPath, symbolName string) (string, error) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, fullPath, nil, 0)
	if err != nil {
		return "", fmt.Errorf("failed parsing Go AST for %s: %w", fullPath, err)
	}

	normQuery := NormalizeSymbol(symbolName)
	var matchedNode ast.Node

	ast.Inspect(node, func(n ast.Node) bool {
		if matchedNode != nil {
			return false
		}
		if n == nil {
			return true
		}

		switch decl := n.(type) {
		case *ast.FuncDecl:
			funcName := decl.Name.Name
			var fullName string
			if decl.Recv != nil && len(decl.Recv.List) > 0 {
				recvType := ""
				switch t := decl.Recv.List[0].Type.(type) {
				case *ast.Ident:
					recvType = t.Name
				case *ast.StarExpr:
					if id, ok := t.X.(*ast.Ident); ok {
						recvType = id.Name
					}
				}
				if recvType != "" {
					fullName = recvType + "." + funcName
				}
			}
			if fullName == "" {
				fullName = funcName
			}

			if fullName == symbolName ||
				funcName == symbolName ||
				NormalizeSymbol(fullName) == normQuery ||
				NormalizeSymbol(funcName) == normQuery {
				matchedNode = decl
				return false
			}

		case *ast.TypeSpec:
			typeName := decl.Name.Name
			if typeName == symbolName || NormalizeSymbol(typeName) == normQuery {
				matchedNode = decl
				return false
			}
		}

		return true
	})

	if matchedNode == nil {
		return "", fmt.Errorf("symbol %q not found in %s", symbolName, fullPath)
	}

	var buf bytes.Buffer
	if err := printer.Fprint(&buf, fset, matchedNode); err != nil {
		return "", fmt.Errorf("failed printing AST node: %w", err)
	}

	h := sha256.Sum256(buf.Bytes())
	return hex.EncodeToString(h[:]), nil
}

// ComputeSymbolBaselines records AST hashes for anchored symbols in Go files.
func (s *Store) ComputeSymbolBaselines(scar *Scar) map[string]string {
	if len(scar.Anchors.Symbols) == 0 {
		return nil
	}

	symbolHashes := make(map[string]string)
	for _, f := range scar.Anchors.Files {
		if !strings.HasSuffix(f, ".go") {
			continue
		}
		fullPath := filepath.Join(s.RepoRoot, f)
		for _, sym := range scar.Anchors.Symbols {
			hash, err := ExtractGoSymbolHash(fullPath, sym)
			if err == nil && hash != "" {
				symbolHashes[sym] = hash
			}
		}
	}

	if len(symbolHashes) == 0 {
		return nil
	}
	return symbolHashes
}
