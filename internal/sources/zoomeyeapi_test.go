package sources

import (
	"context"
	"encoding/base64"
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

func zoomeyeapiFixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "zoomeyeapi", name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// zoomeyeapiRun serves h, points the adapter at it and returns the in-scope
// normalized names plus the number of requests served.
func zoomeyeapiRun(t *testing.T, h http.HandlerFunc, creds map[string]string, maxPages int) ([]string, int, error) {
	t.Helper()
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		h(w, r)
	}))
	defer srv.Close()
	old := zoomeyeapiBaseURL
	zoomeyeapiBaseURL = srv.URL
	defer func() { zoomeyeapiBaseURL = old }()
	s := &Session{
		HTTP:     httpx.New(httpx.Options{SourceName: "zoomeyeapi", TargetDomain: "example.com", MaxRetries: -1, BackoffBase: time.Millisecond}),
		Creds:    creds,
		MaxPages: maxPages,
	}
	set := map[string]bool{}
	err := zoomeyeapi{}.Enumerate(context.Background(), "example.com", s, func(raw string) {
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

func zoomeyeapiSame(a, b []string) bool {
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

func zoomeyeapiServe(body []byte) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) { w.Write(body) }
}

func zoomeyeapiStatus(code int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(code) }
}

// zoomeyeapiPages serves page1/page2 fixtures; check inspects the request (credential
// placement, method, path, body) and returns the page number to serve.
func zoomeyeapiPages(t *testing.T) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		page := zoomeyeapiCheck(t, r)
		w.Write(zoomeyeapiFixture(t, fmt.Sprintf("page%d.json", page)))
	}
}

func Test_zoomeyeapi_Info(t *testing.T) {
	i := zoomeyeapi{}.Info()
	if i.Name != "zoomeyeapi" || i.Disabled || i.Auth != AuthRequired || i.Default != false {
		t.Fatalf("unexpected info: %+v", i)
	}
	if _, ok := Get("zoomeyeapi"); !ok {
		t.Fatal("not registered")
	}
}

func Test_zoomeyeapi_Success(t *testing.T) {
	got, hits, err := zoomeyeapiRun(t, zoomeyeapiPages(t), map[string]string{"api_key": "K1"}, 10)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"api.example.com", "www.example.com"}
	if !zoomeyeapiSame(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
	if hits != 2 {
		t.Fatalf("hits = %d, want 2", hits)
	}
}

func Test_zoomeyeapi_MaxPages(t *testing.T) {
	got, hits, err := zoomeyeapiRun(t, zoomeyeapiPages(t), map[string]string{"api_key": "K1"}, 1)
	if err != nil {
		t.Fatal(err)
	}
	if hits != 1 || !zoomeyeapiSame(got, []string{"www.example.com"}) {
		t.Fatalf("hits=%d got=%v", hits, got)
	}
}

func Test_zoomeyeapi_Empty(t *testing.T) {
	got, _, err := zoomeyeapiRun(t, zoomeyeapiServe(zoomeyeapiFixture(t, "empty.json")), map[string]string{"api_key": "K1"}, 10)
	if err != nil || len(got) != 0 {
		t.Fatalf("err=%v got=%v", err, got)
	}
}

func Test_zoomeyeapi_HTTPErrors(t *testing.T) {
	for _, c := range []struct {
		code int
		want error
	}{{401, ErrAuth}, {403, ErrAuth}, {429, ErrRateLimited}, {503, ErrUnavailable}, {400, ErrUnexpected}} {
		_, _, err := zoomeyeapiRun(t, zoomeyeapiStatus(c.code), map[string]string{"api_key": "K1"}, 10)
		if !errors.Is(err, c.want) {
			t.Errorf("HTTP %d: err=%v want %v", c.code, err, c.want)
		}
	}
}

func Test_zoomeyeapi_ErrorInBody(t *testing.T) {
	for _, c := range []struct {
		file string
		want error
	}{{"error_auth.json", ErrAuth}, {"error_rate.json", ErrRateLimited}, {"error_other.json", ErrUnexpected}} {
		_, _, err := zoomeyeapiRun(t, zoomeyeapiServe(zoomeyeapiFixture(t, c.file)), map[string]string{"api_key": "K1"}, 10)
		if !errors.Is(err, c.want) {
			t.Errorf("%s: err=%v want %v", c.file, err, c.want)
		}
	}
}

func Test_zoomeyeapi_InvalidBody(t *testing.T) {
	_, _, err := zoomeyeapiRun(t, zoomeyeapiServe([]byte("<html>not json")), map[string]string{"api_key": "K1"}, 10)
	if !errors.Is(err, ErrUnexpected) {
		t.Fatalf("err=%v", err)
	}
}

func Test_zoomeyeapi_MissingKey(t *testing.T) {
	_, hits, err := zoomeyeapiRun(t, zoomeyeapiServe([]byte("{}")), map[string]string{}, 10)
	if !errors.Is(err, ErrNoKey) || hits != 0 {
		t.Fatalf("err=%v hits=%d", err, hits)
	}
}

func init() { zoomeyeapiPageSize = 2 }

func Test_zoomeyeapi_HostPrefix(t *testing.T) {
	for _, c := range []struct{ in, host, key string }{
		{"KEY", "zoomeye.ai", "KEY"},
		{"api.zoomeye.hk:KEY", "zoomeye.hk", "KEY"},
		{"zoomeye.org:KEY", "zoomeye.org", "KEY"},
		{"abc:def", "zoomeye.ai", "abc:def"},
	} {
		h, k := zoomeyeapiSplit(c.in)
		if h != c.host || k != c.key {
			t.Errorf("%q -> %q %q", c.in, h, k)
		}
	}
}

func zoomeyeapiCheck(t *testing.T, r *http.Request) int {
	var b struct {
		Q        string `json:"qbase64"`
		Page     int    `json:"page"`
		PageSize int    `json:"pagesize"`
		Sub      string `json:"subtype"`
	}
	body, _ := io.ReadAll(r.Body)
	if json.Unmarshal(body, &b) != nil {
		t.Errorf("bad body %s", body)
	}
	dq, _ := base64.StdEncoding.DecodeString(b.Q)
	if r.Method != "POST" || r.URL.Path != "/v2/search" || r.Header.Get("API-KEY") != "K1" || string(dq) != `domain="example.com"` {
		t.Errorf("bad request %s %s key=%q q=%q", r.Method, r.URL.Path, r.Header.Get("API-KEY"), dq)
	}
	return b.Page
}

func Test_zoomeyeapi_HostIsOptionalField(t *testing.T) {
	i := zoomeyeapi{}.Info()
	if len(i.CredFields) != 1 || i.CredFields[0] != "api_key" || len(i.OptCredFields) != 1 || i.OptCredFields[0] != "host" {
		t.Fatalf("info %+v", i)
	}
	_, hits, err := zoomeyeapiRun(t, zoomeyeapiServe([]byte("{}")), map[string]string{"api_key": "K1", "host": "evil.test/x"}, 10)
	if !errors.Is(err, ErrUnexpected) || hits != 0 {
		t.Fatalf("err=%v hits=%d", err, hits)
	}
}
