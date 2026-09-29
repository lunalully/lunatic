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

func bevigilRun(t *testing.T, h http.HandlerFunc, creds map[string]string) ([]string, error) {
	t.Helper()
	srv := httptest.NewServer(h)
	defer srv.Close()
	old := bevigilBaseURL
	bevigilBaseURL = srv.URL
	defer func() { bevigilBaseURL = old }()
	s := &Session{
		HTTP:     httpx.New(httpx.Options{SourceName: "bevigil", TargetDomain: "example.com", MaxRetries: -1, BackoffBase: time.Millisecond}),
		Creds:    creds,
		MaxPages: 10,
	}
	var got []string
	err := bevigil{}.Enumerate(context.Background(), "example.com", s, func(r string) { got = append(got, r) })
	return got, err
}

func bevigilBody(body string, status int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}
}

func TestBevigilSuccess(t *testing.T) {
	fx, err := os.ReadFile("testdata/bevigil/success.json")
	if err != nil {
		t.Fatal(err)
	}
	var seen *http.Request
	got, err := bevigilRun(t, func(w http.ResponseWriter, r *http.Request) {
		seen = r
		_, _ = w.Write(fx)
	}, map[string]string{"api_key": "k123"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"www.example.com", "api.example.com", "app.other.test"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q want %q", got, want)
	}
	if seen.Method != http.MethodGet {
		t.Fatalf("method %s", seen.Method)
	}
	if seen.URL.Path != "/api/example.com/subdomains/" || seen.Header.Get("X-Access-Token") != "k123" {
		t.Fatalf("bad request %v %v", seen.URL, seen.Header)
	}
}

func TestBevigilEmpty(t *testing.T) {
	got, err := bevigilRun(t, bevigilBody(`{"domain":"example.com","subdomains":[]}`, 200), map[string]string{"api_key": "k123"})
	if err != nil || len(got) != 0 {
		t.Fatalf("got %v, %v", got, err)
	}
}

func TestBevigilHTTPErrors(t *testing.T) {
	for _, c := range []struct {
		status int
		want   error
	}{{401, ErrAuth}, {403, ErrAuth}, {429, ErrRateLimited}, {503, ErrUnavailable}} {
		got, err := bevigilRun(t, bevigilBody("{}", c.status), map[string]string{"api_key": "k123"})
		if !errors.Is(err, c.want) || len(got) != 0 {
			t.Fatalf("status %d: got %v, %v want %v", c.status, got, err, c.want)
		}
	}
}

func TestBevigilProviderErrorIn200(t *testing.T) {
	for _, c := range []struct {
		body string
		want error
	}{
		{"{\"detail\":\"quota exceeded\"}", ErrUnexpected},
	} {
		got, err := bevigilRun(t, bevigilBody(c.body, 200), map[string]string{"api_key": "k123"})
		if !errors.Is(err, c.want) || len(got) != 0 {
			t.Fatalf("body %s: got %v, %v want %v", c.body, got, err, c.want)
		}
	}
}

func TestBevigilInvalidJSON(t *testing.T) {
	_, err := bevigilRun(t, bevigilBody("<html>oops</html>", 200), map[string]string{"api_key": "k123"})
	if !errors.Is(err, ErrUnexpected) {
		t.Fatalf("got %v", err)
	}
}

func TestBevigilMissingKey(t *testing.T) {
	called := false
	_, err := bevigilRun(t, func(w http.ResponseWriter, r *http.Request) { called = true }, map[string]string{})
	if !errors.Is(err, ErrNoKey) || called {
		t.Fatalf("err=%v called=%v", err, called)
	}
}

func TestBevigilInfo(t *testing.T) {
	i := bevigil{}.Info()
	if i.Name != "bevigil" || i.Auth != AuthRequired || !reflect.DeepEqual(i.CredFields, []string{"api_key"}) {
		t.Fatalf("%+v", i)
	}
}
