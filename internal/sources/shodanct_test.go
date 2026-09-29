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

func shodanctFixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "shodanct", name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// shodanctRun serves h, points the adapter at it and returns the in-scope
// normalized names plus the number of requests served.
func shodanctRun(t *testing.T, h http.HandlerFunc, creds map[string]string, maxPages int) ([]string, int, error) {
	t.Helper()
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		h(w, r)
	}))
	defer srv.Close()
	old := shodanctBaseURL
	shodanctBaseURL = srv.URL
	defer func() { shodanctBaseURL = old }()
	s := &Session{
		HTTP:     httpx.New(httpx.Options{SourceName: "shodanct", TargetDomain: "example.com", MaxRetries: -1, BackoffBase: time.Millisecond}),
		Creds:    creds,
		MaxPages: maxPages,
	}
	set := map[string]bool{}
	err := shodanct{}.Enumerate(context.Background(), "example.com", s, func(raw string) {
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

func shodanctSame(a, b []string) bool {
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

func shodanctServe(body []byte) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) { w.Write(body) }
}

func shodanctStatus(code int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(code) }
}

func Test_shodanct_Info(t *testing.T) {
	i := shodanct{}.Info()
	if i.Name != "shodanct" || i.Disabled || i.Auth != AuthNone || i.Default != true {
		t.Fatalf("unexpected info: %+v", i)
	}
	if _, ok := Get("shodanct"); !ok {
		t.Fatal("not registered")
	}
}

func Test_shodanct_HTTPErrors(t *testing.T) {
	for _, c := range []struct {
		code int
		want error
	}{{401, ErrAuth}, {403, ErrAuth}, {429, ErrRateLimited}, {503, ErrUnavailable}, {400, ErrUnexpected}} {
		_, _, err := shodanctRun(t, shodanctStatus(c.code), nil, 10)
		if !errors.Is(err, c.want) {
			t.Errorf("HTTP %d: err=%v want %v", c.code, err, c.want)
		}
	}
}

func Test_shodanct_Success(t *testing.T) {
	h := func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" || r.URL.Path != "/api/v1/domain/example.com/hostnames" {
			t.Errorf("bad request %s %s", r.Method, r.URL.Path)
		}
		w.Write(shodanctFixture(t, "hostnames.json"))
	}
	got, hits, err := shodanctRun(t, h, nil, 10)
	want := []string{"api.example.com", "deep.sub.example.com", "www.example.com"}
	if err != nil || hits != 1 || !shodanctSame(got, want) {
		t.Fatalf("err=%v hits=%d got=%v", err, hits, got)
	}
}

func Test_shodanct_Empty(t *testing.T) {
	got, _, err := shodanctRun(t, shodanctServe(shodanctFixture(t, "empty.json")), nil, 10)
	if err != nil || len(got) != 0 {
		t.Fatalf("err=%v got=%v", err, got)
	}
}

func Test_shodanct_ErrorObjectAndInvalid(t *testing.T) {
	_, _, err := shodanctRun(t, shodanctServe(shodanctFixture(t, "error.json")), nil, 10)
	if !errors.Is(err, ErrUnexpected) {
		t.Errorf("error object: %v", err)
	}
	_, _, err = shodanctRun(t, shodanctServe([]byte("<html>")), nil, 10)
	if !errors.Is(err, ErrUnexpected) {
		t.Errorf("invalid: %v", err)
	}
}
