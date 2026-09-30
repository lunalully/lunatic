// Provider: BeVigil OSINT API (CloudSEK).
// Docs: https://bevigil.com/osint-api (docs portal not readable from the build
// environment; endpoint per subfinder's adapter).
// Endpoint: GET /api/{domain}/subdomains/ . Auth: required, header X-Access-Token.
// Free key available; credit limits not confirmed.
// Checked: 2026-09-29. Verification: fixture only; live test not run for this release
// (run scripts/live-smoke.sh to check).
package sources

import (
	"context"
	"fmt"
	"net/url"
	"strings"
)

var bevigilBaseURL = "https://osint.bevigil.com"

type bevigil struct{}

func init() { Register(bevigil{}) }

func (bevigil) Info() Info {
	return Info{
		Name: "bevigil", URL: "https://bevigil.com/osint-api",
		Auth: AuthRequired, CredFields: []string{"api_key"},
		Default: true, RPS: 1, Burst: 1, // runner skips it when no key is configured
	}
}

func (bevigil) Enumerate(ctx context.Context, domain string, s *Session, emit func(string)) error {
	key, err := PickKey(s.Creds, "api_key")
	if err != nil {
		return err
	}
	var r struct {
		Subdomains []string `json:"subdomains"`
		Detail     string   `json:"detail"`
		Error      string   `json:"error"`
		Message    string   `json:"message"`
	}
	u := bevigilBaseURL + "/api/" + url.PathEscape(domain) + "/subdomains/"
	if err := s.HTTP.GetJSON(ctx, u, map[string]string{"X-Access-Token": key, "Accept": "application/json"}, &r); err != nil {
		return err
	}
	if len(r.Subdomains) == 0 {
		if msg := strings.TrimSpace(r.Error + r.Detail + r.Message); msg != "" {
			return fmt.Errorf("%w: bevigil: %s", ErrUnexpected, msg)
		}
	}
	for _, n := range r.Subdomains {
		emit(n)
	}
	return nil
}
