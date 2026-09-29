// Provider: SSLMate Cert Spotter (CT search API v1).
// Docs: https://sslmate.com/help/reference/ct_search_api_v1
// Endpoint: GET /v1/issuances?domain=D&include_subdomains=true&expand=dns_names[&after=ID]
// Auth: optional; unauthenticated use has a small hourly quota, with a key it is
// sent as "Authorization: Bearer <key>". Pagination: `after` = last issuance id
// until an empty array is returned (bounded by Session.MaxPages).
// Checked: 2026-09-29. Verification: fixture only; live test not possible from
// build environment (egress blocked).
package sources

import (
	"context"
	"fmt"
	"net/url"
)

var certspotterBaseURL = "https://api.certspotter.com"

type certspotter struct{}

func init() { Register(certspotter{}) }

func (certspotter) Info() Info {
	return Info{
		Name: "certspotter", URL: "https://sslmate.com/certspotter",
		Auth: AuthOptional, CredFields: []string{"api_key"},
		Default: true, RPS: 1, Burst: 1,
	}
}

type certspotterIssuance struct {
	ID       string   `json:"id"`
	DNSNames []string `json:"dns_names"`
}

func (certspotter) Enumerate(ctx context.Context, domain string, s *Session, emit func(string)) error {
	headers := map[string]string{"Accept": "application/json"}
	if key, err := PickKey(s.Creds, "api_key"); err == nil {
		headers["Authorization"] = "Bearer " + key
	}
	maxPages := s.MaxPages
	if maxPages <= 0 {
		maxPages = 10
	}
	after := ""
	for page := 0; page < maxPages; page++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		q := url.Values{}
		q.Set("domain", domain)
		q.Set("include_subdomains", "true")
		q.Set("expand", "dns_names")
		if after != "" {
			q.Set("after", after)
		}
		var rows []certspotterIssuance
		if err := s.HTTP.GetJSON(ctx, certspotterBaseURL+"/v1/issuances?"+q.Encode(), headers, &rows); err != nil {
			return err
		}
		if len(rows) == 0 {
			return nil
		}
		last := rows[len(rows)-1].ID
		for _, r := range rows {
			for _, n := range r.DNSNames {
				emit(n)
			}
		}
		if last == "" || last == after {
			return fmt.Errorf("%w: certspotter: cannot paginate (missing or repeated issuance id)", ErrUnexpected)
		}
		after = last
	}
	return nil
}
