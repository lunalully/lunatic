package sources

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/lunalully/lunatic/internal/httpx"
)

// ---- shared helpers for group 2 tests (prefix g2) ----

func g2fixture(t *testing.T, dir, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", dir, name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// g2run points *base at a test server running h, and enumerates example.com.
func g2run(t *testing.T, src Source, base *string, h http.HandlerFunc, creds map[string]string, maxPages int) ([]string, error) {
	t.Helper()
	srv := httptest.NewServer(h)
	defer srv.Close()
	old := *base
	*base = srv.URL
	defer func() { *base = old }()
	s := &Session{
		HTTP:     httpx.New(httpx.Options{SourceName: src.Info().Name, TargetDomain: "example.com", MaxRetries: -1, BackoffBase: time.Millisecond}),
		Creds:    creds,
		MaxPages: maxPages,
	}
	var got []string
	err := src.Enumerate(context.Background(), "example.com", s, func(r string) { got = append(got, r) })
	sort.Strings(got)
	return got, err
}

func g2serve(status int, body []byte) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
		w.Write(body)
	}
}

func g2want(t *testing.T, got []string, want ...string) {
	t.Helper()
	sort.Strings(want)
	if len(got) == 0 && len(want) == 0 {
		return
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func g2is(t *testing.T, err, want error) {
	t.Helper()
	if !errors.Is(err, want) {
		t.Fatalf("err = %v, want %v", err, want)
	}
}

// g2errors runs the standard failure suite: 401, 429, garbage body and missing key.
func g2errors(t *testing.T, src Source, base *string, creds map[string]string, garbage string) {
	t.Helper()
	_, err := g2run(t, src, base, g2serve(401, []byte("nope")), creds, 3)
	g2is(t, err, ErrAuth)
	_, err = g2run(t, src, base, g2serve(429, []byte("slow down")), creds, 3)
	g2is(t, err, ErrRateLimited)
	_, err = g2run(t, src, base, g2serve(200, []byte(garbage)), creds, 3)
	g2is(t, err, ErrUnexpected)
	_, err = g2run(t, src, base, g2serve(200, []byte("{}")), map[string]string{}, 3)
	g2is(t, err, ErrNoKey)
}

func g2disabled(t *testing.T, src Source) {
	t.Helper()
	i := src.Info()
	if !i.Disabled || i.DisabledReason == "" {
		t.Fatalf("%s should be disabled with a reason: %+v", i.Name, i)
	}
	g2is(t, src.Enumerate(context.Background(), "example.com", &Session{}, func(string) {}), ErrDisabled)
}

// ---- hackertarget ----

func TestHackertargetSuccessKeyHeader(t *testing.T) {
	var hdr, q string
	got, err := g2run(t, hackertarget{}, &hackertargetBaseURL, func(w http.ResponseWriter, r *http.Request) {
		hdr, q = r.Header.Get("X-API-Key"), r.URL.String()
		w.Write(g2fixture(t, "hackertarget", "success.txt"))
	}, map[string]string{"api_key": "K1"}, 1)
	if err != nil {
		t.Fatal(err)
	}
	g2want(t, got, "www.example.com", "api.example.com", "mail.example.com")
	if hdr != "K1" || q != "/hostsearch/?q=example.com" {
		t.Fatalf("hdr=%q url=%q", hdr, q)
	}
}

func TestHackertargetNoKeyWorks(t *testing.T) {
	var hdr string
	got, err := g2run(t, hackertarget{}, &hackertargetBaseURL, func(w http.ResponseWriter, r *http.Request) {
		hdr = r.Header.Get("X-API-Key")
		w.Write(g2fixture(t, "hackertarget", "success.txt"))
	}, map[string]string{}, 1)
	if err != nil || len(got) != 3 || hdr != "" {
		t.Fatalf("got=%v err=%v hdr=%q", got, err, hdr)
	}
	if i := (hackertarget{}).Info(); i.Auth != AuthOptional || !i.Default {
		t.Fatalf("info %+v", i)
	}
}

func TestHackertargetErrorsIn200(t *testing.T) {
	got, err := g2run(t, hackertarget{}, &hackertargetBaseURL, g2serve(200, g2fixture(t, "hackertarget", "quota.txt")), nil, 1)
	g2is(t, err, ErrRateLimited)
	if len(got) != 0 {
		t.Fatalf("quota text parsed as data: %v", got)
	}
	got, err = g2run(t, hackertarget{}, &hackertargetBaseURL, g2serve(200, g2fixture(t, "hackertarget", "error.txt")), nil, 1)
	g2is(t, err, ErrUnexpected)
	if len(got) != 0 {
		t.Fatalf("error text parsed as data: %v", got)
	}
	_, err = g2run(t, hackertarget{}, &hackertargetBaseURL, g2serve(200, []byte("error invalid API key")), nil, 1)
	g2is(t, err, ErrAuth)
}

func TestHackertargetEmptyAndInvalid(t *testing.T) {
	got, err := g2run(t, hackertarget{}, &hackertargetBaseURL, g2serve(200, []byte("No records found")), nil, 1)
	if err != nil || len(got) != 0 {
		t.Fatalf("%v %v", got, err)
	}
	_, err = g2run(t, hackertarget{}, &hackertargetBaseURL, g2serve(200, []byte("<html>blocked</html>")), nil, 1)
	g2is(t, err, ErrUnexpected)
	_, err = g2run(t, hackertarget{}, &hackertargetBaseURL, g2serve(429, nil), nil, 1)
	g2is(t, err, ErrRateLimited)
}

func g2session(name string, creds map[string]string, maxPages int) *Session {
	return &Session{
		HTTP:     httpx.New(httpx.Options{SourceName: name, TargetDomain: "example.com", MaxRetries: -1, BackoffBase: time.Millisecond}),
		Creds:    creds,
		MaxPages: maxPages,
	}
}

func newIntelxServer(h http.HandlerFunc, hit *bool) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { *hit = true; h(w, r) }))
}
