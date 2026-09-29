package sources

import (
	"net/http"
	"testing"
)

func TestDnsrepoSuccessCredsPlacement(t *testing.T) {
	var apikey, token, search string
	got, err := g2run(t, dnsrepo{}, &dnsrepoBaseURL, func(w http.ResponseWriter, r *http.Request) {
		apikey, search, token = r.URL.Query().Get("apikey"), r.URL.Query().Get("search"), r.Header.Get("X-API-Access")
		w.Write(g2fixture(t, "dnsrepo", "success.json"))
	}, map[string]string{"apikey": "AK", "token": "TK"}, 5)
	if err != nil {
		t.Fatal(err)
	}
	g2want(t, got, "www.example.com", "api.example.com", "mail.example.com")
	if apikey != "AK" || token != "TK" || search != "example.com" {
		t.Fatalf("apikey=%q token=%q search=%q", apikey, token, search)
	}
}

func TestDnsrepoPaginationAndMaxPages(t *testing.T) {
	// A server that ignores paging returns the same rows: must stop (no new names) rather than loop.
	n := 0
	full := make([]byte, 0)
	full = append(full, '[')
	for i := 0; i < dnsrepoLimit; i++ {
		if i > 0 {
			full = append(full, ',')
		}
		full = append(full, []byte(`{"domain":"h`+itoa(i)+`.example.com"}`)...)
	}
	full = append(full, ']')
	got, err := g2run(t, dnsrepo{}, &dnsrepoBaseURL, func(w http.ResponseWriter, r *http.Request) {
		n++
		w.Write(full)
	}, map[string]string{"apikey": "AK", "token": "TK"}, 5)
	if err != nil || n != 2 || len(got) != dnsrepoLimit {
		t.Fatalf("n=%d got=%d err=%v", n, len(got), err)
	}
	// Distinct full pages: stop at MaxPages.
	n = 0
	_, err = g2run(t, dnsrepo{}, &dnsrepoBaseURL, func(w http.ResponseWriter, r *http.Request) {
		n++
		b := []byte("[")
		for i := 0; i < dnsrepoLimit; i++ {
			if i > 0 {
				b = append(b, ',')
			}
			b = append(b, []byte(`{"domain":"p`+itoa(n)+"h"+itoa(i)+`.example.com"}`)...)
		}
		w.Write(append(b, ']'))
	}, map[string]string{"apikey": "AK", "token": "TK"}, 3)
	if err != nil || n != 3 {
		t.Fatalf("n=%d err=%v", n, err)
	}
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	s := ""
	for ; i > 0; i /= 10 {
		s = string(rune('0'+i%10)) + s
	}
	return s
}

func TestDnsrepoEmptyAndErrorObject(t *testing.T) {
	c := map[string]string{"apikey": "AK", "token": "TK"}
	got, err := g2run(t, dnsrepo{}, &dnsrepoBaseURL, g2serve(200, []byte("[]")), c, 5)
	if err != nil || len(got) != 0 {
		t.Fatalf("%v %v", got, err)
	}
	_, err = g2run(t, dnsrepo{}, &dnsrepoBaseURL, g2serve(200, g2fixture(t, "dnsrepo", "error.json")), c, 5)
	g2is(t, err, ErrAuth)
}

func TestDnsrepoErrors(t *testing.T) {
	c := map[string]string{"apikey": "AK", "token": "TK"}
	g2errors(t, dnsrepo{}, &dnsrepoBaseURL, c, "<html>")
	_, err := g2run(t, dnsrepo{}, &dnsrepoBaseURL, g2serve(200, []byte("[]")), map[string]string{"apikey": "AK"}, 5)
	g2is(t, err, ErrNoKey)
}
