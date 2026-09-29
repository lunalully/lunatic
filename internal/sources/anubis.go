// Provider: Anubis-DB (community subdomain database).
// Docs: https://github.com/jonluca/Anubis-DB and https://anubisdb.com
// Endpoint: GET /anubis/subdomains/{domain} (read-only; the POST submit
// endpoint is never used). Auth: none. Free; 60 requests/10 s per IP.
// Checked: 2026-09-29. Verification: fixture only; live test not possible from
// build environment (egress blocked).
// Note: HTTP 403 (documented as "invalid input") is mapped to ErrAuth by httpx.
package sources

import (
	"context"
	"net/url"
)

var anubisBaseURL = "https://anubisdb.com"

type anubis struct{}

func init() { Register(anubis{}) }

func (anubis) Info() Info {
	return Info{Name: "anubis", URL: "https://anubisdb.com", Auth: AuthNone, Default: true, RPS: 1, Burst: 1}
}

func (anubis) Enumerate(ctx context.Context, domain string, s *Session, emit func(string)) error {
	var names []string // JSON null decodes to an empty list (no results)
	u := anubisBaseURL + "/anubis/subdomains/" + url.PathEscape(domain)
	if err := s.HTTP.GetJSON(ctx, u, map[string]string{"Accept": "application/json"}, &names); err != nil {
		return err
	}
	for _, n := range names {
		if n != "" {
			emit(n)
		}
	}
	return nil
}
