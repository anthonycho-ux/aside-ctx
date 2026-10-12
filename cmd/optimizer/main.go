// aside-optimizer — persistent watcher for aside conversation logs.
//
// Watches ~/.aside/u/0/sessions/ for new/modified messages.jsonl files,
// stores large tool results in aside-ctx, replaces them with handle+digest
// references. Reduces per-request token overhead by ~65%.
//
// Usage: aside-optimizer [-poll 5s] [-min 1024]
//
// The optimizer is idempotent: already-optimized lines (containing
// "stored in aside-ctx") are skipped on re-runs.

package main

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const asideCtxURL = "http://127.0.0.1:9876"

type MessageLine struct {
	Role       string          `json:"role"`
	ToolName   string          `json:"toolName,omitempty"`
	ToolCallID string          `json:"toolCallId,omitempty"`
	Content    json.RawMessage `json:"content,omitempty"`
	IsError    bool            `json:"isError,omitempty"`
}

type StoreResult struct {
	Handle string `json:"handle"`
	Digest string `json:"digest"`
	Size   int    `json:"size"`
	Exists bool   `json:"exists"`
}

var (
	pollInterval = flag.Duration("poll", 5*time.Second, "poll interval")
	minSize      = flag.Int("min", 1024, "minimum blob size in bytes")
	sessionsDir  = os.ExpandEnv("$HOME/.aside/u/0/sessions")
)

func validateArguments(args []string) error {
	if len(args) > 1 {
		return fmt.Errorf("expected at most one file argument; put flags before the file path")
	}
	return nil
}

func main() {
	flag.Parse()

	if err := validateArguments(flag.Args()); err != nil {
		log.Fatal(err)
	}

	// One-shot mode: process a single file
	if flag.NArg() == 1 {
		optimizeFile(flag.Arg(0))
		return
	}

	// Watch mode: poll for new/modified session files
	log.Printf("Watching %s every %v (min blob: %d bytes)", sessionsDir, *pollInterval, *minSize)
	seen := map[string]time.Time{}

	for {
		entries, err := os.ReadDir(sessionsDir)
		if err != nil {
			log.Printf("read sessions dir: %v", err)
			time.Sleep(*pollInterval)
			continue
		}

		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			msgPath := filepath.Join(sessionsDir, e.Name(), "messages.jsonl")
			info, err := os.Stat(msgPath)
			if err != nil {
				continue
			}

			prevMod, ok := seen[msgPath]
			if ok && !info.ModTime().After(prevMod) {
				continue
			}
			seen[msgPath] = info.ModTime()

			if ok {
				// Already seen but modified — re-optimize
				log.Printf("Re-optimizing %s (modified)", e.Name())
			} else {
				log.Printf("Optimizing %s", e.Name())
			}
			optimizeFile(msgPath)
		}

		time.Sleep(*pollInterval)
	}
}

func optimizeFile(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		log.Printf("Read %s: %v", path, err)
		return
	}

	lines := bytes.Split(data, []byte("\n"))
	modified := 0
	stored := 0
	skipped := 0

	var out bytes.Buffer

	for _, line := range lines {
		raw := bytes.TrimSpace(line)
		if len(raw) == 0 {
			continue
		}

		// Skip already-optimized lines
		if bytes.Contains(raw, []byte("stored in aside-ctx")) {
			out.Write(raw)
			out.WriteByte('\n')
			skipped++
			continue
		}

		var msg MessageLine
		if err := json.Unmarshal(raw, &msg); err != nil {
			out.Write(raw)
			out.WriteByte('\n')
			skipped++
			continue
		}

		if msg.Role != "toolResult" {
			out.Write(raw)
			out.WriteByte('\n')
			skipped++
			continue
		}

		contentText := extractContentText(msg.Content)
		contentBytes := []byte(contentText)

		if len(contentBytes) < *minSize {
			out.Write(raw)
			out.WriteByte('\n')
			skipped++
			continue
		}

		storeResp, err := storeInCtx(contentText)
		if err != nil {
			log.Printf("Store error for %s: %v", msg.ToolName, err)
			out.Write(raw)
			out.WriteByte('\n')
			continue
		}

		digest := makeDigest(contentText)

		replacement := fmt.Sprintf(
			`[stored in aside-ctx] tool=%s handle=%s hash=%s digest=%s size=%d%s`,
			msg.ToolName,
			storeResp.Handle,
			shortSHA(contentBytes),
			digest,
			len(contentBytes),
			cond(msg.IsError, " isError=true", ""),
		)

		replContent := []map[string]string{
			{"type": "text", "text": replacement},
		}
		replJSON, _ := json.Marshal(replContent)

		msg.Content = replJSON
		modified++

		if !storeResp.Exists {
			stored++
		}

		modifiedLine, _ := json.Marshal(msg)
		out.Write(modifiedLine)
		out.WriteByte('\n')
	}

	if modified == 0 {
		return
	}

	if err := os.WriteFile(path, out.Bytes(), 0644); err != nil {
		log.Printf("Write %s: %v", path, err)
		return
	}

	log.Printf("Optimized %s: %d modified, %d new, %d already-handled",
		filepath.Base(filepath.Dir(path)), modified, stored, skipped)
}

func extractContentText(raw json.RawMessage) string {
	var items []map[string]json.RawMessage
	if err := json.Unmarshal(raw, &items); err == nil {
		var texts []string
		for _, item := range items {
			if t, ok := item["text"]; ok {
				var s string
				if err := json.Unmarshal(t, &s); err == nil {
					texts = append(texts, s)
				}
			}
		}
		return strings.Join(texts, "\n")
	}

	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	return string(raw)
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

func shortSHA(content []byte) string {
	h := sha256.Sum256(content)
	return hex.EncodeToString(h[:])[:12]
}

func makeDigest(content string) string {
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if len(line) > 10 {
			if len(line) > 200 {
				line = line[:200] + "..."
			}
			return line
		}
	}
	if len(content) > 200 {
		return content[:200] + "..."
	}
	return content
}

func cond(c bool, t, f string) string {
	if c {
		return t
	}
	return f
}
