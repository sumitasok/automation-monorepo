package uiserver

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"
)

// Options configures a UI server.
type Options struct {
	// DataDir is the wallet data directory to read, normally
	// <config-path>/data/expense-domain/wallet.
	DataDir string
	// Static serves the UI's html/css/js. Pass the embedded FS for a normal run
	// or os.DirFS("ui") to edit the front end without rebuilding.
	Static fs.FS
	// Addr is the listen address, e.g. "127.0.0.1:8787".
	Addr string
	// Log receives one line per request; nil disables request logging.
	Log func(format string, v ...any)
}

// Server holds the resolved options behind the HTTP handlers.
type Server struct {
	opts Options
}

// New validates opts and returns a server ready to hand to http.ListenAndServe.
func New(opts Options) (*Server, error) {
	if opts.DataDir == "" {
		return nil, fmt.Errorf("data directory is required")
	}
	st, err := os.Stat(opts.DataDir)
	if err != nil {
		return nil, fmt.Errorf("data directory %s: %w", opts.DataDir, err)
	}
	if !st.IsDir() {
		return nil, fmt.Errorf("data directory %s is not a directory", opts.DataDir)
	}
	if opts.Static == nil {
		return nil, fmt.Errorf("static file system is required")
	}
	if opts.Log == nil {
		opts.Log = func(string, ...any) {}
	}
	return &Server{opts: opts}, nil
}

// Handler returns the full route table: /api/* for data, everything else served
// from the static UI.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/api/meta", s.wrap(s.handleMeta))
	mux.HandleFunc("/api/records", s.wrap(s.handleRecords))
	mux.HandleFunc("/api/duplicates", s.wrap(s.handleDuplicates))
	mux.HandleFunc("/api/sync", s.wrap(s.handleSync))
	mux.HandleFunc("/api/logs", s.wrap(s.handleLogs))
	mux.HandleFunc("/api/logs/", s.wrap(s.handleLogTail))

	mux.Handle("/", http.FileServer(http.FS(s.opts.Static)))
	return s.logRequests(mux)
}

// ListenAndServe starts the server on opts.Addr.
func (s *Server) ListenAndServe() error {
	srv := &http.Server{
		Addr:              s.opts.Addr,
		Handler:           s.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}
	return srv.ListenAndServe()
}

// wrap turns a handler that returns a value or an error into an HTTP handler:
// the value is written as JSON, an error as a JSON problem body. Reads are
// GET-only — this server never mutates the data directory.
func (s *Server) wrap(h func(*http.Request) (any, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "read-only server: use GET"})
			return
		}
		v, err := h(r)
		if err != nil {
			s.opts.Log("api %s: %v", r.URL.Path, err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, v)
	}
}

func (s *Server) handleMeta(*http.Request) (any, error) {
	return map[string]any{
		"dataDir":   s.opts.DataDir,
		"servedAt":  time.Now().UTC().Format(time.RFC3339),
		"readOnly":  true,
		"endpoints": []string{"/api/records", "/api/duplicates", "/api/sync", "/api/logs", "/api/logs/{name}"},
	}, nil
}

func (s *Server) handleRecords(*http.Request) (any, error) {
	return LoadRecords(s.opts.DataDir)
}

func (s *Server) handleDuplicates(*http.Request) (any, error) {
	return LoadDuplicates(s.opts.DataDir)
}

func (s *Server) handleSync(*http.Request) (any, error) {
	return LoadSyncState(s.opts.DataDir)
}

func (s *Server) handleLogs(r *http.Request) (any, error) {
	limit := intParam(r, "limit", 60)
	logs, err := LoadLogs(s.opts.DataDir, limit)
	if err != nil {
		return nil, err
	}
	return map[string]any{"logs": logs}, nil
}

func (s *Server) handleLogTail(r *http.Request) (any, error) {
	name := r.URL.Path[len("/api/logs/"):]
	tail := intParam(r, "tail", 400)
	body, err := ReadLog(s.opts.DataDir, name, tail)
	if err != nil {
		return nil, err
	}
	return map[string]any{"name": name, "tail": tail, "body": body}, nil
}

func (s *Server) logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		s.opts.Log("%s %s (%s)", r.Method, r.URL.Path, time.Since(start).Round(time.Millisecond))
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	// The data is personal finance detail read off local disk; keep it out of
	// any cache and out of other origins.
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("uiserver: encode response: %v", err)
	}
}

func intParam(r *http.Request, name string, def int) int {
	if raw := r.URL.Query().Get(name); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n >= 0 {
			return n
		}
	}
	return def
}
