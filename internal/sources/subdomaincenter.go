// Package sources: subdomaincenter adapter (extra).
//
// Provider:     Subdomain Center (https://www.subdomain.center)
// Docs:         site page; endpoint confirmed via BBOT's client.
// Endpoint:     GET {base}/?domain={domain}  -> JSON array of FQDN strings
// Auth:         none for the free tier (site: up to 500 results, random
//
//	order; a paid key mechanism is not confirmed, so unused).
//
// Pagination:   none.
// Checked:      2026-09-29
// Verification: fixture only; live test not run for this release; run scripts/live-smoke.sh to check.
package sources

import (
	"context"
	"net/url"
)

var subdomaincenterBaseURL = "https://api.subdomain.center"

type subdomaincenter struct{}

func init() { Register(subdomaincenter{}) }

func (subdomaincenter) Info() Info {
	return Info{Name: "subdomaincenter", URL: "https://www.subdomain.center", Auth: AuthNone, Default: true, RPS: 0.5, Burst: 1}
}

func (subdomaincenter) Enumerate(ctx context.Context, domain string, s *Session, emit func(string)) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	var list []string
	if err := s.HTTP.GetJSON(ctx, subdomaincenterBaseURL+"/?domain="+url.QueryEscape(domain), nil, &list); err != nil {
		return err
	}
	for _, h := range list {
		emit(h)
	}
	return nil
}
