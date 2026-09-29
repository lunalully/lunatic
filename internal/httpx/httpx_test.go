package httpx

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type dialLog struct {
	mu    sync.Mutex
	addrs []string
}

func (d *dialLog) dial(ctx context.Context, network, addr string) (net.Conn, error) {
	d.mu.Lock()
	d.addrs = append(d.addrs, addr)
	d.mu.Unlock()
	var nd net.Dialer
	return nd.DialContext(ctx, network, addr)
}

func (d *dialLog) has(sub string) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	for _, a := range d.addrs {
		if strings.Contains(a, sub) {
			return true
		}
	}
	return false
}

// newClient returns a client whose backoff sleeps are recorded, not waited.
func newClient(o Options) (*Client, *[]time.Duration) {
	o.BackoffBase = time.Millisecond
	c := New(o)
	var mu sync.Mutex
	var slept []time.Duration
	c.sleep = func(ctx context.Context, d time.Duration) error {
		mu.Lock()
		slept = append(slept, d)
		mu.Unlock()
		return ctx.Err()
	}
	return c, &slept
}

func TestRetryAfterThenSuccess(t *testing.T) {
	var n int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&n, 1) == 1 {
			w.Header().Set("Retry-After", "3")
			w.WriteHeader(429)
			return
		}
		w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()
	c, slept := newClient(Options{TargetDomain: "example.com"})
	var v struct{ OK bool }
	if err := c.GetJSON(context.Background(), srv.URL, nil, &v); err != nil || !v.OK {
		t.Fatalf("err=%v v=%v", err, v)
	}
	if len(*slept) != 1 || (*slept)[0] != 3*time.Second {
		t.Fatalf("slept = %v; want [3s]", *slept)
	}
}

func TestRetryAfterCapAndDate(t *testing.T) {
	h := http.Header{}
	h.Set("Retry-After", "3600")
	if d, ok := retryAfter(h); !ok || d != time.Hour {
		t.Fatalf("seconds parse: %v %v", d, ok)
	}
	h.Set("Retry-After", time.Now().Add(30*time.Second).UTC().Format(http.TimeFormat))
	if d, ok := retryAfter(h); !ok || d < 25*time.Second || d > 31*time.Second {
		t.Fatalf("date parse: %v %v", d, ok)
	}
	var n int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&n, 1) == 1 {
			w.Header().Set("Retry-After", "3600")
			w.WriteHeader(429)
			return
		}
	}))
	defer srv.Close()
	c, slept := newClient(Options{})
	if _, err := c.Get(context.Background(), srv.URL, nil); err != nil {
		t.Fatal(err)
	}
	if (*slept)[0] != 60*time.Second {
		t.Fatalf("cap: slept %v", *slept)
	}
}

func TestRateLimitedExhausted(t *testing.T) {
	var n int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&n, 1)
		w.WriteHeader(429)
	}))
	defer srv.Close()
	c, _ := newClient(Options{MaxRetries: 2})
	_, err := c.Get(context.Background(), srv.URL, nil)
	if !errors.Is(err, ErrRateLimited) || n != 3 {
		t.Fatalf("err=%v attempts=%d", err, n)
	}
}

func TestAuthNoRetry(t *testing.T) {
	for _, code := range []int{401, 403} {
		var n int32
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			atomic.AddInt32(&n, 1)
			w.WriteHeader(code)
		}))
		c, _ := newClient(Options{})
		_, err := c.Get(context.Background(), srv.URL, nil)
		srv.Close()
		if !errors.Is(err, ErrAuth) || n != 1 {
			t.Fatalf("code %d: err=%v attempts=%d", code, err, n)
		}
	}
}

func TestServerErrorRetriesThenUnavailable(t *testing.T) {
	var n int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&n, 1)
		w.WriteHeader(503)
	}))
	defer srv.Close()
	c, slept := newClient(Options{})
	_, err := c.Get(context.Background(), srv.URL, nil)
	if !errors.Is(err, ErrUnavailable) || n != 3 || len(*slept) != 2 {
		t.Fatalf("err=%v attempts=%d slept=%v", err, n, *slept)
	}
	var he *Error
	if !errors.As(err, &he) || he.StatusCode != 503 {
		t.Fatalf("missing status: %v", err)
	}
}

func TestOtherStatusUnexpected(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(404) }))
	defer srv.Close()
	c, _ := newClient(Options{})
	resp, err := c.Get(context.Background(), srv.URL, nil)
	var he *Error
	if !errors.Is(err, ErrUnexpected) || !errors.As(err, &he) || he.StatusCode != 404 || resp == nil {
		t.Fatalf("err=%v resp=%v", err, resp)
	}
}

func TestConnectionRefusedRetries(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	url := srv.URL
	srv.Close()
	c, slept := newClient(Options{})
	_, err := c.Get(context.Background(), url, nil)
	if !errors.Is(err, ErrUnavailable) || len(*slept) != 2 {
		t.Fatalf("err=%v slept=%v", err, *slept)
	}
}

func TestTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer srv.Close()
	c, _ := newClient(Options{Timeout: 50 * time.Millisecond})
	_, err := c.Get(context.Background(), srv.URL, nil)
	if !errors.Is(err, ErrTimeout) {
		t.Fatalf("err=%v", err)
	}
	// Parent deadline also maps to ErrTimeout.
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	c2, _ := newClient(Options{})
	if _, err := c2.Get(ctx, srv.URL, nil); !errors.Is(err, ErrTimeout) {
		t.Fatalf("parent deadline: %v", err)
	}
}

func TestContextCancel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer srv.Close()
	ctx, cancel := context.WithCancel(context.Background())
	c, _ := newClient(Options{})
	go func() { time.Sleep(30 * time.Millisecond); cancel() }()
	_, err := c.Get(ctx, srv.URL, nil)
	if !errors.Is(err, context.Canceled) || errors.Is(err, ErrTimeout) {
		t.Fatalf("err=%v", err)
	}
}

func TestOversizedBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(strings.Repeat("x", 100)))
	}))
	defer srv.Close()
	c, _ := newClient(Options{MaxBodyBytes: 50})
	if _, err := c.Get(context.Background(), srv.URL, nil); !errors.Is(err, ErrUnexpected) {
		t.Fatalf("err=%v", err)
	}
	c2, _ := newClient(Options{MaxBodyBytes: 100})
	if _, err := c2.Get(context.Background(), srv.URL, nil); err != nil {
		t.Fatalf("exact-size body should pass: %v", err)
	}
}

func TestInvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("<html>")) }))
	defer srv.Close()
	c, _ := newClient(Options{})
	var v map[string]any
	if err := c.GetJSON(context.Background(), srv.URL, nil, &v); !errors.Is(err, ErrUnexpected) {
		t.Fatalf("err=%v", err)
	}
}

func TestPostJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b := make([]byte, 16)
		n, _ := r.Body.Read(b)
		if r.Method != "POST" || r.Header.Get("Content-Type") != "application/json" || string(b[:n]) != `{"a":1}` ||
			!strings.HasPrefix(r.Header.Get("User-Agent"), "lunatic/") || r.Header.Get("X-H") != "v" {
			w.WriteHeader(400)
			return
		}
		w.Write([]byte(`{"a":2}`))
	}))
	defer srv.Close()
	c, _ := newClient(Options{})
	var v struct{ A int }
	if err := c.PostJSON(context.Background(), srv.URL, map[string]string{"X-H": "v"}, []byte(`{"a":1}`), &v); err != nil || v.A != 2 {
		t.Fatalf("err=%v v=%v", err, v)
	}
}

func TestBlockedRequestToTarget(t *testing.T) {
	dl := &dialLog{}
	c, _ := newClient(Options{TargetDomain: "example.com", DialContext: dl.dial})
	for _, u := range []string{"http://example.com/", "https://API.Example.com./x", "http://a.b.example.com:8080/"} {
		if _, err := c.Get(context.Background(), u, nil); !errors.Is(err, ErrBlockedHost) {
			t.Errorf("%s: err=%v", u, err)
		}
	}
	if len(dl.addrs) != 0 {
		t.Fatalf("dialed: %v", dl.addrs)
	}
	// notexample.com is a different domain and must not be blocked by the guard.
	if c.blocked("notexample.com") {
		t.Fatal("label boundary violated")
	}
	c2, _ := newClient(Options{TargetDomain: "a.test", TargetDomains: []string{"example.com"}})
	if _, err := c2.Get(context.Background(), "http://x.example.com/", nil); !errors.Is(err, ErrBlockedHost) {
		t.Fatalf("extra targets: %v", err)
	}
}

func TestRedirects(t *testing.T) {
	dl := &dialLog{}
	target := ""
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target, 302)
	}))
	defer srv.Close()
	c, _ := newClient(Options{TargetDomain: "example.com", DialContext: dl.dial})

	target = "http://api.example.com/"
	if _, err := c.Get(context.Background(), srv.URL, nil); !errors.Is(err, ErrBlockedHost) {
		t.Fatalf("target redirect: %v", err)
	}
	target = "http://other.test/"
	if _, err := c.Get(context.Background(), srv.URL, nil); !errors.Is(err, ErrUnexpected) {
		t.Fatalf("other-host redirect: %v", err)
	}
	if dl.has("example.com") || dl.has("other.test") {
		t.Fatalf("dialed forbidden host: %v", dl.addrs)
	}
}

func TestSameHostRedirectFollowedAndLimited(t *testing.T) {
	var hops int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/final" {
			w.Write([]byte("done"))
			return
		}
		atomic.AddInt32(&hops, 1)
		http.Redirect(w, r, "/final", 302)
	}))
	defer srv.Close()
	c, _ := newClient(Options{})
	resp, err := c.Get(context.Background(), srv.URL+"/start", nil)
	if err != nil || string(resp.Body) != "done" {
		t.Fatalf("err=%v", err)
	}
	loop := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/again", 302)
	}))
	defer loop.Close()
	if _, err := c.Get(context.Background(), loop.URL, nil); !errors.Is(err, ErrUnexpected) {
		t.Fatalf("redirect loop: %v", err)
	}
}

func TestSecretsRedacted(t *testing.T) {
	const key = "SUPERSECRET123"
	const hdr = "HEADERSECRET456"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		w.Write([]byte("oops " + key + " " + hdr))
	}))
	defer srv.Close()
	c, _ := newClient(Options{SourceName: "t", Secrets: []string{key, hdr}, MaxRetries: -1})
	_, err := c.Get(context.Background(), srv.URL+"/x?apikey=OTHERVALUE&q=1&key="+key, map[string]string{"Authorization": "Bearer " + hdr})
	if err == nil {
		t.Fatal("want error")
	}
	for _, s := range []string{key, hdr, "OTHERVALUE"} {
		if strings.Contains(err.Error(), s) {
			t.Errorf("error leaks %q: %v", s, err)
		}
	}
	if !strings.Contains(err.Error(), "[REDACTED]") || !strings.Contains(err.Error(), "q=1") {
		t.Errorf("unexpected message: %v", err)
	}
}

func TestVerboseRedacted(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()
	var out strings.Builder
	c, _ := newClient(Options{SourceName: "t", Secrets: []string{"TOPSECRET"}, Verbose: func(f string, a ...any) {
		out.WriteString(strings.ReplaceAll(f, "%s", "") + strings.Join(strSlice(a), "") + "\n")
	}})
	if _, err := c.Get(context.Background(), srv.URL+"/?token=TOPSECRET&x=TOPSECRET", nil); err != nil {
		t.Fatal(err)
	}
	if out.Len() == 0 || strings.Contains(out.String(), "TOPSECRET") {
		t.Fatalf("verbose output: %q", out.String())
	}
}

func strSlice(a []any) []string {
	var s []string
	for _, x := range a {
		if v, ok := x.(string); ok {
			s = append(s, v)
		}
	}
	return s
}

func TestRedactFunc(t *testing.T) {
	got := Redact("https://x/?key=abc&Token=def&other=ghi s3cret+x", []string{"s3cret x"})
	want := "https://x/?key=[REDACTED]&Token=[REDACTED]&other=ghi [REDACTED]"
	if got != want {
		t.Fatalf("got %q", got)
	}
}

func TestRateLimiter(t *testing.T) {
	l := newLimiter(50, 1)
	start := time.Now()
	for i := 0; i < 4; i++ {
		if err := l.wait(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	if el := time.Since(start); el < 40*time.Millisecond {
		t.Fatalf("limiter too fast: %v", el)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := newLimiter(0.001, 1).wait(ctx); err == nil {
		t.Fatal("canceled ctx should fail")
	}
}

func TestForbiddenRateLimitClassification(t *testing.T) {
	cases := []struct {
		name    string
		hdr     map[string]string
		want    error
		attempt int32
	}{
		{"remaining0", map[string]string{"X-RateLimit-Remaining": "0", "X-RateLimit-Reset": "9999999999"}, ErrRateLimited, 1},
		{"retry-after-retries", map[string]string{"Retry-After": "1"}, ErrRateLimited, 3},
		{"retry-after-too-long", map[string]string{"Retry-After": "3600"}, ErrRateLimited, 1},
		{"remaining-nonzero", map[string]string{"X-RateLimit-Remaining": "5"}, ErrAuth, 1},
		{"plain", nil, ErrAuth, 1},
	}
	for _, tc := range cases {
		var n int32
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			atomic.AddInt32(&n, 1)
			for k, v := range tc.hdr {
				w.Header().Set(k, v)
			}
			w.WriteHeader(403)
		}))
		c, _ := newClient(Options{MaxRetries: 2})
		_, err := c.Get(context.Background(), srv.URL, nil)
		srv.Close()
		if !errors.Is(err, tc.want) || n != tc.attempt {
			t.Errorf("%s: err=%v attempts=%d, want %v x%d", tc.name, err, n, tc.want, tc.attempt)
		}
	}
}

func TestRedactQueryParamKeys(t *testing.T) {
	in := "GET https://x.test/a?k=abc&apikey=def&key=ghi&password=jkl&q=keep"
	out := Redact(in, nil)
	for _, leak := range []string{"abc", "def", "ghi", "jkl"} {
		if strings.Contains(out, leak) {
			t.Fatalf("leaked %q in %q", leak, out)
		}
	}
	if !strings.Contains(out, "q=keep") {
		t.Fatalf("over-redacted: %q", out)
	}
}
