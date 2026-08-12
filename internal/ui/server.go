package ui

import (
	"embed"
	"encoding/json"
	"io/fs"
	"net/http"

	"github.com/maknihamdi/fluxexp/internal/engine"
	"github.com/maknihamdi/fluxexp/internal/resolver"
)

//go:embed web/*
var webAssets embed.FS

// Handler builds the portal's HTTP handler: the JSON API under /api and the
// embedded single-page app at /.
func Handler(s *Service) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/contexts", s.handleContexts)
	mux.HandleFunc("/api/roots", s.handleRoots)
	mux.HandleFunc("/api/expand", s.handleExpand)

	sub, err := fs.Sub(webAssets, "web")
	if err != nil {
		panic(err) // embed path is a build-time constant; cannot fail at runtime
	}
	mux.Handle("/", http.FileServer(http.FS(sub)))
	return mux
}

func (s *Service) handleContexts(w http.ResponseWriter, r *http.Request) {
	ctxs, err := s.Contexts()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, map[string]interface{}{"contexts": ctxs})
}

func (s *Service) handleRoots(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	roots, err := s.Roots(q.Get("context"), q.Get("namespace"))
	if err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, map[string]interface{}{"roots": roots})
}

func (s *Service) handleExpand(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	domain := q.Get("domain")
	if domain == "" {
		domain = resolver.DomainK8s
	}
	if q.Get("type") == "" || q.Get("name") == "" {
		writeError(w, http.StatusBadRequest, errBadRequest("type and name are required"))
		return
	}
	ref := engine.Ref{
		Domain: domain,
		Type:   q.Get("type"),
		Coords: map[string]string{"namespace": q.Get("ns"), "name": q.Get("name")},
	}
	node, err := s.Expand(q.Get("context"), ref)
	if err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, node)
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, code int, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
}

type errBadRequest string

func (e errBadRequest) Error() string { return string(e) }
