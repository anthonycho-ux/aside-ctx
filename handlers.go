package main

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
)

// StoreRequest is the JSON body for POST /store.
type StoreRequest struct {
	Content string `json:"content"`
}

// StoreResponse is the JSON body returned from POST /store.
type StoreResponse struct {
	Handle string `json:"handle"`
	Digest string `json:"digest"`
	Size   int    `json:"size"`
	Exists bool   `json:"exists"`
}

// ResolveResponse is the JSON body returned from GET /resolve.
type ResolveResponse struct {
	Handle  string `json:"handle"`
	Content string `json:"content"`
	Size    int    `json:"size"`
}

// StatsResponse is the JSON body returned from GET /stats.
type StatsResponse struct {
	Entries    int   `json:"entries"`
	TotalBytes int64 `json:"total_bytes"`
}

func handleStore(s *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
			return
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, `{"error":"cannot read body"}`, http.StatusBadRequest)
			return
		}

		var req StoreRequest
		if err := json.Unmarshal(body, &req); err != nil {
			http.Error(w, `{"error":"invalid JSON"}`, http.StatusBadRequest)
			return
		}

		if req.Content == "" {
			http.Error(w, `{"error":"content is empty"}`, http.StatusBadRequest)
			return
		}

		content := []byte(req.Content)
		digest := MakeDigest(content)
		handle, exists, err := s.Store(content, digest)
		if err != nil {
			http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
			return
		}

		resp := StoreResponse{
			Handle: handle,
			Digest: digest,
			Size:   len(content),
			Exists: exists,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

func handleResolve(s *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
			return
		}

		// Path: /resolve/<handle>
		handle := strings.TrimPrefix(r.URL.Path, "/resolve/")
		if handle == "" || handle == r.URL.Path {
			http.Error(w, `{"error":"missing handle"}`, http.StatusBadRequest)
			return
		}

		content, err := s.Resolve(handle)
		if err != nil {
			http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusNotFound)
			return
		}

		resp := ResolveResponse{
			Handle:  handle,
			Content: string(content),
			Size:    len(content),
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

func handleStats(s *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		entries, totalBytes := s.Stats()
		resp := StatsResponse{
			Entries:    entries,
			TotalBytes: totalBytes,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

func handleHealth() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	}
}
