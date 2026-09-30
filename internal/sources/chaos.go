// Provider: ProjectDiscovery Chaos DNS dataset.
// Docs: https://chaos.projectdiscovery.io/docs/fetch-subdomains
// Endpoint: GET /dns/{domain}/subdomains on dns.projectdiscovery.io.
// Auth: required; the raw key goes in the Authorization header (no Bearer prefix,
// per chaos-client). Response holds bare labels; FQDN = label + "." + domain.
// Checked: 2026-09-29. Verification: fixture only; live test not run for this release
// (run scripts/live-smoke.sh to check).
package sources

import (
	"context"
	"fmt"
	"net/url"
	"strings"
)

var chaosBaseURL = "https://dns.projectdiscovery.io"

type chaos struct{}

func init() { Register(chaos{}) }

func (chaos) Info() Info {
	return Info{
		Name: "chaos", URL: "https://chaos.projectdiscovery.io",
		Auth: AuthRequired, CredFields: []string{"api_key"},
		Default: true, RPS: 2, Burst: 1, // runner skips it when no key is configured
	}
}

func (chaos) Enumerate(ctx context.Context, domain string, s *Session, emit func(string)) error {
	key, err := PickKey(s.Creds, "api_key")
	if err != nil {
		return err
	}
	var r struct {
		Domain     string   `json:"domain"`
		Subdomains []string `json:"subdomains"`
		Error      string   `json:"error"`
		Message    string   `json:"message"`
	}
	u := chaosBaseURL + "/dns/" + url.PathEscape(domain) + "/subdomains"
	if err := s.HTTP.GetJSON(ctx, u, map[string]string{"Authorization": key, "Accept": "application/json"}, &r); err != nil {
		return err
	}
	if len(r.Subdomains) == 0 && r.Domain == "" {
		if msg := strings.TrimSpace(r.Error + r.Message); msg != "" {
			return fmt.Errorf("%w: chaos: %s", ErrUnexpected, msg)
		}
	}
	for _, label := range r.Subdomains {
		if label = strings.Trim(label, ". "); label == "" || label == "*" {
			continue
		}
		emit(label + "." + domain)
	}
	return nil
}
