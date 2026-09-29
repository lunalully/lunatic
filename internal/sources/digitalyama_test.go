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

func digitalyamaRun(t *testing.T, h http.HandlerFunc, creds map[string]string) ([]string, error) {
	t.Helper()
	srv := httptest.NewServer(h)
	defer srv.Close()
	old := digitalyamaBaseURL
	digitalyamaBaseURL = srv.URL
	defer func() { digitalyamaBaseURL = old }()
	s := &Session{
		HTTP:     httpx.New(httpx.Options{SourceName: "digitalyama", TargetDomain: "example.com", MaxRetries: -1, BackoffBase: time.Millisecond}),
		Creds:    creds,
		MaxPages: 10,
	}
	var got []string
	err := digitalyama{}.Enumerate(context.Background(), "example.com", s, func(r string) { got = append(got, r) })
	return got, err
}

func digitalyamaBody(body string, status int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}
}

func TestDigitalyamaSuccess(t *testing.T) {
	fx, err := os.ReadFile("testdata/digitalyama/success.json")
	if err != nil {
		t.Fatal(err)
	}
	var seen *http.Request
	got, err := digitalyamaRun(t, func(w http.ResponseWriter, r *http.Request) {
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
	if seen.URL.Path != "/subdomain_finder" || seen.URL.Query().Get("domain") != "example.com" || seen.Header.Get("x-api-key") != "k123" {
		t.Fatalf("bad request %v %v", seen.URL, seen.Header)
	}
}

func TestDigitalyamaEmpty(t *testing.T) {
	got, err := digitalyamaRun(t, digitalyamaBody(`{"query":"example.com","count":0,"subdomains":[]}`, 200), map[string]string{"api_key": "k123"})
	if err != nil || len(got) != 0 {
		t.Fatalf("got %v, %v", got, err)
	}
}

func TestDigitalyamaHTTPErrors(t *testing.T) {
	for _, c := range []struct {
		status int
		want   error
	}{{401, ErrAuth}, {403, ErrAuth}, {429, ErrRateLimited}, {503, ErrUnavailable}} {
		got, err := digitalyamaRun(t, digitalyamaBody("{}", c.status), map[string]string{"api_key": "k123"})
		if !errors.Is(err, c.want) || len(got) != 0 {
			t.Fatalf("status %d: got %v, %v want %v", c.status, got, err, c.want)
		}
	}
}

func TestDigitalyamaProviderErrorIn200(t *testing.T) {
	for _, c := range []struct {
		body string
		want error
	}{
		{"{\"detail\":[{\"msg\":\"field required\"}]}", ErrUnexpected},
	} {
		got, err := digitalyamaRun(t, digitalyamaBody(c.body, 200), map[string]string{"api_key": "k123"})
		if !errors.Is(err, c.want) || len(got) != 0 {
			t.Fatalf("body %s: got %v, %v want %v", c.body, got, err, c.want)
		}
	}
}

func TestDigitalyamaInvalidJSON(t *testing.T) {
	_, err := digitalyamaRun(t, digitalyamaBody("<html>oops</html>", 200), map[string]string{"api_key": "k123"})
	if !errors.Is(err, ErrUnexpected) {
		t.Fatalf("got %v", err)
	}
}

func TestDigitalyamaMissingKey(t *testing.T) {
	called := false
	_, err := digitalyamaRun(t, func(w http.ResponseWriter, r *http.Request) { called = true }, map[string]string{})
	if !errors.Is(err, ErrNoKey) || called {
		t.Fatalf("err=%v called=%v", err, called)
	}
}

func TestDigitalyamaInfo(t *testing.T) {
	i := digitalyama{}.Info()
	if i.Name != "digitalyama" || i.Auth != AuthRequired || !reflect.DeepEqual(i.CredFields, []string{"api_key"}) {
		t.Fatalf("%+v", i)
	}
}
