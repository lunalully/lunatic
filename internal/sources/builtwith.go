// Provider: BuiltWith Domain API.
// Docs: https://api.builtwith.com/domain-api
// Endpoint: GET /v26/api.json?KEY=..&LOOKUP=<domain>&HIDETEXT=yes&NOMETA=yes&NOPII=yes
// (TRUST is never sent; it costs extra credits). Auth: required, KEY query
// parameter (the docs offer no header form; httpx redacts configured secrets).
// Needs a paid Domain API plan; the free API does not return subdomains.
// Result: Results[].Result.Paths[] with SubDomain + Domain -> FQDN.
// Checked: 2026-09-29. Verification: fixture only; live test not possible from
// build environment (egress blocked).
package sources

import (
	"context"
	"fmt"
	"net/url"
	"strings"
)

var builtwithBaseURL = "https://api.builtwith.com"

type builtwith struct{}

func init() { Register(builtwith{}) }

func (builtwith) Info() Info {
	return Info{
		Name: "builtwith", URL: "https://api.builtwith.com",
		Auth: AuthRequired, CredFields: []string{"api_key"},
		Default: true, RPS: 1, Burst: 1, // runner skips it when no key is configured
	}
}

func (builtwith) Enumerate(ctx context.Context, domain string, s *Session, emit func(string)) error {
	key, err := PickKey(s.Creds, "api_key")
	if err != nil {
		return err
	}
	var r struct {
		Errors []struct {
			Message string `json:"Message"`
		} `json:"Errors"`
		Results []struct {
			Result struct {
				Paths []struct {
					Domain    string `json:"Domain"`
					SubDomain string `json:"SubDomain"`
				} `json:"Paths"`
			} `json:"Result"`
		} `json:"Results"`
	}
	q := url.Values{}
	q.Set("KEY", key)
	q.Set("LOOKUP", domain)
	q.Set("HIDETEXT", "yes")
	q.Set("NOMETA", "yes")
	q.Set("NOPII", "yes")
	if err := s.HTTP.GetJSON(ctx, builtwithBaseURL+"/v26/api.json?"+q.Encode(), map[string]string{"Accept": "application/json"}, &r); err != nil {
		return err
	}
	if len(r.Errors) > 0 {
		msg := strings.TrimSpace(r.Errors[0].Message)
		l := strings.ToLower(msg)
		switch {
		case strings.Contains(l, "key") || strings.Contains(l, "unauthor"):
			return fmt.Errorf("%w: builtwith: %s", ErrAuth, msg)
		case strings.Contains(l, "limit") || strings.Contains(l, "credit") || strings.Contains(l, "quota") || strings.Contains(l, "too many"):
			return fmt.Errorf("%w: builtwith: %s", ErrRateLimited, msg)
		}
		return fmt.Errorf("%w: builtwith: %s", ErrUnexpected, msg)
	}
	for _, res := range r.Results {
		for _, p := range res.Result.Paths {
			if p.SubDomain == "" || p.Domain == "" {
				continue
			}
			emit(p.SubDomain + "." + p.Domain)
		}
	}
	return nil
}
