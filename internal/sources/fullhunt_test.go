package sources

import (
	"net/http"
	"testing"
)

func TestFullhuntSuccess(t *testing.T) {
	var key, path string
	got, err := g2run(t, fullhunt{}, &fullhuntBaseURL, func(w http.ResponseWriter, r *http.Request) {
		key, path = r.Header.Get("X-API-KEY"), r.URL.Path
		w.Write(g2fixture(t, "fullhunt", "success.json"))
	}, map[string]string{"api_key": "K"}, 1)
	if err != nil {
		t.Fatal(err)
	}
	g2want(t, got, "www.example.com", "api.example.com", "dev.example.com")
	if key != "K" || path != "/api/v1/domain/example.com/subdomains" {
		t.Fatalf("key=%q path=%q", key, path)
	}
}

func TestFullhuntEmptyAndBodyErrors(t *testing.T) {
	c := map[string]string{"api_key": "K"}
	got, err := g2run(t, fullhunt{}, &fullhuntBaseURL, g2serve(200, []byte(`{"hosts":[],"message":"","status":200}`)), c, 1)
	if err != nil || len(got) != 0 {
		t.Fatalf("%v %v", got, err)
	}
	_, err = g2run(t, fullhunt{}, &fullhuntBaseURL, g2serve(200, g2fixture(t, "fullhunt", "auth.json")), c, 1)
	g2is(t, err, ErrAuth)
	_, err = g2run(t, fullhunt{}, &fullhuntBaseURL, g2serve(200, []byte(`{"message":"boom","status":500}`)), c, 1)
	g2is(t, err, ErrUnexpected)
	_, err = g2run(t, fullhunt{}, &fullhuntBaseURL, g2serve(402, []byte("credits")), c, 1)
	g2is(t, err, ErrRateLimited)
}

func TestFullhuntErrors(t *testing.T) {
	g2errors(t, fullhunt{}, &fullhuntBaseURL, map[string]string{"api_key": "K"}, "<html>")
}
