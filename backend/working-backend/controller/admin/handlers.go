package admin

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"controller/state"
)

type Server struct {
	Tokens    *state.TokenStore
	Reg       *state.Registry
	Tunnelers *state.TunnelerStatusRegistry

	AdminAuthToken    string
	InternalAuthToken string
}

func (s *Server) RegisterRoutes(mux *http.ServeMux) {
	mux.Handle("/api/admin/tokens", s.adminAuth(http.HandlerFunc(s.handleCreateToken)))
	mux.Handle("/api/admin/connectors", s.adminAuth(http.HandlerFunc(s.handleListConnectors)))
	mux.Handle("/api/admin/tunnelers", s.adminAuth(http.HandlerFunc(s.handleListTunnelers)))
	mux.Handle("/api/internal/consume-token", s.internalAuth(http.HandlerFunc(s.handleConsumeToken)))
}

func (s *Server) adminAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.AdminAuthToken == "" {
			http.Error(w, "admin auth not configured", http.StatusServiceUnavailable)
			return
		}
		auth := r.Header.Get("Authorization")
		if auth != "Bearer "+s.AdminAuthToken {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) internalAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.InternalAuthToken == "" {
			http.Error(w, "internal auth not configured", http.StatusServiceUnavailable)
			return
		}
		if r.Header.Get("X-Internal-Token") != s.InternalAuthToken {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) handleCreateToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	token, expires, err := s.Tokens.CreateToken()
	if err != nil {
		http.Error(w, "failed to create token", http.StatusInternalServerError)
		return
	}

	resp := map[string]string{
		"token":      token,
		"expires_at": expires.UTC().Format(time.RFC3339),
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleConsumeToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Token       string `json:"token"`
		ConnectorID string `json:"connector_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if req.ConnectorID == "" {
		http.Error(w, "missing connector_id", http.StatusBadRequest)
		return
	}
	if err := s.Tokens.ConsumeToken(req.Token, req.ConnectorID); err != nil {
		http.Error(w, fmt.Sprintf("token invalid: %v", err), http.StatusUnauthorized)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleListConnectors(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	records := s.Reg.List()
	now := time.Now().UTC()
	type respConnector struct {
		ID        string `json:"id"`
		Status    string `json:"status"`
		PrivateIP string `json:"private_ip"`
		LastSeen  string `json:"last_seen"`
		Version   string `json:"version"`
	}
	resp := make([]respConnector, 0, len(records))
	for _, rec := range records {
		status := "OFFLINE"
		if now.Sub(rec.LastSeen) < 30*time.Second {
			status = "ONLINE"
		}
		resp = append(resp, respConnector{
			ID:        rec.ID,
			Status:    status,
			PrivateIP: rec.PrivateIP,
			LastSeen:  humanizeDuration(now.Sub(rec.LastSeen)),
			Version:   rec.Version,
		})
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleListTunnelers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.Tunnelers == nil {
		writeJSON(w, http.StatusOK, []interface{}{})
		return
	}
	records := s.Tunnelers.List()
	now := time.Now().UTC()
	type respTunneler struct {
		ID          string `json:"id"`
		Status      string `json:"status"`
		ConnectorID string `json:"connector_id"`
		LastSeen    string `json:"last_seen"`
	}
	resp := make([]respTunneler, 0, len(records))
	for _, rec := range records {
		status := "OFFLINE"
		if now.Sub(rec.LastSeen) < 30*time.Second {
			status = "ONLINE"
		}
		resp = append(resp, respTunneler{
			ID:          rec.ID,
			Status:      status,
			ConnectorID: rec.ConnectorID,
			LastSeen:    humanizeDuration(now.Sub(rec.LastSeen)),
		})
	}
	writeJSON(w, http.StatusOK, resp)
}

func writeJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func humanizeDuration(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	seconds := int(d.Seconds())
	switch {
	case seconds < 5:
		return "just now"
	case seconds < 60:
		return fmt.Sprintf("%d seconds ago", seconds)
	case seconds < 3600:
		return fmt.Sprintf("%d minutes ago", seconds/60)
	case seconds < 86400:
		return fmt.Sprintf("%d hours ago", seconds/3600)
	default:
		return fmt.Sprintf("%d days ago", seconds/86400)
	}
}
