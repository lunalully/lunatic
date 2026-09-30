// Provider: C99.nl API.
// Docs: https://api.c99.nl/ (endpoint parameters not readable from the build
// environment; taken from subfinder's adapter).
// Endpoint: GET /subdomainfinder?key=..&domain=..&json  (stored-data finder only;
// the vendor's realtime/scan option is NEVER sent). Auth: required, `key` query
// parameter (httpx redacts configured secrets). Paid key.
// Checked: 2026-09-29. Verification: fixture only; live test not run for this release
// (run scripts/live-smoke.sh to check).
package sources

import (
	"context"
	"fmt"
	"net/url"
	"strings"
)

var c99BaseURL = "https://api.c99.nl"

type c99 struct{}

func init() { Register(c99{}) }

func (c99) Info() Info {
	return Info{
		Name: "c99", URL: "https://api.c99.nl",
		Auth: AuthRequired, CredFields: []string{"api_key"},
		Default: true, RPS: 1, Burst: 1, // runner skips it when no key is configured
	}
}

func (c99) Enumerate(ctx context.Context, domain string, s *Session, emit func(string)) error {
	key, err := PickKey(s.Creds, "api_key")
	if err != nil {
		return err
	}
	var r struct {
		Success    *bool  `json:"success"`
		Error      string `json:"error"`
		Subdomains []struct {
			Subdomain string `json:"subdomain"`
		} `json:"subdomains"`
	}
	u := c99BaseURL + "/subdomainfinder?key=" + url.QueryEscape(key) + "&domain=" + url.QueryEscape(domain) + "&json"
	if err := s.HTTP.GetJSON(ctx, u, map[string]string{"Accept": "application/json"}, &r); err != nil {
		return err
	}
	if (r.Success != nil && !*r.Success) || r.Error != "" {
		msg := strings.TrimSpace(r.Error)
		if msg == "" {
			msg = "request unsuccessful"
		}
		l := strings.ToLower(msg)
		switch {
		case strings.Contains(l, "key") || strings.Contains(l, "unauthor"):
			return fmt.Errorf("%w: c99: %s", ErrAuth, msg)
		case strings.Contains(l, "limit") || strings.Contains(l, "too many"):
			return fmt.Errorf("%w: c99: %s", ErrRateLimited, msg)
		}
		return fmt.Errorf("%w: c99: %s", ErrUnexpected, msg)
	}
	for _, e := range r.Subdomains {
		if e.Subdomain == "" || strings.HasPrefix(e.Subdomain, ".") {
			continue
		}
		emit(e.Subdomain)
	}
	return nil
}
