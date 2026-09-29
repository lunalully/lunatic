// Package httpx is the only HTTP client adapters may use. It enforces the
// passive guard (never contact the target domain), rate limiting, bounded
// retries, body size limits, typed errors and secret redaction.
package httpx

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/lunalully/lunatic/internal/errs"
	"github.com/lunalully/lunatic/internal/version"
)

// Typed errors (identical values to those in package errs / sources).
var (
	ErrAuth        = errs.ErrAuth
	ErrRateLimited = errs.ErrRateLimited
	ErrTimeout     = errs.ErrTimeout
	ErrUnexpected  = errs.ErrUnexpected
	ErrUnavailable = errs.ErrUnavailable
	ErrBlockedHost = errs.ErrBlockedHost
)

const (
	defaultMaxRetries = 2
	defaultTimeout    = 30 * time.Second
	defaultMaxBody    = 25 << 20
	maxRedirects      = 3
	maxRetryAfter     = 60 * time.Second
)

// Options configures a Client. Zero values pick the documented defaults.
type Options struct {
	SourceName        string
	RequestsPerSecond float64       // 0 = unlimited; shared by all requests of this client
	Burst             int           // token bucket size (default 1)
	MaxRetries        int           // 0 = default (2); negative = no retries
	Timeout           time.Duration // per request, default 30s
	MaxBodyBytes      int64         // default 25 MiB
	UserAgent         string        // default "lunatic/<version>"
	TargetDomain      string        // requests/redirects to it or its subdomains are refused
	TargetDomains     []string      // additional guarded domains (multi-domain runs)
	Secrets           []string      // redacted from every error and log line
	Verbose           func(format string, args ...any)

	// Testing hooks.
	Transport   http.RoundTripper                                                 // replaces the default transport
	DialContext func(ctx context.Context, network, addr string) (net.Conn, error) // used by the default transport
	BackoffBase time.Duration                                                     // default 1s
}

// Response is a fully read HTTP response.
type Response struct {
	StatusCode int
	Header     http.Header
	Body       []byte
}

// Error is the error type returned by Client. Use errors.Is with the Err*
// values to classify it, and errors.As to reach the HTTP status (0 when the
// failure was not an HTTP status, e.g. a 404 has StatusCode 404 and Kind
// ErrUnexpected). Msg is already redacted.
type Error struct {
	Kind       error
	StatusCode int
	Msg        string
}

func (e *Error) Error() string { return e.Kind.Error() + ": " + e.Msg }
func (e *Error) Unwrap() error { return e.Kind }

// Client performs guarded, rate-limited requests for one source.
type Client struct {
	opts    Options
	hc      *http.Client
	lim     *limiter
	targets []string
	secrets []string
	sleep   func(ctx context.Context, d time.Duration) error
}

// New builds a Client.
func New(o Options) *Client {
	if o.MaxRetries == 0 {
		o.MaxRetries = defaultMaxRetries
	} else if o.MaxRetries < 0 {
		o.MaxRetries = 0
	}
	if o.Timeout <= 0 {
		o.Timeout = defaultTimeout
	}
	if o.MaxBodyBytes <= 0 {
		o.MaxBodyBytes = defaultMaxBody
	}
	if o.UserAgent == "" {
		o.UserAgent = "lunatic/" + version.Version
	}
	if o.BackoffBase <= 0 {
		o.BackoffBase = time.Second
	}
	c := &Client{opts: o, lim: newLimiter(o.RequestsPerSecond, o.Burst), secrets: prepSecrets(o.Secrets), sleep: sleepCtx}
	for _, t := range append([]string{o.TargetDomain}, o.TargetDomains...) {
		if t = normHost(t); t != "" {
			c.targets = append(c.targets, t)
		}
	}
	rt := o.Transport
	if rt == nil {
		tr := http.DefaultTransport.(*http.Transport).Clone() // honors proxy env
		if o.DialContext != nil {
			tr.DialContext = o.DialContext
		}
		rt = tr
	}
	c.hc = &http.Client{Transport: rt, CheckRedirect: c.checkRedirect}
	return c
}

func normHost(h string) string {
	return strings.TrimSuffix(strings.ToLower(strings.TrimSpace(h)), ".")
}

func (c *Client) blocked(host string) bool {
	host = normHost(host)
	for _, t := range c.targets {
		if host == t || strings.HasSuffix(host, "."+t) {
			return true
		}
	}
	return false
}

func (c *Client) logf(format string, args ...any) {
	if c.opts.Verbose != nil {
		c.opts.Verbose("%s", c.Redact(fmt.Sprintf("[%s] ", c.opts.SourceName)+fmt.Sprintf(format, args...)))
	}
}

func (c *Client) errorf(kind error, status int, format string, args ...any) *Error {
	return &Error{Kind: kind, StatusCode: status, Msg: c.Redact(fmt.Sprintf(format, args...))}
}

func (c *Client) checkRedirect(req *http.Request, via []*http.Request) error {
	if c.blocked(req.URL.Hostname()) {
		return c.errorf(ErrBlockedHost, 0, "redirect to %s", req.URL.Hostname())
	}
	if len(via) > maxRedirects {
		return c.errorf(ErrUnexpected, 0, "stopped after %d redirects", maxRedirects)
	}
	prev := via[len(via)-1].URL
	sameHost := strings.EqualFold(prev.Host, req.URL.Host) ||
		(prev.Scheme == "http" && req.URL.Scheme == "https" && strings.EqualFold(prev.Hostname(), req.URL.Hostname()))
	if !sameHost {
		return c.errorf(ErrUnexpected, 0, "redirect to a different host (%s) refused", req.URL.Host)
	}
	if req.URL.Scheme != "http" && req.URL.Scheme != "https" {
		return c.errorf(ErrUnexpected, 0, "redirect to unsupported scheme %q", req.URL.Scheme)
	}
	return nil
}

func sleepCtx(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

func (c *Client) backoff(attempt int) time.Duration {
	d := c.opts.BackoffBase << uint(attempt)
	return d + time.Duration(rand.Int63n(int64(d)/4+1))
}

func retryAfter(h http.Header) (time.Duration, bool) {
	v := strings.TrimSpace(h.Get("Retry-After"))
	if v == "" {
		return 0, false
	}
	if n, err := strconv.Atoi(v); err == nil && n >= 0 {
		return time.Duration(n) * time.Second, true
	}
	if t, err := http.ParseTime(v); err == nil {
		if d := time.Until(t); d > 0 {
			return d, true
		}
		return 0, true
	}
	return 0, false
}

// ctxErr converts a done parent context into a typed error.
func (c *Client) ctxErr(err error) error {
	if errors.Is(err, context.DeadlineExceeded) {
		return c.errorf(ErrTimeout, 0, "deadline exceeded")
	}
	return &Error{Kind: context.Canceled, Msg: "canceled"}
}

// Do performs a request with retries. On HTTP status failures both the
// Response (for inspection) and the typed error are returned; for transport
// failures the Response is nil. Callers must not log Response bodies without
// redacting them.
func (c *Client) Do(ctx context.Context, method, rawURL string, headers map[string]string, body []byte) (*Response, error) {
	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return nil, c.errorf(ErrUnexpected, 0, "invalid URL %s", rawURL)
	}
	if c.blocked(u.Hostname()) {
		return nil, c.errorf(ErrBlockedHost, 0, "refusing to contact %s", u.Hostname())
	}
	for attempt := 0; ; attempt++ {
		if err := c.lim.wait(ctx); err != nil {
			return nil, c.ctxErr(err)
		}
		resp, retry, wait, err := c.once(ctx, method, rawURL, headers, body)
		if err == nil || !retry {
			return resp, err
		}
		if attempt >= c.opts.MaxRetries {
			return resp, err
		}
		if wait <= 0 {
			wait = c.backoff(attempt)
		}
		c.logf("retry %d/%d in %s: %v", attempt+1, c.opts.MaxRetries, wait.Round(time.Millisecond), err)
		if serr := c.sleep(ctx, wait); serr != nil {
			return nil, c.ctxErr(serr)
		}
	}
}

// once performs a single attempt; retry reports whether the failure is
// transient and wait is a server-requested delay (0 = use backoff).
func (c *Client) once(ctx context.Context, method, rawURL string, headers map[string]string, body []byte) (resp *Response, retry bool, wait time.Duration, err error) {
	rctx, cancel := context.WithTimeout(ctx, c.opts.Timeout)
	defer cancel()
	var rd io.Reader
	if body != nil {
		rd = bytes.NewReader(body)
	}
	req, rerr := http.NewRequestWithContext(rctx, method, rawURL, rd)
	if rerr != nil {
		return nil, false, 0, c.errorf(ErrUnexpected, 0, "building request: %v", rerr)
	}
	req.Header.Set("User-Agent", c.opts.UserAgent)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	c.logf("%s %s", method, rawURL)
	hr, derr := c.hc.Do(req)
	if derr != nil {
		e, r := c.transportErr(ctx, rctx, derr)
		return nil, r, 0, e
	}
	defer hr.Body.Close()
	data, rerr := io.ReadAll(io.LimitReader(hr.Body, c.opts.MaxBodyBytes+1))
	if rerr != nil {
		e, r := c.transportErr(ctx, rctx, rerr)
		return nil, r, 0, e
	}
	if int64(len(data)) > c.opts.MaxBodyBytes {
		return nil, false, 0, c.errorf(ErrUnexpected, hr.StatusCode, "response body exceeds %d bytes", c.opts.MaxBodyBytes)
	}
	resp = &Response{StatusCode: hr.StatusCode, Header: hr.Header, Body: data}
	c.logf("-> %d (%d bytes)", hr.StatusCode, len(data))
	code := hr.StatusCode
	snippet := func() string {
		s := string(data)
		if len(s) > 200 {
			s = s[:200]
		}
		return s
	}
	switch {
	case code >= 200 && code < 300:
		return resp, false, 0, nil
	case code == 403 && rateLimitedForbidden(hr.Header):
		// GitHub-style 403 rate limits: not an auth problem. Retry only when the
		// server asks for a short wait (Retry-After); a primary limit that resets
		// far in the future is reported immediately.
		w, ok := retryAfter(hr.Header)
		if ok && w <= maxRetryAfter {
			return resp, true, w, c.errorf(ErrRateLimited, code, "HTTP 403 (rate limited) from %s", rawURL)
		}
		return resp, false, 0, c.errorf(ErrRateLimited, code, "HTTP 403 (rate limited) from %s", rawURL)
	case code == 401 || code == 403:
		return resp, false, 0, c.errorf(ErrAuth, code, "HTTP %d from %s: %s", code, rawURL, snippet())
	case code == 429:
		w, _ := retryAfter(hr.Header)
		if w > maxRetryAfter {
			w = maxRetryAfter
		}
		return resp, true, w, c.errorf(ErrRateLimited, code, "HTTP 429 from %s", rawURL)
	case code >= 500:
		return resp, true, 0, c.errorf(ErrUnavailable, code, "HTTP %d from %s: %s", code, rawURL, snippet())
	default:
		return resp, false, 0, c.errorf(ErrUnexpected, code, "HTTP %d from %s: %s", code, rawURL, snippet())
	}
}

// rateLimitedForbidden reports whether a 403 carries rate-limit signals.
func rateLimitedForbidden(h http.Header) bool {
	if strings.TrimSpace(h.Get("X-RateLimit-Remaining")) == "0" {
		return true
	}
	_, ok := retryAfter(h)
	return ok
}

// transportErr classifies a failure of the round trip or body read and says
// whether it is transient (worth retrying with backoff).
func (c *Client) transportErr(parent, rctx context.Context, err error) (out error, retry bool) {
	var he *Error
	if errors.As(err, &he) { // guard/redirect refusal raised in CheckRedirect
		return he, false
	}
	if perr := parent.Err(); perr != nil {
		return c.ctxErr(perr), false
	}
	if rctx.Err() != nil || errors.Is(err, context.DeadlineExceeded) {
		return c.errorf(ErrTimeout, 0, "request timed out after %s: %v", c.opts.Timeout, err), false
	}
	var cv *tls.CertificateVerificationError
	if errors.As(err, &cv) {
		return c.errorf(ErrUnexpected, 0, "TLS verification failed: %v", err), false
	}
	var ne net.Error
	if errors.As(err, &ne) && ne.Timeout() {
		return c.errorf(ErrTimeout, 0, "%v", err), false
	}
	return c.errorf(ErrUnavailable, 0, "%v", err), true
}

// Get performs a GET request.
func (c *Client) Get(ctx context.Context, url string, headers map[string]string) (*Response, error) {
	return c.Do(ctx, http.MethodGet, url, headers, nil)
}

// Post performs a POST request with the given body.
func (c *Client) Post(ctx context.Context, url string, headers map[string]string, body []byte) (*Response, error) {
	return c.Do(ctx, http.MethodPost, url, headers, body)
}

func (c *Client) decode(resp *Response, err error, v any) error {
	if err != nil {
		return err
	}
	if err := json.Unmarshal(resp.Body, v); err != nil {
		return c.errorf(ErrUnexpected, resp.StatusCode, "invalid JSON: %v", err)
	}
	return nil
}

// GetJSON performs a GET and decodes the JSON body into v.
func (c *Client) GetJSON(ctx context.Context, url string, headers map[string]string, v any) error {
	resp, err := c.Get(ctx, url, headers)
	return c.decode(resp, err, v)
}

// PostJSON POSTs body (Content-Type application/json unless overridden) and
// decodes the JSON response into v.
func (c *Client) PostJSON(ctx context.Context, url string, headers map[string]string, body []byte, v any) error {
	h := map[string]string{"Content-Type": "application/json"}
	for k, val := range headers {
		h[k] = val
	}
	resp, err := c.Post(ctx, url, h, body)
	return c.decode(resp, err, v)
}
