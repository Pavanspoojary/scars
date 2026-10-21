package scar

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Config holds the repository-scoped retrieval configuration read from
// .gitscars/config.yaml.
type Config struct {
	SchemaVersion    string
	AbstainThreshold int
	TopK             int
}

// ConfigFileName is the name of the configuration file inside the store directory.
const ConfigFileName = "config.yaml"

// DefaultConfig returns the configuration used when no configuration file is
// present or when the file cannot be interpreted.
func DefaultConfig() Config {
	return Config{
		SchemaVersion:    CurrentSchemaVersion,
		AbstainThreshold: 3,
		TopK:             3,
	}
}

// LoadConfig reads .gitscars/config.yaml. A missing, unreadable, or malformed
// file yields the defaults. An individual value that cannot be interpreted is
// ignored rather than failing the command, so a configuration mistake never
// blocks retrieval.
func (s *Store) LoadConfig() Config {
	cfg := DefaultConfig()

	data, err := os.ReadFile(filepath.Join(s.RepoRoot, DefaultStoreDir, ConfigFileName))
	if err != nil {
		return cfg
	}

	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		key, val, ok := strings.Cut(trimmed, ":")
		if !ok {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(key)) {
		case "abstain_threshold":
			if n, err := strconv.Atoi(strings.TrimSpace(val)); err == nil && n > 0 {
				cfg.AbstainThreshold = n
			}
		case "top_k":
			if n, err := strconv.Atoi(strings.TrimSpace(val)); err == nil && n > 0 {
				cfg.TopK = n
			}
		}
	}

	return cfg
}
