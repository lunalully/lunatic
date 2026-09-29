package sources

import (
	"net/http"
	"testing"
)

func TestDnsdumpsterSuccess(t *testing.T) {
	var key, path string
	got, err := g2run(t, dnsdumpster{}, &dnsdumpsterBaseURL, func(w http.ResponseWriter, r *http.Request) {
		key, path = r.Header.Get("X-API-Key"), r.URL.Path
		w.Write(g2fixture(t, "dnsdumpster", "success.json"))
	}, map[string]string{"api_key": "K"}, 1)
	if err != nil {
		t.Fatal(err)
	}
	g2want(t, got, "www.example.com", "api.example.com", "cdn.example.com", "mail.example.com", "ns1.example.com")
	if key != "K" || path != "/domain/example.com" {
		t.Fatalf("key=%q path=%q", key, path)
	}
}

func TestDnsdumpsterEmpty(t *testing.T) {
	got, err := g2run(t, dnsdumpster{}, &dnsdumpsterBaseURL, g2serve(200, []byte(`{"a":[],"ns":[]}`)), map[string]string{"api_key": "K"}, 1)
	if err != nil || len(got) != 0 {
		t.Fatalf("%v %v", got, err)
	}
}

func TestDnsdumpsterErrorBodies(t *testing.T) {
	c := map[string]string{"api_key": "K"}
	_, err := g2run(t, dnsdumpster{}, &dnsdumpsterBaseURL, g2serve(200, g2fixture(t, "dnsdumpster", "ratelimit.json")), c, 1)
	g2is(t, err, ErrRateLimited)
	_, err = g2run(t, dnsdumpster{}, &dnsdumpsterBaseURL, g2serve(200, g2fixture(t, "dnsdumpster", "error.json")), c, 1)
	g2is(t, err, ErrUnexpected)
}

func TestDnsdumpsterErrors(t *testing.T) {
	g2errors(t, dnsdumpster{}, &dnsdumpsterBaseURL, map[string]string{"api_key": "K"}, "<html>")
}
