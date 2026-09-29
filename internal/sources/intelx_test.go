package sources

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

func intelxHandler(t *testing.T, poll ...string) (http.HandlerFunc, *int, *[]string) {
	n := 0
	var seen []string
	return func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Method+" "+r.URL.Path+" k="+r.URL.Query().Get("k"))
		if r.Method == http.MethodPost {
			b, _ := io.ReadAll(r.Body)
			if !strings.Contains(string(b), `"term":"example.com"`) || !strings.Contains(string(b), `"target":1`) {
				t.Errorf("body %s", b)
			}
			w.Write(g2fixture(t, "intelx", "start.json"))
			return
		}
		if r.URL.Query().Get("id") != "11111111-2222-3333-4444-555555555555" {
			t.Errorf("id %q", r.URL.Query().Get("id"))
		}
		i := n
		if i >= len(poll) {
			i = len(poll) - 1
		}
		n++
		w.Write(g2fixture(t, "intelx", poll[i]))
	}, &n, &seen
}

func TestIntelxPollingAndFiltering(t *testing.T) {
	h, n, seen := intelxHandler(t, "poll1.json", "poll2.json")
	got, err := g2run(t, intelx{}, &intelxBaseURL, h, map[string]string{"api_key": "K"}, 10)
	if err != nil {
		t.Fatal(err)
	}
	g2want(t, got, "www.example.com", "api.example.com") // email and URL selectors dropped
	if *n != 2 || (*seen)[0] != "POST /phonebook/search k=K" {
		t.Fatalf("polls=%d seen=%v", *n, *seen)
	}
}

func TestIntelxPollBoundAndTimeout(t *testing.T) {
	h, n, _ := intelxHandler(t, "pending.json")
	_, err := g2run(t, intelx{}, &intelxBaseURL, h, map[string]string{"api_key": "K"}, 3)
	g2is(t, err, ErrTimeout)
	if *n != 3 {
		t.Fatalf("polls=%d", *n)
	}
}

func TestIntelxHostCredential(t *testing.T) {
	// A configured host overrides the default base URL.
	other := ""
	hit := false
	h, _, _ := intelxHandler(t, "poll2.json")
	srv := newIntelxServer(h, &hit)
	defer srv.Close()
	other = srv.URL
	old := intelxBaseURL
	intelxBaseURL = "http://127.0.0.1:1" // would fail if used
	defer func() { intelxBaseURL = old }()
	s := g2session("intelx", map[string]string{"api_key": "K", "host": other}, 5)
	var got []string
	if err := (intelx{}).Enumerate(t.Context(), "example.com", s, func(r string) { got = append(got, r) }); err != nil {
		t.Fatal(err)
	}
	if !hit || len(got) != 1 {
		t.Fatalf("hit=%v got=%v", hit, got)
	}
	if _, err := intelxBase("evil.test/path"); err == nil {
		t.Fatal("host with path must be rejected")
	}
	if b, _ := intelxBase("2.intelx.io"); b != "https://2.intelx.io" {
		t.Fatalf("base %q", b)
	}
}

func TestIntelxEmptyAndStatuses(t *testing.T) {
	c := map[string]string{"api_key": "K"}
	empty := func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			w.Write([]byte(`{"id":"abc","status":0}`))
			return
		}
		w.Write([]byte(`{"selectors":[],"status":1}`))
	}
	got, err := g2run(t, intelx{}, &intelxBaseURL, empty, c, 5)
	if err != nil || len(got) != 0 {
		t.Fatalf("%v %v", got, err)
	}
	_, err = g2run(t, intelx{}, &intelxBaseURL, g2serve(200, []byte(`{"id":"","status":2}`)), c, 5)
	g2is(t, err, ErrRateLimited)
	_, err = g2run(t, intelx{}, &intelxBaseURL, g2serve(200, []byte(`{"id":"","status":1}`)), c, 5)
	g2is(t, err, ErrUnexpected)
	_, err = g2run(t, intelx{}, &intelxBaseURL, g2serve(402, []byte("no credits")), c, 5)
	g2is(t, err, ErrRateLimited)
}

func TestIntelxErrors(t *testing.T) {
	g2errors(t, intelx{}, &intelxBaseURL, map[string]string{"api_key": "K"}, "<html>")
}

func TestIntelxHostIsOptionalField(t *testing.T) {
	i := intelx{}.Info()
	if len(i.CredFields) != 1 || i.CredFields[0] != "api_key" || len(i.OptCredFields) != 1 || i.OptCredFields[0] != "host" {
		t.Fatalf("info %+v", i)
	}
}
