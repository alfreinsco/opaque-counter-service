package httpx

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"sort"
	"strings"
)

type Counter interface {
	Increment(token string) error
	Get(token string) (int64, error)
	All() (map[string]int64, error)
}

type Config struct {
	PathPrefix string
	StatsPath  string
	ViewPath   string
	MinToken   int
	MaxToken   int
}

type Handler struct {
	cfg     Config
	counter Counter
	logger  *slog.Logger
}

func New(cfg Config, counter Counter, logger *slog.Logger) http.Handler {
	if cfg.PathPrefix == "" {
		cfg.PathPrefix = "/x/"
	}
	if !strings.HasPrefix(cfg.PathPrefix, "/") {
		cfg.PathPrefix = "/" + cfg.PathPrefix
	}
	if !strings.HasSuffix(cfg.PathPrefix, "/") {
		cfg.PathPrefix += "/"
	}
	if cfg.StatsPath == "" {
		cfg.StatsPath = "/stats"
	}
	if !strings.HasPrefix(cfg.StatsPath, "/") {
		cfg.StatsPath = "/" + cfg.StatsPath
	}
	if cfg.ViewPath == "" {
		cfg.ViewPath = "/v"
	}
	if !strings.HasPrefix(cfg.ViewPath, "/") {
		cfg.ViewPath = "/" + cfg.ViewPath
	}
	cfg.ViewPath = strings.TrimSuffix(cfg.ViewPath, "/")
	if cfg.MinToken <= 0 {
		cfg.MinToken = 20
	}
	if cfg.MaxToken < cfg.MinToken {
		cfg.MaxToken = 96
	}

	return &Handler{cfg: cfg, counter: counter, logger: logger}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Prevent GET responses from being served from browser/proxy caches.
	w.Header().Set("Cache-Control", "no-store, no-cache, max-age=0")
	w.Header().Set("Pragma", "no-cache")

	if r.Method == http.MethodGet && r.URL.Path == h.cfg.StatsPath {
		h.serveStats(w)
		return
	}
	if r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, h.cfg.StatsPath+"/") {
		token := strings.TrimPrefix(r.URL.Path, h.cfg.StatsPath+"/")
		if h.validToken(token) {
			h.serveCount(w, token)
			return
		}
	}
	if r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, h.cfg.ViewPath+"/") {
		token := strings.TrimPrefix(r.URL.Path, h.cfg.ViewPath+"/")
		if h.validToken(token) {
			h.serveCount(w, token)
			return
		}
	}

	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	token, ok := h.extractToken(r.URL.Path)
	if !ok {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if err := h.counter.Increment(token); err != nil {
		// Error details stay server-side. Client still receives 204.
		h.logger.Error("counter_increment_failed", "error", err)
	}
	w.WriteHeader(http.StatusNoContent)
}

type counterEntry struct {
	Token string `json:"token"`
	Count int64  `json:"count"`
}

type statsResponse struct {
	Counters []counterEntry `json:"counters"`
	Total    int            `json:"total"`
}

func (h *Handler) serveStats(w http.ResponseWriter) {
	counters, err := h.counter.All()
	if err != nil {
		h.logger.Error("counter_list_failed", "error", err)
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("{\"error\":\"failed to read counters\"}\n"))
		return
	}

	entries := make([]counterEntry, 0, len(counters))
	for token, count := range counters {
		entries = append(entries, counterEntry{Token: token, Count: count})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Token < entries[j].Token })

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if err := json.NewEncoder(w).Encode(statsResponse{Counters: entries, Total: len(entries)}); err != nil {
		h.logger.Error("counter_list_response_failed", "error", err)
	}
}

func (h *Handler) serveCount(w http.ResponseWriter, token string) {
	count, err := h.counter.Get(token)
	if err != nil {
		h.logger.Error("counter_read_failed", "error", err)
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("{\"error\":\"failed to read counter\"}\n"))
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if err := json.NewEncoder(w).Encode(count); err != nil {
		h.logger.Error("counter_read_response_failed", "error", err)
	}
}

func (h *Handler) extractToken(path string) (string, bool) {
	if !strings.HasPrefix(path, h.cfg.PathPrefix) {
		return "", false
	}

	token := strings.TrimPrefix(path, h.cfg.PathPrefix)
	return token, h.validToken(token)
}

func (h *Handler) validToken(token string) bool {
	if token == "" || strings.Contains(token, "/") {
		return false
	}
	if len(token) < h.cfg.MinToken || len(token) > h.cfg.MaxToken {
		return false
	}

	for i := 0; i < len(token); i++ {
		c := token[i]
		if (c >= 'a' && c <= 'z') ||
			(c >= 'A' && c <= 'Z') ||
			(c >= '0' && c <= '9') ||
			c == '-' || c == '_' {
			continue
		}
		return false
	}

	return true
}
