package server

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newTestRouter() http.Handler {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewRouter(logger)
}

func TestRouter(t *testing.T) {
	cases := []struct {
		name           string
		method         string
		path           string
		wantStatus     int
		wantBodyIncl   string
		wantContentCT  string
	}{
		{
			name:          "healthz GET returns ok",
			method:        http.MethodGet,
			path:          "/healthz",
			wantStatus:    http.StatusOK,
			wantBodyIncl:  `"status":"ok"`,
			wantContentCT: "application/json",
		},
		{
			name:       "healthz POST is method not allowed",
			method:     http.MethodPost,
			path:       "/healthz",
			wantStatus: http.StatusMethodNotAllowed,
		},
		{
			name:       "unknown route is 404",
			method:     http.MethodGet,
			path:       "/does-not-exist",
			wantStatus: http.StatusNotFound,
		},
	}

	r := newTestRouter()

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			rr := httptest.NewRecorder()
			r.ServeHTTP(rr, req)

			if rr.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d", rr.Code, tc.wantStatus)
			}
			if tc.wantBodyIncl != "" {
				body, _ := io.ReadAll(rr.Body)
				if !strings.Contains(string(body), tc.wantBodyIncl) {
					t.Fatalf("body = %q, want contains %q", string(body), tc.wantBodyIncl)
				}
			}
			if tc.wantContentCT != "" {
				if ct := rr.Header().Get("Content-Type"); ct != tc.wantContentCT {
					t.Fatalf("content-type = %q, want %q", ct, tc.wantContentCT)
				}
			}
		})
	}
}

func TestRequestIDHeaderIsSet(t *testing.T) {
	r := newTestRouter()

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if got := rr.Header().Get("X-Request-Id"); got == "" {
		t.Fatalf("expected X-Request-Id header to be set by middleware")
	}
}
