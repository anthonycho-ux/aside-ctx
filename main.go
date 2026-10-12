package main

import (
	"flag"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func newHandler(s *Store) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("POST /store", handleStore(s))
	mux.Handle("GET /resolve/{handle}", handleResolve(s))
	mux.Handle("GET /stats", handleStats(s))
	mux.Handle("GET /health", handleHealth())
	return mux
}

func expandPath(p string) (string, error) {
	if p != "~" && !strings.HasPrefix(p, "~/") {
		return p, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	if p == "~" {
		return home, nil
	}
	return filepath.Join(home, p[2:]), nil
}

func main() {
	home, err := os.UserHomeDir()
	if err != nil {
		log.Fatalf("cannot determine home directory: %v", err)
	}

	port := flag.Int("port", 9876, "port to listen on")
	storeRoot := flag.String("store", filepath.Join(home, ".aside-ctx"), "store root directory")
	flag.Parse()

	expanded, err := expandPath(*storeRoot)
	if err != nil {
		log.Fatalf("expand store path: %v", err)
	}
	*storeRoot = expanded

	if *port < 1 || *port > 65535 {
		log.Fatalf("invalid port %d: must be 1..65535", *port)
	}

	s, err := NewStore(*storeRoot)
	if err != nil {
		log.Fatalf("open store: %v", err)
	}

	addr := net.JoinHostPort("127.0.0.1", strconv.Itoa(*port))
	srv := &http.Server{
		Addr:              addr,
		Handler:           newHandler(s),
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("aside-ctx listening on http://%s (store root: %s)", addr, *storeRoot)
	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
