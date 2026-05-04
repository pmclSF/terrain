package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/pmclSF/terrain/internal/reporting"
)

// handleRoot serves the HTML analysis report with auto-refresh.
func (s *Server) handleRoot(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	_, report, err := s.getResult(r.Context())
	if err != nil {
		// If the client disconnected, http.Error writes are best-effort
		// — drop them rather than logging a confusing 500.
		if r.Context().Err() != nil {
			return
		}
		http.Error(w, "Analysis failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	var buf bytes.Buffer
	if err := reporting.RenderAnalyzeHTML(&buf, report); err != nil {
		http.Error(w, "Render failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Inject auto-refresh script before </body>.
	html := buf.String()
	refreshScript := `<script>setTimeout(function(){location.reload()},30000)</script>`
	html = strings.Replace(html, "</body>", refreshScript+"</body>", 1)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Write([]byte(html))
}

// handleHealth returns a simple health check.
func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"ok"}`))
}

// handleAnalyze returns the analysis report as JSON.
func (s *Server) handleAnalyze(w http.ResponseWriter, r *http.Request) {
	_, report, err := s.getResult(r.Context())
	if err != nil {
		if r.Context().Err() != nil {
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.Encode(report)
}
