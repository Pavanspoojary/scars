package scar

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// ParseGoModDependencies parses required dependencies and versions from go.mod.
func ParseGoModDependencies(repoRoot string) (map[string]string, error) {
	path := filepath.Join(repoRoot, "go.mod")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]string), nil
		}
		return nil, err
	}

	deps := make(map[string]string)
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	inRequireBlock := false

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}

		if line == "require (" {
			inRequireBlock = true
			continue
		}
		if inRequireBlock && line == ")" {
			inRequireBlock = false
			continue
		}

		if inRequireBlock {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				modName := parts[0]
				modVer := parts[1]
				deps[modName] = modVer
			}
			continue
		}

		if strings.HasPrefix(line, "require ") {
			parts := strings.Fields(line)
			if len(parts) >= 3 {
				modName := parts[1]
				modVer := parts[2]
				deps[modName] = modVer
			}
		}
	}

	return deps, scanner.Err()
}

// ParseNpmLockDependencies parses package versions from package-lock.json (v1, v2, v3).
func ParseNpmLockDependencies(repoRoot string) (map[string]string, error) {
	path := filepath.Join(repoRoot, "package-lock.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]string), nil
		}
		return nil, err
	}

	var lock struct {
		Packages map[string]struct {
			Version string `json:"version"`
		} `json:"packages"`
		Dependencies map[string]struct {
			Version string `json:"version"`
		} `json:"dependencies"`
	}

	if err := json.Unmarshal(data, &lock); err != nil {
		return nil, err
	}

	deps := make(map[string]string)

	// v2 / v3 lockfile format
	for pkgPath, pkgInfo := range lock.Packages {
		if pkgInfo.Version == "" {
			continue
		}
		cleanName := strings.TrimPrefix(pkgPath, "node_modules/")
		if cleanName != "" && !strings.Contains(cleanName, "node_modules/") {
			deps[cleanName] = pkgInfo.Version
		}
	}

	// v1 lockfile format fallback
	for pkgName, pkgInfo := range lock.Dependencies {
		if _, exists := deps[pkgName]; !exists && pkgInfo.Version != "" {
			deps[pkgName] = pkgInfo.Version
		}
	}

	return deps, nil
}

// ParseCargoLockDependencies parses package names and versions from Rust's Cargo.lock.
func ParseCargoLockDependencies(repoRoot string) (map[string]string, error) {
	return parseTomlPackageFile(filepath.Join(repoRoot, "Cargo.lock"))
}

// ParsePoetryLockDependencies parses package names and versions from Python's poetry.lock.
func ParsePoetryLockDependencies(repoRoot string) (map[string]string, error) {
	return parseTomlPackageFile(filepath.Join(repoRoot, "poetry.lock"))
}

// ParsePythonRequirements parses package names and versions from requirements.txt.
func ParsePythonRequirements(repoRoot string) (map[string]string, error) {
	path := filepath.Join(repoRoot, "requirements.txt")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]string), nil
		}
		return nil, err
	}

	deps := make(map[string]string)
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "-") {
			continue
		}
		if idx := strings.Index(line, "#"); idx != -1 {
			line = strings.TrimSpace(line[:idx])
		}
		var name, ver string
		for _, op := range []string{"==", ">=", "<=", "~=", "!=", "<", ">"} {
			if strings.Contains(line, op) {
				parts := strings.SplitN(line, op, 2)
				name = strings.TrimSpace(parts[0])
				ver = strings.TrimSpace(parts[1])
				break
			}
		}
		if name == "" {
			name = line
		}
		if name != "" {
			deps[name] = ver
		}
	}
	return deps, scanner.Err()
}

func parseTomlPackageFile(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]string), nil
		}
		return nil, err
	}

	deps := make(map[string]string)
	scanner := bufio.NewScanner(strings.NewReader(string(data)))

	var curName, curVer string
	flush := func() {
		if curName != "" && curVer != "" {
			deps[curName] = curVer
		}
		curName = ""
		curVer = ""
	}

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "[[package]]" {
			flush()
			continue
		}
		if strings.HasPrefix(line, "name =") {
			curName = extractQuotedString(line)
		} else if strings.HasPrefix(line, "version =") {
			curVer = extractQuotedString(line)
		}
	}
	flush()

	return deps, scanner.Err()
}

func extractQuotedString(line string) string {
	parts := strings.SplitN(line, "=", 2)
	if len(parts) < 2 {
		return ""
	}
	val := strings.TrimSpace(parts[1])
	val = strings.Trim(val, `"`)
	return val
}

// GetRepoDependencies retrieves all Go, npm, Rust, and Python dependencies present in the repository root.
func GetRepoDependencies(repoRoot string) map[string]string {
	combined := make(map[string]string)

	goDeps, _ := ParseGoModDependencies(repoRoot)
	for k, v := range goDeps {
		combined[k] = v
	}

	npmDeps, _ := ParseNpmLockDependencies(repoRoot)
	for k, v := range npmDeps {
		combined[k] = v
	}

	cargoDeps, _ := ParseCargoLockDependencies(repoRoot)
	for k, v := range cargoDeps {
		combined[k] = v
	}

	poetryDeps, _ := ParsePoetryLockDependencies(repoRoot)
	for k, v := range poetryDeps {
		combined[k] = v
	}

	pyReqs, _ := ParsePythonRequirements(repoRoot)
	for k, v := range pyReqs {
		combined[k] = v
	}

	return combined
}

// MatchDependency checks if an anchor dependency matches currently installed dependencies.
func MatchDependency(installed map[string]string, target DependencyAnchor) bool {
	if target.Name == "" {
		return false
	}

	currentVer, found := installed[target.Name]
	if !found {
		return false
	}

	if target.Version == "" || target.Version == "*" {
		return true // matches any version
	}

	cleanCurrent := strings.TrimPrefix(strings.TrimSpace(currentVer), "v")
	cleanTarget := strings.TrimPrefix(strings.TrimSpace(target.Version), "v")

	// Exact match
	if cleanCurrent == cleanTarget {
		return true
	}

	// Prefix match (e.g. "1.22" matches "1.22.4")
	if strings.HasPrefix(cleanCurrent, cleanTarget) {
		return true
	}

	return false
}
