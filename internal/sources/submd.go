// Package sources: submd adapter.
//
// Provider:     sub.md (https://sub.md)
// Docs:         site documentation (https://sub.md); anonymous free tier is
//
//	50 queries/day at 1 req/s; paid plans use a Bearer token.
//
// Endpoint:     GET {base}/v1/search?apex={domain}   (plain text, one host/line)
// Auth:         optional "Authorization: Bearer KEY".
// Pagination:   none; exactly ONE request per run to conserve the daily quota.
// Checked:      2026-09-29
// Verification: fixture only; live test not possible from build environment.
package sources

import (
	"context"
	"net/url"
	"strings"
)

var submdBaseURL = "https://api.sub.md"

type submd struct{}

func init() { Register(submd{}) }

func (submd) Info() Info {
	return Info{
		Name: "submd", URL: "https://sub.md",
		Auth: AuthOptional, CredFields: []string{"api_key"},
		Default: true, RPS: 1, Burst: 1,
	}
}

func (submd) Enumerate(ctx context.Context, domain string, s *Session, emit func(string)) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	var h map[string]string
	if k := s.Creds["api_key"]; k != "" {
		h = map[string]string{"Authorization": "Bearer " + k}
	}
	resp, err := s.HTTP.Get(ctx, submdBaseURL+"/v1/search?apex="+url.QueryEscape(domain), h)
	if err != nil {
		return err
	}
	for _, line := range strings.Split(string(resp.Body), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			emit(line)
		}
	}
	return nil
}
