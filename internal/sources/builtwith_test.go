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

func builtwithRun(t *testing.T, h http.HandlerFunc, creds map[string]string) ([]string, error) {
	t.Helper()
	srv := httptest.NewServer(h)
	defer srv.Close()
	old := builtwithBaseURL
	builtwithBaseURL = srv.URL
	defer func() { builtwithBaseURL = old }()
	s := &Session{
		HTTP:     httpx.New(httpx.Options{SourceName: "builtwith", TargetDomain: "example.com", MaxRetries: -1, BackoffBase: time.Millisecond}),
		Creds:    creds,
		MaxPages: 10,
	}
	var got []string
	err := builtwith{}.Enumerate(context.Background(), "example.com", s, func(r string) { got = append(got, r) })
	return got, err
}

func builtwithBody(body string, status int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}
}

func TestBuiltwithSuccess(t *testing.T) {
	fx, err := os.ReadFile("testdata/builtwith/success.json")
	if err != nil {
		t.Fatal(err)
	}
	var seen *http.Request
	got, err := builtwithRun(t, func(w http.ResponseWriter, r *http.Request) {
		seen = r
		_, _ = w.Write(fx)
	}, map[string]string{"api_key": "k123"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"blog.example.com", "shop.example.com"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q want %q", got, want)
	}
	if seen.Method != http.MethodGet {
		t.Fatalf("method %s", seen.Method)
	}
	q := seen.URL.Query()
	if seen.URL.Path != "/v26/api.json" || q.Get("KEY") != "k123" || q.Get("LOOKUP") != "example.com" || q.Get("TRUST") != "" {
		t.Fatalf("bad request %v", seen.URL)
	}
}

func TestBuiltwithEmpty(t *testing.T) {
	got, err := builtwithRun(t, builtwithBody(`{"Results":[]}`, 200), map[string]string{"api_key": "k123"})
	if err != nil || len(got) != 0 {
		t.Fatalf("got %v, %v", got, err)
	}
}

func TestBuiltwithHTTPErrors(t *testing.T) {
	for _, c := range []struct {
		status int
		want   error
	}{{401, ErrAuth}, {403, ErrAuth}, {429, ErrRateLimited}, {503, ErrUnavailable}} {
		got, err := builtwithRun(t, builtwithBody("{}", c.status), map[string]string{"api_key": "k123"})
		if !errors.Is(err, c.want) || len(got) != 0 {
			t.Fatalf("status %d: got %v, %v want %v", c.status, got, err, c.want)
		}
	}
}

func TestBuiltwithProviderErrorIn200(t *testing.T) {
	for _, c := range []struct {
		body string
		want error
	}{
		{"{\"Errors\":[{\"Message\":\"Invalid API key\"}]}", ErrAuth},
		{"{\"Errors\":[{\"Message\":\"Credit limit reached\"}]}", ErrRateLimited},
		{"{\"Errors\":[{\"Message\":\"bad lookup\"}]}", ErrUnexpected},
	} {
		got, err := builtwithRun(t, builtwithBody(c.body, 200), map[string]string{"api_key": "k123"})
		if !errors.Is(err, c.want) || len(got) != 0 {
			t.Fatalf("body %s: got %v, %v want %v", c.body, got, err, c.want)
		}
	}
}

func TestBuiltwithInvalidJSON(t *testing.T) {
	_, err := builtwithRun(t, builtwithBody("<html>oops</html>", 200), map[string]string{"api_key": "k123"})
	if !errors.Is(err, ErrUnexpected) {
		t.Fatalf("got %v", err)
	}
}

func TestBuiltwithMissingKey(t *testing.T) {
	called := false
	_, err := builtwithRun(t, func(w http.ResponseWriter, r *http.Request) { called = true }, map[string]string{})
	if !errors.Is(err, ErrNoKey) || called {
		t.Fatalf("err=%v called=%v", err, called)
	}
}

func TestBuiltwithInfo(t *testing.T) {
	i := builtwith{}.Info()
	if i.Name != "builtwith" || i.Auth != AuthRequired || !reflect.DeepEqual(i.CredFields, []string{"api_key"}) {
		t.Fatalf("%+v", i)
	}
}
