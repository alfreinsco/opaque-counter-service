package httpx

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

type fakeCounter struct {
	calls  int
	token  string
	err    error
	count  int64
	getErr error
	all    map[string]int64
	allErr error
}

func (f *fakeCounter) Get(token string) (int64, error) {
	f.token = token
	return f.count, f.getErr
}

func (f *fakeCounter) All() (map[string]int64, error) {
	return f.all, f.allErr
}

func (f *fakeCounter) Increment(token string) error {
	f.calls++
	f.token = token
	return f.err
}

func TestGetStats(t *testing.T) {
	fc := &fakeCounter{all: map[string]int64{"token-b": 7, "token-a": 3}}
	h := New(Config{StatsPath: "/stats"}, fc, slog.New(slog.NewTextHandler(io.Discard, nil)))
	req := httptest.NewRequest(http.MethodGet, "/stats", nil)
	res := httptest.NewRecorder()

	h.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusOK)
	}
	if got := res.Header().Get("Content-Type"); got != "application/json; charset=utf-8" {
		t.Fatalf("content type = %q", got)
	}
	var body statsResponse
	if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Total != 2 || len(body.Counters) != 2 {
		t.Fatalf("response = %+v", body)
	}
	if body.Counters[0].Token != "token-a" || body.Counters[0].Count != 3 {
		t.Fatalf("first counter = %+v", body.Counters[0])
	}
	if fc.calls != 0 {
		t.Fatalf("stats request incremented counter %d times", fc.calls)
	}
}

func TestGetStatsRedisError(t *testing.T) {
	fc := &fakeCounter{allErr: errors.New("redis unavailable")}
	h := New(Config{StatsPath: "/stats"}, fc, slog.New(slog.NewTextHandler(io.Discard, nil)))
	res := httptest.NewRecorder()

	h.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/stats", nil))

	if res.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusInternalServerError)
	}
}

func TestGetOneCount(t *testing.T) {
	token := "abcdefghijklmnopqrstuvwx"
	fc := &fakeCounter{count: 7}
	h := New(Config{StatsPath: "/stats"}, fc, slog.New(slog.NewTextHandler(io.Discard, nil)))
	res := httptest.NewRecorder()

	h.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/stats/"+token, nil))

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusOK)
	}
	if got := res.Header().Get("Content-Type"); got != "application/json; charset=utf-8" {
		t.Fatalf("content type = %q", got)
	}
	if got := res.Body.String(); got != "7\n" {
		t.Fatalf("body = %q, want %q", got, "7\\n")
	}
	if fc.token != token {
		t.Fatalf("token = %q, want %q", fc.token, token)
	}
	if fc.calls != 0 {
		t.Fatalf("stats request incremented counter %d times", fc.calls)
	}
}

func TestViewOneCount(t *testing.T) {
	token := "abcdefghijklmnopqrstuvwx"
	fc := &fakeCounter{count: 11}
	h := New(Config{ViewPath: "/v"}, fc, slog.New(slog.NewTextHandler(io.Discard, nil)))
	res := httptest.NewRecorder()

	h.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/v/"+token, nil))

	if res.Code != http.StatusOK || res.Body.String() != "11\n" {
		t.Fatalf("status = %d body = %q", res.Code, res.Body.String())
	}
	if fc.token != token {
		t.Fatalf("token = %q, want %q", fc.token, token)
	}
	if fc.calls != 0 {
		t.Fatalf("view request incremented counter %d times", fc.calls)
	}
}

func TestViewRootDoesNotListCounters(t *testing.T) {
	for _, path := range []string{"/v", "/v/"} {
		t.Run(path, func(t *testing.T) {
			fc := &fakeCounter{all: map[string]int64{"abcdefghijklmnopqrstuvwx": 9}}
			h := New(Config{ViewPath: "/v"}, fc, slog.New(slog.NewTextHandler(io.Discard, nil)))
			res := httptest.NewRecorder()

			h.ServeHTTP(res, httptest.NewRequest(http.MethodGet, path, nil))

			if res.Code != http.StatusNoContent || res.Body.Len() != 0 {
				t.Fatalf("status = %d body = %q", res.Code, res.Body.String())
			}
			if fc.token != "" || fc.calls != 0 {
				t.Fatalf("view root accessed counter: token=%q calls=%d", fc.token, fc.calls)
			}
		})
	}
}

func TestGetOneMissingCountReturnsZero(t *testing.T) {
	token := "abcdefghijklmnopqrstuvwx"
	fc := &fakeCounter{}
	h := New(Config{StatsPath: "/stats"}, fc, slog.New(slog.NewTextHandler(io.Discard, nil)))
	res := httptest.NewRecorder()

	h.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/stats/"+token, nil))

	if res.Code != http.StatusOK || res.Body.String() != "0\n" {
		t.Fatalf("status = %d body = %q", res.Code, res.Body.String())
	}
}

func TestGetOneCountRedisError(t *testing.T) {
	token := "abcdefghijklmnopqrstuvwx"
	fc := &fakeCounter{getErr: errors.New("redis unavailable")}
	h := New(Config{StatsPath: "/stats"}, fc, slog.New(slog.NewTextHandler(io.Discard, nil)))
	res := httptest.NewRecorder()

	h.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/stats/"+token, nil))

	if res.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusInternalServerError)
	}
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
