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

func alienvaultRun(t *testing.T, h http.HandlerFunc, creds map[string]string) ([]string, error) {
	t.Helper()
	srv := httptest.NewServer(h)
	defer srv.Close()
	old := alienvaultBaseURL
	alienvaultBaseURL = srv.URL
	defer func() { alienvaultBaseURL = old }()
	s := &Session{
		HTTP:     httpx.New(httpx.Options{SourceName: "alienvault", TargetDomain: "example.com", MaxRetries: -1, BackoffBase: time.Millisecond}),
		Creds:    creds,
		MaxPages: 10,
	}
	var got []string
	err := alienvault{}.Enumerate(context.Background(), "example.com", s, func(r string) { got = append(got, r) })
	return got, err
}

func alienvaultBody(body string, status int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}
}

func TestAlienvaultSuccess(t *testing.T) {
	fx, err := os.ReadFile("testdata/alienvault/success.json")
	if err != nil {
		t.Fatal(err)
	}
	var seen *http.Request
	got, err := alienvaultRun(t, func(w http.ResponseWriter, r *http.Request) {
		seen = r
		_, _ = w.Write(fx)
	}, map[string]string{"api_key": "k123"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"www.example.com", "api.example.com", "*.wild.example.com", "host.other-example.org"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q want %q", got, want)
	}
	if seen.Method != http.MethodGet {
		t.Fatalf("method %s", seen.Method)
	}
	if seen.URL.Path != "/api/v1/indicators/domain/example.com/passive_dns" || seen.Header.Get("X-OTX-API-KEY") != "k123" {
		t.Fatalf("bad request %v %v", seen.URL, seen.Header)
	}
}

func TestAlienvaultEmpty(t *testing.T) {
	got, err := alienvaultRun(t, alienvaultBody(`{"passive_dns":[]}`, 200), map[string]string{"api_key": "k123"})
	if err != nil || len(got) != 0 {
		t.Fatalf("got %v, %v", got, err)
	}
}

func TestAlienvaultHTTPErrors(t *testing.T) {
	for _, c := range []struct {
		status int
		want   error
	}{{401, ErrAuth}, {403, ErrAuth}, {429, ErrRateLimited}, {503, ErrUnavailable}} {
		got, err := alienvaultRun(t, alienvaultBody("{}", c.status), map[string]string{"api_key": "k123"})
		if !errors.Is(err, c.want) || len(got) != 0 {
			t.Fatalf("status %d: got %v, %v want %v", c.status, got, err, c.want)
		}
	}
}

func TestAlienvaultProviderErrorIn200(t *testing.T) {
	for _, c := range []struct {
		body string
		want error
	}{
		{"{\"error\":\"Invalid API key\"}", ErrAuth},
		{"{\"error\":\"Request was throttled\"}", ErrRateLimited},
		{"{\"error\":\"something broke\"}", ErrUnexpected},
	} {
		got, err := alienvaultRun(t, alienvaultBody(c.body, 200), map[string]string{"api_key": "k123"})
		if !errors.Is(err, c.want) || len(got) != 0 {
			t.Fatalf("body %s: got %v, %v want %v", c.body, got, err, c.want)
		}
	}
}

func TestAlienvaultInvalidJSON(t *testing.T) {
	_, err := alienvaultRun(t, alienvaultBody("<html>oops</html>", 200), map[string]string{"api_key": "k123"})
	if !errors.Is(err, ErrUnexpected) {
		t.Fatalf("got %v", err)
	}
}

func TestAlienvaultNoKeyStillWorks(t *testing.T) {
	var hdr string
	got, err := alienvaultRun(t, func(w http.ResponseWriter, r *http.Request) {
		hdr = r.Header.Get("X-OTX-API-KEY")
		_, _ = w.Write([]byte(`{"passive_dns":[{"hostname":"a.example.com"}]}`))
	}, map[string]string{})
	if err != nil || hdr != "" || !reflect.DeepEqual(got, []string{"a.example.com"}) {
		t.Fatalf("got %v %v hdr=%q", got, err, hdr)
	}
	if i := (alienvault{}).Info(); i.Auth != AuthOptional || !i.Default {
		t.Fatalf("%+v", i)
	}
}
