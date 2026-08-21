package api

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

func (s *Server) fallbackHandler() http.Handler {
	if s.config.WebDir == "" {
		return http.HandlerFunc(s.handleNotFound)
	}
	return http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if strings.HasPrefix(request.URL.Path, "/api/") || request.URL.Path == "/healthz" || request.URL.Path == "/readyz" {
			s.handleNotFound(w, request)
			return
		}
		if request.Method != http.MethodGet && request.Method != http.MethodHead {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed")
			return
		}
		requestedPath := filepath.Join(s.config.WebDir, filepath.Clean(filepath.FromSlash(strings.TrimPrefix(request.URL.Path, "/"))))
		if !insideDirectory(s.config.WebDir, requestedPath) {
			s.handleNotFound(w, request)
			return
		}
		if info, err := os.Stat(requestedPath); err == nil && !info.IsDir() {
			if strings.HasPrefix(request.URL.Path, "/assets/") {
				w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			}
			http.ServeFile(w, request, requestedPath)
			return
		}
		if strings.HasPrefix(request.URL.Path, "/assets/") || filepath.Ext(request.URL.Path) != "" {
			s.handleNotFound(w, request)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		http.ServeFile(w, request, filepath.Join(s.config.WebDir, "index.html"))
	})
}

func insideDirectory(root, path string) bool {
	relative, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) && !filepath.IsAbs(relative)
}
