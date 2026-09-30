package sources

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/lunalully/lunatic/internal/httpx"
)

func internetdbRun(t *testing.T, h http.HandlerFunc, ips []string) ([]string, error) {
	t.Helper()
	srv := httptest.NewServer(h)
	defer srv.Close()
	old := internetdbBaseURL
	internetdbBaseURL = srv.URL
	defer func() { internetdbBaseURL = old }()
	s := &Session{
		HTTP:     httpx.New(httpx.Options{SourceName: "internetdb", TargetDomain: "example.com", MaxRetries: -1, BackoffBase: time.Millisecond}),
		MaxPages: 10,
		IPs:      ips,
	}
	var got []string
	err := internetdb{}.Enumerate(context.Background(), "example.com", s, func(r string) { got = append(got, r) })
	sort.Strings(got)
	return got, err
}

func TestInternetdbSuccessAnd404(t *testing.T) {
	var mu sync.Mutex
	var paths []string
	got, err := internetdbRun(t, func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		paths = append(paths, r.URL.Path)
		mu.Unlock()
		if r.URL.Path == "/198.51.100.7" {
			w.WriteHeader(404)
			w.Write([]byte(`{"detail":"No information available"}`))
			return
		}
		w.Write(g2fixture(t, "internetdb", "success.json"))
	}, []string{"93.184.216.34", "198.51.100.7", "not-an-ip", "2606:2800:220:1::1"})
	if err != nil {
		t.Fatal(err)
	}
	g2want(t, got, "a.example.com", "a.example.com", "b.example.com", "b.example.com")
	if len(paths) != 3 || paths[0] != "/93.184.216.34" || paths[2] != "/2606:2800:220:1::1" {
		t.Fatalf("paths %v", paths)
	}
}

func TestInternetdbNoIPs(t *testing.T) {
	n := 0
	got, err := internetdbRun(t, func(w http.ResponseWriter, r *http.Request) { n++ }, nil)
	if err != nil || len(got) != 0 || n != 0 {
		t.Fatalf("got=%v err=%v requests=%d", got, err, n)
	}
}

func TestInternetdbLimit(t *testing.T) {
	var ips []string
	for i := 0; i < 80; i++ {
		ips = append(ips, fmt.Sprintf("93.184.%d.%d", i/250, i%250+1))
	}
	var mu sync.Mutex
	n := 0
	_, err := internetdbRun(t, func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		n++
		mu.Unlock()
		w.Write([]byte(`{"hostnames":[]}`))
	}, ips)
	if err != nil || n != internetdbMaxIPs {
		t.Fatalf("requests=%d err=%v", n, err)
	}
}

func TestInternetdbErrors(t *testing.T) {
	ips := []string{"93.184.216.34"}
	_, err := internetdbRun(t, g2serve(429, []byte("x")), ips)
	g2is(t, err, ErrRateLimited)
	_, err = internetdbRun(t, g2serve(200, []byte("garbage")), ips)
	g2is(t, err, ErrUnexpected)
	_, err = internetdbRun(t, g2serve(503, []byte("x")), ips)
	g2is(t, err, ErrUnavailable)
	got, err := internetdbRun(t, g2serve(200, []byte(`{}`)), ips)
	if err != nil || len(got) != 0 {
		t.Fatalf("got=%v err=%v", got, err)
	}
}

func TestInternetdbInfo(t *testing.T) {
	i := internetdb{}.Info()
	if !i.Phase2 || !i.Default || i.Auth != AuthNone || i.RPS != 1 {
		t.Fatalf("%+v", i)
	}
}
