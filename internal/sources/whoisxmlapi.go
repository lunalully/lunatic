// Package sources: whoisxmlapi adapter.
//
// Provider:     WhoisXML API Subdomains Lookup (https://subdomains.whoisxmlapi.com)
// Docs:         https://subdomains.whoisxmlapi.com/api/documentation/making-requests
// Endpoint:     GET {base}/api/v2?apiKey=KEY&domainName={domain}&outputFormat=JSON[&searchAfter=C]
//
//	(v2 as per the currently documented version; v1 is not used)
//
// Auth:         API key, query parameter "apiKey". Credits are consumed per
//
//	request; new accounts get a one-time free allotment.
//
// Pagination:   searchAfter cursor taken from "nextPageSearchAfter" (top level
//
//	or inside "result"; position not confirmed, both accepted);
//	bounded by Session.MaxPages.
//
// Errors:       error objects {"code":N,"messages":"..."} in bodies are mapped.
// Checked:      2026-09-29
// Verification: fixture only; live test not possible from build environment.
package sources

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
)

var whoisxmlapiBaseURL = "https://subdomains.whoisxmlapi.com"

type whoisxmlapi struct{}

func init() { Register(whoisxmlapi{}) }

func (whoisxmlapi) Info() Info {
	return Info{
		Name: "whoisxmlapi", URL: "https://subdomains.whoisxmlapi.com",
		Auth: AuthRequired, CredFields: []string{"api_key"},
		Default: true, RPS: 2, Burst: 1,
	}
}

func (whoisxmlapi) Enumerate(ctx context.Context, domain string, s *Session, emit func(string)) error {
	key, err := PickKey(s.Creds, "api_key")
	if err != nil {
		return err
	}
	after := ""
	for page := 0; page < s.MaxPages; page++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		u := whoisxmlapiBaseURL + "/api/v2?apiKey=" + url.QueryEscape(key) + "&domainName=" + url.QueryEscape(domain) + "&outputFormat=JSON"
		if after != "" {
			u += "&searchAfter=" + url.QueryEscape(after)
		}
		var r struct {
			Code     json.Number `json:"code"`
			Messages string      `json:"messages"`
			Result   struct {
				Records []struct {
					Domain string `json:"domain"`
				} `json:"records"`
				Next string `json:"nextPageSearchAfter"`
			} `json:"result"`
			Next string `json:"nextPageSearchAfter"`
		}
		if err := s.HTTP.GetJSON(ctx, u, nil, &r); err != nil {
			return err
		}
		if r.Code != "" && r.Code.String() != "200" {
			switch r.Code.String() {
			case "401", "403":
				return fmt.Errorf("%w: whoisxmlapi: %s", ErrAuth, r.Messages)
			case "429":
				return fmt.Errorf("%w: whoisxmlapi: %s", ErrRateLimited, r.Messages)
			}
			return fmt.Errorf("%w: whoisxmlapi: code %s: %s", ErrUnexpected, r.Code, r.Messages)
		}
		for _, rec := range r.Result.Records {
			emit(rec.Domain)
		}
		next := r.Next
		if next == "" {
			next = r.Result.Next
		}
		if next == "" || next == after || len(r.Result.Records) == 0 {
			return nil
		}
		after = next
	}
	return nil
}
