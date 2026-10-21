package scar

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// ConfigSchemaVersion is the schema version understood by the configuration
// reader. The repository configuration and the scar records are versioned
// separately, so a configuration file declares its own version.
const ConfigSchemaVersion = "1"

// ConfigFileName is the name of the configuration file inside the store directory.
const ConfigFileName = "config.yaml"

// Config holds the repository-scoped retrieval configuration read from
// .gitscars/config.yaml.
type Config struct {
	SchemaVersion    string
	AbstainThreshold int
	TopK             int
}

// ConfigIssue describes one entry of the configuration file that was present
// but could not be used, so that a repository which misunderstands its own
// configuration sees it in doctor instead of silently receiving defaults.
type ConfigIssue struct {
	Key    string
	Reason string
}

// DefaultConfig returns the configuration used when no configuration file is
// present or when the file cannot be interpreted.
func DefaultConfig() Config {
	return Config{
		SchemaVersion:    ConfigSchemaVersion,
		AbstainThreshold: 3,
		TopK:             3,
	}
}

// knownConfigKeys are the keys interpreted by this version of the reader.
var knownConfigKeys = map[string]bool{
	"schema_version":    true,
	"abstain_threshold": true,
	"top_k":             true,
}

type configEntry struct {
	key  string
	raw  string
	line int
}

// parseConfigEntries splits the configuration into key/value entries, keeping
// their order so that diagnostics can point at a stable line.
func parseConfigEntries(data []byte) []configEntry {
	var entries []configEntry
	for i, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(strings.TrimSuffix(line, "\r"))
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		key, val, ok := strings.Cut(trimmed, ":")
		if !ok {
			continue
		}
		entries = append(entries, configEntry{
			key:  strings.ToLower(strings.TrimSpace(key)),
			raw:  strings.TrimSpace(val),
			line: i + 1,
		})
	}
	return entries
}

// unquoteConfigValue removes a single layer of matching quotes from a value.
func unquoteConfigValue(v string) string {
	if len(v) >= 2 && ((v[0] == '"' && v[len(v)-1] == '"') || (v[0] == '\'' && v[len(v)-1] == '\'')) {
		return v[1 : len(v)-1]
	}
	return v
}

// stripInlineComment removes an unquoted trailing comment from a value.
func stripInlineComment(v string) string {
	if i := strings.Index(v, " #"); i >= 0 && !strings.ContainsAny(v[:i], "\"'") {
		return strings.TrimSpace(v[:i])
	}
	return v
}

// LoadConfig reads .gitscars/config.yaml. A missing, unreadable, or malformed
// file yields the defaults. An individual value that cannot be interpreted is
// ignored rather than failing the command, so a configuration mistake never
// blocks retrieval.
func (s *Store) LoadConfig() Config {
	cfg, _ := s.LoadConfigWithIssues()
	return cfg
}

// LoadConfigWithIssues reads the configuration file and reports every entry
// that was present but could not be used. The effective configuration is
// returned together with the issues, so a mistake is visible through doctor
// while retrieval never blocks on it.
//
// A file that declares an unsupported schema version is ignored as a whole:
// the keys of a newer contract may carry different meanings, and honouring
// them under an older reader is not a safe assumption.
func (s *Store) LoadConfigWithIssues() (Config, []ConfigIssue) {
	cfg := DefaultConfig()
	path := filepath.Join(s.RepoRoot, DefaultStoreDir, ConfigFileName)

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return cfg, nil
		}
		return cfg, []ConfigIssue{{Key: ConfigFileName, Reason: fmt.Sprintf("file could not be read: %v", err)}}
	}

	entries := parseConfigEntries(data)

	// If the file declares its version and that version is not understood,
	// the whole file is set aside rather than partially applied.
	for _, e := range entries {
		if e.key != "schema_version" {
			continue
		}
		declared := unquoteConfigValue(stripInlineComment(e.raw))
		if declared != ConfigSchemaVersion {
			return DefaultConfig(), []ConfigIssue{{
				Key:    "schema_version",
				Reason: fmt.Sprintf("line %d declares unsupported version %q; only %q is understood, so the file was ignored entirely", e.line, declared, ConfigSchemaVersion),
			}}
		}
		break
	}

	var issues []ConfigIssue
	updated := make(map[string][]int)
	reportedDuplicate := make(map[string]bool)
	for _, e := range entries {
		value := unquoteConfigValue(stripInlineComment(e.raw))
		updated[e.key] = append(updated[e.key], e.line)

		if !knownConfigKeys[e.key] {
			issues = append(issues, ConfigIssue{
				Key:    e.key,
				Reason: fmt.Sprintf("line %d: not a recognised key; recognised keys are schema_version, abstain_threshold, top_k", e.line),
			})
			continue
		}

		switch e.key {
		case "schema_version":
			if value != ConfigSchemaVersion {
				issues = append(issues, ConfigIssue{Key: e.key, Reason: fmt.Sprintf("line %d: unsupported version %q", e.line, value)})
			}
		case "abstain_threshold":
			n, err := strconv.Atoi(value)
			if err != nil || n <= 0 {
				issues = append(issues, ConfigIssue{Key: e.key, Reason: fmt.Sprintf("line %d: %q is not a positive integer; the default %d is in force", e.line, value, DefaultConfig().AbstainThreshold)})
				continue
			}
			cfg.AbstainThreshold = n
		case "top_k":
			n, err := strconv.Atoi(value)
			if err != nil || n <= 0 {
				issues = append(issues, ConfigIssue{Key: e.key, Reason: fmt.Sprintf("line %d: %q is not a positive integer; the default %d is in force", e.line, value, DefaultConfig().TopK)})
				continue
			}
			cfg.TopK = n
		}
	}

	// A repeated key means one of its appearances is a mistake. The last
	// appearance stays in force, and every position is reported.
	for _, e := range entries {
		if !knownConfigKeys[e.key] || reportedDuplicate[e.key] {
			continue
		}
		if lines := updated[e.key]; len(lines) > 1 {
			reportedDuplicate[e.key] = true
			positions := make([]string, 0, len(lines))
			for _, l := range lines {
				positions = append(positions, strconv.Itoa(l))
			}
			issues = append(issues, ConfigIssue{
				Key:    e.key,
				Reason: fmt.Sprintf("key repeated at line(s) %s; the last appearance is in force", strings.Join(positions, ", ")),
			})
		}
	}

	return cfg, issues
}
