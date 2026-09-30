// Provider: Digitalyama Subdomain Finder API.
// Docs: https://digitalyama.com/api/subdomain-finder (OpenAPI page is JS-rendered
// and not readable here; path per subfinder's adapter). Endpoint:
// GET /subdomain_finder?domain=<domain> . Auth: required, header x-api-key.
// Credit-metered (1 credit per call), vendor limit 1 call/second.
// Checked: 2026-09-29. Verification: fixture only; live test not run for this release
// (run scripts/live-smoke.sh to check).
package sources

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

var digitalyamaBaseURL = "https://api.digitalyama.com"

type digitalyama struct{}

func init() { Register(digitalyama{}) }

func (digitalyama) Info() Info {
	return Info{
		Name: "digitalyama", URL: "https://digitalyama.com",
		Auth: AuthRequired, CredFields: []string{"api_key"},
		Default: true, RPS: 0.5, Burst: 1, // runner skips it when no key is configured
	}
}

func (digitalyama) Enumerate(ctx context.Context, domain string, s *Session, emit func(string)) error {
	key, err := PickKey(s.Creds, "api_key")
	if err != nil {
		return err
	}
	var r struct {
		Subdomains []string        `json:"subdomains"`
		Detail     json.RawMessage `json:"detail"`
		Error      string          `json:"error"`
	}
	u := digitalyamaBaseURL + "/subdomain_finder?domain=" + url.QueryEscape(domain)
	if err := s.HTTP.GetJSON(ctx, u, map[string]string{"x-api-key": key, "Accept": "application/json"}, &r); err != nil {
		return err
	}
	if len(r.Subdomains) == 0 {
		msg := strings.TrimSpace(r.Error)
		if msg == "" && len(r.Detail) > 0 && string(r.Detail) != "null" {
			msg = string(r.Detail)
		}
		if msg != "" {
			return fmt.Errorf("%w: digitalyama: %s", ErrUnexpected, msg)
		}
	}
	for _, n := range r.Subdomains {
		emit(n)
	}
	return nil
}
