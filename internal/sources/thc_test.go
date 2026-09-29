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

func thcFixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "thc", name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// thcRun serves h, points the adapter at it and returns the in-scope
// normalized names plus the number of requests served.
func thcRun(t *testing.T, h http.HandlerFunc, creds map[string]string, maxPages int) ([]string, int, error) {
	t.Helper()
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		h(w, r)
	}))
	defer srv.Close()
	old := thcBaseURL
	thcBaseURL = srv.URL
	defer func() { thcBaseURL = old }()
	s := &Session{
		HTTP:     httpx.New(httpx.Options{SourceName: "thc", TargetDomain: "example.com", MaxRetries: -1, BackoffBase: time.Millisecond}),
		Creds:    creds,
		MaxPages: maxPages,
	}
	set := map[string]bool{}
	err := thc{}.Enumerate(context.Background(), "example.com", s, func(raw string) {
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

func thcSame(a, b []string) bool {
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

func thcServe(body []byte) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) { w.Write(body) }
}

func thcStatus(code int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(code) }
}

// thcPages serves page1/page2 fixtures; check inspects the request (credential
// placement, method, path, body) and returns the page number to serve.
func thcPages(t *testing.T) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		page := thcCheck(t, r)
		w.Write(thcFixture(t, fmt.Sprintf("page%d.json", page)))
	}
}

func Test_thc_Info(t *testing.T) {
	i := thc{}.Info()
	if i.Name != "thc" || i.Disabled || i.Auth != AuthNone || i.Default != true {
		t.Fatalf("unexpected info: %+v", i)
	}
	if _, ok := Get("thc"); !ok {
		t.Fatal("not registered")
	}
}

func Test_thc_Success(t *testing.T) {
	got, hits, err := thcRun(t, thcPages(t), map[string]string{}, 10)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"api.example.com", "mail.example.com", "str.example.com", "www.example.com"}
	if !thcSame(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
	if hits != 2 {
		t.Fatalf("hits = %d, want 2", hits)
	}
}

func Test_thc_MaxPages(t *testing.T) {
	got, hits, err := thcRun(t, thcPages(t), map[string]string{}, 1)
	if err != nil {
		t.Fatal(err)
	}
	if hits != 1 || !thcSame(got, []string{"str.example.com", "www.example.com"}) {
		t.Fatalf("hits=%d got=%v", hits, got)
	}
}

func Test_thc_Empty(t *testing.T) {
	got, _, err := thcRun(t, thcServe(thcFixture(t, "empty.json")), map[string]string{}, 10)
	if err != nil || len(got) != 0 {
		t.Fatalf("err=%v got=%v", err, got)
	}
}

func Test_thc_HTTPErrors(t *testing.T) {
	for _, c := range []struct {
		code int
		want error
	}{{401, ErrAuth}, {403, ErrAuth}, {429, ErrRateLimited}, {503, ErrUnavailable}, {400, ErrUnexpected}} {
		_, _, err := thcRun(t, thcStatus(c.code), map[string]string{}, 10)
		if !errors.Is(err, c.want) {
			t.Errorf("HTTP %d: err=%v want %v", c.code, err, c.want)
		}
	}
}

func Test_thc_ErrorInBody(t *testing.T) {
	for _, c := range []struct {
		file string
		want error
	}{{"error_rate.json", ErrRateLimited}, {"error_other.json", ErrUnexpected}} {
		_, _, err := thcRun(t, thcServe(thcFixture(t, c.file)), map[string]string{}, 10)
		if !errors.Is(err, c.want) {
			t.Errorf("%s: err=%v want %v", c.file, err, c.want)
		}
	}
}

func Test_thc_InvalidBody(t *testing.T) {
	_, _, err := thcRun(t, thcServe([]byte("<html>not json")), map[string]string{}, 10)
	if !errors.Is(err, ErrUnexpected) {
		t.Fatalf("err=%v", err)
	}
}

func Test_thc_RepeatedPageStateStops(t *testing.T) {
	h := func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"domains":[{"domain":"a.example.com"}],"next_page_state":"SAME"}`))
	}
	_, hits, err := thcRun(t, h, map[string]string{}, 10)
	if err != nil || hits != 2 {
		t.Fatalf("err=%v hits=%d", err, hits)
	}
}

func thcCheck(t *testing.T, r *http.Request) int {
	var b struct {
		Domain    string `json:"domain"`
		PageState string `json:"page_state"`
		Limit     int    `json:"limit"`
	}
	body, _ := io.ReadAll(r.Body)
	if r.Method != "POST" || r.URL.Path != "/api/v1/lookup/subdomains" || json.Unmarshal(body, &b) != nil || b.Domain != "example.com" || b.Limit != 1000 {
		t.Errorf("bad request %s %s %s", r.Method, r.URL.Path, body)
	}
	if b.PageState == "CURSOR2" {
		return 2
	}
	return 1
}
