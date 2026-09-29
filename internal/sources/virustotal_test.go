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

func virustotalFixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "virustotal", name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// virustotalRun serves h, points the adapter at it and returns the in-scope
// normalized names plus the number of requests served.
func virustotalRun(t *testing.T, h http.HandlerFunc, creds map[string]string, maxPages int) ([]string, int, error) {
	t.Helper()
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		h(w, r)
	}))
	defer srv.Close()
	old := virustotalBaseURL
	virustotalBaseURL = srv.URL
	defer func() { virustotalBaseURL = old }()
	s := &Session{
		HTTP:     httpx.New(httpx.Options{SourceName: "virustotal", TargetDomain: "example.com", MaxRetries: -1, BackoffBase: time.Millisecond}),
		Creds:    creds,
		MaxPages: maxPages,
	}
	set := map[string]bool{}
	err := virustotal{}.Enumerate(context.Background(), "example.com", s, func(raw string) {
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

func virustotalSame(a, b []string) bool {
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

func virustotalServe(body []byte) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) { w.Write(body) }
}

func virustotalStatus(code int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(code) }
}

// virustotalPages serves page1/page2 fixtures; check inspects the request (credential
// placement, method, path, body) and returns the page number to serve.
func virustotalPages(t *testing.T) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		page := virustotalCheck(t, r)
		w.Write(virustotalFixture(t, fmt.Sprintf("page%d.json", page)))
	}
}

func Test_virustotal_Info(t *testing.T) {
	i := virustotal{}.Info()
	if i.Name != "virustotal" || i.Disabled || i.Auth != AuthRequired || i.Default != true {
		t.Fatalf("unexpected info: %+v", i)
	}
	if _, ok := Get("virustotal"); !ok {
		t.Fatal("not registered")
	}
}

func Test_virustotal_Success(t *testing.T) {
	got, hits, err := virustotalRun(t, virustotalPages(t), map[string]string{"api_key": "K1"}, 10)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"api.example.com", "mail.example.com", "wild.example.com", "www.example.com"}
	if !virustotalSame(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
	if hits != 2 {
		t.Fatalf("hits = %d, want 2", hits)
	}
}

func Test_virustotal_MaxPages(t *testing.T) {
	got, hits, err := virustotalRun(t, virustotalPages(t), map[string]string{"api_key": "K1"}, 1)
	if err != nil {
		t.Fatal(err)
	}
	if hits != 1 || !virustotalSame(got, []string{"api.example.com", "www.example.com"}) {
		t.Fatalf("hits=%d got=%v", hits, got)
	}
}

func Test_virustotal_Empty(t *testing.T) {
	got, _, err := virustotalRun(t, virustotalServe(virustotalFixture(t, "empty.json")), map[string]string{"api_key": "K1"}, 10)
	if err != nil || len(got) != 0 {
		t.Fatalf("err=%v got=%v", err, got)
	}
}

func Test_virustotal_HTTPErrors(t *testing.T) {
	for _, c := range []struct {
		code int
		want error
	}{{401, ErrAuth}, {403, ErrAuth}, {429, ErrRateLimited}, {503, ErrUnavailable}, {400, ErrUnexpected}} {
		_, _, err := virustotalRun(t, virustotalStatus(c.code), map[string]string{"api_key": "K1"}, 10)
		if !errors.Is(err, c.want) {
			t.Errorf("HTTP %d: err=%v want %v", c.code, err, c.want)
		}
	}
}

func Test_virustotal_ErrorInBody(t *testing.T) {
	for _, c := range []struct {
		file string
		want error
	}{{"error_auth.json", ErrAuth}, {"error_rate.json", ErrRateLimited}, {"error_other.json", ErrUnexpected}} {
		_, _, err := virustotalRun(t, virustotalServe(virustotalFixture(t, c.file)), map[string]string{"api_key": "K1"}, 10)
		if !errors.Is(err, c.want) {
			t.Errorf("%s: err=%v want %v", c.file, err, c.want)
		}
	}
}

func Test_virustotal_InvalidBody(t *testing.T) {
	_, _, err := virustotalRun(t, virustotalServe([]byte("<html>not json")), map[string]string{"api_key": "K1"}, 10)
	if !errors.Is(err, ErrUnexpected) {
		t.Fatalf("err=%v", err)
	}
}

func Test_virustotal_MissingKey(t *testing.T) {
	_, hits, err := virustotalRun(t, virustotalServe([]byte("{}")), map[string]string{}, 10)
	if !errors.Is(err, ErrNoKey) || hits != 0 {
		t.Fatalf("err=%v hits=%d", err, hits)
	}
}

func Test_virustotal_NotFoundIsEmpty(t *testing.T) {
	got, _, err := virustotalRun(t, virustotalStatus(404), map[string]string{"api_key": "K1"}, 10)
	if err != nil || len(got) != 0 {
		t.Fatalf("err=%v got=%v", err, got)
	}
}

func virustotalCheck(t *testing.T, r *http.Request) int {
	q := r.URL.Query()
	if r.Method != "GET" || r.URL.Path != "/api/v3/domains/example.com/subdomains" || r.Header.Get("x-apikey") != "K1" {
		t.Errorf("bad request %s %s key=%q", r.Method, r.URL.String(), r.Header.Get("x-apikey"))
	}
	if q.Get("cursor") == "CUR2" {
		return 2
	}
	return 1
}
