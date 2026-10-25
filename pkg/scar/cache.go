package scar

import (
	"maps"
	"os"
	"path/filepath"
	"sync"
)

// fileStamp identifies a version of a file by size and modification time.
// It is the invalidation key for every value cached from a file: a cached
// value is served only while the file still carries the stamp it held when
// the value was derived.
//
// The stamp is taken before the file is read, so a write that races a read
// leaves a mismatching stamp for the next lookup and the entry is refreshed
// instead of pinning stale bytes. A store invalidates its own writes
// explicitly, which covers every file this process changes.
type fileStamp struct {
	size      int64
	mtimeNsec int64
	exists    bool
}

// stampFile reports the current stamp of path. A path that cannot be statted
// reports exists=false, so appearance and disappearance both invalidate.
func stampFile(path string) fileStamp {
	info, err := os.Stat(path)
	if err != nil {
		return fileStamp{}
	}
	return fileStamp{size: info.Size(), mtimeNsec: info.ModTime().UnixNano(), exists: true}
}

// scarCache is the per-process read cache behind Store. Retrieval used to
// re-read and re-parse every candidate scar file and re-hash every anchor
// file on each recall, which dominated the cost of the MCP server and the
// pre-commit hook, both long-lived processes that recall repeatedly against
// an unchanged store. Entries are keyed by file stamp, so any edit, creation,
// or removal invalidates them without a rebuild step.
//
// The cache never hands out its own values: readers receive deep copies, so a
// caller mutating a loaded scar cannot corrupt what the next recall sees.
type scarCache struct {
	mu     sync.Mutex
	parsed map[string]parsedEntry
	hashes map[string]hashEntry
	index  *indexEntry
	deps   *depsEntry
}

type parsedEntry struct {
	stamp fileStamp
	scar  *Scar
}

type hashEntry struct {
	stamp fileStamp
	sha   string
}

// indexEntry caches the decoded index file. Recall validates the index
// against the scar directory on every call, but decoding the same bytes
// again on every call was pure overhead.
type indexEntry struct {
	path  string
	stamp fileStamp
	index *IndexCache
}

// depsEntry remembers the dependency manifest stamps the last parse was
// derived from, so the parse is reused only while every manifest is unchanged.
type depsEntry struct {
	stamps []fileStamp
	deps   map[string]string
}

func newScarCache() *scarCache {
	return &scarCache{
		parsed: make(map[string]parsedEntry),
		hashes: make(map[string]hashEntry),
	}
}

// getParsed returns the cached scar for path while its stamp matches. The
// returned value is the cache's own copy and must be cloned before mutation.
func (c *scarCache) getParsed(path string, stamp fileStamp) (*Scar, bool) {
	if c == nil {
		return nil, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.parsed[path]
	if !ok || entry.stamp != stamp {
		return nil, false
	}
	return entry.scar, true
}

// putParsed stores a parsed scar under its pre-read stamp. The cache takes
// ownership of sc; callers must not mutate it afterwards.
func (c *scarCache) putParsed(path string, stamp fileStamp, sc *Scar) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.parsed[path] = parsedEntry{stamp: stamp, scar: sc}
}

// getHash returns the cached hash of path while its stamp matches.
func (c *scarCache) getHash(path string, stamp fileStamp) (string, bool) {
	if c == nil {
		return "", false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.hashes[path]
	if !ok || entry.stamp != stamp || !stamp.exists {
		return "", false
	}
	return entry.sha, true
}

// putHash stores the hash of path under its pre-read stamp.
func (c *scarCache) putHash(path string, stamp fileStamp, sha string) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.hashes[path] = hashEntry{stamp: stamp, sha: sha}
}

// getDeps returns the cached dependency set when every manifest still holds
// the recorded stamp. The returned map is the cache's own value and is
// treated as read-only by every caller.
func (c *scarCache) getDeps(stamps []fileStamp) (map[string]string, bool) {
	if c == nil || c.deps == nil {
		return nil, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.deps.stamps) != len(stamps) {
		return nil, false
	}
	for i := range stamps {
		if c.deps.stamps[i] != stamps[i] {
			return nil, false
		}
	}
	return c.deps.deps, true
}

// putDeps stores a parsed dependency set with the manifest stamps it was
// derived from.
func (c *scarCache) putDeps(stamps []fileStamp, deps map[string]string) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.deps = &depsEntry{stamps: stamps, deps: deps}
}

// getIndex returns the decoded index file while its stamp matches.
func (c *scarCache) getIndex(path string, stamp fileStamp) (*IndexCache, bool) {
	if c == nil || c.index == nil {
		return nil, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.index.path != path || c.index.stamp != stamp {
		return nil, false
	}
	return c.index.index, true
}

// putIndex stores the decoded index file under its pre-read stamp.
func (c *scarCache) putIndex(path string, stamp fileStamp, idx *IndexCache) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.index = &indexEntry{path: path, stamp: stamp, index: idx}
}

// drop forgets everything cached from path. A store calls this after its own
// writes so that a rewrite preserving size and modification time cannot
// serve the previous content.
func (c *scarCache) drop(path string) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.parsed, path)
	delete(c.hashes, path)
	if c.index != nil && c.index.path == path {
		c.index = nil
	}
}

// clone returns a deep copy so the recipient can mutate the result without
// touching the cache's canonical value or another caller's copy.
func (sc *Scar) clone() *Scar {
	if sc == nil {
		return nil
	}
	cp := *sc
	cp.Anchors.Files = append([]string(nil), sc.Anchors.Files...)
	cp.Anchors.Symbols = append([]string(nil), sc.Anchors.Symbols...)
	cp.Anchors.Commands = append([]string(nil), sc.Anchors.Commands...)
	cp.Anchors.ErrorSignatures = append([]string(nil), sc.Anchors.ErrorSignatures...)
	cp.Anchors.Dependencies = append([]DependencyAnchor(nil), sc.Anchors.Dependencies...)
	cp.Verifier.Argv = append([]string(nil), sc.Verifier.Argv...)
	cp.Verifier.EnvAllowlist = append([]string(nil), sc.Verifier.EnvAllowlist...)
	cp.Lifecycle.ConflictsWith = append([]string(nil), sc.Lifecycle.ConflictsWith...)
	cp.Lifecycle.Supersedes = append([]string(nil), sc.Lifecycle.Supersedes...)
	cp.Baseline.AnchorHashes = maps.Clone(sc.Baseline.AnchorHashes)
	cp.Baseline.SymbolHashes = maps.Clone(sc.Baseline.SymbolHashes)
	return &cp
}

// scarLoadError distinguishes a file that could not be read from one that was
// read but could not be parsed, so each caller can keep reporting its case the
// way it always has: LoadScar prefixes parse failures with the path, while the
// catalog reports the bare parse message.
type scarLoadError struct {
	path   string
	err    error
	parsed bool
}

func (e *scarLoadError) Error() string { return e.err.Error() }

func (e *scarLoadError) Unwrap() error { return e.err }

// loadScarFile reads and parses a scar file through the process cache. The
// returned scar is a private copy that callers may mutate freely.
func (s *Store) loadScarFile(path string) (*Scar, error) {
	// Stamp before reading: a write between the stamp and the read then
	// produces a mismatching stamp on the next lookup, which re-reads,
	// rather than pinning the new bytes under the old stamp.
	stamp := stampFile(path)
	if cached, ok := s.cache.getParsed(path, stamp); ok {
		return cached.clone(), nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, &scarLoadError{path: path, err: err}
	}
	scar, err := ParseFrontmatter(data)
	if err != nil {
		return nil, &scarLoadError{path: path, err: err, parsed: true}
	}

	s.cache.putParsed(path, stamp, scar)
	return scar.clone(), nil
}

// cachedHashFile returns the sha256 of path, reusing the last digest while
// the file keeps its stamp. Freshness is recomputed on every recall, so
// re-reading an untouched anchor was the single largest cost of retrieval.
//
// A missing or unreadable file reports the same error as a direct hash, so
// freshness degrades exactly as it did before.
func (s *Store) cachedHashFile(path string) (string, error) {
	stamp := stampFile(path)
	if sha, ok := s.cache.getHash(path, stamp); ok {
		return sha, nil
	}
	sha, err := hashFile(path)
	if err != nil {
		return "", err
	}
	s.cache.putHash(path, stamp, sha)
	return sha, nil
}

// dependencyManifests are the files GetRepoDependencies parses, in the order
// it reads them. The cached dependency set is reused only while every one of
// them keeps its stamp, so a lockfile edit is picked up on the next recall.
var dependencyManifests = []string{
	"go.mod",
	"package-lock.json",
	"Cargo.lock",
	"poetry.lock",
	"requirements.txt",
}

// repoDependencies returns the repository dependency set, reusing the last
// parse while every manifest is unchanged. It backs recall and injection,
// which previously re-parsed every manifest on each call.
func (s *Store) repoDependencies() map[string]string {
	stamps := make([]fileStamp, 0, len(dependencyManifests))
	for _, name := range dependencyManifests {
		stamps = append(stamps, stampFile(filepath.Join(s.RepoRoot, name)))
	}

	if deps, ok := s.cache.getDeps(stamps); ok {
		return deps
	}

	deps := GetRepoDependencies(s.RepoRoot)
	s.cache.putDeps(stamps, deps)
	return deps
}
