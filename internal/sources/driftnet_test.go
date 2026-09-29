package sources

import (
	"net/http"
	"strings"
	"testing"
)

func TestDriftnetSuccess(t *testing.T) {
	var auth, field string
	var paths []string
	got, err := g2run(t, driftnet{}, &driftnetBaseURL, func(w http.ResponseWriter, r *http.Request) {
		auth, field = r.Header.Get("Authorization"), r.URL.Query().Get("field")
		paths = append(paths, r.URL.Path)
		if strings.HasSuffix(r.URL.Path, "ct/log") {
			w.Write(g2fixture(t, "driftnet", "success.json"))
		} else if strings.HasSuffix(r.URL.Path, "scan/protocols") {
			w.Write(g2fixture(t, "driftnet", "second.json"))
		} else {
			w.WriteHeader(204)
		}
	}, map[string]string{"api_key": "TOK"}, 10)
	if err != nil {
		t.Fatal(err)
	}
	g2want(t, got, "www.example.com", "api.example.com", "dev.example.com", "www.example.com")
	if auth != "Bearer TOK" || field != "host:example.com" || len(paths) != len(driftnetEndpoints) {
		t.Fatalf("auth=%q field=%q paths=%v", auth, field, paths)
	}
}

func TestDriftnetMaxPagesBoundsRequests(t *testing.T) {
	n := 0
	_, err := g2run(t, driftnet{}, &driftnetBaseURL, func(w http.ResponseWriter, r *http.Request) { n++; w.WriteHeader(204) },
		map[string]string{"api_key": "TOK"}, 2)
	if err != nil || n != 2 {
		t.Fatalf("n=%d err=%v", n, err)
	}
}

func TestDriftnetEmpty(t *testing.T) {
	got, err := g2run(t, driftnet{}, &driftnetBaseURL, g2serve(204, nil), map[string]string{"api_key": "TOK"}, 10)
	if err != nil || len(got) != 0 {
		t.Fatalf("%v %v", got, err)
	}
}

func TestDriftnetAllEndpointsFail(t *testing.T) {
	_, err := g2run(t, driftnet{}, &driftnetBaseURL, g2serve(400, []byte("bad context")), map[string]string{"api_key": "TOK"}, 10)
	g2is(t, err, ErrUnexpected)
}

func TestDriftnetErrors(t *testing.T) {
	g2errors(t, driftnet{}, &driftnetBaseURL, map[string]string{"api_key": "TOK"}, "<html>")
}
