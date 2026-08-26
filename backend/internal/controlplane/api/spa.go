package api

import (
	"bytes"
	"io/fs"
	"net/http"
	pathpkg "path"
	"strings"
)

func (s *Server) fallbackHandler() http.Handler {
	if s.webFS == nil {
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
		requestedPath := strings.TrimPrefix(pathpkg.Clean("/"+strings.TrimPrefix(request.URL.Path, "/")), "/")
		if requestedPath == "." || requestedPath == "" {
			requestedPath = "index.html"
		}
		if !fs.ValidPath(requestedPath) {
			s.handleNotFound(w, request)
			return
		}
		if info, err := fs.Stat(s.webFS, requestedPath); err == nil && !info.IsDir() {
			if strings.HasPrefix(request.URL.Path, "/assets/") {
				w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			}
			s.serveWebFile(w, request, requestedPath, info)
			return
		}
		if strings.HasPrefix(request.URL.Path, "/assets/") || pathpkg.Ext(request.URL.Path) != "" {
			s.handleNotFound(w, request)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		indexInfo, err := fs.Stat(s.webFS, "index.html")
		if err != nil {
			s.handleNotFound(w, request)
			return
		}
		s.serveWebFile(w, request, "index.html", indexInfo)
	})
}

func (s *Server) serveWebFile(w http.ResponseWriter, request *http.Request, name string, info fs.FileInfo) {
	content, err := fs.ReadFile(s.webFS, name)
	if err != nil {
		s.handleNotFound(w, request)
		return
	}
	http.ServeContent(w, request, name, info.ModTime(), bytes.NewReader(content))
}
