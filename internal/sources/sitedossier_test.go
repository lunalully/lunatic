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

func sitedossierFixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "sitedossier", name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// sitedossierRun serves h, points the adapter at it and returns the in-scope
// normalized names plus the number of requests served.
func sitedossierRun(t *testing.T, h http.HandlerFunc, creds map[string]string, maxPages int) ([]string, int, error) {
	t.Helper()
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		h(w, r)
	}))
	defer srv.Close()
	old := sitedossierBaseURL
	sitedossierBaseURL = srv.URL
	defer func() { sitedossierBaseURL = old }()
	s := &Session{
		HTTP:     httpx.New(httpx.Options{SourceName: "sitedossier", TargetDomain: "example.com", MaxRetries: -1, BackoffBase: time.Millisecond}),
		Creds:    creds,
		MaxPages: maxPages,
	}
	set := map[string]bool{}
	err := sitedossier{}.Enumerate(context.Background(), "example.com", s, func(raw string) {
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

func sitedossierSame(a, b []string) bool {
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

func sitedossierServe(body []byte) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) { w.Write(body) }
}

func sitedossierStatus(code int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(code) }
}

func Test_sitedossier_Info(t *testing.T) {
	i := sitedossier{}.Info()
	if i.Name != "sitedossier" || i.Disabled || i.Auth != AuthNone || i.Default != false {
		t.Fatalf("unexpected info: %+v", i)
	}
	if _, ok := Get("sitedossier"); !ok {
		t.Fatal("not registered")
	}
}

func Test_sitedossier_HTTPErrors(t *testing.T) {
	for _, c := range []struct {
		code int
		want error
	}{{401, ErrAuth}, {403, ErrAuth}, {429, ErrRateLimited}, {503, ErrUnavailable}, {400, ErrUnexpected}} {
		_, _, err := sitedossierRun(t, sitedossierStatus(c.code), nil, 10)
		if !errors.Is(err, c.want) {
			t.Errorf("HTTP %d: err=%v want %v", c.code, err, c.want)
		}
	}
}

func Test_sitedossier_SuccessAndPagination(t *testing.T) {
	h := func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("method %s", r.Method)
		}
		switch r.URL.Path {
		case "/parentdomain/example.com":
			w.Write(sitedossierFixture(t, "page1.html"))
		case "/parentdomain/example.com/101":
			w.Write(sitedossierFixture(t, "page2.html"))
		case "/parentdomain/example.com/201":
			w.Write(sitedossierFixture(t, "empty.html"))
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
			w.WriteHeader(404)
		}
	}
	got, hits, err := sitedossierRun(t, h, nil, 10)
	want := []string{"api.example.com", "blog.example.com", "www.example.com"}
	if err != nil || hits != 3 || !sitedossierSame(got, want) {
		t.Fatalf("err=%v hits=%d got=%v", err, hits, got)
	}
	got, hits, err = sitedossierRun(t, h, nil, 2)
	if err != nil || hits != 2 || len(got) != 3 {
		t.Fatalf("maxpages: err=%v hits=%d got=%v", err, hits, got)
	}
}

func Test_sitedossier_Empty(t *testing.T) {
	got, _, err := sitedossierRun(t, sitedossierServe(sitedossierFixture(t, "empty.html")), nil, 10)
	if err != nil || len(got) != 0 {
		t.Fatalf("err=%v got=%v", err, got)
	}
}

func Test_sitedossier_CaptchaIsUnavailable(t *testing.T) {
	_, hits, err := sitedossierRun(t, sitedossierServe(sitedossierFixture(t, "captcha.html")), nil, 10)
	if !errors.Is(err, ErrUnavailable) || hits != 1 {
		t.Fatalf("err=%v hits=%d", err, hits)
	}
}

func Test_sitedossier_NextLoopStops(t *testing.T) {
	h := func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<a href="/parentdomain/example.com"><b>again</b></a> a.example.com`))
	}
	_, hits, err := sitedossierRun(t, h, nil, 10)
	if err != nil || hits != 1 {
		t.Fatalf("err=%v hits=%d", err, hits)
	}
}
