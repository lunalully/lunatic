// Provider: LeakIX (https://leakix.net).
// Docs: https://docs.leakix.net/docs/api/subdomains/
// Endpoint: GET https://leakix.net/api/subdomains/<domain> (lookup of stored scan results).
// Auth: required; header api-key. Free plan exists (limited requests, delayed results); ~1 request/s; 429 -> ErrRateLimited.
// Response: JSON array [{"subdomain":"...","distinct_ips":N,"last_seen":"..."}]; no pagination documented. Only "subdomain" is emitted.
// Date checked: 2026-09-29.
// Verification: fixture only; live test not run for this release (keyless request returned 401).
package sources

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

var leakixBaseURL = "https://leakix.net"

type leakix struct{}

func init() { Register(leakix{}) }

func (leakix) Info() Info {
	return Info{Name: "leakix", URL: "https://leakix.net", Auth: AuthRequired,
		CredFields: []string{"api_key"}, Default: true, RPS: 0.5, Burst: 1}
}

func (leakix) Enumerate(ctx context.Context, domain string, s *Session, emit func(string)) error {
	key, err := PickKey(s.Creds, "api_key")
	if err != nil {
		return err
	}
	resp, err := s.HTTP.Get(ctx, leakixBaseURL+"/api/subdomains/"+url.PathEscape(domain), map[string]string{"api-key": key, "Accept": "application/json"})
	if err != nil {
		return err
	}
	body := strings.TrimSpace(string(resp.Body))
	if body == "" || body == "null" {
		return nil
	}
	var rows []struct {
		Subdomain string `json:"subdomain"`
	}
	if err := json.Unmarshal(resp.Body, &rows); err != nil {
		return fmt.Errorf("%w: leakix invalid JSON: %v", ErrUnexpected, err)
	}
	for _, r := range rows {
		if r.Subdomain != "" {
			emit(r.Subdomain)
		}
	}
	return nil
}
