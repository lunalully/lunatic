package sources

import (
	"context"
	"errors"
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

func threatbookFixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "threatbook", name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// threatbookRun serves h, points the adapter at it and returns the in-scope
// normalized names plus the number of requests served.
func threatbookRun(t *testing.T, h http.HandlerFunc, creds map[string]string, maxPages int) ([]string, int, error) {
	t.Helper()
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		h(w, r)
	}))
	defer srv.Close()
	old := threatbookBaseURL
	threatbookBaseURL = srv.URL
	defer func() { threatbookBaseURL = old }()
	s := &Session{
		HTTP:     httpx.New(httpx.Options{SourceName: "threatbook", TargetDomain: "example.com", MaxRetries: -1, BackoffBase: time.Millisecond}),
		Creds:    creds,
		MaxPages: maxPages,
	}
	set := map[string]bool{}
	err := threatbook{}.Enumerate(context.Background(), "example.com", s, func(raw string) {
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

func threatbookSame(a, b []string) bool {
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

func threatbookServe(body []byte) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) { w.Write(body) }
}

func threatbookStatus(code int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(code) }
}

func Test_threatbook_Info(t *testing.T) {
	i := threatbook{}.Info()
	if i.Name != "threatbook" || i.Disabled || i.Auth != AuthRequired || i.Default != false {
		t.Fatalf("unexpected info: %+v", i)
	}
	if _, ok := Get("threatbook"); !ok {
		t.Fatal("not registered")
	}
}

func Test_threatbook_HTTPErrors(t *testing.T) {
	for _, c := range []struct {
		code int
		want error
	}{{401, ErrAuth}, {403, ErrAuth}, {429, ErrRateLimited}, {503, ErrUnavailable}, {400, ErrUnexpected}} {
		_, _, err := threatbookRun(t, threatbookStatus(c.code), map[string]string{"api_key": "K1"}, 10)
		if !errors.Is(err, c.want) {
			t.Errorf("HTTP %d: err=%v want %v", c.code, err, c.want)
		}
	}
}

func Test_threatbook_Empty(t *testing.T) {
	got, _, err := threatbookRun(t, threatbookServe(threatbookFixture(t, "empty.json")), map[string]string{"api_key": "K1"}, 10)
	if err != nil || len(got) != 0 {
		t.Fatalf("err=%v got=%v", err, got)
	}
}

func Test_threatbook_InvalidBody(t *testing.T) {
	_, _, err := threatbookRun(t, threatbookServe([]byte("<html>not json")), map[string]string{"api_key": "K1"}, 10)
	if !errors.Is(err, ErrUnexpected) {
		t.Fatalf("err=%v", err)
	}
}

func Test_threatbook_MissingKey(t *testing.T) {
	_, hits, err := threatbookRun(t, threatbookServe([]byte("{}")), map[string]string{}, 10)
	if !errors.Is(err, ErrNoKey) || hits != 0 {
		t.Fatalf("err=%v hits=%d", err, hits)
	}
}

func Test_threatbook_Success(t *testing.T) {
	h := func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if r.Method != "GET" || r.URL.Path != "/v3/domain/sub_domains" || q.Get("apikey") != "K1" || q.Get("resource") != "example.com" {
			t.Errorf("bad request %s %s", r.Method, r.URL.String())
		}
		w.Write(threatbookFixture(t, "ok.json"))
	}
	got, hits, err := threatbookRun(t, h, map[string]string{"api_key": "K1"}, 10)
	if err != nil || hits != 1 || !threatbookSame(got, []string{"api.example.com", "wild.example.com", "www.example.com"}) {
		t.Fatalf("err=%v hits=%d got=%v", err, hits, got)
	}
}

func Test_threatbook_ErrorInBody(t *testing.T) {
	for _, c := range []struct {
		file string
		want error
	}{{"error_auth.json", ErrAuth}, {"error_rate.json", ErrRateLimited}, {"error_other.json", ErrUnexpected}} {
		_, _, err := threatbookRun(t, threatbookServe(threatbookFixture(t, c.file)), map[string]string{"api_key": "K1"}, 10)
		if !errors.Is(err, c.want) {
			t.Errorf("%s: err=%v want %v", c.file, err, c.want)
		}
	}
}
