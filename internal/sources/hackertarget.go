// Provider: HackerTarget (https://hackertarget.com).
// Docs: https://hackertarget.com/find-dns-host-records/ and https://hackertarget.com/ip-tools/
// Endpoint: GET https://api.hackertarget.com/hostsearch/?q=<domain> (stored DNS dataset, not live lookups).
// Auth: optional; without a key the free per-IP quota applies. A configured key is sent in the X-API-Key header.
// Other ip-tools endpoints were reviewed: dnslookup (live resolution), mtr/ping/nmap/banner/httpheaders/pagelinks (live)
// and reverseiplookup/reversedns (return unrelated hosts of an IP) are NOT used; hostsearch is the only domain-based passive one.
// Response: text/plain CSV "host,ip" per line. Failures (quota, bad parameter) are returned as plain text with HTTP 200,
// so the body is inspected before being parsed; such text is never treated as data. Only the host column is emitted; the ip column is reported via Session.IP (no extra request).
// Date checked: 2026-09-29.
// Verification: fixture only; live test not run for this release; run scripts/live-smoke.sh to check
// (one fetch while writing the adapter returned the quota message "API count exceeded").
package sources

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/lunalully/lunatic/internal/httpx"
)

var hackertargetBaseURL = "https://api.hackertarget.com"

type hackertarget struct{}

func init() { Register(hackertarget{}) }

func (hackertarget) Info() Info {
	return Info{
		Name: "hackertarget", URL: "https://hackertarget.com",
		Auth: AuthOptional, CredFields: []string{"api_key"},
		Default: true, RPS: 0.2, Burst: 1,
	}
}

// g2status returns the HTTP status carried by an httpx error (0 if none).
func g2status(err error) int {
	var he *httpx.Error
	if errors.As(err, &he) {
		return he.StatusCode
	}
	return 0
}

func (hackertarget) Enumerate(ctx context.Context, domain string, s *Session, emit func(string)) error {
	headers := map[string]string{}
	if k := s.Creds["api_key"]; k != "" {
		headers["X-API-Key"] = k
	}
	resp, err := s.HTTP.Get(ctx, hackertargetBaseURL+"/hostsearch/?q="+url.QueryEscape(domain), headers)
	if err != nil {
		return err
	}
	body := strings.TrimSpace(string(resp.Body))
	if body == "" {
		return nil
	}
	low := strings.ToLower(body)
	switch {
	case strings.HasPrefix(low, "api count exceeded"), strings.Contains(low, "increase quota"), strings.Contains(low, "quota exceeded"):
		return fmt.Errorf("%w: hackertarget quota message", ErrRateLimited)
	case strings.HasPrefix(low, "no records found"):
		return nil
	case strings.HasPrefix(low, "error"):
		if strings.Contains(low, "api key") {
			return fmt.Errorf("%w: hackertarget rejected the API key", ErrAuth)
		}
		return fmt.Errorf("%w: hackertarget error message: %.80s", ErrUnexpected, body)
	}
	var hosts, ips []string
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		host, ip, ok := strings.Cut(line, ",")
		if !ok || strings.ContainsAny(host, " <>\t") {
			return fmt.Errorf("%w: hackertarget returned non-CSV text", ErrUnexpected)
		}
		hosts = append(hosts, host)
		ips = append(ips, ip)
	}
	for _, h := range hosts {
		emit(h)
	}
	for _, ip := range ips { // already in the response; used only by phase-2 sources
		s.IP(ip)
	}
	return nil
}
