// Digest generation for tool result content.
package main

import (
	"strings"
)

const (
	// MaxDigestLen is the maximum length of a digest in characters.
	MaxDigestLen = 300
	// MinDigestLen is the minimum length before we truncate.
	MinDigestLen = 100
)

// MakeDigest creates a short digest from tool result content.
// Strategy: take the first meaningful text up to MaxDigestLen chars.
// If the content is JSON, try to extract a human-readable summary.
func MakeDigest(content []byte) string {
	text := string(content)

	// Try JSON extraction for common tool result shapes
	if trimmed := strings.TrimSpace(text); strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
		if d := extractJSONDigest(text); d != "" {
			return truncate(d, MaxDigestLen)
		}
	}

	// Fallback: first meaningful line
	lines := strings.SplitN(text, "\n", 3)
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if len(line) > 10 {
			return truncate(line, MaxDigestLen)
		}
	}

	return truncate(text, MaxDigestLen)
}

// extractJSONDigest tries to pull a meaningful summary from JSON tool results.
func extractJSONDigest(text string) string {
	// Common patterns:
	// {"type":"text","text":"..."} — single text item
	// [{"type":"text","text":"..."}, ...] — list of items
	// {"entries":[...], "count":N} — list responses

	if strings.Contains(text, `"text":"`) {
		// Extract the first text field value
		idx := strings.Index(text, `"text":"`)
		if idx >= 0 {
			start := idx + 8
			end := strings.Index(text[start:], `"`)
			if end > 0 {
				return text[start : start+end]
			}
		}
	}

	if strings.Contains(text, `"error":`) || strings.Contains(text, `"isError":true`) {
		return "[error] " + truncate(text, 200)
	}

	if strings.Contains(text, `"exitCode":0`) {
		return "[ok] " + truncate(text, 200)
	}

	return ""
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
