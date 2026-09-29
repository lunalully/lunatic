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
	"sync/atomic"
	"testing"
	"time"

	"github.com/lunalully/lunatic/internal/httpx"
	"github.com/lunalully/lunatic/internal/scope"
)

// ---- helpers shared by the group-3 adapter tests (merklemap prefix) ----

func merklemapSession(name string, creds map[string]string, maxPages int) *Session {
	return &Session{
		HTTP:     httpx.New(httpx.Options{SourceName: name, TargetDomain: "example.com", MaxRetries: -1, BackoffBase: time.Millisecond}),
		Creds:    creds,
		MaxPages: maxPages,
	}
}

func merklemapFixture(t *testing.T, dir, file string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", dir, file))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// merklemapServe starts a test server and points *base at it until cleanup.
func merklemapServe(t *testing.T, base *string, h http.HandlerFunc) *int32 {
	t.Helper()
	var n int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&n, 1)
		h(w, r)
	}))
	old := *base
	*base = srv.URL
	t.Cleanup(func() { *base = old; srv.Close() })
	return &n
}

func merklemapRun(src Source, s *Session) ([]string, error) {
	var got []string
	err := src.Enumerate(context.Background(), "example.com", s, func(r string) { got = append(got, r) })
	return got, err
}

func merklemapSorted(in []string) []string {
	out := append([]string(nil), in...)
	sort.Strings(out)
	return out
}

func merklemapWrite(w http.ResponseWriter, body []byte) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(body)
}

// merklemapBattery checks the generic failure modes every keyed adapter must map.
// invalidBody is a 200 body the adapter must reject with ErrUnexpected.
func merklemapBattery(t *testing.T, src Source, base *string, creds map[string]string, invalidBody string) {
	t.Helper()
	name := src.Info().Name
	t.Run("nokey", func(t *testing.T) {
		if src.Info().Auth != AuthRequired {
			t.Skip("no key needed")
		}
		hits := merklemapServe(t, base, func(w http.ResponseWriter, r *http.Request) { t.Error("request without key") })
		_, err := merklemapRun(src, merklemapSession(name, map[string]string{}, 3))
		if !errors.Is(err, ErrNoKey) || *hits != 0 {
			t.Fatalf("err=%v hits=%d", err, *hits)
		}
	})
	for _, c := range []struct {
		status int
		want   error
	}{{401, ErrAuth}, {403, ErrAuth}, {429, ErrRateLimited}, {503, ErrUnavailable}} {
		c := c
		t.Run(http.StatusText(c.status), func(t *testing.T) {
			merklemapServe(t, base, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(c.status) })
			_, err := merklemapRun(src, merklemapSession(name, creds, 3))
			if !errors.Is(err, c.want) {
				t.Fatalf("err=%v want %v", err, c.want)
			}
		})
	}
	t.Run("invalid", func(t *testing.T) {
		merklemapServe(t, base, func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(invalidBody)) })
		_, err := merklemapRun(src, merklemapSession(name, creds, 3))
		if !errors.Is(err, ErrUnexpected) {
			t.Fatalf("err=%v", err)
		}
	})
}

func merklemapExpect(t *testing.T, got []string, err error, want ...string) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
	// emitted names are raw; compare after the same scoping the runner applies
	var norm []string
	for _, g := range got {
		if n, ok := scope.Normalize(g, "example.com"); ok {
			norm = append(norm, n)
		}
	}
	if !reflect.DeepEqual(merklemapSorted(norm), merklemapSorted(want)) {
		t.Fatalf("got %v want %v", norm, want)
	}
}

// ---- merklemap tests ----

var merklemapCreds = map[string]string{"api_key": "secret-key"}

func TestMerklemapInfo(t *testing.T) {
	i := merklemap{}.Info()
	if i.Auth != AuthRequired || len(i.CredFields) != 1 || i.CredFields[0] != "api_key" || i.Disabled {
		t.Fatalf("%+v", i)
	}
}

func TestMerklemapPagination(t *testing.T) {
	var pages []string
	merklemapServe(t, &merklemapBaseURL, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/search" {
			t.Errorf("path %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer secret-key" {
			t.Errorf("auth header %q", r.Header.Get("Authorization"))
		}
		if q := r.URL.Query().Get("query"); q != "*.example.com" {
			t.Errorf("query %q", q)
		}
		p := r.URL.Query().Get("page")
		pages = append(pages, p)
		if p == "0" {
			merklemapWrite(w, merklemapFixture(t, "merklemap", "page0.json"))
			return
		}
		merklemapWrite(w, merklemapFixture(t, "merklemap", "page1.json"))
	})
	got, err := merklemapRun(merklemap{}, merklemapSession("merklemap", merklemapCreds, 10))
	merklemapExpect(t, got, err, "www.example.com", "dev.example.com", "api.example.com")
	if !reflect.DeepEqual(pages, []string{"0", "1"}) { // stops once count reached
		t.Fatalf("pages %v", pages)
	}
}

func TestMerklemapMaxPages(t *testing.T) {
	hits := merklemapServe(t, &merklemapBaseURL, func(w http.ResponseWriter, r *http.Request) {
		merklemapWrite(w, []byte(`{"count":1000,"results":[{"hostname":"a.example.com"}]}`))
	})
	_, err := merklemapRun(merklemap{}, merklemapSession("merklemap", merklemapCreds, 2))
	if err != nil || *hits != 2 {
		t.Fatalf("err=%v hits=%d", err, *hits)
	}
}

func TestMerklemapEmpty(t *testing.T) {
	merklemapServe(t, &merklemapBaseURL, func(w http.ResponseWriter, r *http.Request) {
		merklemapWrite(w, merklemapFixture(t, "merklemap", "empty.json"))
	})
	got, err := merklemapRun(merklemap{}, merklemapSession("merklemap", merklemapCreds, 3))
	if err != nil || len(got) != 0 {
		t.Fatalf("%v %v", got, err)
	}
}

func TestMerklemapErrorIn200(t *testing.T) {
	merklemapServe(t, &merklemapBaseURL, func(w http.ResponseWriter, r *http.Request) {
		merklemapWrite(w, merklemapFixture(t, "merklemap", "error200.json"))
	})
	_, err := merklemapRun(merklemap{}, merklemapSession("merklemap", merklemapCreds, 3))
	if !errors.Is(err, ErrAuth) {
		t.Fatalf("err=%v", err)
	}
}

func TestMerklemapMissingResults(t *testing.T) {
	merklemapServe(t, &merklemapBaseURL, func(w http.ResponseWriter, r *http.Request) { merklemapWrite(w, []byte(`{"foo":1}`)) })
	_, err := merklemapRun(merklemap{}, merklemapSession("merklemap", merklemapCreds, 3))
	if !errors.Is(err, ErrUnexpected) {
		t.Fatalf("err=%v", err)
	}
}

func TestMerklemapBattery(t *testing.T) {
	merklemapBattery(t, merklemap{}, &merklemapBaseURL, merklemapCreds, "<html>oops</html>")
}

func TestMerklemapClassify(t *testing.T) {
	if !errors.Is(merklemapClassify("Too many requests"), ErrRateLimited) || !errors.Is(merklemapClassify("weird"), ErrUnexpected) {
		t.Fatal("classify")
	}
	if !reflect.DeepEqual(merklemapFlatten([]any{"a", []any{"b"}, 3.0}), []string{"a", "b"}) {
		t.Fatal("flatten")
	}
}
