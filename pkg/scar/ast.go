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
	"os"
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

// ExtractPythonSymbolHash parses a Python file and computes a deterministic SHA-256 hash
// of the function or class definition corresponding to symbolName.
func ExtractPythonSymbolHash(fullPath, symbolName string) (string, error) {
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return "", err
	}
	content := string(data)
	lines := strings.Split(content, "\n")

	normQuery := NormalizeSymbol(symbolName)

	startLine := -1
	baseIndent := 0

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		var name string
		if strings.HasPrefix(trimmed, "def ") {
			name = strings.TrimPrefix(trimmed, "def ")
		} else if strings.HasPrefix(trimmed, "async def ") {
			name = strings.TrimPrefix(trimmed, "async def ")
		} else if strings.HasPrefix(trimmed, "class ") {
			name = strings.TrimPrefix(trimmed, "class ")
		}

		if name != "" {
			idx := strings.IndexAny(name, "(:")
			if idx > 0 {
				name = strings.TrimSpace(name[:idx])
			}
			if name == symbolName || NormalizeSymbol(name) == normQuery {
				startLine = i
				baseIndent = countIndent(line)
				break
			}
		}
	}

	if startLine == -1 {
		return "", fmt.Errorf("python symbol %q not found in %s", symbolName, fullPath)
	}

	var symbolLines []string
	symbolLines = append(symbolLines, lines[startLine])

	for i := startLine + 1; i < len(lines); i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		indent := countIndent(line)
		if indent <= baseIndent {
			break
		}
		cleanLine := stripLineComment(trimmed, "#")
		if cleanLine != "" {
			symbolLines = append(symbolLines, cleanLine)
		}
	}

	normalized := strings.Join(symbolLines, "\n")
	h := sha256.Sum256([]byte(normalized))
	return hex.EncodeToString(h[:]), nil
}

// ExtractTSSymbolHash parses a TypeScript/JavaScript file and computes a deterministic SHA-256 hash
// of the function, method, or class definition corresponding to symbolName.
func ExtractTSSymbolHash(fullPath, symbolName string) (string, error) {
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return "", err
	}
	content := string(data)
	normQuery := NormalizeSymbol(symbolName)

	lines := strings.Split(content, "\n")
	startLine := -1

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "/*") {
			continue
		}

		if matchesTSSymbolDeclaration(trimmed, symbolName, normQuery) {
			startLine = i
			break
		}
	}

	if startLine == -1 {
		return "", fmt.Errorf("typescript/javascript symbol %q not found in %s", symbolName, fullPath)
	}

	var symbolLines []string
	braceDepth := 0
	foundOpeningBrace := false

	for i := startLine; i < len(lines); i++ {
		line := lines[i]
		cleanLine := stripLineComment(line, "//")
		cleanLine = strings.TrimSpace(cleanLine)
		if cleanLine != "" {
			symbolLines = append(symbolLines, cleanLine)
		}

		for _, ch := range cleanLine {
			if ch == '{' {
				braceDepth++
				foundOpeningBrace = true
			} else if ch == '}' {
				braceDepth--
			}
		}

		if foundOpeningBrace && braceDepth <= 0 {
			break
		}
	}

	normalized := strings.Join(symbolLines, "\n")
	h := sha256.Sum256([]byte(normalized))
	return hex.EncodeToString(h[:]), nil
}

func matchesTSSymbolDeclaration(line, symbolName, normQuery string) bool {
	line = strings.TrimPrefix(line, "export ")
	line = strings.TrimPrefix(line, "default ")
	line = strings.TrimPrefix(line, "async ")

	if strings.HasPrefix(line, "function ") {
		rest := strings.TrimPrefix(line, "function ")
		idx := strings.IndexAny(rest, "(< \t")
		if idx > 0 {
			name := strings.TrimSpace(rest[:idx])
			return name == symbolName || NormalizeSymbol(name) == normQuery
		}
	}

	if strings.HasPrefix(line, "class ") {
		rest := strings.TrimPrefix(line, "class ")
		idx := strings.IndexAny(rest, "{< \t")
		if idx > 0 {
			name := strings.TrimSpace(rest[:idx])
			return name == symbolName || NormalizeSymbol(name) == normQuery
		}
	}

	for _, kw := range []string{"const ", "let ", "var "} {
		if strings.HasPrefix(line, kw) {
			rest := strings.TrimPrefix(line, kw)
			idx := strings.IndexAny(rest, "=:< \t")
			if idx > 0 {
				name := strings.TrimSpace(rest[:idx])
				if name == symbolName || NormalizeSymbol(name) == normQuery {
					return true
				}
			}
		}
	}

	if strings.HasPrefix(line, "async ") {
		line = strings.TrimPrefix(line, "async ")
	}
	idx := strings.Index(line, "(")
	if idx > 0 {
		candidate := strings.TrimSpace(line[:idx])
		if !strings.ContainsAny(candidate, " =;:+-*/[]") {
			return candidate == symbolName || NormalizeSymbol(candidate) == normQuery
		}
	}

	return false
}

func countIndent(line string) int {
	indent := 0
	for _, ch := range line {
		if ch == ' ' {
			indent++
		} else if ch == '\t' {
			indent += 4
		} else {
			break
		}
	}
	return indent
}

func stripLineComment(line, marker string) string {
	idx := strings.Index(line, marker)
	if idx >= 0 {
		return strings.TrimSpace(line[:idx])
	}
	return strings.TrimSpace(line)
}

// ExtractSymbolHash extracts a normalized hash for a symbol in Go, Python, or TypeScript/JavaScript.
func ExtractSymbolHash(fullPath, symbolName string) (string, error) {
	ext := strings.ToLower(filepath.Ext(fullPath))
	switch ext {
	case ".go":
		return ExtractGoSymbolHash(fullPath, symbolName)
	case ".py":
		return ExtractPythonSymbolHash(fullPath, symbolName)
	case ".ts", ".tsx", ".js", ".jsx":
		return ExtractTSSymbolHash(fullPath, symbolName)
	default:
		return "", fmt.Errorf("unsupported symbol extraction file type: %s", ext)
	}
}

// IsSupportedSymbolFile returns true if the file supports fine-grained symbol drift analysis.
func IsSupportedSymbolFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".go", ".py", ".ts", ".tsx", ".js", ".jsx":
		return true
	}
	return false
}

// ComputeSymbolBaselines records AST/token hashes for anchored symbols in supported language files.
func (s *Store) ComputeSymbolBaselines(scar *Scar) map[string]string {
	if len(scar.Anchors.Symbols) == 0 {
		return nil
	}

	symbolHashes := make(map[string]string)
	for _, f := range scar.Anchors.Files {
		if !IsSupportedSymbolFile(f) {
			continue
		}
		fullPath := filepath.Join(s.RepoRoot, f)
		for _, sym := range scar.Anchors.Symbols {
			hash, err := ExtractSymbolHash(fullPath, sym)
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
