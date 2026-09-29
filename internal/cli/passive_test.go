package cli

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/lunalully/lunatic/internal/config"
	"github.com/lunalully/lunatic/internal/sources"
)

// recordingTransport routes ANY host to a local handler and records requests.
type recordingTransport struct {
	mode string // "empty", "rich", "redirect"
	mu   sync.Mutex
	reqs []string // "METHOD host path?query"
}

func (rt *recordingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	rt.mu.Lock()
	rt.reqs = append(rt.reqs, req.Method+" "+req.URL.Host+" "+req.URL.RequestURI())
	rt.mu.Unlock()
	h := http.Header{"Content-Type": []string{"application/json"}}
	code, body := 200, `{}`
	switch rt.mode {
	case "rich":
		body = `{"subdomains":["a.example.com","b.example.com"],"results":[{"name":"c.example.com"}],"data":["d.example.com"],"domains":["e.example.com"],"passive_dns":[{"hostname":"f.example.com"}]}`
	case "redirect":
		code = 302
		if len(req.URL.Host)%2 == 0 { // alternate: target-domain redirect vs other-host redirect
			h.Set("Location", "https://deep.example.com/x")
		} else {
			h.Set("Location", "https://sink.redirect-test.invalid/x")
		}
		body = ""
	}
	return &http.Response{StatusCode: code, Status: http.StatusText(code), Header: h, Body: io.NopCloser(strings.NewReader(body)), Request: req}, nil
}

var scanPathRe = regexp.MustCompile(`/api/v1/scan|/active/|realtime=|/submit|/api/v1/search/.*/scan`)

// TestPassiveRulesEndToEnd runs every registered source through the real CLI
// path against a fake network and checks the passive rules.
func TestPassiveRulesEndToEnd(t *testing.T) {
	old := testTransport
	defer func() { testTransport = old }()
	all := sources.All()
	if len(all) < 54 {
		t.Fatalf("expected >= 54 registered sources, got %d", len(all))
	}
	envMap := map[string]string{}
	for _, s := range all {
		for _, f := range s.Info().AllCredFields() {
			envMap[config.EnvName(s.Info().Name, f)] = "credvalue-" + f
		}
	}
	lookup := func(k string) (string, bool) { v, ok := envMap[k]; return v, ok }

	for _, mode := range []string{"empty", "rich", "redirect"} {
		t.Run(mode, func(t *testing.T) {
			rt := &recordingTransport{mode: mode}
			testTransport = rt
			var out, errb bytes.Buffer
			args := []string{"--all", "-d", "example.com", "--timeout", "4", "--concurrency", "60"}
			run(context.Background(), args, &out, &errb, env{sources: all, lookupEnv: lookup})
			if len(rt.reqs) == 0 {
				t.Fatal("no requests recorded; hook not wired")
			}
			for _, r := range rt.reqs {
				f := strings.Fields(r)
				host, uri := strings.ToLower(f[1]), f[2]
				if host == "example.com" || strings.HasSuffix(host, ".example.com") {
					t.Errorf("request to target domain: %s", r)
				}
				if host == "sink.redirect-test.invalid" {
					t.Errorf("redirect to another host was followed: %s", r)
				}
				if scanPathRe.MatchString(uri) {
					t.Errorf("scan/submit style request: %s", r)
				}
				switch {
				case strings.Contains(host, "onyphe") && strings.Contains(uri, "/v3/"):
					t.Errorf("onyphe v3 on-demand: %s", r)
				case strings.Contains(host, "urlscan") && !strings.HasPrefix(uri, "/api/v1/search/"):
					t.Errorf("urlscan non-search: %s", r)
				case strings.Contains(host, "rsecloud") && strings.Contains(uri, "/active"):
					t.Errorf("rsecloud active: %s", r)
				}
			}
			// No credential value may reach stdout or stderr.
			for _, v := range envMap {
				if strings.Contains(out.String(), v) || strings.Contains(errb.String(), v) {
					t.Fatalf("credential value %q leaked into output", v)
				}
			}
			t.Logf("%s: %d requests", mode, len(rt.reqs))
		})
	}
}
