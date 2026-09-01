package httpx

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

type fakeCounter struct {
	calls int
	token string
	err   error
}

func (f *fakeCounter) Increment(token string) error {
	f.calls++
	f.token = token
	return f.err
}

func TestValidGetAndPostIncrement(t *testing.T) {
	methods := []string{http.MethodGet, http.MethodPost}
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			fc := &fakeCounter{}
			h := New(Config{PathPrefix: "/x/", MinToken: 20, MaxToken: 96}, fc, slog.New(slog.NewTextHandler(io.Discard, nil)))
			token := "abcdefghijklmnopqrstuvwx"

			req := httptest.NewRequest(method, "/x/"+token, nil)
			res := httptest.NewRecorder()
			h.ServeHTTP(res, req)

			if res.Code != http.StatusNoContent {
				t.Fatalf("status = %d, want %d", res.Code, http.StatusNoContent)
			}
			if res.Body.Len() != 0 {
				t.Fatalf("expected empty body, got %q", res.Body.String())
			}
			if fc.calls != 1 || fc.token != token {
				t.Fatalf("counter calls=%d token=%q", fc.calls, fc.token)
			}
		})
	}
}

func TestInvalidPublicRequestsStillReturn204(t *testing.T) {
	tests := []struct {
		name   string
		method string
		path   string
	}{
		{"wrong path", http.MethodGet, "/nope/abcdefghijklmnopqrstuvwx"},
		{"short token", http.MethodGet, "/x/abc"},
		{"nested token", http.MethodPost, "/x/abcdefghijklmnopqrstuvwx/more"},
		{"invalid chars", http.MethodPost, "/x/abcdefghijklmnopqrstu!xx"},
		{"other method", http.MethodDelete, "/x/abcdefghijklmnopqrstuvwx"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fc := &fakeCounter{}
			h := New(Config{PathPrefix: "/x/", MinToken: 20, MaxToken: 96}, fc, slog.New(slog.NewTextHandler(io.Discard, nil)))
			req := httptest.NewRequest(tt.method, tt.path, nil)
			res := httptest.NewRecorder()
			h.ServeHTTP(res, req)

			if res.Code != http.StatusNoContent {
				t.Fatalf("status = %d, want %d", res.Code, http.StatusNoContent)
			}
			if res.Body.Len() != 0 {
				t.Fatalf("expected empty body")
			}
			if fc.calls != 0 {
				t.Fatalf("invalid request incremented counter %d times", fc.calls)
			}
		})
	}
}
