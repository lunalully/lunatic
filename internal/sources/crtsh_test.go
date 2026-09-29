package sources

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/lunalully/lunatic/internal/httpx"
)

func crtshRun(t *testing.T, h http.HandlerFunc) ([]string, error) {
	t.Helper()
	srv := httptest.NewServer(h)
	defer srv.Close()
	old := crtshBaseURL
	crtshBaseURL = srv.URL
	defer func() { crtshBaseURL = old }()
	s := &Session{
		HTTP:     httpx.New(httpx.Options{SourceName: "crtsh", TargetDomain: "example.com", MaxRetries: -1, BackoffBase: time.Millisecond}),
		MaxPages: 10,
	}
	var got []string
	err := crtsh{}.Enumerate(context.Background(), "example.com", s, func(r string) { got = append(got, r) })
	return got, err
}

func TestCrtshSuccess(t *testing.T) {
	fx, err := os.ReadFile("testdata/crtsh/success.json")
	if err != nil {
		t.Fatal(err)
	}
	var seen *http.Request
	got, err := crtshRun(t, func(w http.ResponseWriter, r *http.Request) {
		seen = r
		_, _ = w.Write(fx)
	})
	if err != nil {
		t.Fatal(err)
	}
	// Raw names: wildcard and out-of-scope kept for the runner to filter; e-mail SAN dropped.
	want := []string{"www.example.com", "example.com", "*.example.com", "api.example.com", "x.other.test"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q", got)
	}
	if seen.URL.Query().Get("q") != "%.example.com" || seen.URL.Query().Get("output") != "json" {
		t.Fatalf("query %v", seen.URL.RawQuery)
	}
	if seen.URL.RawQuery != "q=%25.example.com&output=json" {
		t.Fatalf("raw query %q", seen.URL.RawQuery)
	}
}

func TestCrtshEmpty(t *testing.T) {
	got, err := crtshRun(t, func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("[]")) })
	if err != nil || len(got) != 0 {
		t.Fatalf("%v %v", got, err)
	}
}

func TestCrtshErrors(t *testing.T) {
	for _, c := range []struct {
		status int
		body   string
		want   error
	}{
		{429, "", ErrRateLimited},
		{503, "busy", ErrUnavailable},
		{502, "<html>Bad gateway</html>", ErrUnavailable},
		{200, "<html>error</html>", ErrUnexpected},
		{200, "", ErrUnexpected},
		{403, "", ErrAuth},
	} {
		got, err := crtshRun(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(c.status)
			_, _ = w.Write([]byte(c.body))
		})
		if !errors.Is(err, c.want) || len(got) != 0 {
			t.Fatalf("%d %q: %v %v", c.status, c.body, got, err)
		}
	}
}

func TestCrtshInfo(t *testing.T) {
	if i := (crtsh{}).Info(); i.Auth != AuthNone || !i.Default || i.Disabled {
		t.Fatalf("%+v", i)
	}
}
