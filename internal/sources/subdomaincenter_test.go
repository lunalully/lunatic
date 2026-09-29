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

func subdomaincenterFixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "subdomaincenter", name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// subdomaincenterRun serves h, points the adapter at it and returns the in-scope
// normalized names plus the number of requests served.
func subdomaincenterRun(t *testing.T, h http.HandlerFunc, creds map[string]string, maxPages int) ([]string, int, error) {
	t.Helper()
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		h(w, r)
	}))
	defer srv.Close()
	old := subdomaincenterBaseURL
	subdomaincenterBaseURL = srv.URL
	defer func() { subdomaincenterBaseURL = old }()
	s := &Session{
		HTTP:     httpx.New(httpx.Options{SourceName: "subdomaincenter", TargetDomain: "example.com", MaxRetries: -1, BackoffBase: time.Millisecond}),
		Creds:    creds,
		MaxPages: maxPages,
	}
	set := map[string]bool{}
	err := subdomaincenter{}.Enumerate(context.Background(), "example.com", s, func(raw string) {
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

func subdomaincenterSame(a, b []string) bool {
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

func subdomaincenterServe(body []byte) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) { w.Write(body) }
}

func subdomaincenterStatus(code int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(code) }
}

func Test_subdomaincenter_Info(t *testing.T) {
	i := subdomaincenter{}.Info()
	if i.Name != "subdomaincenter" || i.Disabled || i.Auth != AuthNone || i.Default != true {
		t.Fatalf("unexpected info: %+v", i)
	}
	if _, ok := Get("subdomaincenter"); !ok {
		t.Fatal("not registered")
	}
}

func Test_subdomaincenter_HTTPErrors(t *testing.T) {
	for _, c := range []struct {
		code int
		want error
	}{{401, ErrAuth}, {403, ErrAuth}, {429, ErrRateLimited}, {503, ErrUnavailable}, {400, ErrUnexpected}} {
		_, _, err := subdomaincenterRun(t, subdomaincenterStatus(c.code), nil, 10)
		if !errors.Is(err, c.want) {
			t.Errorf("HTTP %d: err=%v want %v", c.code, err, c.want)
		}
	}
}

func Test_subdomaincenter_Empty(t *testing.T) {
	got, _, err := subdomaincenterRun(t, subdomaincenterServe(subdomaincenterFixture(t, "empty.json")), nil, 10)
	if err != nil || len(got) != 0 {
		t.Fatalf("err=%v got=%v", err, got)
	}
}

func Test_subdomaincenter_InvalidBody(t *testing.T) {
	_, _, err := subdomaincenterRun(t, subdomaincenterServe([]byte("<html>not json")), nil, 10)
	if !errors.Is(err, ErrUnexpected) {
		t.Fatalf("err=%v", err)
	}
}

func Test_subdomaincenter_Success(t *testing.T) {
	h := func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" || r.URL.Path != "/" || r.URL.Query().Get("domain") != "example.com" {
			t.Errorf("bad request %s %s", r.Method, r.URL.String())
		}
		w.Write(subdomaincenterFixture(t, "ok.json"))
	}
	got, hits, err := subdomaincenterRun(t, h, nil, 10)
	want := []string{"api.example.com", "dev.example.com", "www.example.com"}
	if err != nil || hits != 1 || !subdomaincenterSame(got, want) {
		t.Fatalf("err=%v hits=%d got=%v", err, hits, got)
	}
}

func Test_subdomaincenter_ErrorObject(t *testing.T) {
	_, _, err := subdomaincenterRun(t, subdomaincenterServe(subdomaincenterFixture(t, "error.json")), nil, 10)
	if !errors.Is(err, ErrUnexpected) {
		t.Fatalf("err=%v", err)
	}
}
