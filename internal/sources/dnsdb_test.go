package sources

import (
	"net/http"
	"strings"
	"testing"
)

func TestDnsdbPagination(t *testing.T) {
	var offsets []string
	var key, accept, path string
	got, err := g2run(t, dnsdb{}, &dnsdbBaseURL, func(w http.ResponseWriter, r *http.Request) {
		key, accept, path = r.Header.Get("X-API-Key"), r.Header.Get("Accept"), r.URL.Path
		off := r.URL.Query().Get("offset")
		offsets = append(offsets, off)
		if off == "" {
			w.Write(g2fixture(t, "dnsdb", "page1.jsonl"))
		} else {
			w.Write(g2fixture(t, "dnsdb", "page2.jsonl"))
		}
	}, map[string]string{"api_key": "K"}, 10)
	if err != nil {
		t.Fatal(err)
	}
	g2want(t, got, "www.example.com", "api.example.com", "mail.example.com")
	if key != "K" || !strings.Contains(accept, "ndjson") || path != "/dnsdb/v2/lookup/rrset/name/*.example.com" {
		t.Fatalf("key=%q accept=%q path=%q", key, accept, path)
	}
	if len(offsets) != 2 || offsets[1] != "2" {
		t.Fatalf("offsets %v", offsets)
	}
}

func TestDnsdbMaxPagesStop(t *testing.T) {
	n := 0
	got, err := g2run(t, dnsdb{}, &dnsdbBaseURL, func(w http.ResponseWriter, r *http.Request) {
		n++
		w.Write(g2fixture(t, "dnsdb", "page1.jsonl")) // always "limited"
	}, map[string]string{"api_key": "K"}, 2)
	if err != nil || n != 2 || len(got) != 4 {
		t.Fatalf("n=%d got=%v err=%v", n, got, err)
	}
}

func TestDnsdbEmptyAnd404(t *testing.T) {
	got, err := g2run(t, dnsdb{}, &dnsdbBaseURL, g2serve(200, g2fixture(t, "dnsdb", "empty.jsonl")), map[string]string{"api_key": "K"}, 3)
	if err != nil || len(got) != 0 {
		t.Fatalf("%v %v", got, err)
	}
	got, err = g2run(t, dnsdb{}, &dnsdbBaseURL, g2serve(404, []byte("Error: no results found for query.")), map[string]string{"api_key": "K"}, 3)
	if err != nil || len(got) != 0 {
		t.Fatalf("%v %v", got, err)
	}
}

func TestDnsdbStreamErrors(t *testing.T) {
	c := map[string]string{"api_key": "K"}
	_, err := g2run(t, dnsdb{}, &dnsdbBaseURL, g2serve(200, g2fixture(t, "dnsdb", "truncated.jsonl")), c, 3)
	g2is(t, err, ErrUnexpected)
	_, err = g2run(t, dnsdb{}, &dnsdbBaseURL, g2serve(200, g2fixture(t, "dnsdb", "failed.jsonl")), c, 3)
	g2is(t, err, ErrUnexpected)
}

func TestDnsdbErrors(t *testing.T) {
	g2errors(t, dnsdb{}, &dnsdbBaseURL, map[string]string{"api_key": "K"}, "Error: not json")
}
