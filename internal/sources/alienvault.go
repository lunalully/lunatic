// Provider: AlienVault OTX (LevelBlue Open Threat Exchange).
// Docs: https://otx.alienvault.com/assets/static/external_api.html (not readable
// when this adapter was written; endpoint path taken from subfinder's adapter).
// Endpoint: GET /api/v1/indicators/domain/{domain}/passive_dns (read-only query).
// Auth: optional. When configured the key is sent in the X-OTX-API-KEY header
// (the header named by the OTX docs). Free account; higher hourly quota with key.
// Checked: 2026-09-29. Verification: fixture only; live test not run for this release
// (run scripts/live-smoke.sh to check).
package sources

import (
	"context"
	"fmt"
	"net/url"
	"strings"
)

var alienvaultBaseURL = "https://otx.alienvault.com"

type alienvault struct{}

func init() { Register(alienvault{}) }

func (alienvault) Info() Info {
	return Info{
		Name: "alienvault", URL: "https://otx.alienvault.com",
		Auth: AuthOptional, CredFields: []string{"api_key"},
		Default: true, RPS: 2, Burst: 1,
	}
}

type alienvaultResp struct {
	PassiveDNS []struct {
		Hostname string `json:"hostname"`
	} `json:"passive_dns"`
	Detail string `json:"detail"`
	Error  string `json:"error"`
}

func (alienvault) Enumerate(ctx context.Context, domain string, s *Session, emit func(string)) error {
	headers := map[string]string{"Accept": "application/json"}
	if key, err := PickKey(s.Creds, "api_key"); err == nil {
		headers["X-OTX-API-KEY"] = key
	}
	var r alienvaultResp
	u := alienvaultBaseURL + "/api/v1/indicators/domain/" + url.PathEscape(domain) + "/passive_dns"
	if err := s.HTTP.GetJSON(ctx, u, headers, &r); err != nil {
		return err
	}
	if msg := strings.TrimSpace(r.Error); msg != "" {
		return alienvaultProviderErr(msg)
	}
	for _, e := range r.PassiveDNS {
		if e.Hostname != "" {
			emit(e.Hostname)
		}
	}
	return nil
}

func alienvaultProviderErr(msg string) error {
	l := strings.ToLower(msg)
	switch {
	case strings.Contains(l, "api key") || strings.Contains(l, "unauthor") || strings.Contains(l, "forbidden"):
		return fmt.Errorf("%w: alienvault: %s", ErrAuth, msg)
	case strings.Contains(l, "throttl") || strings.Contains(l, "rate limit") || strings.Contains(l, "too many"):
		return fmt.Errorf("%w: alienvault: %s", ErrRateLimited, msg)
	}
	return fmt.Errorf("%w: alienvault: %s", ErrUnexpected, msg)
}
