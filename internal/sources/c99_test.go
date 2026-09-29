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

func c99Run(t *testing.T, h http.HandlerFunc, creds map[string]string) ([]string, error) {
	t.Helper()
	srv := httptest.NewServer(h)
	defer srv.Close()
	old := c99BaseURL
	c99BaseURL = srv.URL
	defer func() { c99BaseURL = old }()
	s := &Session{
		HTTP:     httpx.New(httpx.Options{SourceName: "c99", TargetDomain: "example.com", MaxRetries: -1, BackoffBase: time.Millisecond}),
		Creds:    creds,
		MaxPages: 10,
	}
	var got []string
	err := c99{}.Enumerate(context.Background(), "example.com", s, func(r string) { got = append(got, r) })
	return got, err
}

func c99Body(body string, status int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}
}

func TestC99Success(t *testing.T) {
	fx, err := os.ReadFile("testdata/c99/success.json")
	if err != nil {
		t.Fatal(err)
	}
	var seen *http.Request
	got, err := c99Run(t, func(w http.ResponseWriter, r *http.Request) {
		seen = r
		_, _ = w.Write(fx)
	}, map[string]string{"api_key": "k123"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"www.example.com", "api.example.com"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q want %q", got, want)
	}
	if seen.Method != http.MethodGet {
		t.Fatalf("method %s", seen.Method)
	}
	q := seen.URL.Query()
	if seen.URL.Path != "/subdomainfinder" || q.Get("key") != "k123" || q.Get("domain") != "example.com" || q.Has("realtime") || q.Has("scan") {
		t.Fatalf("bad request %v", seen.URL)
	}
}

func TestC99Empty(t *testing.T) {
	got, err := c99Run(t, c99Body(`{"success":true,"subdomains":[]}`, 200), map[string]string{"api_key": "k123"})
	if err != nil || len(got) != 0 {
		t.Fatalf("got %v, %v", got, err)
	}
}

func TestC99HTTPErrors(t *testing.T) {
	for _, c := range []struct {
		status int
		want   error
	}{{401, ErrAuth}, {403, ErrAuth}, {429, ErrRateLimited}, {503, ErrUnavailable}} {
		got, err := c99Run(t, c99Body("{}", c.status), map[string]string{"api_key": "k123"})
		if !errors.Is(err, c.want) || len(got) != 0 {
			t.Fatalf("status %d: got %v, %v want %v", c.status, got, err, c.want)
		}
	}
}

func TestC99ProviderErrorIn200(t *testing.T) {
	for _, c := range []struct {
		body string
		want error
	}{
		{"{\"success\":false,\"error\":\"Invalid API key\"}", ErrAuth},
		{"{\"success\":false,\"error\":\"rate limit exceeded\"}", ErrRateLimited},
		{"{\"success\":false,\"error\":\"nope\"}", ErrUnexpected},
	} {
		got, err := c99Run(t, c99Body(c.body, 200), map[string]string{"api_key": "k123"})
		if !errors.Is(err, c.want) || len(got) != 0 {
			t.Fatalf("body %s: got %v, %v want %v", c.body, got, err, c.want)
		}
	}
}

func TestC99InvalidJSON(t *testing.T) {
	_, err := c99Run(t, c99Body("<html>oops</html>", 200), map[string]string{"api_key": "k123"})
	if !errors.Is(err, ErrUnexpected) {
		t.Fatalf("got %v", err)
	}
}

func TestC99MissingKey(t *testing.T) {
	called := false
	_, err := c99Run(t, func(w http.ResponseWriter, r *http.Request) { called = true }, map[string]string{})
	if !errors.Is(err, ErrNoKey) || called {
		t.Fatalf("err=%v called=%v", err, called)
	}
}

func TestC99Info(t *testing.T) {
	i := c99{}.Info()
	if i.Name != "c99" || i.Auth != AuthRequired || !reflect.DeepEqual(i.CredFields, []string{"api_key"}) {
		t.Fatalf("%+v", i)
	}
}
