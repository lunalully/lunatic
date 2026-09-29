package sources

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/lunalully/lunatic/internal/httpx"
)

func commoncrawlFixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/commoncrawl/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func commoncrawlRun(t *testing.T, h http.HandlerFunc, maxPages int) ([]string, error) {
	t.Helper()
	srv := httptest.NewServer(h)
	defer srv.Close()
	old := commoncrawlBaseURL
	commoncrawlBaseURL = srv.URL
	defer func() { commoncrawlBaseURL = old }()
	s := &Session{
		HTTP:     httpx.New(httpx.Options{SourceName: "commoncrawl", TargetDomain: "example.com", MaxRetries: -1, BackoffBase: time.Millisecond}),
		MaxPages: maxPages,
	}
	var got []string
	err := commoncrawl{}.Enumerate(context.Background(), "example.com", s, func(r string) { got = append(got, r) })
	return got, err
}

type commoncrawlServer struct {
	mu   sync.Mutex
	reqs []string
}

func (c *commoncrawlServer) handler(t *testing.T, notFoundIndex string) http.HandlerFunc {
	coll, np := commoncrawlFixture(t, "collinfo.json"), commoncrawlFixture(t, "numpages.json")
	p0, p1 := commoncrawlFixture(t, "page0.ndjson"), commoncrawlFixture(t, "page1.ndjson")
	return func(w http.ResponseWriter, r *http.Request) {
		c.mu.Lock()
		c.reqs = append(c.reqs, r.URL.RequestURI())
		c.mu.Unlock()
		if r.URL.Path == "/collinfo.json" {
			_, _ = w.Write(coll)
			return
		}
		if notFoundIndex != "" && strings.Contains(r.URL.Path, notFoundIndex) {
			w.WriteHeader(404)
			_, _ = w.Write([]byte("No Captures found for: *.example.com"))
			return
		}
		q := r.URL.Query()
		switch {
		case q.Get("showNumPages") == "true":
			_, _ = w.Write(np)
		case q.Get("page") == "0":
			_, _ = w.Write(p0)
		case q.Get("page") == "1":
			_, _ = w.Write(p1)
		default:
			w.WriteHeader(400)
		}
	}
}

func TestCommoncrawlSuccessLatestThreeIndexes(t *testing.T) {
	srv := &commoncrawlServer{}
	got, err := commoncrawlRun(t, srv.handler(t, ""), 10)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"www.example.com", "api.example.com", "blog.example.com", "evil.other.test"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q", got)
	}
	idx := map[string]bool{}
	for _, r := range srv.reqs {
		if strings.Contains(r, "-index") {
			idx[strings.SplitN(strings.TrimPrefix(r, "/"), "-index", 2)[0]] = true
			if !strings.Contains(r, "url=%2A.example.com") || !strings.Contains(r, "output=json") || !strings.Contains(r, "fl=url") {
				t.Fatalf("bad query %s", r)
			}
		}
	}
	if !reflect.DeepEqual(idx, map[string]bool{"CC-MAIN-2026-39": true, "CC-MAIN-2026-33": true, "CC-MAIN-2026-26": true}) {
		t.Fatalf("indexes queried: %v", idx)
	}
	// 1 collinfo + 3 * (showNumPages + 2 pages)
	if len(srv.reqs) != 10 {
		t.Fatalf("requests=%d: %v", len(srv.reqs), srv.reqs)
	}
}

func TestCommoncrawlMaxPagesPerIndex(t *testing.T) {
	srv := &commoncrawlServer{}
	got, err := commoncrawlRun(t, srv.handler(t, ""), 1)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, []string{"www.example.com", "api.example.com"}) {
		t.Fatalf("got %q", got)
	}
	if len(srv.reqs) != 7 { // 1 + 3*(numpages + page 0)
		t.Fatalf("requests=%d", len(srv.reqs))
	}
}

func TestCommoncrawlNoCapturesIs404Zero(t *testing.T) {
	coll := commoncrawlFixture(t, "collinfo.json")
	got, err := commoncrawlRun(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/collinfo.json" {
			_, _ = w.Write(coll)
			return
		}
		w.WriteHeader(404)
		_, _ = w.Write([]byte("No Captures found for: *.example.com"))
	}, 10)
	if err != nil || len(got) != 0 {
		t.Fatalf("%v %v", got, err)
	}
}

func TestCommoncrawlOneIndexNotFoundOthersContinue(t *testing.T) {
	srv := &commoncrawlServer{}
	got, err := commoncrawlRun(t, srv.handler(t, "CC-MAIN-2026-39"), 10)
	if err != nil || len(got) != 4 {
		t.Fatalf("%v %v", got, err)
	}
}

func TestCommoncrawlErrors(t *testing.T) {
	coll := commoncrawlFixture(t, "collinfo.json")
	// collinfo itself failing is an error, never zero results.
	for _, c := range []struct {
		status int
		want   error
	}{{503, ErrUnavailable}, {429, ErrRateLimited}, {404, ErrUnexpected}} {
		_, err := commoncrawlRun(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(c.status) }, 10)
		if !errors.Is(err, c.want) {
			t.Fatalf("collinfo %d: %v", c.status, err)
		}
	}
	// Index request failing with 503 (rate limiting by provider) is an error.
	_, err := commoncrawlRun(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/collinfo.json" {
			_, _ = w.Write(coll)
			return
		}
		w.WriteHeader(503)
	}, 10)
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("%v", err)
	}
	// Invalid collinfo JSON.
	_, err = commoncrawlRun(t, func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("<html>")) }, 10)
	if !errors.Is(err, ErrUnexpected) {
		t.Fatalf("%v", err)
	}
	// Empty collinfo.
	_, err = commoncrawlRun(t, func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("[]")) }, 10)
	if !errors.Is(err, ErrUnexpected) {
		t.Fatalf("%v", err)
	}
	// Garbage NDJSON and provider error line.
	for _, body := range []string{"not json\n", `{"error":"blocked"}` + "\n"} {
		_, err = commoncrawlRun(t, func(w http.ResponseWriter, r *http.Request) {
			switch {
			case r.URL.Path == "/collinfo.json":
				_, _ = w.Write(coll)
			case r.URL.Query().Get("showNumPages") == "true":
				_, _ = w.Write([]byte(`{"pages":1}`))
			default:
				_, _ = w.Write([]byte(body))
			}
		}, 10)
		if !errors.Is(err, ErrUnexpected) {
			t.Fatalf("%q: %v", body, err)
		}
	}
	// Invalid showNumPages response.
	_, err = commoncrawlRun(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/collinfo.json" {
			_, _ = w.Write(coll)
			return
		}
		_, _ = w.Write([]byte("nope"))
	}, 10)
	if !errors.Is(err, ErrUnexpected) {
		t.Fatalf("%v", err)
	}
}

func TestCommoncrawlHost(t *testing.T) {
	for in, want := range map[string]string{
		"https://www.example.com/a?b=1": "www.example.com",
		"http://a.example.com:8080/":    "a.example.com",
		"b.example.com/path":            "b.example.com",
		"":                              "",
	} {
		if got := commoncrawlHost(in); got != want {
			t.Errorf("%q: got %q want %q", in, got, want)
		}
	}
}

func TestCommoncrawlInfo(t *testing.T) {
	if i := (commoncrawl{}).Info(); i.Auth != AuthNone || i.Default {
		t.Fatalf("%+v", i)
	}
}
