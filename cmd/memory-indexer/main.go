// memory-indexer — compress MEMORY.md to an index, store full entries in aside-ctx.
//
// Reads ~/.aside/u/0/memory/MEMORY.md, splits by date headings,
// stores each section in aside-ctx, replaces the file with a compact index.
//
// Usage: go run cmd/memory-indexer/main.go

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
)

const asideCtxURL = "http://127.0.0.1:9876"

type StoreResult struct {
	Handle string `json:"handle"`
	Digest string `json:"digest"`
	Size   int    `json:"size"`
	Exists bool   `json:"exists"`
}

type MemEntry struct {
	Date    string
	Content string
}

type IdxEntry struct {
	Date   string
	Desc   string
	Handle string
	Size   int
}

func main() {
	memoryPath := os.ExpandEnv("$HOME/.aside/u/0/memory/MEMORY.md")
	backupPath := memoryPath + ".bak"

	data, err := os.ReadFile(memoryPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Read %s: %v\n", memoryPath, err)
		os.Exit(1)
	}

	// Backup
	if err := os.WriteFile(backupPath, data, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Backup failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Backup at %s\n", backupPath)

	// Split by date headings: "- **Entry:** YYYY-MM-DD"
	entryRe := regexp.MustCompile(`(?m)^\s*-\s*\*\*Entry:\*\*\s*(\d{4}-\d{2}-\d{2})`)
	lines := strings.Split(string(data), "\n")

	// Find header content (before first entry)
	headerEnd := 0
	for i, line := range lines {
		if entryRe.MatchString(line) {
			headerEnd = i
			break
		}
	}

	// Group entries by date
	var entries []MemEntry
	var current MemEntry

	for _, line := range lines[headerEnd:] {
		m := entryRe.FindStringSubmatch(line)
		if m != nil {
			if current.Content != "" {
				entries = append(entries, current)
			}
			current = MemEntry{Date: m[1], Content: line}
		} else if current.Date != "" {
			current.Content += "\n" + line
		}
	}
	if current.Content != "" {
		entries = append(entries, current)
	}

	fmt.Printf("Found %d entries across %d dates\n", len(entries), countDates(entries))

	// Store each entry in aside-ctx, build index
	var index []IdxEntry

	for _, e := range entries {
		desc := extractDesc(e.Content)
		storeResp, err := storeInCtx(e.Content)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Store error for %s: %v\n", e.Date, err)
			continue
		}
		index = append(index, IdxEntry{
			Date:   e.Date,
			Desc:   desc,
			Handle: storeResp.Handle,
			Size:   storeResp.Size,
		})
		if !storeResp.Exists {
			fmt.Printf("Stored %s: %d bytes -> %s\n", e.Date, storeResp.Size, storeResp.Handle[:20])
		}
	}

	// Build index content
	var idxBuf bytes.Buffer
	idxBuf.WriteString("# Memory Briefing - Index\n")
	idxBuf.WriteString("<!-- Full entries stored in aside-ctx. Resolve: curl -s http://127.0.0.1:9876/resolve/<handle> -->\n\n")

	currentDate := ""
	for _, ie := range index {
		if ie.Date != currentDate {
			currentDate = ie.Date
			idxBuf.WriteString(fmt.Sprintf("## %s\n\n", ie.Date))
		}
		desc := ie.Desc
		if len(desc) > 120 {
			desc = desc[:117] + "..."
		}
		h := ie.Handle
		if len(h) > 16 {
			h = h[:16]
		}
		idxBuf.WriteString(fmt.Sprintf("- %s `%s` (%d bytes)\n", desc, h, ie.Size))
	}

	idxBuf.WriteString("\n---\n")
	idxBuf.WriteString(fmt.Sprintf("Total: %d entries, ~%d bytes stored externally\n", len(index), sumSizes(index)))

	// Also store the full MEMORY.md as a single blob for bulk resolution
	fullStoreResp, err := storeInCtx(string(data))
	if err == nil {
		idxBuf.WriteString(fmt.Sprintf("Full file handle: `%s`\n", fullStoreResp.Handle))
	}

	if err := os.WriteFile(memoryPath, idxBuf.Bytes(), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Write index: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\nMEMORY.md replaced with index (%d bytes, was %d bytes - %d%% reduction)\n",
		idxBuf.Len(), len(data), 100-(idxBuf.Len()*100/len(data)))
	fmt.Printf("Full content backup at: %s\n", backupPath)
	fmt.Printf("Resolve with: curl -s http://127.0.0.1:9876/resolve/%s\n", fullStoreResp.Handle)
}

func extractDesc(content string) string {
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		line = strings.TrimPrefix(line, "- **Entry:**")
		line = strings.TrimSpace(line)
		parts := strings.SplitN(line, " - ", 2)
		if len(parts) > 1 {
			return strings.TrimSpace(parts[1])
		}
		if len(line) > 10 {
			return line
		}
	}
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if len(line) > 10 {
			return line
		}
	}
	if len(content) > 120 {
		return content[:117] + "..."
	}
	return content
}

func storeInCtx(content string) (*StoreResult, error) {
	body, _ := json.Marshal(map[string]string{"content": content})
	resp, err := http.Post(
		asideCtxURL+"/store",
		"application/json",
		bytes.NewReader(body),
	)
	if err != nil {
		return nil, fmt.Errorf("http: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("status %d: %s", resp.StatusCode, string(respBody))
	}

	var result StoreResult
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("parse: %w", err)
	}
	return &result, nil
}

func countDates(entries []MemEntry) int {
	seen := map[string]bool{}
	for _, e := range entries {
		seen[e.Date] = true
	}
	return len(seen)
}

func sumSizes(index []IdxEntry) int {
	total := 0
	for _, ie := range index {
		total += ie.Size
	}
	return total
}
