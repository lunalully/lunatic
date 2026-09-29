package sources

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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

func windvaneFixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "windvane", name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// windvaneRun serves h, points the adapter at it and returns the in-scope
// normalized names plus the number of requests served.
func windvaneRun(t *testing.T, h http.HandlerFunc, creds map[string]string, maxPages int) ([]string, int, error) {
	t.Helper()
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		h(w, r)
	}))
	defer srv.Close()
	old := windvaneBaseURL
	windvaneBaseURL = srv.URL
	defer func() { windvaneBaseURL = old }()
	s := &Session{
		HTTP:     httpx.New(httpx.Options{SourceName: "windvane", TargetDomain: "example.com", MaxRetries: -1, BackoffBase: time.Millisecond}),
		Creds:    creds,
		MaxPages: maxPages,
	}
	set := map[string]bool{}
	err := windvane{}.Enumerate(context.Background(), "example.com", s, func(raw string) {
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

func windvaneSame(a, b []string) bool {
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

func windvaneServe(body []byte) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) { w.Write(body) }
}

func windvaneStatus(code int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(code) }
}

// windvanePages serves page1/page2 fixtures; check inspects the request (credential
// placement, method, path, body) and returns the page number to serve.
func windvanePages(t *testing.T) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		page := windvaneCheck(t, r)
		w.Write(windvaneFixture(t, fmt.Sprintf("page%d.json", page)))
	}
}

func Test_windvane_Info(t *testing.T) {
	i := windvane{}.Info()
	if i.Name != "windvane" || i.Disabled || i.Auth != AuthRequired || i.Default != true {
		t.Fatalf("unexpected info: %+v", i)
	}
	if _, ok := Get("windvane"); !ok {
		t.Fatal("not registered")
	}
}

func Test_windvane_Success(t *testing.T) {
	got, hits, err := windvaneRun(t, windvanePages(t), map[string]string{"api_key": "K1"}, 10)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"api.example.com", "www.example.com"}
	if !windvaneSame(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
	if hits != 2 {
		t.Fatalf("hits = %d, want 2", hits)
	}
}

func Test_windvane_MaxPages(t *testing.T) {
	got, hits, err := windvaneRun(t, windvanePages(t), map[string]string{"api_key": "K1"}, 1)
	if err != nil {
		t.Fatal(err)
	}
	if hits != 1 || !windvaneSame(got, []string{"www.example.com"}) {
		t.Fatalf("hits=%d got=%v", hits, got)
	}
}

func Test_windvane_Empty(t *testing.T) {
	got, _, err := windvaneRun(t, windvaneServe(windvaneFixture(t, "empty.json")), map[string]string{"api_key": "K1"}, 10)
	if err != nil || len(got) != 0 {
		t.Fatalf("err=%v got=%v", err, got)
	}
}

func Test_windvane_HTTPErrors(t *testing.T) {
	for _, c := range []struct {
		code int
		want error
	}{{401, ErrAuth}, {403, ErrAuth}, {429, ErrRateLimited}, {503, ErrUnavailable}, {400, ErrUnexpected}} {
		_, _, err := windvaneRun(t, windvaneStatus(c.code), map[string]string{"api_key": "K1"}, 10)
		if !errors.Is(err, c.want) {
			t.Errorf("HTTP %d: err=%v want %v", c.code, err, c.want)
		}
	}
}

func Test_windvane_ErrorInBody(t *testing.T) {
	for _, c := range []struct {
		file string
		want error
	}{{"error_auth.json", ErrAuth}, {"error_rate.json", ErrRateLimited}, {"error_other.json", ErrUnexpected}} {
		_, _, err := windvaneRun(t, windvaneServe(windvaneFixture(t, c.file)), map[string]string{"api_key": "K1"}, 10)
		if !errors.Is(err, c.want) {
			t.Errorf("%s: err=%v want %v", c.file, err, c.want)
		}
	}
}

func Test_windvane_InvalidBody(t *testing.T) {
	_, _, err := windvaneRun(t, windvaneServe([]byte("<html>not json")), map[string]string{"api_key": "K1"}, 10)
	if !errors.Is(err, ErrUnexpected) {
		t.Fatalf("err=%v", err)
	}
}

func Test_windvane_MissingKey(t *testing.T) {
	_, hits, err := windvaneRun(t, windvaneServe([]byte("{}")), map[string]string{}, 10)
	if !errors.Is(err, ErrNoKey) || hits != 0 {
		t.Fatalf("err=%v hits=%d", err, hits)
	}
}

func init() { windvaneCount = 2 } // small pages so the fixtures exercise pagination

func windvaneCheck(t *testing.T, r *http.Request) int {
	var b struct {
		Domain string `json:"domain"`
		PR     struct {
			Page  int `json:"page"`
			Count int `json:"count"`
		} `json:"page_request"`
	}
	body, _ := io.ReadAll(r.Body)
	if r.Method != "POST" || r.URL.Path != "/trpc.backendhub.public.WindvaneService/ListSubDomain" ||
		r.Header.Get("X-Api-Key") != "K1" || json.Unmarshal(body, &b) != nil || b.Domain != "example.com" || b.PR.Count != windvaneCount {
		t.Errorf("bad request %s %s key=%q body=%s", r.Method, r.URL.Path, r.Header.Get("X-Api-Key"), body)
	}
	return b.PR.Page
}
