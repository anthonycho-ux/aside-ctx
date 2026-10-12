// Content-addressed file store for tool result blobs.
//
// Each blob is stored by its SHA-256 hash at ~/.aside-ctx/store/<hex>.
// An in-memory index tracks metadata (size, created_at, last_access, access_count)
// and is persisted as JSONL for recovery.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	storeDir  = "store"
	indexFile = "index.jsonl"
)

// Entry records metadata for one stored blob.
type Entry struct {
	Handle      string    `json:"handle"`
	Size        int64     `json:"size"`
	CreatedAt   time.Time `json:"created_at"`
	LastAccess  time.Time `json:"last_access"`
	AccessCount int64     `json:"access_count"`
}

// Store is a content-addressed blob store.
type Store struct {
	mu       sync.RWMutex
	basePath string
	entries  map[string]*Entry
}

// NewStore opens or creates a content-addressed store at basePath.
func NewStore(basePath string) (*Store, error) {
	storePath := filepath.Join(basePath, storeDir)
	if err := os.MkdirAll(storePath, 0755); err != nil {
		return nil, fmt.Errorf("create store dir: %w", err)
	}

	s := &Store{
		basePath: basePath,
		entries:  make(map[string]*Entry),
	}

	// Recover index
	idxPath := filepath.Join(basePath, indexFile)
	if data, err := os.ReadFile(idxPath); err == nil {
		lines := 0
		for _, line := range splitLines(data) {
			line = trimSpace(line)
			if line == "" {
				continue
			}
			var e Entry
			if err := json.Unmarshal([]byte(line), &e); err != nil {
				continue
			}
			s.entries[e.Handle] = &e
			lines++
		}
		_ = lines
	}

	return s, nil
}

// Store accepts raw content, stores it by SHA-256 hash, and returns the handle
// plus a short digest. If the content is already stored, returns the existing handle.
func (s *Store) Store(content []byte, digest string) (handle string, exists bool, err error) {
	h := sha256.Sum256(content)
	handle = fmt.Sprintf("sha256:%s", hex.EncodeToString(h[:]))

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.entries[handle]; ok {
		// Already stored — update last access
		s.entries[handle].LastAccess = time.Now()
		s.entries[handle].AccessCount++
		return handle, true, nil
	}

	// Write blob file
	blobPath := filepath.Join(s.basePath, storeDir, hex.EncodeToString(h[:]))
	if err := os.WriteFile(blobPath, content, 0644); err != nil {
		return "", false, fmt.Errorf("write blob: %w", err)
	}

	entry := &Entry{
		Handle:      handle,
		Size:        int64(len(content)),
		CreatedAt:   time.Now(),
		LastAccess:  time.Now(),
		AccessCount: 1,
	}
	s.entries[handle] = entry

	// Append to index
	idxPath := filepath.Join(s.basePath, indexFile)
	f, err := os.OpenFile(idxPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err == nil {
		line, _ := json.Marshal(entry)
		line = append(line, '\n')
		f.Write(line)
		f.Close()
	}

	return handle, false, nil
}

// Resolve retrieves the full content for a handle.
func (s *Store) Resolve(handle string) ([]byte, error) {
	if len(handle) != len("sha256:")+64 || !strings.HasPrefix(handle, "sha256:") {
		return nil, fmt.Errorf("invalid handle format: %s", handle)
	}
	hexHash := handle[7:]
	decoded, err := hex.DecodeString(hexHash)
	if err != nil || hex.EncodeToString(decoded) != hexHash {
		return nil, fmt.Errorf("invalid handle format: %s", handle)
	}

	s.mu.Lock()
	if e, ok := s.entries[handle]; ok {
		e.LastAccess = time.Now()
		e.AccessCount++
	}
	s.mu.Unlock()

	blobPath := filepath.Join(s.basePath, storeDir, hexHash)
	content, err := os.ReadFile(blobPath)
	if err != nil {
		return nil, fmt.Errorf("blob not found: %s", handle)
	}
	return content, nil
}

// Stats returns store statistics.
func (s *Store) Stats() (entries int, totalBytes int64) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	entries = len(s.entries)
	for _, e := range s.entries {
		totalBytes += e.Size
	}
	return
}

// splitLines splits bytes into lines, handling both \r\n and \n.
func splitLines(data []byte) []string {
	var lines []string
	start := 0
	for i, b := range data {
		if b == '\n' {
			// Trim trailing \r
			end := i
			if end > start && data[end-1] == '\r' {
				end--
			}
			lines = append(lines, string(data[start:end]))
			start = i + 1
		}
	}
	if start < len(data) {
		lines = append(lines, string(data[start:]))
	}
	return lines
}

// trimSpace removes leading and trailing whitespace.
func trimSpace(s string) string {
	start, end := 0, len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t' || s[start] == '\r') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\r') {
		end--
	}
	return s[start:end]
}
