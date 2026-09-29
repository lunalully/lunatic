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

func certspotterRun(t *testing.T, h http.HandlerFunc, creds map[string]string, maxPages int) ([]string, error) {
	t.Helper()
	srv := httptest.NewServer(h)
	defer srv.Close()
	old := certspotterBaseURL
	certspotterBaseURL = srv.URL
	defer func() { certspotterBaseURL = old }()
	s := &Session{
		HTTP:     httpx.New(httpx.Options{SourceName: "certspotter", TargetDomain: "example.com", MaxRetries: -1, BackoffBase: time.Millisecond}),
		Creds:    creds,
		MaxPages: maxPages,
	}
	var got []string
	err := certspotter{}.Enumerate(context.Background(), "example.com", s, func(r string) { got = append(got, r) })
	return got, err
}

func certspotterFixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/certspotter/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func certspotterPager(t *testing.T, reqs *[]*http.Request) http.HandlerFunc {
	p1, p2 := certspotterFixture(t, "page1.json"), certspotterFixture(t, "page2.json")
	return func(w http.ResponseWriter, r *http.Request) {
		*reqs = append(*reqs, r)
		switch r.URL.Query().Get("after") {
		case "":
			_, _ = w.Write(p1)
		case "101":
			_, _ = w.Write(p2)
		default:
			_, _ = w.Write([]byte("[]"))
		}
	}
}

func TestCertspotterPaginationAndKey(t *testing.T) {
	var reqs []*http.Request
	got, err := certspotterRun(t, certspotterPager(t, &reqs), map[string]string{"api_key": "k123"}, 10)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"example.com", "www.example.com", "*.example.com", "api.example.com", "unrelated.other.test", "dev.example.com"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q", got)
	}
	if len(reqs) != 3 {
		t.Fatalf("requests=%d want 3 (last one empty)", len(reqs))
	}
	q := reqs[0].URL.Query()
	if reqs[0].URL.Path != "/v1/issuances" || q.Get("domain") != "example.com" || q.Get("include_subdomains") != "true" || q.Get("expand") != "dns_names" {
		t.Fatalf("bad query %v", reqs[0].URL)
	}
	if reqs[0].Header.Get("Authorization") != "Bearer k123" {
		t.Fatalf("auth header %q", reqs[0].Header.Get("Authorization"))
	}
	if reqs[1].URL.Query().Get("after") != "101" {
		t.Fatalf("after=%q", reqs[1].URL.Query().Get("after"))
	}
}

func TestCertspotterNoKeyNoAuthHeader(t *testing.T) {
	var reqs []*http.Request
	if _, err := certspotterRun(t, certspotterPager(t, &reqs), map[string]string{}, 10); err != nil {
		t.Fatal(err)
	}
	if reqs[0].Header.Get("Authorization") != "" {
		t.Fatal("auth header sent without key")
	}
	if i := (certspotter{}).Info(); i.Auth != AuthOptional || !i.Default {
		t.Fatalf("%+v", i)
	}
}

func TestCertspotterMaxPages(t *testing.T) {
	var reqs []*http.Request
	got, err := certspotterRun(t, certspotterPager(t, &reqs), nil, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(reqs) != 1 || len(got) != 5 {
		t.Fatalf("reqs=%d got=%q", len(reqs), got)
	}
}

func TestCertspotterEmpty(t *testing.T) {
	got, err := certspotterRun(t, func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("[]")) }, nil, 10)
	if err != nil || len(got) != 0 {
		t.Fatalf("%v %v", got, err)
	}
}

func TestCertspotterErrors(t *testing.T) {
	for _, c := range []struct {
		status int
		body   string
		want   error
	}{
		{401, `{"code":"unauthorized"}`, ErrAuth},
		{429, `{"code":"rate_limited"}`, ErrRateLimited},
		{200, `{"code":"rate_limited","message":"slow down"}`, ErrUnexpected},
		{200, `<html>`, ErrUnexpected},
	} {
		h := func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(c.status)
			_, _ = w.Write([]byte(c.body))
		}
		got, err := certspotterRun(t, h, nil, 10)
		if !errors.Is(err, c.want) || len(got) != 0 {
			t.Fatalf("%d %s: %v %v", c.status, c.body, got, err)
		}
	}
}

func TestCertspotterRateLimitMidPagination(t *testing.T) {
	p1 := certspotterFixture(t, "page1.json")
	h := func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("after") == "" {
			_, _ = w.Write(p1)
			return
		}
		w.WriteHeader(429)
	}
	got, err := certspotterRun(t, h, nil, 10)
	if !errors.Is(err, ErrRateLimited) || len(got) != 5 {
		t.Fatalf("%v %v", got, err)
	}
}

func TestCertspotterRepeatedIDStops(t *testing.T) {
	h := func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[{"id":"7","dns_names":["a.example.com"]}]`))
	}
	_, err := certspotterRun(t, h, nil, 10)
	if !errors.Is(err, ErrUnexpected) {
		t.Fatalf("%v", err)
	}
}

func TestCertspotterContextCancel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[{"id":"1","dns_names":["a.example.com"]}]`))
	}))
	defer srv.Close()
	old := certspotterBaseURL
	certspotterBaseURL = srv.URL
	defer func() { certspotterBaseURL = old }()
	ctx, cancel := context.WithCancel(context.Background())
	s := &Session{HTTP: httpx.New(httpx.Options{SourceName: "certspotter", TargetDomain: "example.com", MaxRetries: -1}), MaxPages: 10}
	n := 0
	err := certspotter{}.Enumerate(ctx, "example.com", s, func(string) { n++; cancel() })
	if err == nil || n != 1 {
		t.Fatalf("err=%v n=%d", err, n)
	}
}
