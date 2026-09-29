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

func anubisRun(t *testing.T, h http.HandlerFunc, creds map[string]string) ([]string, error) {
	t.Helper()
	srv := httptest.NewServer(h)
	defer srv.Close()
	old := anubisBaseURL
	anubisBaseURL = srv.URL
	defer func() { anubisBaseURL = old }()
	s := &Session{
		HTTP:     httpx.New(httpx.Options{SourceName: "anubis", TargetDomain: "example.com", MaxRetries: -1, BackoffBase: time.Millisecond}),
		Creds:    creds,
		MaxPages: 10,
	}
	var got []string
	err := anubis{}.Enumerate(context.Background(), "example.com", s, func(r string) { got = append(got, r) })
	return got, err
}

func anubisBody(body string, status int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}
}

func TestAnubisSuccess(t *testing.T) {
	fx, err := os.ReadFile("testdata/anubis/success.json")
	if err != nil {
		t.Fatal(err)
	}
	var seen *http.Request
	got, err := anubisRun(t, func(w http.ResponseWriter, r *http.Request) {
		seen = r
		_, _ = w.Write(fx)
	}, map[string]string{})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"www.example.com", "api.example.com", "*.example.com", "junk.other.test"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q want %q", got, want)
	}
	if seen.Method != http.MethodGet {
		t.Fatalf("method %s", seen.Method)
	}
	if seen.URL.Path != "/anubis/subdomains/example.com" {
		t.Fatalf("path %s", seen.URL.Path)
	}
}

func TestAnubisEmpty(t *testing.T) {
	got, err := anubisRun(t, anubisBody(`[]`, 200), map[string]string{})
	if err != nil || len(got) != 0 {
		t.Fatalf("got %v, %v", got, err)
	}
}

func TestAnubisHTTPErrors(t *testing.T) {
	for _, c := range []struct {
		status int
		want   error
	}{{401, ErrAuth}, {403, ErrAuth}, {429, ErrRateLimited}, {503, ErrUnavailable}} {
		got, err := anubisRun(t, anubisBody("{}", c.status), map[string]string{})
		if !errors.Is(err, c.want) || len(got) != 0 {
			t.Fatalf("status %d: got %v, %v want %v", c.status, got, err, c.want)
		}
	}
}

func TestAnubisProviderErrorIn200(t *testing.T) {
	for _, c := range []struct {
		body string
		want error
	}{
		{"{\"error\":\"x\"}", ErrUnexpected},
	} {
		got, err := anubisRun(t, anubisBody(c.body, 200), map[string]string{})
		if !errors.Is(err, c.want) || len(got) != 0 {
			t.Fatalf("body %s: got %v, %v want %v", c.body, got, err, c.want)
		}
	}
}

func TestAnubisInvalidJSON(t *testing.T) {
	_, err := anubisRun(t, anubisBody("<html>oops</html>", 200), map[string]string{})
	if !errors.Is(err, ErrUnexpected) {
		t.Fatalf("got %v", err)
	}
}

func TestAnubisNullIsEmpty(t *testing.T) {
	got, err := anubisRun(t, anubisBody("null", 200), nil)
	if err != nil || len(got) != 0 {
		t.Fatalf("%v %v", got, err)
	}
	if i := (anubis{}).Info(); i.Auth != AuthNone || !i.Default {
		t.Fatalf("%+v", i)
	}
}
