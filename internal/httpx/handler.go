package httpx

import (
	"log/slog"
	"net/http"
	"strings"
)

type Counter interface {
	Increment(token string) error
}

type Config struct {
	PathPrefix string
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

	// Every public outcome deliberately looks identical.
	defer w.WriteHeader(http.StatusNoContent)

	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		return
	}

	token, ok := h.extractToken(r.URL.Path)
	if !ok {
		return
	}

	if err := h.counter.Increment(token); err != nil {
		// Error details stay server-side. Client still receives 204.
		h.logger.Error("counter_increment_failed", "error", err)
	}
}

func (h *Handler) extractToken(path string) (string, bool) {
	if !strings.HasPrefix(path, h.cfg.PathPrefix) {
		return "", false
	}

	token := strings.TrimPrefix(path, h.cfg.PathPrefix)
	if token == "" || strings.Contains(token, "/") {
		return "", false
	}
	if len(token) < h.cfg.MinToken || len(token) > h.cfg.MaxToken {
		return "", false
	}

	for i := 0; i < len(token); i++ {
		c := token[i]
		if (c >= 'a' && c <= 'z') ||
			(c >= 'A' && c <= 'Z') ||
			(c >= '0' && c <= '9') ||
			c == '-' || c == '_' {
			continue
		}
		return "", false
	}

	return token, true
}
