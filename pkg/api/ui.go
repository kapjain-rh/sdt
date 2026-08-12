package api

import (
	"embed"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

//go:embed all:ui/dist
var uiFiles embed.FS

func (s *Server) EnableUI() {
	distFS, err := fs.Sub(uiFiles, "ui/dist")
	if err != nil {
		return
	}

	entries, _ := fs.ReadDir(distFS, ".")
	if len(entries) == 0 || (len(entries) == 1 && entries[0].Name() == ".gitkeep") {
		return
	}

	s.uiFS = http.FileServer(http.FS(distFS))
	s.uiDist = distFS
}

func (s *Server) serveUI(w http.ResponseWriter, r *http.Request) {
	if s.uiFS == nil {
		http.NotFound(w, r)
		return
	}

	p := strings.TrimPrefix(r.URL.Path, "/")
	if p == "" {
		p = "index.html"
	}

	// For paths without extension, try .html first (Next.js static export: /cases → cases.html)
	if ext := path.Ext(p); ext == "" {
		htmlPath := p + ".html"
		if _, err := fs.Stat(s.uiDist, htmlPath); err == nil {
			r.URL.Path = "/" + htmlPath
			s.uiFS.ServeHTTP(w, r)
			return
		}
		indexPath := p + "/index.html"
		if _, err := fs.Stat(s.uiDist, indexPath); err == nil {
			r.URL.Path = "/" + indexPath
			s.uiFS.ServeHTTP(w, r)
			return
		}
		// Dynamic route fallback: replace numeric path segments with "0"
		// to match Next.js static export (e.g. /runs/42 → runs/0.html)
		if resolved := resolveDynamicRoute(s.uiDist, p); resolved != "" {
			r.URL.Path = "/" + resolved
			s.uiFS.ServeHTTP(w, r)
			return
		}
	}

	// Serve exact file (JS, CSS, images, etc.)
	if info, err := fs.Stat(s.uiDist, p); err == nil && !info.IsDir() {
		s.uiFS.ServeHTTP(w, r)
		return
	}

	// Dynamic route fallback for files with extensions (RSC data files like
	// /runs/2/__next.runs.$d$id.__PAGE__.txt → /runs/0/__next.runs.$d$id.__PAGE__.txt)
	if resolved := resolveDynamicFile(s.uiDist, p); resolved != "" {
		r.URL.Path = "/" + resolved
		s.uiFS.ServeHTTP(w, r)
		return
	}

	// SPA fallback — serve index.html directly (bypass FileServer to avoid redirect)
	s.serveIndexHTML(w)
}

func resolveDynamic(dist fs.FS, p string, suffixes ...string) string {
	segments := strings.Split(p, "/")
	replaced := make([]string, len(segments))
	changed := false
	for i, seg := range segments {
		if isNumeric(seg) {
			replaced[i] = "0"
			changed = true
		} else {
			replaced[i] = seg
		}
	}
	if !changed {
		return ""
	}
	candidate := strings.Join(replaced, "/")
	for _, suffix := range suffixes {
		if fileExists(dist, candidate+suffix) {
			return candidate + suffix
		}
	}
	return ""
}

func resolveDynamicRoute(dist fs.FS, p string) string {
	return resolveDynamic(dist, p, ".html", "/index.html")
}

func resolveDynamicFile(dist fs.FS, p string) string {
	return resolveDynamic(dist, p, "")
}

func isNumeric(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

func fileExists(dist fs.FS, p string) bool {
	info, err := fs.Stat(dist, p)
	return err == nil && !info.IsDir()
}

func (s *Server) serveIndexHTML(w http.ResponseWriter) {
	data, err := fs.ReadFile(s.uiDist, "index.html")
	if err != nil {
		http.Error(w, "index.html not found", 500)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(data)
}
