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

func whoisxmlapiFixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "whoisxmlapi", name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// whoisxmlapiRun serves h, points the adapter at it and returns the in-scope
// normalized names plus the number of requests served.
func whoisxmlapiRun(t *testing.T, h http.HandlerFunc, creds map[string]string, maxPages int) ([]string, int, error) {
	t.Helper()
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		h(w, r)
	}))
	defer srv.Close()
	old := whoisxmlapiBaseURL
	whoisxmlapiBaseURL = srv.URL
	defer func() { whoisxmlapiBaseURL = old }()
	s := &Session{
		HTTP:     httpx.New(httpx.Options{SourceName: "whoisxmlapi", TargetDomain: "example.com", MaxRetries: -1, BackoffBase: time.Millisecond}),
		Creds:    creds,
		MaxPages: maxPages,
	}
	set := map[string]bool{}
	err := whoisxmlapi{}.Enumerate(context.Background(), "example.com", s, func(raw string) {
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

func whoisxmlapiSame(a, b []string) bool {
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

func whoisxmlapiServe(body []byte) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) { w.Write(body) }
}

func whoisxmlapiStatus(code int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(code) }
}

// whoisxmlapiPages serves page1/page2 fixtures; check inspects the request (credential
// placement, method, path, body) and returns the page number to serve.
func whoisxmlapiPages(t *testing.T) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		page := whoisxmlapiCheck(t, r)
		w.Write(whoisxmlapiFixture(t, fmt.Sprintf("page%d.json", page)))
	}
}

func Test_whoisxmlapi_Info(t *testing.T) {
	i := whoisxmlapi{}.Info()
	if i.Name != "whoisxmlapi" || i.Disabled || i.Auth != AuthRequired || i.Default != true {
		t.Fatalf("unexpected info: %+v", i)
	}
	if _, ok := Get("whoisxmlapi"); !ok {
		t.Fatal("not registered")
	}
}

func Test_whoisxmlapi_Success(t *testing.T) {
	got, hits, err := whoisxmlapiRun(t, whoisxmlapiPages(t), map[string]string{"api_key": "K1"}, 10)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"api.example.com", "wild.example.com", "www.example.com"}
	if !whoisxmlapiSame(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
	if hits != 2 {
		t.Fatalf("hits = %d, want 2", hits)
	}
}

func Test_whoisxmlapi_MaxPages(t *testing.T) {
	got, hits, err := whoisxmlapiRun(t, whoisxmlapiPages(t), map[string]string{"api_key": "K1"}, 1)
	if err != nil {
		t.Fatal(err)
	}
	if hits != 1 || !whoisxmlapiSame(got, []string{"www.example.com"}) {
		t.Fatalf("hits=%d got=%v", hits, got)
	}
}

func Test_whoisxmlapi_Empty(t *testing.T) {
	got, _, err := whoisxmlapiRun(t, whoisxmlapiServe(whoisxmlapiFixture(t, "empty.json")), map[string]string{"api_key": "K1"}, 10)
	if err != nil || len(got) != 0 {
		t.Fatalf("err=%v got=%v", err, got)
	}
}

func Test_whoisxmlapi_HTTPErrors(t *testing.T) {
	for _, c := range []struct {
		code int
		want error
	}{{401, ErrAuth}, {403, ErrAuth}, {429, ErrRateLimited}, {503, ErrUnavailable}, {400, ErrUnexpected}} {
		_, _, err := whoisxmlapiRun(t, whoisxmlapiStatus(c.code), map[string]string{"api_key": "K1"}, 10)
		if !errors.Is(err, c.want) {
			t.Errorf("HTTP %d: err=%v want %v", c.code, err, c.want)
		}
	}
}

func Test_whoisxmlapi_ErrorInBody(t *testing.T) {
	for _, c := range []struct {
		file string
		want error
	}{{"error_auth.json", ErrAuth}, {"error_rate.json", ErrRateLimited}, {"error_other.json", ErrUnexpected}} {
		_, _, err := whoisxmlapiRun(t, whoisxmlapiServe(whoisxmlapiFixture(t, c.file)), map[string]string{"api_key": "K1"}, 10)
		if !errors.Is(err, c.want) {
			t.Errorf("%s: err=%v want %v", c.file, err, c.want)
		}
	}
}

func Test_whoisxmlapi_InvalidBody(t *testing.T) {
	_, _, err := whoisxmlapiRun(t, whoisxmlapiServe([]byte("<html>not json")), map[string]string{"api_key": "K1"}, 10)
	if !errors.Is(err, ErrUnexpected) {
		t.Fatalf("err=%v", err)
	}
}

func Test_whoisxmlapi_MissingKey(t *testing.T) {
	_, hits, err := whoisxmlapiRun(t, whoisxmlapiServe([]byte("{}")), map[string]string{}, 10)
	if !errors.Is(err, ErrNoKey) || hits != 0 {
		t.Fatalf("err=%v hits=%d", err, hits)
	}
}

func whoisxmlapiCheck(t *testing.T, r *http.Request) int {
	q := r.URL.Query()
	if r.Method != "GET" || r.URL.Path != "/api/v2" || q.Get("apiKey") != "K1" || q.Get("domainName") != "example.com" {
		t.Errorf("bad request %s %s", r.Method, r.URL.String())
	}
	if q.Get("searchAfter") == "CUR2" {
		return 2
	}
	return 1
}
