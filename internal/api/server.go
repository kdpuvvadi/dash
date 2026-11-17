package api

import (
	"encoding/json"
	"io/fs"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/puvvadi/dash/internal/apps"
	"github.com/puvvadi/dash/internal/config"
	"github.com/puvvadi/dash/internal/store"
)

type Server struct {
	Cfg      func() *config.Config
	Store    *store.Store
	Defs     func() map[string]*apps.Def
	Hub      *Hub
	UI       fs.FS
	RepoURL  string
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/health", s.handleHealth)
	mux.HandleFunc("/api/layout", s.handleLayout)
	mux.HandleFunc("/api/fleet", s.handleFleet)
	mux.HandleFunc("/api/apps", s.handleApps)
	mux.HandleFunc("/api/apps/pending", s.handlePending)
	mux.HandleFunc("/api/history", s.handleHistory)
	mux.HandleFunc("/api/icons/", s.handleIcon)
	mux.HandleFunc("/api/ws", func(w http.ResponseWriter, r *http.Request) {
		s.Hub.ServeWS(w, r, s.Store.Snapshot)
	})
	if s.UI != nil {
		fileServer := http.FileServer(http.FS(s.UI))
		mux.Handle("/", spaHandler(fileServer, s.UI))
	}
	return mux
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]any{"ok": true, "updated_at": s.Store.Snapshot().UpdatedAt})
}

func (s *Server) handleFleet(w http.ResponseWriter, r *http.Request) {
	snap := s.Store.Snapshot()
	writeJSON(w, snap)
}

func (s *Server) handleApps(w http.ResponseWriter, r *http.Request) {
	defs := s.Defs()
	list := make([]*apps.Def, 0, len(defs))
	for _, d := range defs {
		list = append(list, d)
	}
	writeJSON(w, list)
}

func (s *Server) handlePending(w http.ResponseWriter, r *http.Request) {
	snap := s.Store.Snapshot()
	var pending []any
	for _, a := range snap.Apps {
		if a.NeedsKey || a.Error != "" {
			pending = append(pending, a)
		}
	}
	writeJSON(w, map[string]any{
		"pending": pending,
		"unknown": snap.Unknown,
	})
}

func (s *Server) handleHistory(w http.ResponseWriter, r *http.Request) {
	host := r.URL.Query().Get("host")
	metric := r.URL.Query().Get("metric")
	hours := 24
	cfg := s.Cfg()
	if cfg != nil && cfg.HistoryHours > 0 {
		hours = cfg.HistoryHours
	}
	since := time.Now().Add(-time.Duration(hours) * time.Hour)
	pts, err := s.Store.History(host, metric, since)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	writeJSON(w, pts)
}

func (s *Server) handleLayout(w http.ResponseWriter, r *http.Request) {
	cfg := s.Cfg()
	snap := s.Store.Snapshot()
	hist := map[string][]store.Point{}
	for name, h := range snap.Hosts {
		if pts, err := s.Store.History(name, "cpu", time.Now().Add(-24*time.Hour)); err == nil {
			hist[name+":cpu"] = pts
		}
		_ = h
	}
	repo := s.RepoURL
	if repo == "" {
		repo = "https://github.com/puvvadi/dash"
	}
	payload := map[string]any{
		"categories": cfg.Categories,
		"groups":     cfg.Groups,
		"layout":     cfg.Layout,
		"snapshot":   snap,
		"history":    hist,
		"brand":      map[string]string{"name": "homelab", "tagline": "A little lab. A lot of possibility."},
		"read_only":  true,
		"repo_url":   repo,
		"app_request_url": repo + "/issues/new?template=app_request.md",
		"poll_seconds": cfg.PollSeconds,
	}
	writeJSON(w, payload)
}

func (s *Server) handleIcon(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, "/api/icons/")
	name = strings.TrimSpace(name)
	if name == "" {
		http.NotFound(w, r)
		return
	}
	// Prefer Dashboard Icons, then Simple Icons
	candidates := []string{
		"https://cdn.jsdelivr.net/gh/homarr-labs/dashboard-icons/svg/" + name + ".svg",
		"https://cdn.jsdelivr.net/gh/homarr-labs/dashboard-icons/png/" + name + ".png",
		"https://cdn.jsdelivr.net/npm/simple-icons@v11/icons/" + strings.ToLower(name) + ".svg",
	}
	if strings.HasPrefix(name, "si:") {
		candidates = []string{"https://cdn.jsdelivr.net/npm/simple-icons@v11/icons/" + strings.TrimPrefix(name, "si:") + ".svg"}
	}
	client := &http.Client{Timeout: 8 * time.Second}
	for _, u := range candidates {
		req, _ := http.NewRequest(http.MethodGet, u, nil)
		resp, err := client.Do(req)
		if err != nil || resp.StatusCode != 200 {
			if resp != nil {
				resp.Body.Close()
			}
			continue
		}
		defer resp.Body.Close()
		ct := resp.Header.Get("Content-Type")
		if ct == "" {
			if strings.HasSuffix(u, ".svg") {
				ct = "image/svg+xml"
			} else {
				ct = "image/png"
			}
		}
		w.Header().Set("Content-Type", ct)
		w.Header().Set("Cache-Control", "public, max-age=86400")
		buf := make([]byte, 0, 4096)
		tmp := make([]byte, 4096)
		for {
			n, err := resp.Body.Read(tmp)
			if n > 0 {
				buf = append(buf, tmp[:n]...)
			}
			if err != nil {
				break
			}
		}
		_, _ = w.Write(buf)
		return
	}
	// placeholder SVG
	w.Header().Set("Content-Type", "image/svg+xml")
	_, _ = w.Write([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 64 64"><rect width="64" height="64" rx="12" fill="#2a2f3a"/><circle cx="32" cy="32" r="10" fill="#6ee7b7"/></svg>`))
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)
}

func spaHandler(fileServer http.Handler, uifs fs.FS) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			http.NotFound(w, r)
			return
		}
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}
		if _, err := fs.Stat(uifs, path); err == nil {
			fileServer.ServeHTTP(w, r)
			return
		}
		r2 := r.Clone(r.Context())
		r2.URL = cloneURL(r.URL, "/index.html")
		fileServer.ServeHTTP(w, r2)
	})
}

func cloneURL(u *url.URL, path string) *url.URL {
	c := *u
	c.Path = path
	return &c
}
