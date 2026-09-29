package sources

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lunalully/lunatic/internal/httpx"
	"github.com/lunalully/lunatic/internal/scope"
)

func urlscanFixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "urlscan", name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// urlscanRun serves h, points the adapter at it and returns the in-scope
// normalized names plus the number of requests served.
func urlscanRun(t *testing.T, h http.HandlerFunc, creds map[string]string, maxPages int) ([]string, int, error) {
	t.Helper()
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		h(w, r)
	}))
	defer srv.Close()
	old := urlscanBaseURL
	urlscanBaseURL = srv.URL
	defer func() { urlscanBaseURL = old }()
	s := &Session{
		HTTP:     httpx.New(httpx.Options{SourceName: "urlscan", TargetDomain: "example.com", MaxRetries: -1, BackoffBase: time.Millisecond}),
		Creds:    creds,
		MaxPages: maxPages,
	}
	set := map[string]bool{}
	err := urlscan{}.Enumerate(context.Background(), "example.com", s, func(raw string) {
		if n, ok := scope.Normalize(raw, "example.com"); ok {
			set[n] = true
		}
	})
	var out []string
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out, int(atomic.LoadInt32(&hits)), err
}

func urlscanSame(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func urlscanServe(body []byte) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) { w.Write(body) }
}

func urlscanStatus(code int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(code) }
}

// urlscanPages serves page1/page2 fixtures; check inspects the request (credential
// placement, method, path, body) and returns the page number to serve.
func urlscanPages(t *testing.T) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		page := urlscanCheck(t, r)
		w.Write(urlscanFixture(t, fmt.Sprintf("page%d.json", page)))
	}
}

func Test_urlscan_Info(t *testing.T) {
	i := urlscan{}.Info()
	if i.Name != "urlscan" || i.Disabled || i.Auth != AuthOptional || i.Default != true {
		t.Fatalf("unexpected info: %+v", i)
	}
	if _, ok := Get("urlscan"); !ok {
		t.Fatal("not registered")
	}
}

func Test_urlscan_Success(t *testing.T) {
	got, hits, err := urlscanRun(t, urlscanPages(t), map[string]string{"api_key": "K1"}, 10)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"api.example.com", "shop.example.com", "www.example.com"}
	if !urlscanSame(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
	if hits != 2 {
		t.Fatalf("hits = %d, want 2", hits)
	}
}

func Test_urlscan_MaxPages(t *testing.T) {
	got, hits, err := urlscanRun(t, urlscanPages(t), map[string]string{"api_key": "K1"}, 1)
	if err != nil {
		t.Fatal(err)
	}
	if hits != 1 || !urlscanSame(got, []string{"shop.example.com", "www.example.com"}) {
		t.Fatalf("hits=%d got=%v", hits, got)
	}
}

func Test_urlscan_Empty(t *testing.T) {
	got, _, err := urlscanRun(t, urlscanServe(urlscanFixture(t, "empty.json")), map[string]string{"api_key": "K1"}, 10)
	if err != nil || len(got) != 0 {
		t.Fatalf("err=%v got=%v", err, got)
	}
}

func Test_urlscan_HTTPErrors(t *testing.T) {
	for _, c := range []struct {
		code int
		want error
	}{{401, ErrAuth}, {403, ErrAuth}, {429, ErrRateLimited}, {503, ErrUnavailable}, {400, ErrUnexpected}} {
		_, _, err := urlscanRun(t, urlscanStatus(c.code), map[string]string{"api_key": "K1"}, 10)
		if !errors.Is(err, c.want) {
			t.Errorf("HTTP %d: err=%v want %v", c.code, err, c.want)
		}
	}
}

func Test_urlscan_ErrorInBody(t *testing.T) {
	for _, c := range []struct {
		file string
		want error
	}{{"error_auth.json", ErrAuth}, {"error_rate.json", ErrRateLimited}, {"error_other.json", ErrUnexpected}} {
		_, _, err := urlscanRun(t, urlscanServe(urlscanFixture(t, c.file)), map[string]string{"api_key": "K1"}, 10)
		if !errors.Is(err, c.want) {
			t.Errorf("%s: err=%v want %v", c.file, err, c.want)
		}
	}
}

func Test_urlscan_InvalidBody(t *testing.T) {
	_, _, err := urlscanRun(t, urlscanServe([]byte("<html>not json")), map[string]string{"api_key": "K1"}, 10)
	if !errors.Is(err, ErrUnexpected) {
		t.Fatalf("err=%v", err)
	}
}

// The adapter must only ever GET the search endpoint, never submit scans.
func Test_urlscan_OnlySearchEndpoint(t *testing.T) {
	h := func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/v1/search/" {
			t.Errorf("forbidden request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(500)
			return
		}
		if r.URL.Query().Get("search_after") != "" {
			w.Write(urlscanFixture(t, "page2.json"))
			return
		}
		w.Write(urlscanFixture(t, "page1.json"))
	}
	if _, _, err := urlscanRun(t, h, map[string]string{}, 10); err != nil {
		t.Fatal(err)
	}
}

func Test_urlscan_AnonymousNoKeyHeader(t *testing.T) {
	h := func(w http.ResponseWriter, r *http.Request) {
		if _, ok := r.Header["Api-Key"]; ok {
			t.Errorf("API-Key sent without a configured key")
		}
		w.Write(urlscanFixture(t, "empty.json"))
	}
	got, _, err := urlscanRun(t, h, map[string]string{}, 10)
	if err != nil || len(got) != 0 {
		t.Fatalf("err=%v got=%v", err, got)
	}
}

func urlscanCheck(t *testing.T, r *http.Request) int {
	q := r.URL.Query()
	if r.Method != http.MethodGet || r.URL.Path != "/api/v1/search/" || q.Get("q") != "domain:example.com" {
		t.Errorf("bad request %s %s", r.Method, r.URL.String())
	}
	if r.Header.Get("API-Key") != "K1" {
		t.Errorf("API-Key header missing: %q", r.Header.Get("API-Key"))
	}
	if q.Get("search_after") == "1700000001000,a2" {
		return 2
	}
	if q.Get("search_after") != "" {
		t.Errorf("unexpected search_after %q", q.Get("search_after"))
	}
	return 1
}
