// Package sources: thc adapter.
//
// Provider:     THC IP/Domain intelligence (https://ip.thc.org)
// Docs:         https://ip.thc.org/docs/API/subdomain-lookup
//
//	(docs page could not be read in full; request/response
//	field names follow subfinder's client and are parsed
//	tolerantly).
//
// Endpoint:     POST {base}/api/v1/lookup/subdomains
//
//	body {"domain":D,"page_state":S,"limit":1000}
//
// Auth:         none. Limits/ToS not confirmed.
// Pagination:   next_page_state cursor; bounded by Session.MaxPages.
// Checked:      2026-09-29
// Verification: fixture only; live test not run for this release; run scripts/live-smoke.sh to check.
package sources

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

var thcBaseURL = "https://ip.thc.org"

type thc struct{}

func init() { Register(thc{}) }

func (thc) Info() Info {
	return Info{Name: "thc", URL: "https://ip.thc.org", Auth: AuthNone, Default: true, RPS: 1, Burst: 1}
}

func (thc) Enumerate(ctx context.Context, domain string, s *Session, emit func(string)) error {
	state := ""
	for page := 0; page < s.MaxPages; page++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		body, _ := json.Marshal(map[string]any{"domain": domain, "page_state": state, "limit": 1000})
		var r struct {
			Domains       []json.RawMessage `json:"domains"`
			NextPageState string            `json:"next_page_state"`
			Error         string            `json:"error"`
			Message       string            `json:"message"`
		}
		if err := s.HTTP.PostJSON(ctx, thcBaseURL+"/api/v1/lookup/subdomains", nil, body, &r); err != nil {
			return err
		}
		if r.Error != "" {
			l := strings.ToLower(r.Error)
			if strings.Contains(l, "rate") || strings.Contains(l, "too many") {
				return fmt.Errorf("%w: thc: %s", ErrRateLimited, r.Error)
			}
			return fmt.Errorf("%w: thc: %s", ErrUnexpected, r.Error)
		}
		for _, d := range r.Domains {
			var o struct {
				Domain string `json:"domain"`
			}
			var str string
			if json.Unmarshal(d, &o) == nil && o.Domain != "" {
				emit(o.Domain)
			} else if json.Unmarshal(d, &str) == nil && str != "" {
				emit(str)
			}
		}
		if r.NextPageState == "" || r.NextPageState == state || len(r.Domains) == 0 {
			break
		}
		state = r.NextPageState
	}
	return nil
}
