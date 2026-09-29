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

func waybackarchiveFixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "waybackarchive", name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// waybackarchiveRun serves h, points the adapter at it and returns the in-scope
// normalized names plus the number of requests served.
func waybackarchiveRun(t *testing.T, h http.HandlerFunc, creds map[string]string, maxPages int) ([]string, int, error) {
	t.Helper()
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		h(w, r)
	}))
	defer srv.Close()
	old := waybackarchiveBaseURL
	waybackarchiveBaseURL = srv.URL
	defer func() { waybackarchiveBaseURL = old }()
	s := &Session{
		HTTP:     httpx.New(httpx.Options{SourceName: "waybackarchive", TargetDomain: "example.com", MaxRetries: -1, BackoffBase: time.Millisecond}),
		Creds:    creds,
		MaxPages: maxPages,
	}
	set := map[string]bool{}
	err := waybackarchive{}.Enumerate(context.Background(), "example.com", s, func(raw string) {
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

func waybackarchiveSame(a, b []string) bool {
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

func waybackarchiveServe(body []byte) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) { w.Write(body) }
}

func waybackarchiveStatus(code int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(code) }
}

func Test_waybackarchive_Info(t *testing.T) {
	i := waybackarchive{}.Info()
	if i.Name != "waybackarchive" || i.Disabled || i.Auth != AuthNone || i.Default != true {
		t.Fatalf("unexpected info: %+v", i)
	}
	if _, ok := Get("waybackarchive"); !ok {
		t.Fatal("not registered")
	}
}

func Test_waybackarchive_HTTPErrors(t *testing.T) {
	for _, c := range []struct {
		code int
		want error
	}{{401, ErrAuth}, {403, ErrAuth}, {429, ErrRateLimited}, {503, ErrUnavailable}, {400, ErrUnexpected}} {
		_, _, err := waybackarchiveRun(t, waybackarchiveStatus(c.code), nil, 10)
		if !errors.Is(err, c.want) {
			t.Errorf("HTTP %d: err=%v want %v", c.code, err, c.want)
		}
	}
}

// Only the CDX index may be requested: any other path (notably archived
// pages under /web/<timestamp>/) fails the test.
func waybackarchiveGuard(t *testing.T) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/cdx/search/cdx" {
			t.Errorf("forbidden request: %s %s", r.Method, r.URL.String())
			w.WriteHeader(500)
			return
		}
		q := r.URL.Query()
		if q.Get("url") != "*.example.com" || q.Get("fl") != "original" || q.Get("output") != "txt" || q.Get("limit") == "" {
			t.Errorf("unexpected query %s", r.URL.RawQuery)
		}
		if q.Get("resumeKey") == "RESUME123" {
			w.Write(waybackarchiveFixture(t, "page2.txt"))
			return
		}
		w.Write(waybackarchiveFixture(t, "page1.txt"))
	}
}

func Test_waybackarchive_SuccessAndResumeKey(t *testing.T) {
	got, hits, err := waybackarchiveRun(t, waybackarchiveGuard(t), nil, 10)
	want := []string{"api.example.com", "blog.example.com", "mail.example.com", "shop.example.com", "www.example.com"}
	if err != nil || hits != 2 || !waybackarchiveSame(got, want) {
		t.Fatalf("err=%v hits=%d got=%v", err, hits, got)
	}
}

func Test_waybackarchive_MaxPages(t *testing.T) {
	got, hits, err := waybackarchiveRun(t, waybackarchiveGuard(t), nil, 1)
	want := []string{"api.example.com", "mail.example.com", "shop.example.com", "www.example.com"}
	if err != nil || hits != 1 || !waybackarchiveSame(got, want) {
		t.Fatalf("err=%v hits=%d got=%v", err, hits, got)
	}
}

func Test_waybackarchive_Empty(t *testing.T) {
	got, hits, err := waybackarchiveRun(t, waybackarchiveServe(waybackarchiveFixture(t, "empty.txt")), nil, 10)
	if err != nil || len(got) != 0 || hits != 1 {
		t.Fatalf("err=%v hits=%d got=%v", err, hits, got)
	}
}

func Test_waybackarchive_HostExtraction(t *testing.T) {
	for in, want := range map[string]string{
		"http://WWW.Example.com:80/a?b=c": "www.example.com",
		"https://u:p@api.example.com/x":   "api.example.com",
		"http://a.example.com%2fpath":     "a.example.com",
		"example.com/plain":               "example.com",
		"":                                "",
	} {
		if got := waybackarchiveHost(in); got != want {
			t.Errorf("%q -> %q want %q", in, got, want)
		}
	}
}
