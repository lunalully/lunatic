// Package sources: threatbook adapter.
//
// Provider:     ThreatBook (https://threatbook.io / https://api.threatbook.cn)
// Docs:         https://docs.threatbook.io (subdomain endpoint NOT listed
//
//	there; endpoint and shape follow subfinder's client).
//
// Endpoint:     GET {base}/v3/domain/sub_domains?apikey=KEY&resource={domain}
// Auth:         API key, query parameter "apikey".
// Pagination:   none (single response).
// Errors:       200 bodies carry response_code (0 = ok) and verbose_msg.
// Checked:      2026-09-29
// Verification: fixture only; live test not possible from build environment.
package sources

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

var threatbookBaseURL = "https://api.threatbook.cn"

type threatbook struct{}

func init() { Register(threatbook{}) }

func (threatbook) Info() Info {
	return Info{
		Name: "threatbook", URL: "https://threatbook.io",
		Auth: AuthRequired, CredFields: []string{"api_key"},
		Default: false, RPS: 1, Burst: 1,
	}
}

func (threatbook) Enumerate(ctx context.Context, domain string, s *Session, emit func(string)) error {
	key, err := PickKey(s.Creds, "api_key")
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	var r struct {
		ResponseCode json.Number `json:"response_code"`
		VerboseMsg   string      `json:"verbose_msg"`
		Data         struct {
			SubDomains struct {
				Data []string `json:"data"`
			} `json:"sub_domains"`
		} `json:"data"`
	}
	u := threatbookBaseURL + "/v3/domain/sub_domains?apikey=" + url.QueryEscape(key) + "&resource=" + url.QueryEscape(domain)
	if err := s.HTTP.GetJSON(ctx, u, nil, &r); err != nil {
		return err
	}
	if r.ResponseCode == "" {
		return fmt.Errorf("%w: threatbook: response_code missing", ErrUnexpected)
	}
	if r.ResponseCode.String() != "0" {
		l := strings.ToLower(r.VerboseMsg)
		switch {
		case strings.Contains(l, "key") || strings.Contains(l, "auth") || strings.Contains(l, "permission"):
			return fmt.Errorf("%w: threatbook: %s", ErrAuth, r.VerboseMsg)
		case strings.Contains(l, "limit") || strings.Contains(l, "quota") || strings.Contains(l, "frequen"):
			return fmt.Errorf("%w: threatbook: %s", ErrRateLimited, r.VerboseMsg)
		}
		return fmt.Errorf("%w: threatbook: response_code %s: %s", ErrUnexpected, r.ResponseCode, r.VerboseMsg)
	}
	for _, h := range r.Data.SubDomains.Data {
		emit(h)
	}
	return nil
}
