package main

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var sampleContent = []byte("this is a test tool result with lots of useful information about something")

func sampleHandle() string {
	h := sha256.Sum256(sampleContent)
	return "sha256:" + hex.EncodeToString(h[:])
}

func TestStoreRoundTrip(t *testing.T) {
	root := t.TempDir()

	s, err := NewStore(root)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	handle, exists, err := s.Store(sampleContent, MakeDigest(sampleContent))
	if err != nil {
		t.Fatalf("Store: %v", err)
	}
	if exists {
		t.Fatal("first Store returned exists=true")
	}
	if handle != sampleHandle() {
		t.Fatalf("handle %q != expected %q", handle, sampleHandle())
	}

	entries, totalBytes := s.Stats()
	if entries != 1 {
		t.Fatalf("entries = %d, want 1", entries)
	}
	if totalBytes != int64(len(sampleContent)) {
		t.Fatalf("totalBytes = %d, want %d", totalBytes, len(sampleContent))
	}

	hexHash := handle[len("sha256:"):]
	blobPath := filepath.Join(root, "store", hexHash)
	blob, err := os.ReadFile(blobPath)
	if err != nil {
		t.Fatalf("read blob %s: %v", blobPath, err)
	}
	if string(blob) != string(sampleContent) {
		t.Fatal("blob bytes mismatch")
	}
	if _, err := os.Stat(filepath.Join(root, "index.jsonl")); err != nil {
		t.Fatalf("index.jsonl missing: %v", err)
	}

	content, err := s.Resolve(handle)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if string(content) != string(sampleContent) {
		t.Fatal("resolved content mismatch")
	}

	s2, err := NewStore(root)
	if err != nil {
		t.Fatalf("NewStore reopen: %v", err)
	}
	entries2, _ := s2.Stats()
	if entries2 != 1 {
		t.Fatalf("reopened entries = %d, want 1", entries2)
	}
	content2, err := s2.Resolve(handle)
	if err != nil {
		t.Fatalf("Resolve after reopen: %v", err)
	}
	if string(content2) != string(sampleContent) {
		t.Fatal("resolved content mismatch after reopen")
	}
}

func TestStoreDuplicate(t *testing.T) {
	root := t.TempDir()
	s, err := NewStore(root)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	if _, _, err := s.Store(sampleContent, "d"); err != nil {
		t.Fatalf("Store: %v", err)
	}
	_, exists, err := s.Store(sampleContent, "d")
	if err != nil {
		t.Fatalf("Store dup: %v", err)
	}
	if !exists {
		t.Fatal("second Store returned exists=false")
	}
}

func TestResolveInvalidHandles(t *testing.T) {
	root := t.TempDir()
	s, err := NewStore(root)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	valid := sampleHandle()
	bad := []string{
		"",
		"md5:" + valid[7:],
		"sha256:" + valid[7:70],
		"sha256:" + valid[7:] + "ff",
		"sha256:z" + valid[8:],
		"sha256:" + strings.ToUpper(valid[7:]),
		"sha256:../index.jsonl",
		"sha256:..",
		"sha256:" + valid[7:20] + "/" + valid[20:],
		"sha256:" + valid[7:20] + "." + valid[21:],
	}
	for _, h := range bad {
		if _, err := s.Resolve(h); err == nil {
			t.Errorf("Resolve(%q) succeeded, want error", h)
		}
	}

	unknown := "sha256:" + hex.EncodeToString(make([]byte, 32))
	if unknown != valid {
		if _, err := s.Resolve(unknown); err == nil {
			t.Error("Resolve of valid-but-unknown handle succeeded, want error")
		}
	}
}
