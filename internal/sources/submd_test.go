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

func submdFixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "submd", name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// submdRun serves h, points the adapter at it and returns the in-scope
// normalized names plus the number of requests served.
func submdRun(t *testing.T, h http.HandlerFunc, creds map[string]string, maxPages int) ([]string, int, error) {
	t.Helper()
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		h(w, r)
	}))
	defer srv.Close()
	old := submdBaseURL
	submdBaseURL = srv.URL
	defer func() { submdBaseURL = old }()
	s := &Session{
		HTTP:     httpx.New(httpx.Options{SourceName: "submd", TargetDomain: "example.com", MaxRetries: -1, BackoffBase: time.Millisecond}),
		Creds:    creds,
		MaxPages: maxPages,
	}
	set := map[string]bool{}
	err := submd{}.Enumerate(context.Background(), "example.com", s, func(raw string) {
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

func submdSame(a, b []string) bool {
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

func submdServe(body []byte) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) { w.Write(body) }
}

func submdStatus(code int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(code) }
}

func Test_submd_Info(t *testing.T) {
	i := submd{}.Info()
	if i.Name != "submd" || i.Disabled || i.Auth != AuthOptional || i.Default != true {
		t.Fatalf("unexpected info: %+v", i)
	}
	if _, ok := Get("submd"); !ok {
		t.Fatal("not registered")
	}
}

func Test_submd_HTTPErrors(t *testing.T) {
	for _, c := range []struct {
		code int
		want error
	}{{401, ErrAuth}, {403, ErrAuth}, {429, ErrRateLimited}, {503, ErrUnavailable}, {400, ErrUnexpected}} {
		_, _, err := submdRun(t, submdStatus(c.code), map[string]string{}, 10)
		if !errors.Is(err, c.want) {
			t.Errorf("HTTP %d: err=%v want %v", c.code, err, c.want)
		}
	}
}

func Test_submd_Success(t *testing.T) {
	h := func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" || r.URL.Path != "/v1/search" || r.URL.Query().Get("apex") != "example.com" {
			t.Errorf("bad request %s %s", r.Method, r.URL.String())
		}
		if r.Header.Get("Authorization") != "" {
			t.Errorf("Authorization sent without a key")
		}
		w.Write(submdFixture(t, "ok.txt"))
	}
	got, hits, err := submdRun(t, h, map[string]string{}, 10)
	want := []string{"api.example.com", "mail.example.com", "wild.example.com", "www.example.com"}
	if err != nil || hits != 1 || !submdSame(got, want) {
		t.Fatalf("err=%v hits=%d got=%v", err, hits, got)
	}
}

func Test_submd_BearerWhenKeyed(t *testing.T) {
	h := func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer K1" {
			t.Errorf("Authorization = %q", r.Header.Get("Authorization"))
		}
	}
	got, hits, err := submdRun(t, h, map[string]string{"api_key": "K1"}, 10)
	if err != nil || hits != 1 || len(got) != 0 { // empty body = no results
		t.Fatalf("err=%v hits=%d got=%v", err, hits, got)
	}
}

func Test_submd_SingleRequestEvenWithManyPages(t *testing.T) {
	_, hits, err := submdRun(t, submdServe(submdFixture(t, "ok.txt")), nil, 10)
	if err != nil || hits != 1 {
		t.Fatalf("err=%v hits=%d", err, hits)
	}
}
