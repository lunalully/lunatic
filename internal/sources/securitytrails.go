// Package sources: securitytrails adapter.
//
// Provider: SecurityTrails (now part of Recorded Future).
// Docs: https://docs.securitytrails.com/reference/list-subdomains-old-1
// Endpoint: GET https://api.securitytrails.com/v1/domain/<d>/subdomains (returns labels; the domain
// is appended). The DSL scroll endpoints are not used.
// Auth: "APIKEY: <api_key>" header. Paid plans; free API access unconfirmed.
// Checked: 2026-09-29. Verification: fixture only; live test not run for this release; run scripts/live-smoke.sh to check.
package sources

import (
	"context"
	"fmt"
	"strings"
)

var securitytrailsBaseURL = "https://api.securitytrails.com"

type securitytrails struct{}

func init() { Register(securitytrails{}) }

func (securitytrails) Info() Info {
	return Info{
		Name: "securitytrails", URL: "https://securitytrails.com",
		Auth: AuthRequired, CredFields: []string{"api_key"},
		Default: true, RPS: 1, Burst: 1,
	}
}

func (securitytrails) Enumerate(ctx context.Context, domain string, s *Session, emit func(string)) error {
	key, err := PickKey(s.Creds, "api_key")
	if err != nil {
		return err
	}
	var resp struct {
		Subdomains *[]string `json:"subdomains"`
		Message    string    `json:"message"`
		Error      string    `json:"error"`
	}
	if err := s.HTTP.GetJSON(ctx, securitytrailsBaseURL+"/v1/domain/"+domain+"/subdomains?children_only=false&include_inactive=true",
		map[string]string{"APIKEY": key, "Accept": "application/json"}, &resp); err != nil {
		return err
	}
	if resp.Subdomains == nil {
		if m := resp.Message + resp.Error; m != "" {
			return merklemapClassify(m)
		}
		return fmt.Errorf("%w: securitytrails response has no subdomains field", ErrUnexpected)
	}
	for _, l := range *resp.Subdomains {
		l = strings.Trim(strings.TrimSpace(l), ".")
		if l != "" {
			emit(l + "." + domain)
		}
	}
	return nil
}
