package sources

import (
	"net/http"
	"testing"
)

func TestLeakixSuccess(t *testing.T) {
	var key, path string
	got, err := g2run(t, leakix{}, &leakixBaseURL, func(w http.ResponseWriter, r *http.Request) {
		key, path = r.Header.Get("api-key"), r.URL.Path
		w.Write(g2fixture(t, "leakix", "success.json"))
	}, map[string]string{"api_key": "K"}, 1)
	if err != nil {
		t.Fatal(err)
	}
	g2want(t, got, "www.example.com", "api.example.com")
	if key != "K" || path != "/api/subdomains/example.com" {
		t.Fatalf("key=%q path=%q", key, path)
	}
}

func TestLeakixEmpty(t *testing.T) {
	for _, b := range []string{"[]", "null", ""} {
		got, err := g2run(t, leakix{}, &leakixBaseURL, g2serve(200, []byte(b)), map[string]string{"api_key": "K"}, 1)
		if err != nil || len(got) != 0 {
			t.Fatalf("%q: %v %v", b, got, err)
		}
	}
}

func TestLeakixErrors(t *testing.T) {
	g2errors(t, leakix{}, &leakixBaseURL, map[string]string{"api_key": "K"}, "<html>")
}
