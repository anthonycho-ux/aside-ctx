package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	s, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	return httptest.NewServer(newHandler(s))
}

func decodeJSON(t *testing.T, resp *http.Response, v interface{}) {
	t.Helper()
	if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
		t.Fatalf("decode %s response: %v", resp.Request.URL.Path, err)
	}
	resp.Body.Close()
}

func TestHTTPStoreResolveStatsHealth(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	body := `{"content":` + `"` + string(sampleContent) + `"}`
	resp, err := http.Post(srv.URL+"/store", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("POST /store: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		t.Fatalf("POST /store status = %d", resp.StatusCode)
	}
	var sr StoreResponse
	decodeJSON(t, resp, &sr)
	if sr.Handle != sampleHandle() {
		t.Fatalf("handle = %q, want %q", sr.Handle, sampleHandle())
	}
	if sr.Digest != MakeDigest(sampleContent) {
		t.Fatalf("digest = %q, want %q", sr.Digest, MakeDigest(sampleContent))
	}
	if sr.Size != len(sampleContent) {
		t.Fatalf("size = %d, want %d", sr.Size, len(sampleContent))
	}
	if sr.Exists {
		t.Fatal("first store exists=true")
	}

	r, err := http.Get(srv.URL + "/resolve/" + sr.Handle)
	if err != nil {
		t.Fatalf("GET /resolve: %v", err)
	}
	if r.StatusCode != http.StatusOK {
		r.Body.Close()
		t.Fatalf("GET /resolve status = %d", r.StatusCode)
	}
	var rr ResolveResponse
	decodeJSON(t, r, &rr)
	if rr.Handle != sr.Handle {
		t.Fatalf("resolve handle = %q, want %q", rr.Handle, sr.Handle)
	}
	if rr.Content != string(sampleContent) {
		t.Fatal("resolved content mismatch")
	}
	if rr.Size != len(sampleContent) {
		t.Fatalf("resolve size = %d, want %d", rr.Size, len(sampleContent))
	}

	resp2, err := http.Post(srv.URL+"/store", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("POST /store dup: %v", err)
	}
	if resp2.StatusCode != http.StatusOK {
		resp2.Body.Close()
		t.Fatalf("POST /store dup status = %d", resp2.StatusCode)
	}
	var sr2 StoreResponse
	decodeJSON(t, resp2, &sr2)
	if !sr2.Exists {
		t.Fatal("duplicate store exists=false")
	}

	rs, err := http.Get(srv.URL + "/stats")
	if err != nil {
		t.Fatalf("GET /stats: %v", err)
	}
	if rs.StatusCode != http.StatusOK {
		rs.Body.Close()
		t.Fatalf("GET /stats status = %d", rs.StatusCode)
	}
	var stats StatsResponse
	decodeJSON(t, rs, &stats)
	if stats.Entries != 1 {
		t.Fatalf("entries = %d, want 1", stats.Entries)
	}
	if stats.TotalBytes != int64(len(sampleContent)) {
		t.Fatalf("totalBytes = %d, want %d", stats.TotalBytes, len(sampleContent))
	}

	rh, err := http.Get(srv.URL + "/health")
	if err != nil {
		t.Fatalf("GET /health: %v", err)
	}
	if rh.StatusCode != http.StatusOK {
		rh.Body.Close()
		t.Fatalf("GET /health status = %d", rh.StatusCode)
	}
	var health map[string]string
	decodeJSON(t, rh, &health)
	if health["status"] != "ok" {
		t.Fatalf("health = %v", health)
	}
}

func TestHTTPErrorCases(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/store", "application/json", strings.NewReader("not json"))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("malformed JSON status = %d, want 400", resp.StatusCode)
	}

	resp, err = http.Post(srv.URL+"/store", "application/json", strings.NewReader(`{"content":""}`))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("empty content status = %d, want 400", resp.StatusCode)
	}

	unknown := "sha256:" + strings.Repeat("ab", 32)
	r, err := http.Get(srv.URL + "/resolve/" + unknown)
	if err != nil {
		t.Fatal(err)
	}
	r.Body.Close()
	if r.StatusCode != http.StatusNotFound {
		t.Fatalf("missing blob status = %d, want 404", r.StatusCode)
	}
}

func TestHTTPMethodNotAllowed(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	cases := []struct {
		method, path string
	}{
		{http.MethodGet, "/store"},
		{http.MethodPost, "/stats"},
		{http.MethodPost, "/health"},
		{http.MethodPost, "/resolve/" + sampleHandle()},
	}
	for _, c := range cases {
		req, err := http.NewRequest(c.method, srv.URL+c.path, strings.NewReader("{}"))
		if err != nil {
			t.Fatal(err)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("%s %s: %v", c.method, c.path, err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusMethodNotAllowed {
			t.Errorf("%s %s status = %d, want 405", c.method, c.path, resp.StatusCode)
		}
	}
}
