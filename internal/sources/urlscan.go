// Package sources: urlscan adapter.
//
// Provider:     urlscan.io (https://urlscan.io)
// Docs:         https://urlscan.io/docs/api/  (Search API)
// Endpoint:     GET {base}/api/v1/search/?q=domain:{domain}&size=100[&search_after=...]
//
//	ONLY the search index is queried. The scan submission endpoint
//	(/api/v1/scan/) is never used, so nothing is ever scanned.
//
// Auth:         optional header "API-Key: KEY". Unauthenticated searches are
//
//	allowed with small quotas (docs), so the source is
//	AuthOptional and on by default; a key raises the quotas.
//
// Pagination:   search_after = the last result's "sort" values joined by ","
//
//	while "has_more" is true; bounded by Session.MaxPages.
//
// Fields used:  results[].page.domain, results[].task.domain and the host part
//
//	of page.url / task.url (hostnames only; nothing is fetched).
//
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

var urlscanBaseURL = "https://urlscan.io"

type urlscan struct{}

func init() { Register(urlscan{}) }

func (urlscan) Info() Info {
	return Info{
		Name: "urlscan", URL: "https://urlscan.io",
		Auth: AuthOptional, CredFields: []string{"api_key"},
		Default: true, RPS: 0.5, Burst: 1,
	}
}

func urlscanHost(raw string) string {
	if raw == "" {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return u.Hostname()
}

func (urlscan) Enumerate(ctx context.Context, domain string, s *Session, emit func(string)) error {
	var h map[string]string
	if k := s.Creds["api_key"]; k != "" {
		h = map[string]string{"API-Key": k}
	}
	after := ""
	for page := 0; page < s.MaxPages; page++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		u := urlscanBaseURL + "/api/v1/search/?q=" + url.QueryEscape("domain:"+domain) + "&size=100"
		if after != "" {
			u += "&search_after=" + url.QueryEscape(after)
		}
		var r struct {
			Results []struct {
				Sort []json.RawMessage `json:"sort"`
				Page struct {
					Domain string `json:"domain"`
					URL    string `json:"url"`
				} `json:"page"`
				Task struct {
					Domain string `json:"domain"`
					URL    string `json:"url"`
				} `json:"task"`
			} `json:"results"`
			HasMore bool            `json:"has_more"`
			Status  int             `json:"status"`
			Message string          `json:"message"`
			Total   json.RawMessage `json:"total"`
		}
		if err := s.HTTP.GetJSON(ctx, u, h, &r); err != nil {
			return err
		}
		if r.Status >= 400 {
			switch {
			case r.Status == 401 || r.Status == 403:
				return fmt.Errorf("%w: urlscan: %s", ErrAuth, r.Message)
			case r.Status == 429:
				return fmt.Errorf("%w: urlscan: %s", ErrRateLimited, r.Message)
			}
			return fmt.Errorf("%w: urlscan: status %d: %s", ErrUnexpected, r.Status, r.Message)
		}
		if len(r.Results) == 0 {
			return nil
		}
		for _, x := range r.Results {
			for _, d := range []string{x.Page.Domain, x.Task.Domain, urlscanHost(x.Page.URL), urlscanHost(x.Task.URL)} {
				if d != "" {
					emit(d)
				}
			}
		}
		last := r.Results[len(r.Results)-1].Sort
		if !r.HasMore || len(last) == 0 {
			return nil
		}
		parts := make([]string, 0, len(last))
		for _, p := range last {
			var str string
			if json.Unmarshal(p, &str) == nil {
				parts = append(parts, str)
			} else {
				parts = append(parts, strings.TrimSpace(string(p)))
			}
		}
		next := strings.Join(parts, ",")
		if next == after {
			return nil
		}
		after = next
	}
	return nil
}
