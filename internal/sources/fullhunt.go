// Provider: FullHunt (https://fullhunt.io).
// Docs: https://docs.fullhunt.io/api/domain-apis (limits: /rate-limiting, plans: /plans-and-credits).
// Endpoint: GET https://fullhunt.io/api/v1/domain/<domain>/subdomains (a lookup of the stored attack-surface dataset;
// FullHunt's on-demand scan endpoints are never called).
// Auth: required; header X-API-KEY. Limit 200 requests/hour on this endpoint; 429 -> ErrRateLimited, 402 (credits) -> ErrRateLimited.
// Response: {"hosts":[...],"message":"","status":200}; a non-200 "status" in the body is mapped to a typed error.
// Results can be truncated by plan (not signalled reliably).
// Date checked: 2026-09-29.
// Verification: fixture only; live test not run for this release; run scripts/live-smoke.sh to check.
package sources

import (
	"context"
	"fmt"
	"net/url"
)

var fullhuntBaseURL = "https://fullhunt.io"

type fullhunt struct{}

func init() { Register(fullhunt{}) }

func (fullhunt) Info() Info {
	return Info{Name: "fullhunt", URL: "https://fullhunt.io", Auth: AuthRequired,
		CredFields: []string{"api_key"}, Default: true, RPS: 0.05, Burst: 1}
}

func (fullhunt) Enumerate(ctx context.Context, domain string, s *Session, emit func(string)) error {
	key, err := PickKey(s.Creds, "api_key")
	if err != nil {
		return err
	}
	var out struct {
		Hosts   []string `json:"hosts"`
		Message string   `json:"message"`
		Status  int      `json:"status"`
	}
	err = s.HTTP.GetJSON(ctx, fullhuntBaseURL+"/api/v1/domain/"+url.PathEscape(domain)+"/subdomains", map[string]string{"X-API-KEY": key}, &out)
	if err != nil {
		if g2status(err) == 402 {
			return fmt.Errorf("%w: fullhunt credits exhausted", ErrRateLimited)
		}
		return err
	}
	switch out.Status {
	case 0, 200:
	case 401, 403:
		return fmt.Errorf("%w: fullhunt: %.80s", ErrAuth, out.Message)
	case 402, 429:
		return fmt.Errorf("%w: fullhunt: %.80s", ErrRateLimited, out.Message)
	default:
		return fmt.Errorf("%w: fullhunt status %d: %.80s", ErrUnexpected, out.Status, out.Message)
	}
	for _, h := range out.Hosts {
		emit(h)
	}
	return nil
}
