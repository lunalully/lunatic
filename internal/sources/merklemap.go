// Package sources: merklemap adapter.
//
// Provider: Merklemap (Certificate Transparency + DNS search).
// Docs: https://www.merklemap.com/documentation/search
// Endpoint: GET https://api.merklemap.com/v1/search?query=*.<domain>&page=<n> (page is zero-indexed).
// Auth: "Authorization: Bearer <api_key>". Paid plan (no free tier confirmed).
// Only stored data is queried; no scan is triggered.
// Checked: 2026-09-29. Verification: fixture only; live test not possible from build environment.
// Response fields (count, results[].hostname) follow the research file; they are not confirmed live.
//
// This file also holds two small helpers (merklemapClassify, merklemapFlatten)
// shared by the other group-3 adapters.
package sources

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

var merklemapBaseURL = "https://api.merklemap.com"

type merklemap struct{}

func init() { Register(merklemap{}) }

func (merklemap) Info() Info {
	return Info{
		Name: "merklemap", URL: "https://www.merklemap.com",
		Auth: AuthRequired, CredFields: []string{"api_key"},
		Default: true, RPS: 2, Burst: 1,
	}
}

// merklemapClassify maps a provider error message found inside a 200 body to a typed error.
func merklemapClassify(msg string) error {
	l := strings.ToLower(msg)
	switch {
	case strings.Contains(l, "api key") || strings.Contains(l, "apikey") || strings.Contains(l, "unauthor") ||
		strings.Contains(l, "forbidden") || strings.Contains(l, "invalid token") || strings.Contains(l, "authenticat") ||
		strings.Contains(l, "permission") || strings.Contains(l, "not allowed"):
		return fmt.Errorf("%w: %s", ErrAuth, msg)
	case strings.Contains(l, "rate limit") || strings.Contains(l, "too many") || strings.Contains(l, "quota") ||
		strings.Contains(l, "limit exceeded") || strings.Contains(l, "usage limit") || strings.Contains(l, "exceeded") || strings.Contains(l, "credit"):
		return fmt.Errorf("%w: %s", ErrRateLimited, msg)
	}
	return fmt.Errorf("%w: provider error: %s", ErrUnexpected, msg)
}

// merklemapFlatten returns the strings inside a JSON value that is a string or an array of strings.
func merklemapFlatten(v any) []string {
	switch t := v.(type) {
	case string:
		return []string{t}
	case []any:
		var out []string
		for _, e := range t {
			out = append(out, merklemapFlatten(e)...)
		}
		return out
	}
	return nil
}

func merklemapPages(s *Session) int {
	if s.MaxPages > 0 {
		return s.MaxPages
	}
	return 10
}

func (merklemap) Enumerate(ctx context.Context, domain string, s *Session, emit func(string)) error {
	key, err := PickKey(s.Creds, "api_key")
	if err != nil {
		return err
	}
	hdr := map[string]string{"Authorization": "Bearer " + key, "Accept": "application/json"}
	seen := 0
	for page := 0; page < merklemapPages(s); page++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		var resp struct {
			Count   *int64 `json:"count"`
			Error   string `json:"error"`
			Message string `json:"message"`
			Results *[]struct {
				Hostname string `json:"hostname"`
			} `json:"results"`
		}
		u := merklemapBaseURL + "/v1/search?query=" + url.QueryEscape("*."+domain) + "&page=" + strconv.Itoa(page)
		if err := s.HTTP.GetJSON(ctx, u, hdr, &resp); err != nil {
			return err
		}
		if resp.Error != "" {
			return merklemapClassify(resp.Error)
		}
		if resp.Results == nil {
			if resp.Message != "" {
				return merklemapClassify(resp.Message)
			}
			return fmt.Errorf("%w: merklemap response has no results field", ErrUnexpected)
		}
		if len(*resp.Results) == 0 {
			return nil
		}
		for _, r := range *resp.Results {
			emit(r.Hostname)
		}
		seen += len(*resp.Results)
		if resp.Count != nil && int64(seen) >= *resp.Count {
			return nil
		}
	}
	return nil
}
